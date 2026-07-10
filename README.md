# Webhook Buffer

Высокопроизводительный Go-микросервис, который буферизирует вебхуки между внешними системами и 1С:Предприятие. Поглощает пиковые нагрузки, ставя запросы в очередь Redis и передавая данные в 1С контролируемыми батчами.

## Архитектура

```
Внешние системы ──POST──▶ Gin Router (аутентификация, rate limit, request ID)
                                │
                                ├──▶ PostgreSQL (лог как "received")
                                ├──▶ Redis List RPUSH (очередь)
                                └──▶ HTTP 202 Accepted
                                
Фоновый Worker (горутина)
    │
    ├──▶ Redis BLPOP (диз batch)
    ├──▶ 1C SendOrder (POST /hs/webhook/orders)
    │       ├──▶ Успех → PG статус = "processed"
    │       └──▶ Ошибка → re-enqueue или dead_letter
    │
    └──▶ Retry Ticker (каждые 5 мин, повторная отправка ошибочных)
```

## Возможности

- **Высокопроизводительная обработка вебхуков** через очередь Redis
- **Батч-обработка** — один INSERT + Redis pipeline на 100 вебхуков
- **Кеширование инвентаря** с TTL в Redis
- **Логирование в PostgreSQL** со статистикой и dead letter queue
- **Автоматические ретраи** с настраиваемым количеством попыток
- **Dead letter queue** для перманентно ошибочных вебхуков
- **Мониторинг доступности 1C** — периодические проверки, graceful degradation
- **Graceful shutdown** с propagation context
- **Аутентификация по API key** (опционально)
- **Rate limiting по IP** (настраиваемый RPS)
- **Request ID** отслеживается во всех компонентах
- **Структурированный JSON-логинг** (slog)
- **Prometheus метрики** на `/metrics`
- **Health checks** с реальной проверкой Redis/PG/1C
- **Лимит размера очереди** с HTTP 503 backpressure
- **Docker Compose** с healthchecks и env_file

## Стек технологий

| Компонент | Технология |
|-----------|-----------|
| Язык | Go 1.25 |
| HTTP | Gin v1.12 |
| Очередь | Redis 7 (list-based) |
| Хранилище | PostgreSQL 16 |
| Метрики | Prometheus client_golang |
| Логирование | log/slog (structured JSON) |

## API Эндпоинты

| Метод | Путь | Авторизация | Описание |
|-------|------|-------------|----------|
| `POST` | `/webhook` | API key | Принять один вебхук |
| `POST` | `/webhook/batch` | API key | Принять до 100 вебхуков |
| `GET` | `/webhook/status` | — | Размер очереди + статистика |
| `GET` | `/inventory/:sku` | — | Получить инвентарь (кеш → 1C) |
| `DELETE` | `/inventory/:sku/cache` | — | Инвалидировать кеш |
| `GET` | `/health` | — | Проверка здоровья (Redis/PG/1C) |
| `GET` | `/metrics` | — | Prometheus метрики |

## Быстрый старт

### Локальная разработка

```bash
cp .env.example .env
# Отредактируйте .env под ваши настройки
make deps
make run
```

### Docker

```bash
cp .env.example .env
# Отредактируйте .env (особенно POSTGRES_PASSWORD)
make docker-up
make docker-logs
```

### Тестирование

```bash
make test          # Запустить все тесты
make test-cover    # Отчёт покрытия
make lint          # Запустить линтер
```

## Конфигурация

Вся конфигурация через переменные окружения (см. `.env.example`):

| Переменная | По умолчанию | Описание |
|------------|-------------|----------|
| `SERVER_PORT` | 8080 | HTTP порт |
| `REDIS_ADDR` | localhost:6379 | Адрес Redis |
| `REDIS_PASSWORD` | (пусто) | Пароль Redis |
| `POSTGRES_CONN_STRING` | — | DSN PostgreSQL |
| `ONEC_BASE_URL` | http://localhost:8081 | Базовый URL 1С |
| `ONEC_USERNAME` | admin | Логин 1С |
| `ONEC_PASSWORD` | password | Пароль 1С |
| `WORKER_BATCH_SIZE` | 10 | Вебхуков на батч |
| `WORKER_POLL_INTERVAL` | 1s | Интервал опроса очереди |
| `WORKER_MAX_RETRIES` | 5 | Макс. попыток повтора |
| `WORKER_CACHE_TTL` | 5m | TTL кеша инвентаря |
| `WORKER_QUEUE_MAX_SIZE` | 100000 | Лимит очереди |
| `API_KEY` | (пусто) | API ключ (пусто = отключено) |
| `RATE_LIMIT_RPS` | 1000 | Запросов в секунду |
| `RATE_LIMIT_BURST` | 2000 | Буст-ёмкость |

## Интеграция с 1С

Ожидает HTTP-сервисы 1С:
- `POST /hs/webhook/orders` — Приём данных заказа
- `GET /hs/webhook/inventory/:sku` — Возврат инвентаря
- `GET /hs/webhook/health` — Проверка доступности

Аутентификация: HTTP Basic Auth.

## Формат вебхука

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

## Метрики

Prometheus метрики на `GET /metrics`:

| Метрика | Тип | Описание |
|---------|-----|----------|
| `webhook_buffer_received_total` | Counter | Получено вебхуков (по эндпоинту) |
| `webhook_buffer_processed_total` | Counter | Обработано вебхуков (по статусу) |
| `webhook_buffer_queue_size` | Gauge | Текущий размер очереди |
| `webhook_buffer_request_duration_seconds` | Histogram | Латентность запросов |

## Структура проекта

```
├── main.go              # Точка входа, роутер, graceful shutdown
├── config/              # Загрузка конфигурации
├── handlers/            # HTTP-хендлеры (вебхук, инвентарь)
├── middleware/           # Аутентификация, rate limit, request ID, логирование
├── models/              # Модели данных
├── mocks/               # Тестовые моки (на интерфейсах)
├── services/            # Redis, PostgreSQL, клиент 1С
├── worker/              # Фоновый обработчик очереди
├── Makefile             # Команды сборки, тестов, линтера
├── Dockerfile           # Multi-stage сборка
├── docker-compose.yml   # Оркестрация всего стека
└── .golangci.yml        # Конфигурация линтера
```

## Лицензия

MIT
