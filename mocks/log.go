package mocks

import (
	"context"

	"webhook-buffer/models"
)

type MockLogService struct {
	LogWebhookFunc          func(ctx context.Context, log models.WebhookLog, webhook models.Webhook) (int64, error)
	LogWebhookBatchFunc     func(ctx context.Context, logs []models.WebhookLog, webhooks []models.Webhook) ([]int64, error)
	UpdateWebhookStatusFunc func(ctx context.Context, id int64, status string, retries int, errorMsg *string) error
	UpdateBatchFunc         func(ctx context.Context, ids []int64, status string, retries int) error
	GetStatsFunc            func(ctx context.Context) (map[string]int64, error)
	GetFailedWebhooksFunc   func(ctx context.Context, limit int, maxRetries int) ([]models.WebhookLog, error)
	MoveToDeadLetterFunc    func(ctx context.Context, id int64) error
	PingFunc                func(ctx context.Context) error

	LogWebhookCalled    int
	UpdateStatusCalls   []int64
	MoveToDeadLetterIds []int64
}

func (m *MockLogService) LogWebhook(ctx context.Context, log models.WebhookLog, webhook models.Webhook) (int64, error) {
	m.LogWebhookCalled++
	if m.LogWebhookFunc != nil {
		return m.LogWebhookFunc(ctx, log, webhook)
	}
	return int64(m.LogWebhookCalled), nil
}

func (m *MockLogService) LogWebhookBatch(ctx context.Context, logs []models.WebhookLog, webhooks []models.Webhook) ([]int64, error) {
	if m.LogWebhookBatchFunc != nil {
		return m.LogWebhookBatchFunc(ctx, logs, webhooks)
	}
	ids := make([]int64, len(logs))
	for i := range logs {
		ids[i] = int64(i + 1)
	}
	return ids, nil
}

func (m *MockLogService) UpdateWebhookStatus(ctx context.Context, id int64, status string, retries int, errorMsg *string) error {
	m.UpdateStatusCalls = append(m.UpdateStatusCalls, id)
	if m.UpdateWebhookStatusFunc != nil {
		return m.UpdateWebhookStatusFunc(ctx, id, status, retries, errorMsg)
	}
	return nil
}

func (m *MockLogService) UpdateWebhookStatusBatch(ctx context.Context, ids []int64, status string, retries int) error {
	if m.UpdateBatchFunc != nil {
		return m.UpdateBatchFunc(ctx, ids, status, retries)
	}
	return nil
}

func (m *MockLogService) GetStats(ctx context.Context) (map[string]int64, error) {
	if m.GetStatsFunc != nil {
		return m.GetStatsFunc(ctx)
	}
	return map[string]int64{"received": 0, "queued": 0, "processed": 0, "failed": 0}, nil
}

func (m *MockLogService) GetFailedWebhooks(ctx context.Context, limit int, maxRetries int) ([]models.WebhookLog, error) {
	if m.GetFailedWebhooksFunc != nil {
		return m.GetFailedWebhooksFunc(ctx, limit, maxRetries)
	}
	return nil, nil
}

func (m *MockLogService) MoveToDeadLetter(ctx context.Context, id int64) error {
	m.MoveToDeadLetterIds = append(m.MoveToDeadLetterIds, id)
	if m.MoveToDeadLetterFunc != nil {
		return m.MoveToDeadLetterFunc(ctx, id)
	}
	return nil
}

func (m *MockLogService) InitSchema() error { return nil }

func (m *MockLogService) Ping(ctx context.Context) error {
	if m.PingFunc != nil {
		return m.PingFunc(ctx)
	}
	return nil
}

func (m *MockLogService) Close() error { return nil }
