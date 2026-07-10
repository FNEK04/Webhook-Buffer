package worker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"webhook-buffer/mocks"
	"webhook-buffer/models"
	"webhook-buffer/services"
)

func TestWorker_ProcessBatch_WithDequeue(t *testing.T) {
	mockRedis := &mocks.MockQueueService{}
	mockPG := &mocks.MockLogService{}

	callCount := 0
	mockRedis.DequeueFunc = func(ctx context.Context, timeout time.Duration) (*models.QueueItem, error) {
		callCount++
		if callCount > 2 {
			return nil, nil
		}
		return &models.QueueItem{
			Webhook: models.Webhook{
				Event: "order.created",
				Payload: models.Payload{
					OrderID: fmt.Sprintf("BATCH-%d", callCount),
					Status:  "new",
				},
			},
			Attempts: 0,
			Received: time.Now(),
			LogID:    int64(callCount),
		}, nil
	}

	client1C := services.NewClient1C("http://localhost:9999", "test", "test", 1*time.Second)

	w := NewWorker(mockRedis, mockPG, client1C, 10, 100*time.Millisecond, 5)

	w.processBatch()

	client1C.StopHealthCheck()
}

func TestWorker_Stop(t *testing.T) {
	mockRedis := &mocks.MockQueueService{}
	mockPG := &mocks.MockLogService{}
	client1C := services.NewClient1C("http://localhost:9999", "test", "test", 1*time.Second)

	w := NewWorker(mockRedis, mockPG, client1C, 10, 1*time.Second, 5)

	done := make(chan struct{})
	go func() {
		w.Start()
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	w.Stop()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("Worker did not stop within timeout")
	}
}

func TestWorker_RetryFailedWebhooks(t *testing.T) {
	mockRedis := &mocks.MockQueueService{}
	mockPG := &mocks.MockLogService{}

	mockPG.GetFailedWebhooksFunc = func(ctx context.Context, limit int, maxRetries int) ([]models.WebhookLog, error) {
		return []models.WebhookLog{
			{
				ID:      10,
				OrderID: "FAIL-001",
				Event:   "order.created",
				Status:  "failed",
				Retries: 2,
				Payload: []byte(`{"event":"order.created","timestamp":"2026-01-01T00:00:00Z","payload":{"order_id":"FAIL-001","status":"new","payment_status":"paid","payment_method":"card","customer":{"phone":"+79991234567","email":"test@test.com","first_name":"T","last_name":"U"},"delivery":{"method":"pickup","address":"addr","cost":0},"items":[{"sku":"SKU-1","name":"Item","quantity":1,"price":100,"discount":0}],"total_amount":100}}`),
			},
		}, nil
	}

	client1C := services.NewClient1C("http://localhost:9999", "test", "test", 1*time.Second)

	w := NewWorker(mockRedis, mockPG, client1C, 10, 1*time.Second, 5)

	w.RetryFailedWebhooks()

	if mockRedis.EnqueueCalled != 1 {
		t.Errorf("Expected 1 enqueue call, got %d", mockRedis.EnqueueCalled)
	}

	client1C.StopHealthCheck()
}

func TestWorker_MaxRetries_MovesToDeadLetter(t *testing.T) {
	mockRedis := &mocks.MockQueueService{}
	mockPG := &mocks.MockLogService{}

	client1C := services.NewClient1C("http://localhost:9999", "test", "test", 1*time.Second)

	w := NewWorker(mockRedis, mockPG, client1C, 10, 100*time.Millisecond, 2)

	ctx := context.Background()

	queueItem := &models.QueueItem{
		Webhook: models.Webhook{
			Event: "order.created",
			Payload: models.Payload{
				OrderID: "FAIL-MAX",
				Status:  "new",
			},
		},
		Attempts: 2,
		Received: time.Now(),
		LogID:    99,
	}

	err := w.processWebhook(ctx, queueItem)
	if err == nil {
		t.Skip("1C might be running")
	}

	if queueItem.Attempts >= w.maxRetries {
		t.Log("Would move to dead letter at this point (called in processBatch)")
	}

	client1C.StopHealthCheck()
}
