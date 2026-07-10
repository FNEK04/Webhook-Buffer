package services

import (
	"context"
	"testing"
	"time"

	"webhook-buffer/models"
)

func TestPostgresService_LogWebhookAndUpdateStatus(t *testing.T) {
	t.Skip("Skipping integration test - requires PostgreSQL instance")

	connString := "postgres://webhook:webhook123@localhost:5432/webhook_buffer?sslmode=disable"
	pgService, err := NewPostgresService(connString)
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer pgService.Close()

	if err := pgService.InitSchema(); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	ctx := context.Background()

	webhook := models.Webhook{
		Event:     "order.created",
		Timestamp: time.Now(),
		Payload: models.Payload{
			OrderID: "WB-1-TEST-001",
			Status:  "new",
		},
	}

	log := models.WebhookLog{
		OrderID:    webhook.Payload.OrderID,
		Event:      webhook.Event,
		Status:     "received",
		Retries:    0,
		ReceivedAt: time.Now(),
	}

	id, err := pgService.LogWebhook(ctx, log, webhook)
	if err != nil {
		t.Fatalf("Failed to log webhook: %v", err)
	}

	if id == 0 {
		t.Fatal("Expected non-zero ID from LogWebhook")
	}

	errorMsg := "test error"
	err = pgService.UpdateWebhookStatus(ctx, id, "processed", 1, &errorMsg)
	if err != nil {
		t.Fatalf("Failed to update webhook status by ID: %v", err)
	}

	stats, err := pgService.GetStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats["processed"] != 1 {
		t.Errorf("Expected 1 processed webhook, got %d", stats["processed"])
	}
}
