package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"webhook-buffer/models"

	_ "github.com/lib/pq"
)

type PostgresService struct {
	db *sql.DB
}

func NewPostgresService(connString string) (*PostgresService, error) {
	db, err := sql.Open("postgres", connString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresService{db: db}, nil
}

func (p *PostgresService) Close() error {
	return p.db.Close()
}

// InitSchema creates necessary tables
func (p *PostgresService) InitSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS webhook_logs (
		id SERIAL PRIMARY KEY,
		order_id VARCHAR(100) NOT NULL,
		event VARCHAR(100) NOT NULL,
		status VARCHAR(50) NOT NULL,
		retries INTEGER DEFAULT 0,
		received_at TIMESTAMP NOT NULL,
		processed_at TIMESTAMP,
		error_msg TEXT,
		payload JSONB,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_webhook_logs_order_id ON webhook_logs(order_id);
	CREATE INDEX IF NOT EXISTS idx_webhook_logs_status ON webhook_logs(status);
	CREATE INDEX IF NOT EXISTS idx_webhook_logs_received_at ON webhook_logs(received_at);
	`

	_, err := p.db.Exec(query)
	return err
}

// LogWebhook records webhook processing
func (p *PostgresService) LogWebhook(log models.WebhookLog, webhook interface{}) error {
	payload, err := json.Marshal(webhook)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	query := `
	INSERT INTO webhook_logs (order_id, event, status, retries, received_at, processed_at, error_msg, payload)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	RETURNING id
	`

	var id int64
	err = p.db.QueryRow(
		query,
		log.OrderID,
		log.Event,
		log.Status,
		log.Retries,
		log.ReceivedAt,
		log.ProcessedAt,
		log.ErrorMsg,
		payload,
	).Scan(&id)

	if err != nil {
		return fmt.Errorf("failed to insert webhook log: %w", err)
	}

	return nil
}

// UpdateWebhookStatus updates webhook processing status
func (p *PostgresService) UpdateWebhookStatus(orderID string, status string, retries int, errorMsg *string) error {
	query := `
	UPDATE webhook_logs
	SET status = $1, retries = $2, error_msg = $3, processed_at = $4
	WHERE order_id = $5
	ORDER BY received_at DESC
	LIMIT 1
	`

	now := time.Now()
	_, err := p.db.Exec(query, status, retries, errorMsg, &now, orderID)
	return err
}

// GetStats returns processing statistics
func (p *PostgresService) GetStats() (map[string]int64, error) {
	query := `
	SELECT 
		COUNT(*) FILTER (WHERE status = 'received') as received,
		COUNT(*) FILTER (WHERE status = 'queued') as queued,
		COUNT(*) FILTER (WHERE status = 'processed') as processed,
		COUNT(*) FILTER (WHERE status = 'failed') as failed
	FROM webhook_logs
	WHERE received_at > $1
	`

	stats := make(map[string]int64)
	var received, queued, processed, failed int64
	yesterday := time.Now().Add(-24 * time.Hour)

	err := p.db.QueryRow(query, yesterday).Scan(&received, &queued, &processed, &failed)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	stats["received"] = received
	stats["queued"] = queued
	stats["processed"] = processed
	stats["failed"] = failed

	return stats, nil
}

// GetFailedWebhooks returns webhooks that failed processing
func (p *PostgresService) GetFailedWebhooks(limit int) ([]models.WebhookLog, error) {
	query := `
	SELECT id, order_id, event, status, retries, received_at, processed_at, error_msg
	FROM webhook_logs
	WHERE status = 'failed' AND retries < 5
	ORDER BY received_at ASC
	LIMIT $1
	`

	rows, err := p.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get failed webhooks: %w", err)
	}
	defer rows.Close()

	var logs []models.WebhookLog
	for rows.Next() {
		var log models.WebhookLog
		err := rows.Scan(
			&log.ID,
			&log.OrderID,
			&log.Event,
			&log.Status,
			&log.Retries,
			&log.ReceivedAt,
			&log.ProcessedAt,
			&log.ErrorMsg,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan webhook log: %w", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}
