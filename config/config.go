package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server   ServerConfig
	Redis    RedisConfig
	Postgres PostgresConfig
	OneC     OneCConfig
	Worker   WorkerConfig
	Security SecurityConfig
}

type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	AppEnv       string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type PostgresConfig struct {
	ConnectionString string
}

type OneCConfig struct {
	BaseURL  string
	Username string
	Password string
	Timeout  time.Duration
}

type WorkerConfig struct {
	BatchSize     int
	PollInterval  time.Duration
	MaxRetries    int
	CacheTTL      time.Duration
	QueueMaxSize  int64
}

type SecurityConfig struct {
	APIKey         string
	RateLimitRPS   float64
	RateLimitBurst int
}

func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", "8080"),
			ReadTimeout:  getDurationEnv("SERVER_READ_TIMEOUT", "10s"),
			WriteTimeout: getDurationEnv("SERVER_WRITE_TIMEOUT", "10s"),
			AppEnv:       getEnv("APP_ENV", "development"),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getIntEnv("REDIS_DB", 0),
		},
		Postgres: PostgresConfig{
			ConnectionString: getEnv("POSTGRES_CONN_STRING", "postgres://user:password@localhost/webhook_buffer?sslmode=disable"),
		},
		OneC: OneCConfig{
			BaseURL:  getEnv("ONEC_BASE_URL", "http://localhost:8081"),
			Username: getEnv("ONEC_USERNAME", "admin"),
			Password: getEnv("ONEC_PASSWORD", "password"),
			Timeout:  getDurationEnv("ONEC_TIMEOUT", "30s"),
		},
		Worker: WorkerConfig{
			BatchSize:    getIntEnv("WORKER_BATCH_SIZE", 10),
			PollInterval: getDurationEnv("WORKER_POLL_INTERVAL", "1s"),
			MaxRetries:   getIntEnv("WORKER_MAX_RETRIES", 5),
			CacheTTL:     getDurationEnv("WORKER_CACHE_TTL", "5m"),
			QueueMaxSize: getInt64Env("WORKER_QUEUE_MAX_SIZE", 100000),
		},
		Security: SecurityConfig{
			APIKey:         getEnv("API_KEY", ""),
			RateLimitRPS:   getFloatEnv("RATE_LIMIT_RPS", 1000),
			RateLimitBurst: getIntEnv("RATE_LIMIT_BURST", 2000),
		},
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getInt64Env(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getFloatEnv(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if fVal, err := strconv.ParseFloat(value, 64); err == nil {
			return fVal
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue string) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	if duration, err := time.ParseDuration(defaultValue); err == nil {
		return duration
	}
	return 0
}

func (c *Config) Validate() error {
	if c.Server.Port == "" {
		return fmt.Errorf("server port is required")
	}
	if c.Redis.Addr == "" {
		return fmt.Errorf("redis address is required")
	}
	if c.Postgres.ConnectionString == "" {
		return fmt.Errorf("postgres connection string is required")
	}
	if c.OneC.BaseURL == "" {
		return fmt.Errorf("1C base URL is required")
	}
	return nil
}
