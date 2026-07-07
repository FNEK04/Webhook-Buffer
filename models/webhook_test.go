package models

import (
	"testing"
	"time"
)

func TestWebhook_Validation(t *testing.T) {
	webhook := Webhook{
		Event:     "order.created",
		Timestamp: time.Now(),
		Payload: Payload{
			OrderID:       "TEST-001",
			Status:        "new",
			PaymentStatus: "paid",
			PaymentMethod: "card",
			Customer: Customer{
				Phone:     "+79991234567",
				Email:     "test@test.com",
				FirstName: "Test",
				LastName:  "User",
			},
			Delivery: Delivery{
				Method:  "pickup",
				Address: "Test address",
				Cost:    0,
			},
			Items: []Item{
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

	if webhook.Event == "" {
		t.Error("Event should not be empty")
	}

	if webhook.Payload.OrderID == "" {
		t.Error("OrderID should not be empty")
	}

	if len(webhook.Payload.Items) == 0 {
		t.Error("Items should not be empty")
	}
}

func TestQueueItem_Creation(t *testing.T) {
	webhook := Webhook{
		Event:     "order.created",
		Timestamp: time.Now(),
		Payload: Payload{
			OrderID: "TEST-002",
			Status:  "new",
		},
	}

	queueItem := QueueItem{
		Webhook:  webhook,
		Attempts: 0,
		Received: time.Now(),
	}

	if queueItem.Attempts != 0 {
		t.Errorf("Expected 0 attempts, got %d", queueItem.Attempts)
	}

	if queueItem.Webhook.Payload.OrderID != "TEST-002" {
		t.Errorf("Expected order ID TEST-002, got %s", queueItem.Webhook.Payload.OrderID)
	}
}

func TestInventoryCache_Creation(t *testing.T) {
	cache := InventoryCache{
		SKU:      "SKU-001",
		Quantity: 100,
		Price:    9.99,
		CachedAt: time.Now(),
	}

	if cache.SKU != "SKU-001" {
		t.Errorf("Expected SKU SKU-001, got %s", cache.SKU)
	}

	if cache.Quantity != 100 {
		t.Errorf("Expected quantity 100, got %d", cache.Quantity)
	}

	if cache.Price != 9.99 {
		t.Errorf("Expected price 9.99, got %f", cache.Price)
	}
}
