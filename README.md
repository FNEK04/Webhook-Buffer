# Webhook Buffer

High-performance Go microservice that buffers webhooks between external systems and 1C:Enterprise. Absorbs peak loads by queuing requests in Redis and transmitting to 1C in controlled batches.

## Architecture

```
External Systems ──POST──▶ Gin Router (auth, rate limit, request ID)
                                │
                                ├──▶ PostgreSQL (log as "received")
                                ├──▶ Redis List RPUSH (queue)
                                └──▶ HTTP 202 Accepted
                                
Background Worker (goroutine)
    │
    ├──▶ Redis BLPOP (dequeue batch)
    ├──▶ 1C SendOrder (POST /hs/webhook/orders)
    │       ├──▶ Success → PG status = "processed"
    │       └──▶ Failure → re-enqueue or dead_letter
    │
    └──▶ Retry Ticker (every 5 min, re-queue failed)
```

## Features

- **High-throughput webhook processing** with Redis queue
- **Batch processing** — single INSERT + Redis pipeline for 100 webhooks
- **Inventory caching** with Redis TTL
- **PostgreSQL logging** with statistics and dead letter queue
- **Automatic retries** with configurable max attempts
- **Dead letter queue** for permanently failed webhooks
- **1C health monitoring** — periodic checks, graceful degradation
- **Graceful shutdown** with context propagation
- **API key authentication** (optional)
- **Per-IP rate limiting** (configurable RPS)
- **Request ID tracking** across all components
- **Structured JSON logging** (slog)
- **Prometheus metrics** at `/metrics`
- **Health checks** with real Redis/PG/1C connectivity
- **Queue size limits** with HTTP 503 backpressure
- **Docker Compose** with healthchecks and env_file

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.25 |
| HTTP | Gin v1.12 |
| Queue | Redis 7 (list-based) |
| Storage | PostgreSQL 16 |
| Metrics | Prometheus client_golang |
| Logging | log/slog (structured JSON) |

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/webhook` | API key | Accept single webhook |
| `POST` | `/webhook/batch` | API key | Accept up to 100 webhooks |
| `GET` | `/webhook/status` | — | Queue size + stats |
| `GET` | `/inventory/:sku` | — | Get inventory (cache → 1C) |
| `DELETE` | `/inventory/:sku/cache` | — | Invalidate cache |
| `GET` | `/health` | — | Health check (Redis/PG/1C) |
| `GET` | `/metrics` | — | Prometheus metrics |

## Quick Start

### Local Development

```bash
cp .env.example .env
# Edit .env with your settings
make deps
make run
```

### Docker

```bash
cp .env.example .env
# Edit .env (especially POSTGRES_PASSWORD)
make docker-up
make docker-logs
```

### Testing

```bash
make test          # Run all tests
make test-cover    # Coverage report
make lint          # Run linter
```

## Configuration

All configuration via environment variables (see `.env.example`):

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_PORT` | 8080 | HTTP port |
| `REDIS_ADDR` | localhost:6379 | Redis address |
| `REDIS_PASSWORD` | (empty) | Redis password |
| `POSTGRES_CONN_STRING` | — | PostgreSQL DSN |
| `ONEC_BASE_URL` | http://localhost:8081 | 1C base URL |
| `ONEC_USERNAME` | admin | 1C auth username |
| `ONEC_PASSWORD` | password | 1C auth password |
| `WORKER_BATCH_SIZE` | 10 | Webhooks per batch |
| `WORKER_POLL_INTERVAL` | 1s | Queue poll interval |
| `WORKER_MAX_RETRIES` | 5 | Max retry attempts |
| `WORKER_CACHE_TTL` | 5m | Inventory cache TTL |
| `WORKER_QUEUE_MAX_SIZE` | 100000 | Queue size limit |
| `API_KEY` | (empty) | API key for auth (empty = disabled) |
| `RATE_LIMIT_RPS` | 1000 | Requests per second |
| `RATE_LIMIT_BURST` | 2000 | Burst capacity |

## 1C Integration

Expects 1C HTTP services:
- `POST /hs/webhook/orders` — Accept order data
- `GET /hs/webhook/inventory/:sku` — Return inventory
- `GET /hs/webhook/health` — Health check

Authentication: HTTP Basic Auth.

## Webhook Format

```json
{
  "event": "order.created",
  "timestamp": "2026-07-07T20:15:30Z",
  "payload": {
    "order_id": "WEB-99812",
    "status": "new",
    "payment_status": "paid",
    "payment_method": "card_online",
    "customer": {
      "phone": "+79991234567",
      "email": "ivanov@example.ru",
      "first_name": "Иван",
      "last_name": "Иванов"
    },
    "delivery": {
      "method": "cdek",
      "address": "г. Москва, ул. Ленина, д. 10, кв. 25",
      "cost": 350.00
    },
    "items": [
      {
        "sku": "SKU-DIR-412",
        "name": "Дрель-шуруповерт Bosch GSR 120-LI",
        "quantity": 1,
        "price": 6490.00,
        "discount": 490.00
      }
    ],
    "total_amount": 7900.00
  }
}
```

## Metrics

Prometheus metrics at `GET /metrics`:

| Metric | Type | Description |
|--------|------|-------------|
| `webhook_buffer_received_total` | Counter | Webhooks received (by endpoint) |
| `webhook_buffer_processed_total` | Counter | Webhooks processed (by status) |
| `webhook_buffer_queue_size` | Gauge | Current queue size |
| `webhook_buffer_request_duration_seconds` | Histogram | Request latency |

## Project Structure

```
├── main.go              # Entry point, router, graceful shutdown
├── config/              # Configuration loading
├── handlers/            # HTTP handlers (webhook, inventory)
├── middleware/           # Auth, rate limit, request ID, logging
├── models/              # Data models
├── mocks/               # Test mocks (interface-based)
├── services/            # Redis, PostgreSQL, 1C client
├── worker/              # Background queue processor
├── Makefile             # Build, test, lint commands
├── Dockerfile           # Multi-stage build
├── docker-compose.yml   # Full stack orchestration
└── .golangci.yml        # Linter configuration
```

## License

MIT
