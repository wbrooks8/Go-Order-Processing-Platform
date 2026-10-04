# Order Service and Kafka demos

This module contains the working Order API and two standalone Kafka learning programs. See the [root README](../README.md) for setup, valid API requests, tests, and observability commands. The [architecture plan](../docs/architecture.md) describes the future microservices platform.

## Executables

Run these from this directory:

| Command | Responsibility |
| --- | --- |
| `go run ./cmd/server` | Starts the HTTP API backed by PostgreSQL. |
| `go run ./cmd/producer` | Sends ten generated demo events to Kafka. |
| `go run ./cmd/consumer` | Reads and logs demo events from Kafka. |

The producer is independent of HTTP order creation. The consumer is not an Inventory Service yet.

## Following a request

For creation:

1. `internal/adapters/http/router.go` selects the handler.
2. `handler.go` reads JSON; `dto.go` converts the request into domain values.
3. `domain.NewOrder` checks UUIDs and order rules, sets `Accepted`, and calculates the total in memory.
4. `service.CreateOrder` validates the supplied order and asks the repository to save it.
5. `adapters/postgres/repository.go` writes the order and items; the handler returns 202.

For cancellation:

1. The handler gets the ID from the URL.
2. The service validates the UUID and loads the order through the repository.
3. The domain's `Cancel` method checks the status and changes it when allowed.
4. The service calls `UpdateOrder` to persist order fields (items are left unchanged). The update matches the loaded version and increments it; stale updates return a conflict.
5. The handler maps the result to an HTTP status and JSON response.

`ports/repository.go` is an interface: the list of database operations the service can use. It does not execute a separate step. HTTP middleware records request counts and durations around these handlers.

## Files to change for common tasks

| Task | Location |
| --- | --- |
| Add a URL | `internal/adapters/http/router.go` |
| Read a request or choose an HTTP response | `internal/adapters/http/handler.go` |
| Change JSON request/response fields | `internal/adapters/http/dto.go` |
| Change order rules | `internal/domain/order.go` |
| Coordinate loading, changing, saving | `internal/service/order_usecase.go` |
| Change database queries | `internal/adapters/postgres/repository.go` |
| Change database mapping | `internal/adapters/postgres/models.go` |
| Change database connection settings | `internal/config/database.go` |
| Change demo Kafka JSON fields | `kafka/events/order_created.go` |

## Current boundaries

- The domain validates customer/product UUIDs, required IDs, items, quantities, prices, USD currency, and totals. Both constructors and services enforce these rules, including for non-HTTP callers. HTTP binding separately checks request shape, SKU and address fields.
- The demo event is deliberately smaller than the domain order. Both Kafka demo programs import its shared definition.
- The `Version` field protects updates from stale writers. On a 409 conflict, retrieve the current order before deciding whether to retry.
- Monetary fields use `domain.Money`, an integer count of ten-thousandths of a dollar. In Go, $12.50 is `Money(125000)` or `ParseMoney("12.50")`; JSON and SQL stay decimal. Currency is currently USD only.
- Schema creation uses GORM AutoMigrate; versioned database migrations are planned.
- No authentication, outbox, inventory reservation, payment, or fulfillment is implemented.

For now, preserve these few layers and add new service folders only when there is a runnable feature to put in them. The [original design](../docs/architecture-original.md) remains available as a planning reference rather than setup documentation.

## Shutdown and Kafka acknowledgement

The command entry points own signal handling and error reporting. Kafka helpers accept a context and return errors, allowing deferred connection cleanup to run. The consumer processes one record at a time, retries processor failures, and writes invalid/exhausted messages to `orders.dlq` before committing their source offset. See the [Kafka reference](../docs/kafka-reference.md) for commands and replay limitations.
