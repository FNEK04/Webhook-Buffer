package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"webhook-buffer/config"
	"webhook-buffer/handlers"
	"webhook-buffer/middleware"
	"webhook-buffer/services"
	"webhook-buffer/worker"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	webhooksReceived = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webhook_buffer_received_total",
			Help: "Total number of webhooks received",
		},
		[]string{"endpoint"},
	)
	webhooksProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webhook_buffer_processed_total",
			Help: "Total number of webhooks processed",
		},
		[]string{"status"},
	)
	queueSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "webhook_buffer_queue_size",
			Help: "Current queue size",
		},
	)
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "webhook_buffer_request_duration_seconds",
			Help:    "Request duration in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 2, 5},
		},
		[]string{"method", "path", "status"},
	)
)

func init() {
	prometheus.MustRegister(webhooksReceived, webhooksProcessed, queueSize, requestDuration)
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	if err := cfg.Validate(); err != nil {
		slog.Error("configuration validation failed", "error", err)
		os.Exit(1)
	}

	redisService := services.NewRedisService(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisService.Ping(ctx); err != nil {
		slog.Error("failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	defer redisService.Close()

	pgService, err := services.NewPostgresService(cfg.Postgres.ConnectionString)
	if err != nil {
		slog.Error("failed to connect to PostgreSQL", "error", err)
		os.Exit(1)
	}
	defer pgService.Close()

	if err := pgService.InitSchema(); err != nil {
		slog.Error("failed to initialize database schema", "error", err)
		os.Exit(1)
	}

	client1C := services.NewClient1C(cfg.OneC.BaseURL, cfg.OneC.Username, cfg.OneC.Password, cfg.OneC.Timeout)

	webhookHandler := handlers.NewWebhookHandler(redisService, pgService)
	inventoryHandler := handlers.NewInventoryHandler(redisService, client1C, cfg.Worker.CacheTTL)

	w := worker.NewWorker(
		redisService,
		pgService,
		client1C,
		cfg.Worker.BatchSize,
		cfg.Worker.PollInterval,
		cfg.Worker.MaxRetries,
	)

	go w.Start()

	retryTicker := time.NewTicker(5 * time.Minute)
	defer retryTicker.Stop()
	go func() {
		for range retryTicker.C {
			w.RetryFailedWebhooks()
		}
	}()

	metricsTicker := time.NewTicker(15 * time.Second)
	defer metricsTicker.Stop()
	go func() {
		for range metricsTicker.C {
			mCtx, mCancel := context.WithTimeout(context.Background(), 5*time.Second)
			size, err := redisService.GetQueueSize(mCtx)
			if err == nil {
				queueSize.Set(float64(size))
			}
			mCancel()
		}
	}()

	if cfg.Server.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	rateLimiter := middleware.NewRateLimiter(cfg.Security.RateLimitRPS, cfg.Security.RateLimitBurst)

	router := gin.New()
	router.Use(middleware.RequestID())
	router.Use(middleware.StructuredLogger())
	router.Use(middleware.Recovery())
	router.Use(middleware.AuthAPIKey(cfg.Security.APIKey))
	router.Use(middleware.QueueSizeLimit(cfg.Worker.QueueMaxSize))

	router.GET("/health", func(c *gin.Context) {
		hCtx, hCancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer hCancel()

		redisOK := redisService.Ping(hCtx) == nil
		pgOK := pgService.Ping(hCtx) == nil
		c1cOK := client1C.IsHealthy()

		status := "healthy"
		code := http.StatusOK
		if !redisOK || !pgOK {
			status = "degraded"
			code = http.StatusServiceUnavailable
		}

		c.JSON(code, gin.H{
			"status":     status,
			"timestamp":  time.Now().Unix(),
			"redis":      redisOK,
			"postgres":   pgOK,
			"1c":         c1cOK,
			"request_id": c.GetString("request_id"),
		})
	})

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	webhookGroup := router.Group("/webhook")
	webhookGroup.Use(rateLimiter.Middleware())
	webhookGroup.Use(middleware.CheckQueueSize(func() (int64, error) {
		return redisService.GetQueueSize(context.Background())
	}))
	{
		webhookGroup.POST("", func(c *gin.Context) {
			webhooksReceived.WithLabelValues("single").Inc()
			webhookHandler.HandleWebhook(c)
		})
		webhookGroup.POST("/batch", func(c *gin.Context) {
			webhooksReceived.WithLabelValues("batch").Inc()
			webhookHandler.HandleBatchWebhook(c)
		})
		webhookGroup.GET("/status", webhookHandler.GetQueueStatus)
	}

	router.GET("/inventory/:sku", inventoryHandler.GetInventory)
	router.DELETE("/inventory/:sku/cache", inventoryHandler.InvalidateCache)

	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		slog.Info("server starting", "port", cfg.Server.Port, "env", cfg.Server.AppEnv)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")

	w.Stop()

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	slog.Info("server exited")
}
