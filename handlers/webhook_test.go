package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"webhook-buffer/mocks"
	"webhook-buffer/models"

	"github.com/gin-gonic/gin"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("request_id", "test-req-001")
		c.Next()
	})
	return router
}

func validWebhookJSON() string {
	w := models.Webhook{
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
				Cost:    10.0,
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
	data, _ := json.Marshal(w)
	return string(data)
}

func TestHandleWebhook_Valid(t *testing.T) {
	mockRedis := &mocks.MockQueueService{}
	mockPG := &mocks.MockLogService{}
	handler := NewWebhookHandler(mockRedis, mockPG)

	router := setupTestRouter()
	router.POST("/webhook", handler.HandleWebhook)

	req := httptest.NewRequest("POST", "/webhook", bytes.NewBufferString(validWebhookJSON()))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("Expected 202, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp["order_id"] != "TEST-001" {
		t.Errorf("Expected order_id TEST-001, got %v", resp["order_id"])
	}

	if resp["request_id"] != "test-req-001" {
		t.Errorf("Expected request_id in response")
	}

	if mockPG.LogWebhookCalled != 1 {
		t.Errorf("Expected LogWebhook to be called once, got %d", mockPG.LogWebhookCalled)
	}

	if mockRedis.EnqueueCalled != 1 {
		t.Errorf("Expected Enqueue to be called once, got %d", mockRedis.EnqueueCalled)
	}
}

func TestHandleWebhook_InvalidJSON(t *testing.T) {
	mockRedis := &mocks.MockQueueService{}
	mockPG := &mocks.MockLogService{}
	handler := NewWebhookHandler(mockRedis, mockPG)

	router := setupTestRouter()
	router.POST("/webhook", handler.HandleWebhook)

	req := httptest.NewRequest("POST", "/webhook", bytes.NewBufferString(`{"invalid": true}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}

	if mockPG.LogWebhookCalled != 0 {
		t.Errorf("Expected no DB calls for invalid JSON, got %d", mockPG.LogWebhookCalled)
	}
}

func TestHandleWebhook_PGError(t *testing.T) {
	mockRedis := &mocks.MockQueueService{}
	mockPG := &mocks.MockLogService{
		LogWebhookFunc: func(ctx context.Context, log models.WebhookLog, webhook models.Webhook) (int64, error) {
			return 0, fmt.Errorf("db connection lost")
		},
	}
	handler := NewWebhookHandler(mockRedis, mockPG)

	router := setupTestRouter()
	router.POST("/webhook", handler.HandleWebhook)

	req := httptest.NewRequest("POST", "/webhook", bytes.NewBufferString(validWebhookJSON()))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500, got %d", w.Code)
	}

	if mockRedis.EnqueueCalled != 0 {
		t.Errorf("Expected no enqueue on PG error")
	}
}

func TestHandleWebhook_RedisError(t *testing.T) {
	mockRedis := &mocks.MockQueueService{
		EnqueueFunc: func(ctx context.Context, webhook models.Webhook, logID ...int64) error {
			return fmt.Errorf("redis connection lost")
		},
	}
	mockPG := &mocks.MockLogService{}
	handler := NewWebhookHandler(mockRedis, mockPG)

	router := setupTestRouter()
	router.POST("/webhook", handler.HandleWebhook)

	req := httptest.NewRequest("POST", "/webhook", bytes.NewBufferString(validWebhookJSON()))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500, got %d", w.Code)
	}

	if mockPG.LogWebhookCalled != 1 {
		t.Error("Expected PG log to be called before Redis enqueue")
	}
}

func TestHandleBatchWebhook_Valid(t *testing.T) {
	mockRedis := &mocks.MockQueueService{}
	mockPG := &mocks.MockLogService{}
	handler := NewWebhookHandler(mockRedis, mockPG)

	router := setupTestRouter()
	router.POST("/webhook/batch", handler.HandleBatchWebhook)

	batch := []json.RawMessage{
		json.RawMessage(validWebhookJSON()),
		json.RawMessage(validWebhookJSON()),
	}
	body, _ := json.Marshal(batch)

	req := httptest.NewRequest("POST", "/webhook/batch", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("Expected 202, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp["success_count"] != float64(2) {
		t.Errorf("Expected success_count 2, got %v", resp["success_count"])
	}

	if mockRedis.EnqueueBatchCount != 2 {
		t.Errorf("Expected batch enqueue of 2 items, got %d", mockRedis.EnqueueBatchCount)
	}
}

func TestHandleBatchWebhook_Empty(t *testing.T) {
	mockRedis := &mocks.MockQueueService{}
	mockPG := &mocks.MockLogService{}
	handler := NewWebhookHandler(mockRedis, mockPG)

	router := setupTestRouter()
	router.POST("/webhook/batch", handler.HandleBatchWebhook)

	req := httptest.NewRequest("POST", "/webhook/batch", bytes.NewBufferString("[]"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

func TestHandleBatchWebhook_ExceedsMax(t *testing.T) {
	mockRedis := &mocks.MockQueueService{}
	mockPG := &mocks.MockLogService{}
	handler := NewWebhookHandler(mockRedis, mockPG)

	router := setupTestRouter()
	router.POST("/webhook/batch", handler.HandleBatchWebhook)

	batch := make([]json.RawMessage, 101)
	for i := range batch {
		batch[i] = json.RawMessage(validWebhookJSON())
	}
	body, _ := json.Marshal(batch)

	req := httptest.NewRequest("POST", "/webhook/batch", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

func TestGetQueueStatus(t *testing.T) {
	mockRedis := &mocks.MockQueueService{
		GetQueueSizeFunc: func(ctx context.Context) (int64, error) {
			return 42, nil
		},
	}
	mockPG := &mocks.MockLogService{
		GetStatsFunc: func(ctx context.Context) (map[string]int64, error) {
			return map[string]int64{"received": 100, "queued": 10, "processed": 80, "failed": 10}, nil
		},
	}
	handler := NewWebhookHandler(mockRedis, mockPG)

	router := setupTestRouter()
	router.GET("/webhook/status", handler.GetQueueStatus)

	req := httptest.NewRequest("GET", "/webhook/status", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp["queue_size"] != float64(42) {
		t.Errorf("Expected queue_size 42, got %v", resp["queue_size"])
	}
}
