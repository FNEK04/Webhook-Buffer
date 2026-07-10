package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"webhook-buffer/models"

	"github.com/redis/go-redis/v9"
)

type RedisService struct {
	client *redis.Client
}

func NewRedisService(addr, password string, db int) *RedisService {
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 2,
	})

	return &RedisService{
		client: rdb,
	}
}

func (r *RedisService) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *RedisService) Close() error {
	return r.client.Close()
}

func (r *RedisService) Enqueue(ctx context.Context, webhook models.Webhook, logID ...int64) error {
	queueItem := models.QueueItem{
		Webhook:  webhook,
		Attempts: 0,
		Received: time.Now(),
	}

	if len(logID) > 0 {
		queueItem.LogID = logID[0]
	}

	data, err := json.Marshal(queueItem)
	if err != nil {
		return fmt.Errorf("failed to marshal queue item: %w", err)
	}

	return r.client.RPush(ctx, "webhook:queue", data).Err()
}

func (r *RedisService) EnqueueBatch(ctx context.Context, items []models.QueueItem) error {
	if len(items) == 0 {
		return nil
	}

	pipe := r.client.Pipeline()
	for _, item := range items {
		data, err := json.Marshal(item)
		if err != nil {
			return fmt.Errorf("failed to marshal queue item: %w", err)
		}
		pipe.RPush(ctx, "webhook:queue", data)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to enqueue batch: %w", err)
	}

	slog.Debug("batch enqueued to redis", "count", len(items))
	return nil
}

func (r *RedisService) Dequeue(ctx context.Context, timeout time.Duration) (*models.QueueItem, error) {
	result, err := r.client.BLPop(ctx, timeout, "webhook:queue").Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to dequeue: %w", err)
	}

	var queueItem models.QueueItem
	if err := json.Unmarshal([]byte(result[1]), &queueItem); err != nil {
		return nil, fmt.Errorf("failed to unmarshal queue item: %w", err)
	}

	return &queueItem, nil
}

func (r *RedisService) GetQueueSize(ctx context.Context) (int64, error) {
	return r.client.LLen(ctx, "webhook:queue").Result()
}

func (r *RedisService) CacheInventory(ctx context.Context, sku string, quantity int, price float64, ttl time.Duration) error {
	cacheItem := models.InventoryCache{
		SKU:      sku,
		Quantity: quantity,
		Price:    price,
		CachedAt: time.Now(),
	}

	data, err := json.Marshal(cacheItem)
	if err != nil {
		return fmt.Errorf("failed to marshal inventory cache: %w", err)
	}

	key := fmt.Sprintf("inventory:%s", sku)
	return r.client.Set(ctx, key, data, ttl).Err()
}

func (r *RedisService) GetInventory(ctx context.Context, sku string) (*models.InventoryCache, error) {
	key := fmt.Sprintf("inventory:%s", sku)
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get inventory cache: %w", err)
	}

	var cacheItem models.InventoryCache
	if err := json.Unmarshal([]byte(data), &cacheItem); err != nil {
		return nil, fmt.Errorf("failed to unmarshal inventory cache: %w", err)
	}

	return &cacheItem, nil
}

func (r *RedisService) InvalidateCache(ctx context.Context, sku string) error {
	key := fmt.Sprintf("inventory:%s", sku)
	return r.client.Del(ctx, key).Err()
}
