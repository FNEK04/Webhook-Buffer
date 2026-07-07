package models

import "time"

// WebhookLog represents webhook processing log in PostgreSQL
type WebhookLog struct {
	ID          int64     `json:"id"`
	OrderID     string    `json:"order_id"`
	Event       string    `json:"event"`
	Status      string    `json:"status"` // received, queued, processed, failed
	Retries     int       `json:"retries"`
	ReceivedAt  time.Time `json:"received_at"`
	ProcessedAt *time.Time `json:"processed_at,omitempty"`
	ErrorMsg    *string   `json:"error_msg,omitempty"`
}

// InventoryCache represents cached inventory data
type InventoryCache struct {
	SKU       string  `json:"sku"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
	CachedAt  time.Time `json:"cached_at"`
}
