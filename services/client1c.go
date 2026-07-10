package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"webhook-buffer/models"
)

type Client1C struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
	timeout    time.Duration

	mu          sync.RWMutex
	healthy     bool
	lastCheck   time.Time
	checkEvery  time.Duration
	stopCheck   chan struct{}
}

func NewClient1C(baseURL, username, password string, timeout time.Duration) *Client1C {
	c := &Client1C{
		baseURL:   baseURL,
		username:  username,
		password:  password,
		timeout:   timeout,
		checkEvery: 30 * time.Second,
		stopCheck:  make(chan struct{}),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}

	c.mu.Lock()
	c.healthy = true
	c.lastCheck = time.Now()
	c.mu.Unlock()

	go c.periodicHealthCheck()

	return c
}

func (c *Client1C) periodicHealthCheck() {
	ticker := time.NewTicker(c.checkEvery)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCheck:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
			err := c.HealthCheck(ctx)
			cancel()
			c.mu.Lock()
			c.healthy = (err == nil)
			c.lastCheck = time.Now()
			c.mu.Unlock()

			if err != nil {
				slog.Warn("1C health check failed", "error", err)
			} else {
				slog.Debug("1C health check passed")
			}
		}
	}
}

func (c *Client1C) IsHealthy() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.healthy
}

func (c *Client1C) StopHealthCheck() {
	close(c.stopCheck)
}

func (c *Client1C) SendOrder(ctx context.Context, webhook models.Webhook) error {
	url := fmt.Sprintf("%s/hs/webhook/orders", c.baseURL)

	payload, err := json.Marshal(webhook)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to 1C: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("1C returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client1C) GetInventory(ctx context.Context, sku string) (*models.InventoryCache, error) {
	url := fmt.Sprintf("%s/hs/webhook/inventory/%s", c.baseURL, sku)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory from 1C: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("1C returned status %d: %s", resp.StatusCode, string(body))
	}

	var inventory struct {
		SKU      string  `json:"sku"`
		Quantity int     `json:"quantity"`
		Price    float64 `json:"price"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&inventory); err != nil {
		return nil, fmt.Errorf("failed to decode inventory response: %w", err)
	}

	return &models.InventoryCache{
		SKU:      inventory.SKU,
		Quantity: inventory.Quantity,
		Price:    inventory.Price,
		CachedAt: time.Now(),
	}, nil
}

func (c *Client1C) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/hs/webhook/health", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to check 1C health: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("1C health check failed with status %d", resp.StatusCode)
	}

	return nil
}
