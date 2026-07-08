package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"webhook-buffer/config"
	"webhook-buffer/handlers"
	"webhook-buffer/services"
	"webhook-buffer/worker"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	// Initialize services
	redisService := services.NewRedisService(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err := redisService.Ping(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisService.Close()

	pgService, err := services.NewPostgresService(cfg.Postgres.ConnectionString)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer pgService.Close()

	if err := pgService.InitSchema(); err != nil {
		log.Fatalf("Failed to initialize database schema: %v", err)
	}

	client1C := services.NewClient1C(cfg.OneC.BaseURL, cfg.OneC.Username, cfg.OneC.Password, cfg.OneC.Timeout)

	// Initialize handlers
	webhookHandler := handlers.NewWebhookHandler(redisService, pgService)
	inventoryHandler := handlers.NewInventoryHandler(redisService, client1C, cfg.Worker.CacheTTL)

	// Initialize worker
	w := worker.NewWorker(
		redisService,
		pgService,
		client1C,
		cfg.Worker.BatchSize,
		cfg.Worker.PollInterval,
		cfg.Worker.MaxRetries,
	)

	// Start worker in background
	go w.Start()

	// Start retry failed webhooks ticker
	retryTicker := time.NewTicker(5 * time.Minute)
	defer retryTicker.Stop()
	go func() {
		for range retryTicker.C {
			w.RetryFailedWebhooks()
		}
	}()

	// Set up Gin router
	if cfg.Server.Port == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"timestamp": time.Now().Unix(),
		})
	})

	// Webhook endpoints
	router.POST("/webhook", webhookHandler.HandleWebhook)
	router.POST("/webhook/batch", webhookHandler.HandleBatchWebhook)
	router.GET("/webhook/status", webhookHandler.GetQueueStatus)

	// Inventory endpoints
	router.GET("/inventory/:sku", inventoryHandler.GetInventory)
	router.DELETE("/inventory/:sku/cache", inventoryHandler.InvalidateCache)

	// Start HTTP server
	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Graceful shutdown
	go func() {
		log.Printf("Server starting on port %s", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Stop worker
	w.Stop()

	// Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
