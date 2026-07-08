package models

import "time"

// Webhook represents the incoming webhook structure
type Webhook struct {
	Event     string    `json:"event" binding:"required"`
	Timestamp time.Time `json:"timestamp" binding:"required"`
	Payload   Payload   `json:"payload" binding:"required"`
}

// Payload contains the order data
type Payload struct {
	OrderID        string    `json:"order_id" binding:"required"`
	Status         string    `json:"status" binding:"required"`
	PaymentStatus  string    `json:"payment_status" binding:"required"`
	PaymentMethod  string    `json:"payment_method" binding:"required"`
	Customer       Customer  `json:"customer" binding:"required"`
	Delivery       Delivery  `json:"delivery" binding:"required"`
	Items          []Item    `json:"items" binding:"required,min=1"`
	TotalAmount    float64   `json:"total_amount" binding:"required"`
}

// Customer represents customer information
type Customer struct {
	Phone     string `json:"phone" binding:"required"`
	Email     string `json:"email" binding:"required,email"`
	FirstName string `json:"first_name" binding:"required"`
	LastName  string `json:"last_name" binding:"required"`
}

// Delivery represents delivery information
type Delivery struct {
	Method string  `json:"method" binding:"required"`
	Address string `json:"address" binding:"required"`
	Cost   float64 `json:"cost" binding:"min=0"`
}

// Item represents order item
type Item struct {
	SKU      string  `json:"sku" binding:"required"`
	Name     string  `json:"name" binding:"required"`
	Quantity int     `json:"quantity" binding:"required,min=1"`
	Price    float64 `json:"price" binding:"required,min=0"`
	Discount float64 `json:"discount" binding:"min=0"`
}

// QueueItem represents item in Redis queue
type QueueItem struct {
	Webhook  Webhook `json:"webhook"`
	Attempts int     `json:"attempts"`
	Received time.Time `json:"received"`
	LogID    int64    `json:"log_id"`
}
