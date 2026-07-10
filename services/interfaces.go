package services

import (
	"context"
	"time"

	"webhook-buffer/models"
)

type QueueService interface {
	Enqueue(ctx context.Context, webhook models.Webhook, logID ...int64) error
	Dequeue(ctx context.Context, timeout time.Duration) (*models.QueueItem, error)
	GetQueueSize(ctx context.Context) (int64, error)
	EnqueueBatch(ctx context.Context, items []models.QueueItem) error
	Ping(ctx context.Context) error
	Close() error
}

type LogService interface {
	LogWebhook(ctx context.Context, log models.WebhookLog, webhook models.Webhook) (int64, error)
	LogWebhookBatch(ctx context.Context, logs []models.WebhookLog, webhooks []models.Webhook) ([]int64, error)
	UpdateWebhookStatus(ctx context.Context, id int64, status string, retries int, errorMsg *string) error
	UpdateWebhookStatusBatch(ctx context.Context, ids []int64, status string, retries int) error
	GetStats(ctx context.Context) (map[string]int64, error)
	GetFailedWebhooks(ctx context.Context, limit int, maxRetries int) ([]models.WebhookLog, error)
	MoveToDeadLetter(ctx context.Context, id int64) error
	InitSchema() error
	Ping(ctx context.Context) error
	Close() error
}

type InventoryCacheService interface {
	CacheInventory(ctx context.Context, sku string, quantity int, price float64, ttl time.Duration) error
	GetInventory(ctx context.Context, sku string) (*models.InventoryCache, error)
	InvalidateCache(ctx context.Context, sku string) error
}

type OrderService interface {
	SendOrder(ctx context.Context, webhook models.Webhook) error
	GetInventory(ctx context.Context, sku string) (*models.InventoryCache, error)
	HealthCheck(ctx context.Context) error
}
