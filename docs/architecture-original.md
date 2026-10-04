# Original architecture proposal (planning reference)

This preserves the original design, including its illustrative schemas and folder layouts. It is a proposal, not a description of implemented behavior or runnable setup instructions. See [the current roadmap](architecture.md) for the staged plan and [the root README](../README.md) for commands that match this repository.

---

# Order Processing Platform

An enterprise-grade, event-driven microservices platform built for high-throughput e-commerce order ingestion, processing, and fulfillment.

## System Architecture

The platform combines Event-Driven Architecture (EDA) with Domain-Driven Design (DDD) to support high availability, reliable order handling, and horizontal scalability under peak load.

```text
+---------------------------------------------------------------------------------------+
|                                    Client Layer                                       |
|                       (Web App / Mobile App / Third-Party APIs)                       |
+---------------------------------------------------------------------------------------+
                                           |
                                           v
+---------------------------------------------------------------------------------------+
|                                  API Gateway (Kong / Envoy)                           |
|                      - AuthN/AuthZ  - Rate Limiting  - SSL Offloading                 |
+---------------------------------------------------------------------------------------+
                                           |
                   +-----------------------+-----------------------+
                   |                                               |
                   v                                               v
+--------------------------------------+       +----------------------------------------+
|          Order Service (Core)        |       |            Inventory Service           |
|  - REST/gRPC Interface               |       |  - Stock Reservation                   |
|  - Saga Orchestrator                 |       |  - Warehouse Routing                   |
|  - Outbox Publisher                  |       |                                        |
+--------------------------------------+       +----------------------------------------+
         |                    |                            |
  (Read/Write DB)        (Cache)                      (Read/Write DB)
         v                    v                            v
  +--------------+    +---------------+            +--------------+
  | PostgreSQL   |    | Redis Cluster |            | PostgreSQL   |
  | (Order DB)   |    | (Order Cache) |            | (Inv DB)     |
  +--------------+    +---------------+            +--------------+
         |
         +-----------------+ (Transactional Outbox Poller)
                           v
+---------------------------------------------------------------------------------------+
|                               Apache Kafka Event Bus                                  |
|   Topics: order.created | order.paid | order.Cancelled | inventory.reserved | ...     |
+---------------------------------------------------------------------------------------+
         |                                         |
         v                                         v
+--------------------------------------+       +----------------------------------------+
|           Payment Service            |       |          Notification Service          |
|  - Stripe / PayPal Adapters          |       |  - Email / SMS / Push                  |
|  - Payment Saga Participant          |       |  - Event Consumer                      |
+--------------------------------------+       +----------------------------------------+
```

## Order Service Architecture

The Order Service orchestrates user checkouts. It follows Hexagonal Architecture (Ports and Adapters) to decouple business logic from databases, messaging brokers, and transport layers.

### Service Topology

```text
                         [ REST API / gRPC Controllers ]
                                       |
                                       v
                     +-----------------------------------+
                     |     Application / Use Cases       |
                     |  - CreateOrderUseCase             |
                     |  - CancelOrderUseCase             |
                     |  - ProcessPaymentCallbackUseCase  |
                     +-----------------------------------+
                                       |
                                       v
                     +-----------------------------------+
                     |       Domain Model Core           |
                     |  - Order Aggregate Root           |
                     |  - OrderItem Entity               |
                     |  - OrderStatus State Machine      |
                     |  - Value Objects (Money, Address) |
                     +-----------------------------------+
                                       |
        +------------------------------+------------------------------+
        |                              |                              |
        v                              v                              v
[ DB Repository Adapter ]   [ Event Outbox Adapter ]   [ Payment Client Adapter ]
 (PostgreSQL Persistence)     (Kafka Publisher)            (gRPC Payment Proxy)
```

### Order State Machine

The order lifecycle is a finite state machine enforced at the Aggregate Root level.

```text
[ CREATED ] ---> (Inventory Reserved) ---> [ PENDING_PAYMENT ]
     |                                             |
     | (Stock Unavailable)                         | (Payment Success)
     v                                             v
[ Cancelled ]                                  [ PAID ]
                                                   |
                                                   v
                                     [ FULFILLMENT_IN_PROGRESS ]
                                                   |
                                                   v
                                              [ SHIPPED ] ---> [ DELIVERED ]
```

## Data Architecture and Persistence

### PostgreSQL Schema

Orders are stored in PostgreSQL with the Transactional Outbox Pattern to prevent dual-write inconsistencies during order creation.

```sql
-- Orders Table
CREATE TABLE orders (
    order_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL,
    status VARCHAR(32) NOT NULL,
    total_amount DECIMAL(12, 4) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    shipping_address JSONB NOT NULL,
    billing_address JSONB NOT NULL,
    version INT NOT NULL DEFAULT 1, -- Optimistic Locking
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Order Items Table
CREATE TABLE order_items (
    item_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(order_id) ON DELETE CASCADE,
    product_id UUID NOT NULL,
    sku VARCHAR(64) NOT NULL,
    quantity INT NOT NULL CHECK (quantity > 0),
    unit_price DECIMAL(12, 4) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Transactional Outbox Table for Reliable Event Publishing
CREATE TABLE outbox_events (
    event_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type VARCHAR(64) NOT NULL,
    aggregate_id VARCHAR(64) NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'PENDING', -- PENDING, PUBLISHED, FAILED
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    processed_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_orders_customer ON orders(customer_id);
CREATE INDEX idx_outbox_pending ON outbox_events(status, created_at) WHERE status = 'PENDING';
```

## Distributed Transactions

The Order Service manages multi-service workflows using an Orchestrated Saga Pattern via Kafka.

### Successful Order Workflow

```text
Client         Order Service        Inventory Service       Payment Service       Outbox Relay
  |                  |                      |                      |                    |
  |--- POST /orders->|                      |                      |                    |
  |                  |-- Save Order & Outbox|                      |                    |
  |<-- 202 Accepted--|   (Local Transaction)|                      |                    |
  |                  |                      |                      |                    |
  |                  |----------------------------------------------------------------->|
  |                  |                      |                      |   Polls & Publishes|
  |                  |                      |<--- order.created ------------------------|
  |                  |                      |                      |                    |
  |                  |                      |-- Reserve Inventory -|                    |
  |                  |                      |-- Publish Success -->|                    |
  |                  |<-- inventory.reserved ----------------------|                    |
  |                  |                      |                      |                    |
  |                  |-- Request Payment ------------------------->|                    |
  |                  |                      |                      |-- Charge Card ----|
  |                  |                      |                      |-- Publish Paid -->|
  |                  |<-- payment.success ----------------------------------------------|
  |                  |                      |                      |                    |
  |                  |-- Mark Order PAID --|                      |                    |
```

### Rollback and Compensation

If the Payment Service fails during an active transaction:

1. The Payment Service emits `payment.failed`.
2. The Order Service catches the event and updates the status to `CANCEL_PENDING`.
3. The Order Service emits `inventory.release_reservation`.
4. The Inventory Service releases reserved stock and responds with `inventory.released`.
5. The Order Service updates the status to `Cancelled`.

## API Documentation

### Create Order

| Property | Value |
| --- | --- |
| URL | `/api/v1/orders` |
| Method | `POST` |
| Headers | `Content-Type: application/json`, `Authorization: Bearer <JWT>` |

#### Request Payload

```json
{
  "customerId": "d8f3b2a1-0000-4a8a-8e2b-123456789abc",
  "items": [
    {
      "productId": "a1b2c3d4-e5f6-7a8b-9c0d-112233445566",
      "sku": "PROD-LPT-001",
      "quantity": 1,
      "unitPrice": 1299.99
    }
  ],
  "shippingAddress": {
    "street": "123 Tech Boulevard",
    "city": "Austin",
    "state": "TX",
    "postalCode": "78701",
    "country": "USA"
  },
  "billingAddress": {
    "street": "123 Tech Boulevard",
    "city": "Austin",
    "state": "TX",
    "postalCode": "78701",
    "country": "USA"
  }
}
```

#### Response Payload (`202 Accepted`)

```json
{
  "orderId": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "status": "CREATED",
  "totalAmount": 1299.99,
  "currency": "USD",
  "createdAt": "2026-09-29T13:45:00.000Z",
  "_links": {
    "self": "/api/v1/orders/3fa85f64-5717-4562-b3fc-2c963f66afa6",
    "cancel": "/api/v1/orders/3fa85f64-5717-4562-b3fc-2c963f66afa6/cancel"
  }
}
```

## Technical Stack

| Component | Technology | Description |
| --- | --- | --- |
| Language and Framework | Go / Java Spring Boot / Node.js NestJS | Core execution environment |
| Primary Database | PostgreSQL 16+ | ACID transactional state storage |
| Cache and Distributed Locks | Redis 7.x | Order idempotency locks and read caching |
| Event Broker | Apache Kafka 3.x | Event streams and asynchronous IPC |
| Containerization | Docker and Kubernetes | Orchestration and runtime |
| Observability | OpenTelemetry, Prometheus, Jaeger, Grafana | Tracing, metrics, and logs |

## Resiliency Strategies

- **Idempotent ingestion:** Every incoming order request requires an `X-Idempotency-Key` header stored in Redis with a 24-hour TTL to prevent duplicate order generation.
- **Circuit breaking:** Outbound calls, such as the Payment Proxy, use resilience patterns (Resilience4j/gRPC retries) with fallback behavior.
- **Dead letter queues (DLQ):** Kafka events that fail processing after three exponential-backoff attempts are routed to `order.events.dlq` for operator inspection.

## Local Development and Setup

### Prerequisites

- Docker Desktop 24.0+
- Docker Compose v2.20+
- `kubectl` / Helm (for local Kubernetes testing)

### Quickstart

Clone the repository:

```bash
git clone https://github.com/organization/order-platform.git
cd order-platform
```

Start local dependencies (Postgres, Redis, Kafka, and ZooKeeper):

```bash
docker-compose up -d postgres redis kafka zookeeper
```

Run database migrations:

```bash
make migrate-up
```

Start the Order Service locally:

```bash
make run-order-service
```

## Telemetry and Monitoring

### Prometheus Metrics

Metrics are exposed at `:9090/metrics`.

```text
order_creation_requests_total{status="202|400|500"}
saga_execution_duration_seconds_bucket
```

### Distributed Tracing

OpenTelemetry instrumentation correlates incoming HTTP/gRPC contexts through Kafka headers to downstream consumer services.

## File Structure ##
order-service/
├── api/
│   └── openapi/
│       └── order-v1.yaml            # OpenAPI / Swagger specification
├── cmd/
│   └── server/
│       └── main.go                  # Application entry point (Dependency Injection & wire-up)
├── configs/
│   └── config.yaml                  # Base configuration file (overridden by environment variables)
├── deployments/
│   ├── Dockerfile                   # Multi-stage production build container
│   └── docker-compose.yml           # Local dev environment (Postgres, Kafka, Prometheus, Grafana)
├── db/
│   └── migrations/                  # Database migration scripts (e.g., golang-migrate or goose)
│       ├── 000001_create_orders_table.up.sql
│       └── 000001_create_orders_table.down.sql
├── internal/                        # Private application code (Go compiler prevents external import)
│   ├── config/                      # App configuration struct parser (using Viper or envconfig)
│   │   └── config.go
│   ├── domain/                      # CORE LAYER 1: Pure Business Entities & Rules (No 3rd party deps)
│   │   ├── order.go                 # Order aggregate root & OrderItem struct
│   │   ├── status.go                # Order state transitions & validation logic
│   │   └── errors.go                # Custom domain errors (e.g., ErrOrderAlreadyCancelled)
│   ├── ports/                       # CORE LAYER 2: Interface Contracts (Driven & Driving)
│   │   ├── repository.go            # DB Interface (e.g., OrderRepository)
│   │   ├── publisher.go             # Messaging Interface (e.g., EventPublisher)
│   │   └── service.go               # Use Case Interface (e.g., OrderUseCase)
│   ├── service/                     # CORE LAYER 3: Application Use Cases (Orchestrates Business Logic)
│   │   ├── order_usecase.go         # Executes CreateOrder, CancelOrder workflows
│   │   └── order_usecase_test.go    # Unit tests using mocks for ports
│   └── adapters/                    # INFRASTRUCTURE LAYER: Framework & External Drivers
│       ├── http/                    # Driving Adapter: REST API
│       │   ├── handler.go           # HTTP Handlers (Gin / Fiber / net/http)
│       │   ├── dto.go               # API Request / Response DTOs
│       │   ├── router.go            # Route definitions & endpoint registration
│       │   └── middleware.go        # Logging, Auth, CORS, and Prometheus metrics middleware
│       ├── postgres/                # Driven Adapter: Database Persistence
│       │   ├── repository.go        # Implements ports.OrderRepository (pgx or GORM)
│       │   └── models.go            # DB table schemas & ORM mappings
│       ├── kafka/                   # Driven/Driving Adapter: Event Streaming
│       │   ├── producer.go          # Implements ports.EventPublisher (confluent-kafka-go / kafka-go)
│       │   ├── consumer.go          # Background Consumer worker (reads PaymentProcessed events)
│       │   └── mapper.go            # Maps Domain Events <-> Kafka JSON/Protobuf payloads
│       └── metrics/                 # Driven Adapter: Observability
│           └── prometheus.go        # Prometheus metric definitions (Counters, Histograms)
├── pkg/                             # Public packages (Code intended to be shared across services)
│   └── contracts/                   # Shared event schemas exported to other Go services
│       └── order_events.go
├── .env.example
├── .golangci.yml                    # Linter configuration rules
├── go.mod
├── go.sum
├── Makefile                         # Build, test, run, and migration CLI commands
└── README.md

## Complete Setup File Structure ##
ecommerce-platform/
├── .github/                           # CI/CD Workflows
│   └── workflows/
│       ├── test.yml                   # Runs unit & integration tests across services
│       └── deploy.yml                 # Builds Docker images and pushes to registry
│
├── deploy/                            # Infrastructure & Observability Configurations
│   ├── grafana/                       # Grafana setup
│   │   ├── dashboards/                # Custom JSON dashboards
│   │   │   ├── order-service.json
│   │   │   └── kafka-overview.json
│   │   └── provisioning/              # Auto-load datasources & dashboards
│   │       ├── datasources/
│   │       │   └── prometheus.yml
│   │       └── dashboards/
│   │           └── dashboards.yml
│   ├── postgres/                      # Global database setup
│   │   └── init.sql                   # Creates separate DBs for each service
│   └── prometheus/                    # Prometheus scrape configurations
│       └── prometheus.yml             # Targets order, inventory, payment, shipping
│
├── pkg/                               # Shared Go Libraries (Imported by microservices)
│   └── contracts/                     # Event Contracts shared across Kafka producers/consumers
│       ├── order_events.go            # OrderCreatedEvent, OrderCancelledEvent
│       ├── inventory_events.go        # InventoryReservedEvent, InventoryFailedEvent
│       ├── payment_events.go          # PaymentProcessedEvent, PaymentFailedEvent
│       └── shipping_events.go         # ShippingLabelCreatedEvent
│
├── services/                          # ALL MICROSERVICES
│   │
│   ├── order-service/                 # SERVICE 1: Order Service (API Gateway & Manager)
│   │   ├── cmd/
│   │   │   └── server/
│   │   │       └── main.go            # Application entry point & dependency wiring
│   │   ├── db/
│   │   │   └── migrations/            # Order DB migrations (golang-migrate)
│   │   │       ├── 000001_create_orders_table.up.sql
│   │   │       └── 000001_create_orders_table.down.sql
│   │   ├── internal/
│   │   │   ├── adapters/
│   │   │   │   ├── http/              # Gin HTTP Handlers & DTOs
│   │   │   │   │   ├── handler.go
│   │   │   │   │   └── router.go
│   │   │   │   ├── kafka/             # Kafka Event Producer & Consumer
│   │   │   │   │   ├── producer.go
│   │   │   │   │   └── consumer.go
│   │   │   │   └── postgres/          # Order PostgreSQL Repository
│   │   │   │       └── repository.go
│   │   │   ├── domain/                # Core Order Aggregate & State Machine
│   │   │   │   └── order.go
│   │   │   ├── ports/                 # DB, Kafka, & Service Interfaces
│   │   │   │   └── ports.go
│   │   │   └── service/               # Order Use Cases
│   │   │       └── order_usecase.go
│   │   ├── Dockerfile                 # Multi-stage Docker build
│   │   └── go.mod
│   │
│   ├── inventory-service/             # SERVICE 2: Inventory Service (Worker & DB)
│   │   ├── cmd/
│   │   │   └── worker/
│   │   │       └── main.go            # Kafka event loop worker entry point
│   │   ├── db/
│   │   │   └── migrations/            # Inventory DB migrations
│   │   │       ├── 000001_create_inventory_table.up.sql
│   │   │       └── 000001_create_inventory_table.down.sql
│   │   ├── internal/
│   │   │   ├── adapters/
│   │   │   │   ├── kafka/             # Consumes order.created, publishes inventory.reserved
│   │   │   │   │   └── consumer.go
│   │   │   │   └── postgres/          # Inventory reservation queries (Row locks)
│   │   │   │       └── repository.go
│   │   │   ├── domain/                # StockItem Aggregate
│   │   │   │   └── stock.go
│   │   │   └── service/               # Reserve/Release Stock logic
│   │   │       └── inventory_usecase.go
│   │   ├── Dockerfile
│   │   └── go.mod
│   │
│   ├── payment-service/               # SERVICE 3: Payment Service (Third-Party Gateway)
│   │   ├── cmd/
│   │   │   └── worker/
│   │   │       └── main.go
│   │   ├── db/
│   │   │   └── migrations/            # Payment transactions log schema
│   │   │       └── 000001_create_payments_table.up.sql
│   │   ├── internal/
│   │   │   ├── adapters/
│   │   │   │   ├── kafka/             # Consumes inventory.reserved, publishes payment.processed
│   │   │   │   │   └── consumer.go
│   │   │   │   ├── postgres/          # Transaction record persistence
│   │   │   │   │   └── repository.go
│   │   │   │   └── stripe/            # External Payment Client API adapter
│   │   │   │       └── client.go
│   │   │   ├── domain/                # Transaction Aggregate
│   │   │   │   └── transaction.go
│   │   │   └── service/               # Process Payment workflow
│   │   │       └── payment_usecase.go
│   │   ├── Dockerfile
│   │   └── go.mod
│   │
│   ├── shipping-service/             # SERVICE 4: Shipping Service (Fulfillment)
│   │   ├── cmd/
│   │   │   └── worker/
│   │   │       └── main.go
│   │   ├── db/
│   │   │   └── migrations/            # Shipping labels schema
│   │   │       └── 000001_create_shipping_table.up.sql
│   │   ├── internal/
│   │   │   ├── adapters/
│   │   │   │   ├── kafka/             # Consumes payment.processed
│   │   │   │   │   └── consumer.go
│   │   │   │   └── postgres/          # Waybill/Label persistence
│   │   │   │       └── repository.go
│   │   │   ├── domain/                # Shipment Aggregate
│   │   │   │   └── shipment.go
│   │   │   └── service/               # Label generation workflow
│   │   │       └── shipping_usecase.go
│   │   ├── Dockerfile
│   │   └── go.mod
│   │
│   └── notification-service/          # SUPPORTING SERVICE: Email/SMS Dispatcher
│       ├── cmd/
│       │   └── worker/
│       │       └── main.go            # Listens to all topics & sends notifications
│       ├── internal/
│       │   └── adapters/
│       │       ├── kafka/             # Consumes order, payment, and shipping events
│       │       └── email/             # SMTP / SendGrid client
│       ├── Dockerfile
│       └── go.mod
│
├── scripts/                           # Dev Automation Scripts
│   ├── seed_data.sh                   # Populates PostgreSQL with test inventory items
│   └── run_migrations.sh              # Runs all DB migrations across all 4 services
│
├── .env.example                       # Root environment variable template
├── .gitignore
├── docker-compose.yml                 # Orchestrates Postgres, Kafka, Prometheus, Grafana, & 4 Go apps
├── go.work                            # Go Workspace file linking pkg/ and all services/
├── Makefile                           # Master build, run, and test control script
└── README.md                          # Application documentation