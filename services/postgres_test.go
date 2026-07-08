package services

import (
	"testing"
	"time"

	"webhook-buffer/models"
)

// TestPostgresService_LogWebhookAndUpdateStatus tests the fix for WB-1
// This is an integration test that requires a running PostgreSQL instance
func TestPostgresService_LogWebhookAndUpdateStatus(t *testing.T) {
	t.Skip("Skipping integration test - requires PostgreSQL instance")
	
	connString := "postgres://webhook:webhook123@localhost:5432/webhook_buffer?sslmode=disable"
	pgService, err := NewPostgresService(connString)
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer pgService.Close()
	
	// Initialize schema
	if err := pgService.InitSchema(); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}
	
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
	
	// Log webhook and get ID
	id, err := pgService.LogWebhook(log, webhook)
	if err != nil {
		t.Fatalf("Failed to log webhook: %v", err)
	}
	
	if id == 0 {
		t.Fatal("Expected non-zero ID from LogWebhook")
	}
	
	// Update status by ID
	errorMsg := "test error"
	err = pgService.UpdateWebhookStatus(id, "processed", 1, &errorMsg)
	if err != nil {
		t.Fatalf("Failed to update webhook status by ID: %v", err)
	}
	
	// Verify update by getting stats (simple check)
	stats, err := pgService.GetStats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}
	
	if stats["processed"] != 1 {
		t.Errorf("Expected 1 processed webhook, got %d", stats["processed"])
	}
}
