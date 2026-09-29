# Order Processing Platform

An educational Go microservices project for learning REST APIs, PostgreSQL, event-driven architecture, Kafka, goroutines, Prometheus, Grafana, and distributed order processing.

The project is being built incrementally. The Order API Gateway is the first working service. Inventory, Payment, Fulfillment, Notification, Kafka, and observability are planned next.

## Architecture

```text
Client
  |
  v
Order API Gateway (Gin / Go)
  |\
  | \-- PostgreSQL: orders and order items
  |
  \---- Kafka: order.created
             |
             +--> Inventory Service ---- inventory.reserved / inventory.failed
             |          |
             |          \--> Inventory PostgreSQL database
             |
             +--> Payment Service <---- inventory.reserved
             |          |
             |          \---- payment.processed / payment.failed
             |
             +--> Fulfillment Service <- payment.processed
             |          |
             |          \---- shipping.label_created
             |
             \--> Notification Service
                        \---- email / SMS / push notifications

Prometheus scrapes service metrics and Grafana visualizes them.
```

### Service responsibilities

| Service | Responsibility | Planned data store |
| --- | --- | --- |
| Order API Gateway | Accept orders, expose order status, own the order lifecycle | PostgreSQL |
| Inventory Service | Reserve and release stock | PostgreSQL |
| Payment Service | Authorize payment through a payment provider | PostgreSQL |
| Fulfillment Service | Create shipments and shipping labels | PostgreSQL |
| Notification Service | Consume events and notify customers | Stateless or Redis |

Each service owns its data. Services communicate through Kafka events instead of reading another service's database directly. This makes the workflow eventually consistent and allows each service to scale independently.

## Order Service Design

The Order Service follows a ports-and-adapters layout:

```text
HTTP adapter (Gin)
        |
        v
Order use cases
        |
        v
Domain model and order state rules
        |
        +--> Repository port --> PostgreSQL adapter
        +--> Event publisher port --> Kafka adapter (planned)
```

### Order lifecycle

```text
Accepted
   |
   +--> Inventory reserved --> Payment processed --> Fulfilled
   |
   +--> Inventory failed -------------------------> Cancelled
   |
   +--> Payment failed --> Release inventory ------> Cancelled
```

The eventual production design uses a transactional outbox: the order and its `order.created` event are written in one database transaction, then an outbox worker publishes the event to Kafka. This prevents an order from being saved successfully while its event is lost.

### Current Order API endpoints

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/api/v1/orders` | Create an order |
| `GET` | `/api/v1/orders/:id` | Retrieve an order |
| `POST` | `/api/v1/orders/:id/cancel` | Cancel an order |

The current service runs on `http://localhost:8080` and uses PostgreSQL. Authentication, metrics, health checks, Kafka publishing, and the outbox are planned capabilities.

## Repository Structure

```text
.
├── docker_compose.yml                 # Local PostgreSQL environment
├── README.md
├── .gitignore
├── orderApiGateway/                    # Implemented Go service
│   ├── cmd/server/main.go              # Application entry point and wiring
│   ├── internal/
│   │   ├── adapters/http/              # Gin handlers, DTOs, and routes
│   │   ├── adapters/postgres/          # GORM repository and models
│   │   ├── config/                     # Database connection setup
│   │   ├── domain/                     # Order aggregate and domain errors
│   │   ├── ports/                      # Repository interfaces
│   │   └── service/                    # Order use cases
│   ├── go.mod
│   └── README.md
├── inventoryService/                  # Planned Inventory Service
├── paymentService/                    # Planned Payment Service
└── fulfullmentService/                # Planned Fulfillment Service
```

The `fulfullmentService` directory keeps the existing repository name. It can be renamed to `fulfillmentService` later when the service is implemented.

## Technology Stack

- Go and Gin for HTTP services
- PostgreSQL for transactional service-owned data
- GORM for the current Order API persistence layer
- Apache Kafka for asynchronous service communication
- Goroutines and channels for concurrent workers
- Prometheus for metrics
- Grafana for dashboards
- Docker Compose for local infrastructure
- OpenTelemetry and Jaeger for planned distributed tracing

## Getting Started

### Prerequisites

- Go 1.27 or newer, as declared by `orderApiGateway/go.mod`
- Docker and Docker Compose

### Start PostgreSQL

From the repository root:

```bash
docker compose -f docker_compose.yml up -d postgres
```

The local database is available with:

```text
host: localhost
port: 5432
database: app
user: app
password: localdev
```

### Run the Order API

```bash
cd orderApiGateway
go run ./cmd/server
```

The service starts at `http://localhost:8080`.

### Try the API

Create an order:

```bash
curl -X POST http://localhost:8080/api/v1/orders \
  -H 'Content-Type: application/json' \
  -d '{
    "customerId": "customer-123",
    "items": [
      {
        "productId": "product-123",
        "sku": "SKU-001",
        "quantity": 1,
        "unitPrice": 1299.99
      }
    ],
    "shippingAddress": {
      "street": "123 Main Street",
      "city": "Austin",
      "state": "TX",
      "postalCode": "78701",
      "country": "US"
    },
    "billingAddress": {
      "street": "123 Main Street",
      "city": "Austin",
      "state": "TX",
      "postalCode": "78701",
      "country": "US"
    }
  }'
```

Retrieve an order by replacing the ID with the value returned by the create request:

```bash
curl http://localhost:8080/api/v1/orders/<order-id>
```

### Run tests

```bash
cd orderApiGateway
go test ./...
```

Integration tests require the PostgreSQL container to be running.

## Learning Roadmap

Build the system in vertical slices so every milestone produces something runnable:

1. **In-memory HTTP API:** Learn Gin routing, JSON binding, validation, and response handling.
2. **PostgreSQL persistence:** Learn database connections, models, repositories, and migrations.
3. **Prometheus metrics:** Add request, order, database, and runtime metrics plus `/metrics`.
4. **Kafka events:** Publish `order.created` after persistence and learn event contracts.
5. **Inventory worker:** Build a Kafka consumer and a bounded goroutine worker pool.
6. **Payment and fulfillment:** Add downstream consumers and model the order Saga.
7. **Reliability:** Add idempotency, retries, dead-letter topics, graceful shutdown, and the transactional outbox.
8. **Observability:** Add Prometheus dashboards, structured logs, and distributed tracing.
9. **Production packaging:** Add Dockerfiles, CI, health/readiness endpoints, and Kubernetes manifests.

## Event Contracts

The planned event flow is:

| Event | Producer | Consumers |
| --- | --- | --- |
| `order.created` | Order Service | Inventory, Notification |
| `inventory.reserved` | Inventory Service | Order, Payment |
| `inventory.failed` | Inventory Service | Order, Notification |
| `payment.processed` | Payment Service | Order, Fulfillment, Notification |
| `payment.failed` | Payment Service | Order, Notification |
| `shipping.label_created` | Fulfillment Service | Notification |

Events should include an event ID, event type, timestamp, aggregate/order ID, and a versioned payload. Consumers must be idempotent because Kafka delivery can be repeated.

## Reliability and Observability Goals

- Use an idempotency key to prevent duplicate order creation.
- Use the transactional outbox for reliable event publication.
- Use retries with backoff for temporary failures.
- Route repeatedly failing messages to dead-letter topics.
- Use database transactions and row locking when reserving inventory.
- Expose health and readiness endpoints for orchestration.
- Track HTTP latency, order outcomes, Kafka consumer lag, worker throughput, and database latency.
- Propagate correlation and trace IDs across HTTP and Kafka messages.

## Project Status

| Area | Status |
| --- | --- |
| Order API structure and domain model | In progress |
| Gin HTTP endpoints | Implemented |
| PostgreSQL persistence | Implemented |
| Unit and repository tests | Implemented |
| Kafka | Planned |
| Inventory, Payment, Fulfillment, Notification services | Planned |
| Prometheus and Grafana | Planned |
| Transactional outbox and Saga orchestration | Planned |
| Docker builds for services | Planned |

## License

This project is for learning and experimentation.
