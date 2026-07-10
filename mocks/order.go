package mocks

import (
	"context"

	"webhook-buffer/models"
)

type MockOrderService struct {
	SendOrderFunc     func(ctx context.Context, webhook models.Webhook) error
	GetInventoryFunc  func(ctx context.Context, sku string) (*models.InventoryCache, error)
	HealthCheckFunc   func(ctx context.Context) error

	SendOrderCalled int
}

func (m *MockOrderService) SendOrder(ctx context.Context, webhook models.Webhook) error {
	m.SendOrderCalled++
	if m.SendOrderFunc != nil {
		return m.SendOrderFunc(ctx, webhook)
	}
	return nil
}

func (m *MockOrderService) GetInventory(ctx context.Context, sku string) (*models.InventoryCache, error) {
	if m.GetInventoryFunc != nil {
		return m.GetInventoryFunc(ctx, sku)
	}
	return nil, nil
}

func (m *MockOrderService) HealthCheck(ctx context.Context) error {
	if m.HealthCheckFunc != nil {
		return m.HealthCheckFunc(ctx)
	}
	return nil
}
