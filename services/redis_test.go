package services

import (
	"context"
	"testing"
	"time"

	"webhook-buffer/models"
)

func TestRedisService_Enqueue_Dequeue(t *testing.T) {
	t.Skip("Skipping integration test - requires Redis instance")

	redisService := NewRedisService("localhost:6379", "", 0)
	defer redisService.Close()

	ctx := context.Background()

	if err := redisService.Ping(ctx); err != nil {
		t.Fatalf("Failed to ping Redis: %v", err)
	}

	webhook := models.Webhook{
		Event:     "order.created",
		Timestamp: time.Now(),
		Payload: models.Payload{
			OrderID:       "TEST-001",
			Status:        "new",
			PaymentStatus: "paid",
			PaymentMethod: "card",
			Customer: models.Customer{
				Phone:     "+79991234567",
				Email:     "test@test.com",
				FirstName: "Test",
				LastName:  "User",
			},
			Delivery: models.Delivery{
				Method:  "pickup",
				Address: "Test address",
				Cost:    0,
			},
			Items: []models.Item{
				{
					SKU:      "SKU-001",
					Name:     "Test Item",
					Quantity: 1,
					Price:    100,
					Discount: 0,
				},
			},
			TotalAmount: 100,
		},
	}

	err := redisService.Enqueue(ctx, webhook)
	if err != nil {
		t.Fatalf("Failed to enqueue: %v", err)
	}

	item, err := redisService.Dequeue(ctx, 1*time.Second)
	if err != nil {
		t.Fatalf("Failed to dequeue: %v", err)
	}

	if item == nil {
		t.Fatal("Expected item, got nil")
	}

	if item.Webhook.Payload.OrderID != "TEST-001" {
		t.Errorf("Expected order ID TEST-001, got %s", item.Webhook.Payload.OrderID)
	}
}

func TestRedisService_CacheOperations(t *testing.T) {
	t.Skip("Skipping integration test - requires Redis instance")

	redisService := NewRedisService("localhost:6379", "", 0)
	defer redisService.Close()

	ctx := context.Background()

	sku := "SKU-TEST-001"
	quantity := 100
	price := 9.99
	ttl := 5 * time.Minute

	err := redisService.CacheInventory(ctx, sku, quantity, price, ttl)
	if err != nil {
		t.Fatalf("Failed to cache inventory: %v", err)
	}

	cached, err := redisService.GetInventory(ctx, sku)
	if err != nil {
		t.Fatalf("Failed to get cached inventory: %v", err)
	}

	if cached == nil {
		t.Fatal("Expected cached item, got nil")
	}

	if cached.SKU != sku {
		t.Errorf("Expected SKU %s, got %s", sku, cached.SKU)
	}

	if cached.Quantity != quantity {
		t.Errorf("Expected quantity %d, got %d", quantity, cached.Quantity)
	}

	if cached.Price != price {
		t.Errorf("Expected price %f, got %f", price, cached.Price)
	}

	err = redisService.InvalidateCache(ctx, sku)
	if err != nil {
		t.Fatalf("Failed to invalidate cache: %v", err)
	}

	cached, err = redisService.GetInventory(ctx, sku)
	if err != nil {
		t.Fatalf("Failed to get inventory after invalidation: %v", err)
	}

	if cached != nil {
		t.Error("Expected nil after cache invalidation, got item")
	}
}
