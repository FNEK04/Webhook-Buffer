package mocks

import (
	"context"
	"time"

	"webhook-buffer/models"
)

type MockInventoryCacheService struct {
	CacheInventoryFunc func(ctx context.Context, sku string, quantity int, price float64, ttl time.Duration) error
	GetInventoryFunc   func(ctx context.Context, sku string) (*models.InventoryCache, error)
	InvalidateCacheFunc func(ctx context.Context, sku string) error

	CacheCalled    int
	InvalidateCall string
}

func (m *MockInventoryCacheService) CacheInventory(ctx context.Context, sku string, quantity int, price float64, ttl time.Duration) error {
	m.CacheCalled++
	if m.CacheInventoryFunc != nil {
		return m.CacheInventoryFunc(ctx, sku, quantity, price, ttl)
	}
	return nil
}

func (m *MockInventoryCacheService) GetInventory(ctx context.Context, sku string) (*models.InventoryCache, error) {
	if m.GetInventoryFunc != nil {
		return m.GetInventoryFunc(ctx, sku)
	}
	return nil, nil
}

func (m *MockInventoryCacheService) InvalidateCache(ctx context.Context, sku string) error {
	m.InvalidateCall = sku
	if m.InvalidateCacheFunc != nil {
		return m.InvalidateCacheFunc(ctx, sku)
	}
	return nil
}
