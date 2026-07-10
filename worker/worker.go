package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"webhook-buffer/models"
	"webhook-buffer/services"
)

type Worker struct {
	redisService services.QueueService
	pgService    services.LogService
	client1C     *services.Client1C
	batchSize    int
	pollInterval time.Duration
	maxRetries   int
	stopChan     chan struct{}
	processing   atomic.Bool
}

func NewWorker(
	redisService services.QueueService,
	pgService services.LogService,
	client1C *services.Client1C,
	batchSize int,
	pollInterval time.Duration,
	maxRetries int,
) *Worker {
	return &Worker{
		redisService: redisService,
		pgService:    pgService,
		client1C:     client1C,
		batchSize:    batchSize,
		pollInterval: pollInterval,
		maxRetries:   maxRetries,
		stopChan:     make(chan struct{}),
	}
}

func (w *Worker) Start() {
	slog.Info("worker started", "batch_size", w.batchSize, "poll_interval", w.pollInterval)

	for {
		select {
		case <-w.stopChan:
			slog.Info("worker stopping")
			return
		default:
			w.processBatch()
		}
	}
}

func (w *Worker) Stop() {
	slog.Info("worker stopping...")
	close(w.stopChan)
	w.client1C.StopHealthCheck()
}

func (w *Worker) processBatch() {
	w.processing.Store(true)
	defer w.processing.Store(false)

	processed := 0
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	for i := 0; i < w.batchSize; i++ {
		select {
		case <-w.stopChan:
			cancel()
			return
		default:
		}

		queueItem, err := w.redisService.Dequeue(ctx, w.pollInterval)
		if err != nil {
			slog.Error("error dequeuing", "error", err)
			continue
		}

		if queueItem == nil {
			break
		}

		if err := w.processWebhook(ctx, queueItem); err != nil {
			slog.Error("error processing webhook",
				"order_id", queueItem.Webhook.Payload.OrderID,
				"error", err,
			)

			if queueItem.Attempts < w.maxRetries {
				queueItem.Attempts++
				if err := w.redisService.Enqueue(ctx, queueItem.Webhook, queueItem.LogID); err != nil {
					slog.Error("failed to re-queue webhook",
						"order_id", queueItem.Webhook.Payload.OrderID,
						"error", err,
					)
				}
				errorMsg := err.Error()
				_ = w.pgService.UpdateWebhookStatus(ctx, queueItem.LogID, "failed", queueItem.Attempts, &errorMsg)
			} else {
				slog.Warn("max retries exceeded, moving to dead letter",
					"order_id", queueItem.Webhook.Payload.OrderID,
					"attempts", queueItem.Attempts,
				)
				errorMsg := fmt.Sprintf("max retries (%d) exceeded: %v", w.maxRetries, err)
				_ = w.pgService.UpdateWebhookStatus(ctx, queueItem.LogID, "dead_letter", queueItem.Attempts, &errorMsg)
				_ = w.pgService.MoveToDeadLetter(ctx, queueItem.LogID)
			}
		} else {
			processed++
			_ = w.pgService.UpdateWebhookStatus(ctx, queueItem.LogID, "processed", queueItem.Attempts, nil)
		}
	}

	cancel()

	if processed > 0 {
		slog.Info("batch processed", "count", processed)
	}
}

func (w *Worker) processWebhook(ctx context.Context, queueItem *models.QueueItem) error {
	if !w.client1C.IsHealthy() {
		return fmt.Errorf("1C is not healthy, skipping")
	}

	if err := w.client1C.SendOrder(ctx, queueItem.Webhook); err != nil {
		return fmt.Errorf("failed to send order to 1C: %w", err)
	}

	slog.Info("successfully sent order to 1C", "order_id", queueItem.Webhook.Payload.OrderID)
	return nil
}

func (w *Worker) RetryFailedWebhooks() {
	slog.Info("checking for failed webhooks to retry...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	failedLogs, err := w.pgService.GetFailedWebhooks(ctx, 50, w.maxRetries)
	if err != nil {
		slog.Error("error getting failed webhooks", "error", err)
		return
	}

	if len(failedLogs) == 0 {
		slog.Info("no failed webhooks to retry")
		return
	}

	slog.Info("found failed webhooks to retry", "count", len(failedLogs))

	retriedCount := 0
	for _, logEntry := range failedLogs {
		var webhook models.Webhook
		if err := json.Unmarshal(logEntry.Payload, &webhook); err != nil {
			slog.Error("failed to unmarshal webhook payload",
				"order_id", logEntry.OrderID,
				"error", err,
			)
			continue
		}

		if err := w.redisService.Enqueue(ctx, webhook, logEntry.ID); err != nil {
			slog.Error("failed to re-queue webhook",
				"order_id", logEntry.OrderID,
				"error", err,
			)
			continue
		}

		if err := w.pgService.UpdateWebhookStatus(ctx, logEntry.ID, "queued", logEntry.Retries, nil); err != nil {
			slog.Error("failed to update webhook status",
				"order_id", logEntry.OrderID,
				"error", err,
			)
		}

		retriedCount++
	}

	if retriedCount > 0 {
		slog.Info("retried failed webhooks", "count", retriedCount)
	}
}
