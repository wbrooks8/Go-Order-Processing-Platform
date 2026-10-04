# Order Processing Platform

An educational Go project growing toward a Kafka-based order processing system with separate Inventory, Payment, Fulfillment, and Notification services. The [architecture and roadmap](docs/architecture.md) describe that destination; this README describes what runs today.

## What works today

- An Order API built with Gin, PostgreSQL, and GORM: create, retrieve, and cancel orders.
- Domain validation, UUID validation at the API boundary, and repeatable cancellation.
- HTTP request counters and duration histograms at `/metrics`.
- Prometheus scraping the API and a provisioned Grafana Prometheus data source.
- A standalone Kafka producer and consumer demo, using the `orders` topic.
- Automated domain, service, HTTP, and persistence tests; database integration tests are opt-in.

**HTTP order creation does not publish Kafka events yet.** The demo producer generates sample events independently of the API and database. Inventory, payment, fulfillment, the outbox, and Saga coordination are future work.

```text
Client → Order API → PostgreSQL
             ↑
         Prometheus scrapes /metrics
             ↑
         Grafana queries Prometheus

Demo producer → Kafka topic: orders → Demo consumer
```

## Repository map

```text
.
├── docker_compose.yml              # PostgreSQL, Kafka, Prometheus, Grafana
├── prometheus/prometheus.yml       # Scrape targets
├── grafana/datasource.yml           # Grafana's Prometheus connection
├── docs/
│   ├── architecture.md             # Target system and staged roadmap
│   ├── architecture-original.md    # Preserved original design proposal
│   └── kafka-reference.md          # Kafka commands and troubleshooting
└── orderApiGateway/                # Current Go module and Order Service
    ├── cmd/
    │   ├── server/main.go          # HTTP API executable
    │   ├── producer/main.go        # Kafka demo executable
    │   └── consumer/main.go        # Kafka demo executable
    ├── internal/
    │   ├── adapters/http/          # Routes, JSON conversion, handlers, metrics
    │   ├── adapters/postgres/      # Database operations and models
    │   ├── config/                 # Database connection settings
    │   ├── domain/                 # Order data, validation, cancellation rules
    │   ├── ports/                  # Repository interface
    │   └── service/                # Coordinates order operations
    └── kafka/
        ├── events/                # Shared demo message definition
        ├── producer/              # Writes sample messages
        ├── consumer/              # Decodes and logs messages
        └── config/                # Demo broker/topic settings
```

The existing folder name `orderApiGateway` is retained, but its current role is the Order Service. Future service folders will be introduced when their implementations begin.

## Start the API

Prerequisites: a Go toolchain compatible with `orderApiGateway/go.mod` (currently `go 1.27.1`), Docker, and Docker Compose.

From the repository root:

```bash
docker compose -f docker_compose.yml up -d --wait postgres
cd orderApiGateway
go run ./cmd/server
```

The API listens on `http://localhost:8080`. Ctrl+C drains active requests for up to ten seconds before closing the database. On startup, GORM prepares the order tables using `AutoMigrate`.

Money is represented exactly as integer ten-thousandths of a dollar in Go. JSON still uses decimal numbers (for example `12.5000`) and SQL retains `decimal(12,4)`, so existing decimal columns need no conversion. Excess precision and amounts beyond that column range are rejected.

The default database connection matches Compose: host `localhost`, port `5432`, database `app`, user `app`, password `localdev`. Override it before starting the API when needed:

```bash
export DATABASE_DSN='host=localhost user=app password=localdev dbname=app port=5432 sslmode=disable'
```

These are local development credentials. Configuration currently uses environment variables; no YAML application configuration is loaded.

## Try the API

| Method | Path | Result |
| --- | --- | --- |
| POST | `/api/v1/orders` | Creates an order; returns 202 |
| GET | `/api/v1/orders/:id` | Retrieves an order; returns 200 |
| POST | `/api/v1/orders/:id/cancel` | Cancels an accepted order; returns 200 |
| GET | `/metrics` | Prometheus text metrics |

From a second terminal:

```bash
curl -i http://localhost:8080/api/v1/orders \
  -H 'Content-Type: application/json' \
  -d '{
    "customerId": "d8f3b2a1-0000-4a8a-8e2b-123456789abc",
    "items": [{
      "productId": "a1b2c3d4-e5f6-4a8b-9c0d-112233445566",
      "sku": "SKU-001",
      "quantity": 2,
      "unitPrice": 12.50
    }],
    "shippingAddress": {
      "street": "123 Main Street", "city": "Austin", "state": "TX",
      "postalCode": "78701", "country": "US"
    },
    "billingAddress": {
      "street": "123 Main Street", "city": "Austin", "state": "TX",
      "postalCode": "78701", "country": "US"
    }
  }'
```

Copy the returned `orderId` into this variable:

```bash
ORDER_ID='replace-with-the-returned-UUID'
curl -i "http://localhost:8080/api/v1/orders/$ORDER_ID"
curl -i -X POST "http://localhost:8080/api/v1/orders/$ORDER_ID/cancel"
curl -i "http://localhost:8080/api/v1/orders/$ORDER_ID"
```

New orders have status `Accepted`. Cancellation changes it to `Cancelled`; cancelling again succeeds with that same status. Invalid inputs and malformed UUIDs return 400, missing orders return 404, and disallowed cancellation or a concurrent-update conflict returns 409. Each successful update increments `version`; a stale writer must reload before retrying. Authentication and payment processing are not implemented. Prices are currently supplied by the caller for this learning API.

## Tests

From `orderApiGateway`:

```bash
go test ./...
```

PostgreSQL integration tests **skip** unless `TEST_DATABASE_DSN` is set. With the local PostgreSQL service running:

```bash
export TEST_DATABASE_DSN='host=localhost user=app password=localdev dbname=app port=5432 sslmode=disable'
go test ./internal/adapters/postgres -v -count=1
```

Integration tests prepare tables and create/delete their own test orders. Use a development or dedicated test database. Kafka unit tests cover message encoding, cancellation, retries, dead-letter handling, and acknowledgement ordering without a broker. An opt-in integration test creates and removes temporary topics and uses an isolated group on a local single-broker Kafka:

```bash
TEST_KAFKA_BROKER=localhost:9092 go test ./kafka/consumer -run TestKafkaIntegration -v -count=1
```

## Metrics and dashboards

From the repository root, with the API also running on your computer:

```bash
docker compose -f docker_compose.yml up -d prometheus grafana
```

- API metrics: http://localhost:8080/metrics
- Prometheus targets: http://localhost:9090/targets — look for `order-api` showing UP.
- Grafana: http://localhost:3000 — the Prometheus data source is provisioned from the repository.

Prometheus reaches the host API through `host.docker.internal:8080`. Grafana reaches Prometheus through `http://prometheus:9090` on the Compose network.

Request count metric: `http_requests_total` (method, route pattern, status).
Duration metric: `http_request_duration_seconds` (method, route pattern).

Example requests-per-second query:

```promql
sum(rate(http_requests_total{job="order-api",route!="/metrics"}[5m]))
```

Dashboards created in Grafana are stored in its Docker volume. Dashboard exports are not yet included in the repository.

## Kafka learning demo

Use the [Kafka reference](docs/kafka-reference.md) to start Kafka and create both `orders` and `orders.dlq`. Then, from `orderApiGateway`, run in separate terminals:

```bash
go run ./cmd/consumer
```

```bash
go run ./cmd/producer
```

The producer sends ten JSON messages with generated UUIDs. The consumer uses group `order-workers`, checks JSON and required IDs, and logs valid messages. The shared `OrderCreatedEvent` is a demo payload containing `orderId` and `customerId`; it is not the full domain order or final event contract.

Ctrl+C stops the demo commands and closes their Kafka connections. The consumer fetches without acknowledging, processes sequentially, and commits only after success or confirmed dead-letter delivery. Invalid JSON/missing IDs go to `orders.dlq`; processing failures get three attempts with backoff before dead-lettering. Failed fetches, dead-letter writes, or commits stop the command so it can be restarted without skipping later offsets. The processor currently only logs; it has no inventory side effects yet. Duplicate processing and duplicate dead letters remain possible after a crash, so real consumers will still need idempotency. Kafka is advertised at `localhost:9092` for host applications and tools inside the broker container; other service containers will need an internal advertised listener.

## Where this is going

The next steps connect real order creation to Kafka, introduce an Inventory Service, then Payment, Fulfillment, and Notification services. The target includes service-owned data, reliable event publishing with an outbox, idempotent consumers, compensation for failures, and observability across services.

See the [architecture and roadmap](docs/architecture.md) for the full plan and the [Order Service guide](orderApiGateway/README.md) for how today's code fits together.
