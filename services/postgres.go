package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
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

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(3 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresService{db: db}, nil
}

func (p *PostgresService) Close() error {
	return p.db.Close()
}

func (p *PostgresService) Ping(ctx context.Context) error {
	return p.db.PingContext(ctx)
}

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

	CREATE TABLE IF NOT EXISTS dead_letter (
		id SERIAL PRIMARY KEY,
		original_log_id INTEGER REFERENCES webhook_logs(id),
		order_id VARCHAR(100) NOT NULL,
		event VARCHAR(100) NOT NULL,
		payload JSONB NOT NULL,
		error_msg TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_dead_letter_order_id ON dead_letter(order_id);
	`

	_, err := p.db.Exec(query)
	return err
}

func (p *PostgresService) LogWebhook(ctx context.Context, log models.WebhookLog, webhook models.Webhook) (int64, error) {
	payload, err := json.Marshal(webhook)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	query := `
	INSERT INTO webhook_logs (order_id, event, status, retries, received_at, processed_at, error_msg, payload)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	RETURNING id
	`

	var id int64
	err = p.db.QueryRowContext(ctx, query,
		log.OrderID, log.Event, log.Status, log.Retries,
		log.ReceivedAt, log.ProcessedAt, log.ErrorMsg, payload,
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("failed to insert webhook log: %w", err)
	}

	return id, nil
}

func (p *PostgresService) LogWebhookBatch(ctx context.Context, logs []models.WebhookLog, webhooks []models.Webhook) ([]int64, error) {
	if len(logs) == 0 {
		return nil, nil
	}

	query := `
	INSERT INTO webhook_logs (order_id, event, status, retries, received_at, processed_at, error_msg, payload)
	VALUES %s
	RETURNING id
	`

	valueStrings := make([]string, 0, len(logs))
	valueArgs := make([]interface{}, 0, len(logs)*8)

	for i, log := range logs {
		payload, err := json.Marshal(webhooks[i])
		if err != nil {
			return nil, fmt.Errorf("failed to marshal webhook payload: %w", err)
		}

		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			i*8+1, i*8+2, i*8+3, i*8+4, i*8+5, i*8+6, i*8+7, i*8+8))
		valueArgs = append(valueArgs,
			log.OrderID, log.Event, log.Status, log.Retries,
			log.ReceivedAt, log.ProcessedAt, log.ErrorMsg, payload,
		)
	}

	finalQuery := fmt.Sprintf(query, strings.Join(valueStrings, ", "))

	rows, err := p.db.QueryContext(ctx, finalQuery, valueArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to insert webhook log batch: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan webhook log id: %w", err)
		}
		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating webhook log rows: %w", err)
	}

	slog.Debug("batch inserted webhook logs", "count", len(ids))
	return ids, nil
}

func (p *PostgresService) UpdateWebhookStatus(ctx context.Context, id int64, status string, retries int, errorMsg *string) error {
	query := `
	UPDATE webhook_logs
	SET status = $1, retries = $2, error_msg = $3, processed_at = $4
	WHERE id = $5
	`

	var processedAt *time.Time
	if status == "processed" || status == "failed" {
		now := time.Now()
		processedAt = &now
	}

	_, err := p.db.ExecContext(ctx, query, status, retries, errorMsg, processedAt, id)
	return err
}

func (p *PostgresService) UpdateWebhookStatusBatch(ctx context.Context, ids []int64, status string, retries int) error {
	if len(ids) == 0 {
		return nil
	}

	query := `
	UPDATE webhook_logs
	SET status = $1, retries = $2, processed_at = $3
	WHERE id = ANY($4)
	`

	now := time.Now()
	_, err := p.db.ExecContext(ctx, query, status, retries, now, pqInt64Array(ids))
	return err
}

func (p *PostgresService) GetStats(ctx context.Context) (map[string]int64, error) {
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

	err := p.db.QueryRowContext(ctx, query, yesterday).Scan(&received, &queued, &processed, &failed)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	stats["received"] = received
	stats["queued"] = queued
	stats["processed"] = processed
	stats["failed"] = failed

	return stats, nil
}

func (p *PostgresService) GetFailedWebhooks(ctx context.Context, limit int, maxRetries int) ([]models.WebhookLog, error) {
	query := `
	SELECT id, order_id, event, status, retries, received_at, processed_at, error_msg, payload
	FROM webhook_logs
	WHERE status = 'failed' AND retries < $1
	ORDER BY received_at ASC
	LIMIT $2
	`

	rows, err := p.db.QueryContext(ctx, query, maxRetries, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get failed webhooks: %w", err)
	}
	defer rows.Close()

	var logs []models.WebhookLog
	for rows.Next() {
		var l models.WebhookLog
		var payload []byte
		err := rows.Scan(
			&l.ID, &l.OrderID, &l.Event, &l.Status,
			&l.Retries, &l.ReceivedAt, &l.ProcessedAt,
			&l.ErrorMsg, &payload,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan webhook log: %w", err)
		}
		l.Payload = payload
		logs = append(logs, l)
	}

	return logs, rows.Err()
}

func (p *PostgresService) MoveToDeadLetter(ctx context.Context, id int64) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO dead_letter (original_log_id, order_id, event, payload, error_msg)
		SELECT id, order_id, event, payload, error_msg
		FROM webhook_logs WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("failed to insert into dead letter: %w", err)
	}

	_, err = tx.ExecContext(ctx, `UPDATE webhook_logs SET status = 'dead_letter' WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to update status to dead_letter: %w", err)
	}

	return tx.Commit()
}

type pqInt64Array []int64

func (a pqInt64Array) Value() (interface{}, error) {
	return fmt.Sprintf("{%s}", strings.Trim(strings.Replace(
		fmt.Sprint(a), " ", ",", -1), "[]")), nil
}
