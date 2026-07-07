package handlers

import (
	"net/http"
	"time"

	"webhook-buffer/services"

	"github.com/gin-gonic/gin"
)

type InventoryHandler struct {
	redisService *services.RedisService
	client1C     *services.Client1C
	cacheTTL     time.Duration
}

func NewInventoryHandler(redisService *services.RedisService, client1C *services.Client1C, cacheTTL time.Duration) *InventoryHandler {
	return &InventoryHandler{
		redisService: redisService,
		client1C:     client1C,
		cacheTTL:     cacheTTL,
	}
}

// GetInventory retrieves inventory data with cache fallback
func (h *InventoryHandler) GetInventory(c *gin.Context) {
	sku := c.Param("sku")
	if sku == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "SKU parameter is required"})
		return
	}

	// Try to get from cache first
	cached, err := h.redisService.GetInventory(sku)
	if err == nil && cached != nil {
		c.JSON(http.StatusOK, gin.H{
			"sku":      cached.SKU,
			"quantity": cached.Quantity,
			"price":    cached.Price,
			"cached":   true,
		})
		return
	}

	// Cache miss - fetch from 1C
	inventory, err := h.client1C.GetInventory(sku)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Failed to fetch inventory from 1C",
			"details": err.Error(),
		})
		return
	}

	// Cache the result
	h.redisService.CacheInventory(inventory.SKU, inventory.Quantity, inventory.Price, h.cacheTTL)

	c.JSON(http.StatusOK, gin.H{
		"sku":      inventory.SKU,
		"quantity": inventory.Quantity,
		"price":    inventory.Price,
		"cached":   false,
	})
}

// InvalidateCache removes inventory from cache
func (h *InventoryHandler) InvalidateCache(c *gin.Context) {
	sku := c.Param("sku")
	if sku == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "SKU parameter is required"})
		return
	}

	if err := h.redisService.InvalidateCache(sku); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to invalidate cache"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Cache invalidated successfully"})
}
