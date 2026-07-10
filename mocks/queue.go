package mocks

import (
	"context"
	"time"

	"webhook-buffer/models"
)

type MockQueueService struct {
	EnqueueFunc       func(ctx context.Context, webhook models.Webhook, logID ...int64) error
	EnqueueBatchFunc  func(ctx context.Context, items []models.QueueItem) error
	DequeueFunc       func(ctx context.Context, timeout time.Duration) (*models.QueueItem, error)
	GetQueueSizeFunc  func(ctx context.Context) (int64, error)
	PingFunc          func(ctx context.Context) error
	EnqueueCalled     int
	EnqueueBatchCount int
}

func (m *MockQueueService) Enqueue(ctx context.Context, webhook models.Webhook, logID ...int64) error {
	m.EnqueueCalled++
	if m.EnqueueFunc != nil {
		return m.EnqueueFunc(ctx, webhook, logID...)
	}
	return nil
}

func (m *MockQueueService) EnqueueBatch(ctx context.Context, items []models.QueueItem) error {
	m.EnqueueBatchCount = len(items)
	if m.EnqueueBatchFunc != nil {
		return m.EnqueueBatchFunc(ctx, items)
	}
	return nil
}

func (m *MockQueueService) Dequeue(ctx context.Context, timeout time.Duration) (*models.QueueItem, error) {
	if m.DequeueFunc != nil {
		return m.DequeueFunc(ctx, timeout)
	}
	return nil, nil
}

func (m *MockQueueService) GetQueueSize(ctx context.Context) (int64, error) {
	if m.GetQueueSizeFunc != nil {
		return m.GetQueueSizeFunc(ctx)
	}
	return 0, nil
}

func (m *MockQueueService) Ping(ctx context.Context) error {
	if m.PingFunc != nil {
		return m.PingFunc(ctx)
	}
	return nil
}

func (m *MockQueueService) Close() error { return nil }
