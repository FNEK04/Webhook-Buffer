package worker

import (
	"fmt"
	"log"
	"time"

	"webhook-buffer/models"
	"webhook-buffer/services"
)

type Worker struct {
	redisService *services.RedisService
	pgService    *services.PostgresService
	client1C     *services.Client1C
	batchSize    int
	pollInterval time.Duration
	maxRetries   int
	stopChan     chan struct{}
}

func NewWorker(
	redisService *services.RedisService,
	pgService *services.PostgresService,
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

// Start begins processing webhooks from the queue
func (w *Worker) Start() {
	log.Println("Worker started, processing webhooks...")

	for {
		select {
		case <-w.stopChan:
			log.Println("Worker stopping...")
			return
		default:
			w.processBatch()
		}
	}
}

// Stop gracefully stops the worker
func (w *Worker) Stop() {
	close(w.stopChan)
}

// processBatch processes a batch of webhooks from the queue
func (w *Worker) processBatch() {
	processed := 0

	for i := 0; i < w.batchSize; i++ {
		queueItem, err := w.redisService.Dequeue(w.pollInterval)
		if err != nil {
			log.Printf("Error dequeuing: %v", err)
			continue
		}

		if queueItem == nil {
			// No items in queue, wait before next poll
			break
		}

		if err := w.processWebhook(queueItem); err != nil {
			log.Printf("Error processing webhook %s: %v", queueItem.Webhook.Payload.OrderID, err)
			
			// Re-queue if retries not exhausted
			if queueItem.Attempts < w.maxRetries {
				queueItem.Attempts++
				if err := w.redisService.Enqueue(queueItem.Webhook); err != nil {
					log.Printf("Failed to re-queue webhook %s: %v", queueItem.Webhook.Payload.OrderID, err)
				}
				
				errorMsg := err.Error()
				w.pgService.UpdateWebhookStatus(queueItem.Webhook.Payload.OrderID, "failed", queueItem.Attempts, &errorMsg)
			} else {
				errorMsg := fmt.Sprintf("Max retries (%d) exceeded: %v", w.maxRetries, err)
				w.pgService.UpdateWebhookStatus(queueItem.Webhook.Payload.OrderID, "failed", queueItem.Attempts, &errorMsg)
			}
		} else {
			processed++
			w.pgService.UpdateWebhookStatus(queueItem.Webhook.Payload.OrderID, "processed", queueItem.Attempts, nil)
		}
	}

	if processed > 0 {
		log.Printf("Processed %d webhooks successfully", processed)
	}
}

// processWebhook sends a single webhook to 1C
func (w *Worker) processWebhook(queueItem *models.QueueItem) error {
	// Check 1C health before sending
	if err := w.client1C.HealthCheck(); err != nil {
		return fmt.Errorf("1C health check failed: %w", err)
	}

	// Send to 1C
	if err := w.client1C.SendOrder(queueItem.Webhook); err != nil {
		return fmt.Errorf("failed to send order to 1C: %w", err)
	}

	log.Printf("Successfully sent order %s to 1C", queueItem.Webhook.Payload.OrderID)
	return nil
}

// RetryFailedWebhooks processes webhooks that previously failed
func (w *Worker) RetryFailedWebhooks() {
	log.Println("Checking for failed webhooks to retry...")

	failedLogs, err := w.pgService.GetFailedWebhooks(50)
	if err != nil {
		log.Printf("Error getting failed webhooks: %v", err)
		return
	}

	if len(failedLogs) == 0 {
		log.Println("No failed webhooks to retry")
		return
	}

	log.Printf("Found %d failed webhooks to retry", len(failedLogs))

	for _, logEntry := range failedLogs {
		// In a real implementation, you would need to store the full webhook payload
		// and retrieve it from PostgreSQL. For now, this is a placeholder.
		log.Printf("Would retry webhook %s (attempt %d)", logEntry.OrderID, logEntry.Retries+1)
	}
}
