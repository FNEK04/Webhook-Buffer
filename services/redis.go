package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"webhook-buffer/models"

	"github.com/redis/go-redis/v9"
)

type RedisService struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisService(addr, password string, db int) *RedisService {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	return &RedisService{
		client: rdb,
		ctx:    context.Background(),
	}
}

func (r *RedisService) Ping() error {
	return r.client.Ping(r.ctx).Err()
}

func (r *RedisService) Close() error {
	return r.client.Close()
}

// Enqueue adds webhook to the queue
func (r *RedisService) Enqueue(webhook models.Webhook) error {
	queueItem := models.QueueItem{
		Webhook:  webhook,
		Attempts: 0,
		Received: time.Now(),
	}

	data, err := json.Marshal(queueItem)
	if err != nil {
		return fmt.Errorf("failed to marshal queue item: %w", err)
	}

	return r.client.RPush(r.ctx, "webhook:queue", data).Err()
}

// Dequeue retrieves webhook from the queue with timeout
func (r *RedisService) Dequeue(timeout time.Duration) (*models.QueueItem, error) {
	result, err := r.client.BLPop(r.ctx, timeout, "webhook:queue").Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // No items in queue
		}
		return nil, fmt.Errorf("failed to dequeue: %w", err)
	}

	var queueItem models.QueueItem
	if err := json.Unmarshal([]byte(result[1]), &queueItem); err != nil {
		return nil, fmt.Errorf("failed to unmarshal queue item: %w", err)
	}

	return &queueItem, nil
}

// GetQueueSize returns current queue size
func (r *RedisService) GetQueueSize() (int64, error) {
	return r.client.LLen(r.ctx, "webhook:queue").Result()
}

// CacheInventory stores inventory data in Redis cache
func (r *RedisService) CacheInventory(sku string, quantity int, price float64, ttl time.Duration) error {
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
	return r.client.Set(r.ctx, key, data, ttl).Err()
}

// GetInventory retrieves inventory data from cache
func (r *RedisService) GetInventory(sku string) (*models.InventoryCache, error) {
	key := fmt.Sprintf("inventory:%s", sku)
	data, err := r.client.Get(r.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss
		}
		return nil, fmt.Errorf("failed to get inventory cache: %w", err)
	}

	var cacheItem models.InventoryCache
	if err := json.Unmarshal([]byte(data), &cacheItem); err != nil {
		return nil, fmt.Errorf("failed to unmarshal inventory cache: %w", err)
	}

	return &cacheItem, nil
}

// InvalidateCache removes inventory from cache
func (r *RedisService) InvalidateCache(sku string) error {
	key := fmt.Sprintf("inventory:%s", sku)
	return r.client.Del(r.ctx, key).Err()
}
