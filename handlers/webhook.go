package handlers

import (
	"net/http"
	"time"

	"webhook-buffer/models"
	"webhook-buffer/services"

	"github.com/gin-gonic/gin"
)

type WebhookHandler struct {
	redisService *services.RedisService
	pgService    *services.PostgresService
}

func NewWebhookHandler(redisService *services.RedisService, pgService *services.PostgresService) *WebhookHandler {
	return &WebhookHandler{
		redisService: redisService,
		pgService:    pgService,
	}
}

// HandleWebhook processes incoming webhook requests
func (h *WebhookHandler) HandleWebhook(c *gin.Context) {
	var webhook models.Webhook

	if err := c.ShouldBindJSON(&webhook); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Log webhook as received
	log := models.WebhookLog{
		OrderID:    webhook.Payload.OrderID,
		Event:      webhook.Event,
		Status:     "received",
		Retries:    0,
		ReceivedAt: time.Now(),
	}

	if err := h.pgService.LogWebhook(log, webhook); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to log webhook"})
		return
	}

	// Enqueue to Redis
	if err := h.redisService.Enqueue(webhook); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enqueue webhook"})
		return
	}

	// Update log status to queued
	if err := h.pgService.UpdateWebhookStatus(webhook.Payload.OrderID, "queued", 0, nil); err != nil {
		// Log error but don't fail the request
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Webhook accepted and queued",
		"order_id": webhook.Payload.OrderID,
	})
}

// HandleBatchWebhook processes multiple webhooks in a single request
func (h *WebhookHandler) HandleBatchWebhook(c *gin.Context) {
	var webhooks []models.Webhook

	if err := c.ShouldBindJSON(&webhooks); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	if len(webhooks) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Empty webhook array"})
		return
	}

	if len(webhooks) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Batch size exceeds maximum of 100"})
		return
	}

	successCount := 0
	failedCount := 0
	orderIDs := make([]string, 0, len(webhooks))

	for _, webhook := range webhooks {
		log := models.WebhookLog{
			OrderID:    webhook.Payload.OrderID,
			Event:      webhook.Event,
			Status:     "received",
			Retries:    0,
			ReceivedAt: time.Now(),
		}

		if err := h.pgService.LogWebhook(log, webhook); err != nil {
			failedCount++
			continue
		}

		if err := h.redisService.Enqueue(webhook); err != nil {
			failedCount++
			continue
		}

		h.pgService.UpdateWebhookStatus(webhook.Payload.OrderID, "queued", 0, nil)
		successCount++
		orderIDs = append(orderIDs, webhook.Payload.OrderID)
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message":       "Batch processing completed",
		"success_count": successCount,
		"failed_count":  failedCount,
		"order_ids":     orderIDs,
	})
}

// GetQueueStatus returns current queue statistics
func (h *WebhookHandler) GetQueueStatus(c *gin.Context) {
	queueSize, err := h.redisService.GetQueueSize()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get queue size"})
		return
	}

	stats, err := h.pgService.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get statistics"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"queue_size": queueSize,
		"stats":      stats,
	})
}
