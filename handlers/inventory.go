package handlers

import (
	"net/http"
	"time"

	"webhook-buffer/services"

	"github.com/gin-gonic/gin"
)

type InventoryHandler struct {
	redisService services.InventoryCacheService
	client1C     services.OrderService
	cacheTTL     time.Duration
}

func NewInventoryHandler(redisService services.InventoryCacheService, client1C services.OrderService, cacheTTL time.Duration) *InventoryHandler {
	return &InventoryHandler{
		redisService: redisService,
		client1C:     client1C,
		cacheTTL:     cacheTTL,
	}
}

func (h *InventoryHandler) GetInventory(c *gin.Context) {
	sku := c.Param("sku")
	if sku == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "SKU parameter is required"})
		return
	}

	ctx := c.Request.Context()

	cached, err := h.redisService.GetInventory(ctx, sku)
	if err == nil && cached != nil {
		c.JSON(http.StatusOK, gin.H{
			"sku":      cached.SKU,
			"quantity": cached.Quantity,
			"price":    cached.Price,
			"cached":   true,
		})
		return
	}

	inventory, err := h.client1C.GetInventory(ctx, sku)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "Failed to fetch inventory from 1C",
			"details": err.Error(),
		})
		return
	}

	_ = h.redisService.CacheInventory(ctx, inventory.SKU, inventory.Quantity, inventory.Price, h.cacheTTL)

	c.JSON(http.StatusOK, gin.H{
		"sku":      inventory.SKU,
		"quantity": inventory.Quantity,
		"price":    inventory.Price,
		"cached":   false,
	})
}

func (h *InventoryHandler) InvalidateCache(c *gin.Context) {
	sku := c.Param("sku")
	if sku == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "SKU parameter is required"})
		return
	}

	ctx := c.Request.Context()

	if err := h.redisService.InvalidateCache(ctx, sku); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to invalidate cache"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Cache invalidated successfully"})
}
