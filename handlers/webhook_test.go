package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"webhook-buffer/models"

	"github.com/gin-gonic/gin"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	return router
}

func TestWebhookHandler_HandleWebhook_Valid(t *testing.T) {
	router := setupTestRouter()
	
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
	
	body, _ := json.Marshal(webhook)
	req := httptest.NewRequest("POST", "/webhook", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	
	router.POST("/webhook", func(c *gin.Context) {
		var wh models.Webhook
		if err := c.ShouldBindJSON(&wh); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusAccepted, gin.H{
			"message":  "Webhook accepted",
			"order_id": wh.Payload.OrderID,
		})
	})
	
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusAccepted {
		t.Errorf("Expected status 202, got %d: %s", w.Code, w.Body.String())
	}
	
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if response["order_id"] != "TEST-001" {
		t.Errorf("Expected order_id TEST-001, got %v", response["order_id"])
	}
}

func TestWebhookHandler_HandleWebhook_Invalid(t *testing.T) {
	router := setupTestRouter()
	
	invalidJSON := `{"invalid": "data"}`
	req := httptest.NewRequest("POST", "/webhook", bytes.NewBufferString(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	
	router.POST("/webhook", func(c *gin.Context) {
		var wh models.Webhook
		if err := c.ShouldBindJSON(&wh); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
		c.JSON(http.StatusAccepted, gin.H{"message": "Webhook accepted"})
	})
	
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestWebhookHandler_HandleBatchWebhook(t *testing.T) {
	router := setupTestRouter()
	
	webhooks := []models.Webhook{
		{
			Event:     "order.created",
			Timestamp: time.Now(),
			Payload: models.Payload{
				OrderID:       "BATCH-001",
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
		},
		{
			Event:     "order.created",
			Timestamp: time.Now(),
			Payload: models.Payload{
				OrderID:       "BATCH-002",
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
						SKU:      "SKU-002",
						Name:     "Test Item 2",
						Quantity: 1,
						Price:    200,
						Discount: 0,
					},
				},
				TotalAmount: 200,
			},
		},
	}
	
	body, _ := json.Marshal(webhooks)
	req := httptest.NewRequest("POST", "/webhook/batch", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	
	router.POST("/webhook/batch", func(c *gin.Context) {
		var wh []models.Webhook
		if err := c.ShouldBindJSON(&wh); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusAccepted, gin.H{
			"message":       "Batch accepted",
			"success_count": len(wh),
		})
	})
	
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusAccepted {
		t.Errorf("Expected status 202, got %d: %s", w.Code, w.Body.String())
	}
	
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if response["success_count"] != float64(2) {
		t.Errorf("Expected success_count 2, got %v", response["success_count"])
	}
}
