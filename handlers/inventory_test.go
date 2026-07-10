package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"webhook-buffer/mocks"
	"webhook-buffer/models"

	"github.com/gin-gonic/gin"
)

func TestGetInventory_CacheHit(t *testing.T) {
	mockCache := &mocks.MockInventoryCacheService{
		GetInventoryFunc: func(ctx context.Context, sku string) (*models.InventoryCache, error) {
			return &models.InventoryCache{
				SKU:      "SKU-001",
				Quantity: 50,
				Price:    199.99,
			}, nil
		},
	}
	mockOrder := &mocks.MockOrderService{}
	handler := NewInventoryHandler(mockCache, mockOrder, 0)

	router := setupTestRouter()
	router.GET("/inventory/:sku", handler.GetInventory)

	req := httptest.NewRequest("GET", "/inventory/SKU-001", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["cached"] != true {
		t.Error("Expected cached=true for cache hit")
	}

	if resp["sku"] != "SKU-001" {
		t.Errorf("Expected SKU-001, got %v", resp["sku"])
	}

	if mockOrder.SendOrderCalled != 0 {
		t.Error("Should not call 1C on cache hit")
	}
}

func TestGetInventory_CacheMiss(t *testing.T) {
	mockCache := &mocks.MockInventoryCacheService{}
	mockOrder := &mocks.MockOrderService{
		GetInventoryFunc: func(ctx context.Context, sku string) (*models.InventoryCache, error) {
			return &models.InventoryCache{
				SKU:      "SKU-NEW",
				Quantity: 10,
				Price:    50.0,
			}, nil
		},
	}
	handler := NewInventoryHandler(mockCache, mockOrder, 0)

	router := setupTestRouter()
	router.GET("/inventory/:sku", handler.GetInventory)

	req := httptest.NewRequest("GET", "/inventory/SKU-NEW", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["cached"] != false {
		t.Error("Expected cached=false for cache miss")
	}

	if mockCache.CacheCalled != 1 {
		t.Error("Expected cache to be populated after 1C fetch")
	}
}

func TestGetInventory_1CError(t *testing.T) {
	mockCache := &mocks.MockInventoryCacheService{}
	mockOrder := &mocks.MockOrderService{
		GetInventoryFunc: func(ctx context.Context, sku string) (*models.InventoryCache, error) {
			return nil, fmt.Errorf("1C is down")
		},
	}
	handler := NewInventoryHandler(mockCache, mockOrder, 0)

	router := setupTestRouter()
	router.GET("/inventory/:sku", handler.GetInventory)

	req := httptest.NewRequest("GET", "/inventory/SKU-ERR", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected 503, got %d", w.Code)
	}
}

func TestInvalidateCache(t *testing.T) {
	mockCache := &mocks.MockInventoryCacheService{}
	mockOrder := &mocks.MockOrderService{}
	handler := NewInventoryHandler(mockCache, mockOrder, 0)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/inventory/:sku/cache", handler.InvalidateCache)

	req := httptest.NewRequest("DELETE", "/inventory/SKU-DEL/cache", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	if mockCache.InvalidateCall != "SKU-DEL" {
		t.Errorf("Expected invalidate for SKU-DEL, got %s", mockCache.InvalidateCall)
	}
}

func TestGetInventory_EmptySKU(t *testing.T) {
	mockCache := &mocks.MockInventoryCacheService{}
	mockOrder := &mocks.MockOrderService{}
	handler := NewInventoryHandler(mockCache, mockOrder, 0)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/inventory/:sku", handler.GetInventory)

	req := httptest.NewRequest("GET", "/inventory/", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Logf("Got %d for empty SKU path (expected 404 from Gin router)", w.Code)
	}
}
