package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"webhook-buffer/models"
	"webhook-buffer/services"

	"github.com/gin-gonic/gin"
)

type WebhookHandler struct {
	redisService services.QueueService
	pgService    services.LogService
}

func NewWebhookHandler(redisService services.QueueService, pgService services.LogService) *WebhookHandler {
	return &WebhookHandler{
		redisService: redisService,
		pgService:    pgService,
	}
}

func (h *WebhookHandler) HandleWebhook(c *gin.Context) {
	var webhook models.Webhook

	if err := c.ShouldBindJSON(&webhook); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": c.GetString("request_id"),
		})
		return
	}

	ctx := c.Request.Context()
	log := models.WebhookLog{
		OrderID:    webhook.Payload.OrderID,
		Event:      webhook.Event,
		Status:     "received",
		Retries:    0,
		ReceivedAt: time.Now(),
	}

	webhookID, err := h.pgService.LogWebhook(ctx, log, webhook)
	if err != nil {
		slog.Error("failed to log webhook", "order_id", webhook.Payload.OrderID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to log webhook",
			"request_id": c.GetString("request_id"),
		})
		return
	}

	if err := h.redisService.Enqueue(ctx, webhook, webhookID); err != nil {
		slog.Error("failed to enqueue webhook", "order_id", webhook.Payload.OrderID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to enqueue webhook",
			"request_id": c.GetString("request_id"),
		})
		return
	}

	if err := h.pgService.UpdateWebhookStatus(ctx, webhookID, "queued", 0, nil); err != nil {
		slog.Error("failed to update webhook status",
			"order_id", webhook.Payload.OrderID,
			"webhook_id", webhookID,
			"error", err,
		)
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message":    "Webhook accepted and queued",
		"order_id":   webhook.Payload.OrderID,
		"webhook_id": webhookID,
		"request_id": c.GetString("request_id"),
	})
}

func (h *WebhookHandler) HandleBatchWebhook(c *gin.Context) {
	var webhooks []models.Webhook

	if err := c.ShouldBindJSON(&webhooks); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": c.GetString("request_id"),
		})
		return
	}

	if len(webhooks) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Empty webhook array",
			"request_id": c.GetString("request_id"),
		})
		return
	}

	if len(webhooks) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Batch size exceeds maximum of 100",
			"request_id": c.GetString("request_id"),
		})
		return
	}

	ctx := c.Request.Context()
	now := time.Now()

	logs := make([]models.WebhookLog, 0, len(webhooks))
	for _, wh := range webhooks {
		logs = append(logs, models.WebhookLog{
			OrderID:    wh.Payload.OrderID,
			Event:      wh.Event,
			Status:     "received",
			Retries:    0,
			ReceivedAt: now,
		})
	}

	ids, err := h.pgService.LogWebhookBatch(ctx, logs, webhooks)
	if err != nil {
		slog.Error("failed to batch log webhooks", "count", len(webhooks), "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to log webhooks",
			"request_id": c.GetString("request_id"),
		})
		return
	}

	queueItems := make([]models.QueueItem, 0, len(ids))
	orderIDs := make([]string, 0, len(ids))
	for i, id := range ids {
		queueItems = append(queueItems, models.QueueItem{
			Webhook:  webhooks[i],
			Attempts: 0,
			Received: now,
			LogID:    id,
		})
		orderIDs = append(orderIDs, webhooks[i].Payload.OrderID)
	}

	if err := h.redisService.EnqueueBatch(ctx, queueItems); err != nil {
		slog.Error("failed to batch enqueue webhooks", "count", len(queueItems), "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to enqueue webhooks",
			"request_id": c.GetString("request_id"),
		})
		return
	}

	if err := h.pgService.UpdateWebhookStatusBatch(ctx, ids, "queued", 0); err != nil {
		slog.Error("failed to batch update webhook status", "error", err)
	}

	slog.Info("batch webhooks processed",
		"count", len(ids),
		"request_id", c.GetString("request_id"),
	)

	c.JSON(http.StatusAccepted, gin.H{
		"message":       "Batch processing completed",
		"success_count": len(ids),
		"failed_count":  0,
		"order_ids":     orderIDs,
		"request_id":    c.GetString("request_id"),
	})
}

func (h *WebhookHandler) GetQueueStatus(c *gin.Context) {
	ctx := c.Request.Context()

	queueSize, err := h.redisService.GetQueueSize(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get queue size"})
		return
	}

	stats, err := h.pgService.GetStats(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get statistics"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"queue_size": queueSize,
		"stats":      stats,
	})
}
