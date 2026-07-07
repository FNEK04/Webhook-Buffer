package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"webhook-buffer/models"
)

type Client1C struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
	timeout    time.Duration
}

func NewClient1C(baseURL, username, password string, timeout time.Duration) *Client1C {
	return &Client1C{
		baseURL:  baseURL,
		username: username,
		password: password,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

// SendOrder sends order data to 1C via REST API
func (c *Client1C) SendOrder(webhook models.Webhook) error {
	url := fmt.Sprintf("%s/hs/webhook/orders", c.baseURL)

	payload, err := json.Marshal(webhook)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
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

// GetInventory retrieves inventory data from 1C
func (c *Client1C) GetInventory(sku string) (*models.InventoryCache, error) {
	url := fmt.Sprintf("%s/hs/webhook/inventory/%s", c.baseURL, sku)

	req, err := http.NewRequest("GET", url, nil)
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

// HealthCheck checks if 1C service is available
func (c *Client1C) HealthCheck() error {
	url := fmt.Sprintf("%s/hs/webhook/health", c.baseURL)

	req, err := http.NewRequest("GET", url, nil)
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
