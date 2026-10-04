# Target architecture and learning roadmap

The goal remains a Kafka-based order processing platform with multiple Go services. This is a plan, not a claim that those services are already implemented. The [root README](../README.md) documents the runnable system.

## Intended system

```text
Client → Order Service → Order PostgreSQL + transactional outbox
                              ↓
                            Kafka
                              ↓
             Inventory → Payment → Fulfillment
                              ↓
                         Notification

Order Service consumes workflow results and maintains order status.
Each stateful service owns its database; Prometheus and Grafana observe them.
```

| Service | Responsibility | Planned storage |
| --- | --- | --- |
| Order | Accept orders and manage their lifecycle | PostgreSQL |
| Inventory | Reserve/release stock | PostgreSQL |
| Payment | Authorize or capture payment through a provider | PostgreSQL |
| Fulfillment | Create shipments and shipping labels | PostgreSQL |
| Notification | React to events and notify customers | Initially stateless; delivery tracking if needed |

Services exchange Kafka messages instead of querying one another's databases. The workflow completes over time, so the API initially returns an accepted order rather than a completed purchase.

## Planned workflow

1. Save an accepted order and an outgoing event in one database transaction.
2. An outbox worker publishes `order.created` to Kafka.
3. Inventory reserves stock and emits `inventory.reserved` or `inventory.failed`.
4. Payment processes a reserved order and emits `payment.processed` or `payment.failed`.
5. Fulfillment creates a shipment after successful payment and emits `shipping.label_created`.
6. Order consumes results to update status; Notification consumes relevant events.
7. On failure, coordinate compensating actions such as releasing reserved stock before marking the order cancelled.

The outbox is a database table of outgoing events. Saving it with the order avoids losing an event between a successful database write and a failed Kafka publish. Delivery can repeat, so consumers must recognize already-processed event IDs.

The first downstream slice can use events directly to trigger the next participant. As compensation becomes more complex, the Order Service can coordinate explicit commands and responses (an orchestrated Saga). Choose one trigger for each action so a payment cannot be started both by an event listener and by an orchestrator command.

## Planned contracts

| Message | Producer | Intended consumers |
| --- | --- | --- |
| `order.created` | Order | Inventory, Notification |
| `inventory.reserved` | Inventory | Order, Payment |
| `inventory.failed` | Inventory | Order, Notification |
| `payment.processed` | Payment | Order, Fulfillment, Notification |
| `payment.failed` | Payment | Order and compensation workflow, Notification |
| `inventory.release_reservation` | Workflow coordinator | Inventory |
| `inventory.released` | Inventory | Order |
| `shipping.label_created` | Fulfillment | Order, Notification |

Use an envelope with event ID, type, timestamp, schema version, order ID, and a versioned payload. The inventory payload needs product IDs and quantities. Finalize contracts when implementing each workflow; the current two-field demo on `orders` is not that complete contract. Keep the existing demo topic separate during the transition to real events.

## Staged roadmap

| Milestone | Status and next result |
| --- | --- |
| HTTP API and PostgreSQL | Implemented: create, retrieve, cancel, validation, tests. |
| Basic observability | Implemented: request/runtime metrics, Prometheus scraping, Grafana data source. Export dashboards for reproducibility. |
| Kafka basics | Implemented as separate producer/consumer demos; no API publishing yet. |
| Operational cleanup | Implemented graceful shutdown, exact money, optimistic updates, and automated message tests. Explicit Kafka data directory, pinned images, and exported dashboards remain future setup work. |
| Real order events | Define the contract, publisher interface, Kafka adapter, and outbox. Verify a saved order reaches a consumer, including retry after broker downtime. |
| Inventory Service | Give it its own database, reservation rules, an idempotent consumer, and a bounded worker pool. |
| Payment and Fulfillment | Add downstream consumers, simulated providers first, and persisted results. |
| Failure compensation | Coordinate cancellation, inventory release, retries/backoff, dead-letter handling, and safe replay. |
| Notifications and tracing | Add notifications, correlation IDs, structured logs, tracing, and workflow dashboards. |
| Packaging | Versioned migrations, health/readiness endpoints, Dockerfiles, CI; Kubernetes when useful. |

Exact four-decimal money and version-checked updates are implemented. Before real payment processing, control authoritative pricing and define provider-specific rounding. The demo consumer now commits after processing or confirmed dead-letter delivery; real inventory/payment side effects still require idempotency and carefully designed replay. These are separate learning milestones, not reasons to add every abstraction immediately.

## Structure as services grow

Keep the current Order Service layout while learning. When Inventory becomes runnable, introduce its module and executable with a similar small set of responsibilities. A future `services/` layout and Go workspace are options when multiple modules exist, not prerequisites for the next event.

The publisher interface and Kafka adapter for real API events will fit alongside the existing repository interface and PostgreSQL adapter. Share message contracts without sharing database models between services.

Optional later capabilities from the original proposal include an edge gateway, authentication/rate limiting, Redis caching or idempotency storage, gRPC, OpenTelemetry/Jaeger, and Kubernetes. Their exact tools can change while preserving the Kafka microservices design.

The [original architecture proposal](architecture-original.md) preserves the earlier diagrams, illustrative schemas, and full folder ideas. Its old commands, response examples, and claims of completed features are not current runtime documentation.
