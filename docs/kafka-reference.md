# Kafka reference for this project

Run the commands below from the repository root (the folder containing `docker_compose.yml`). No Kafka installation on your computer is needed: the tools run inside its Docker container.

## Current setup

| Setting | Value |
| --- | --- |
| Compose service | `kafka` |
| Image | `apache/kafka:latest` |
| Broker address for Go running on your computer | `localhost:9092` |
| Topic used by the Go code | `orders` |
| Partitions | 3 |
| Replication factor | 1 (one local broker) |
| Go consumer group | `order-workers` |
| Dead-letter topic | `orders.dlq` |

The roadmap mentions `order.created`; the current code uses `orders`. Use `orders` for these exercises. A topic is created in Kafka, not by creating a Go source file.

## Words to know

| Term | Meaning |
| --- | --- |
| Broker | The Kafka server that stores and serves messages. |
| Producer | A program that sends messages. |
| Consumer | A program that reads messages. |
| Topic | A named stream of messages. |
| Partition | One ordered part of a topic. Ordering applies within a partition, not across the whole topic. |
| Offset | A message's position within its partition. |
| Consumer group | Consumers sharing work, with saved progress for each partition. |
| Lag | How far a group's committed position trails the available messages. |

Reading a message does not delete it. Kafka removes old records according to its retention settings.

## Start here each session

```bash
docker compose -f docker_compose.yml config --quiet
docker compose -f docker_compose.yml up -d kafka
docker compose -f docker_compose.yml ps kafka
```

The first command checks the Compose file. No output means it passed. A running container may still be initializing; if the next command cannot connect, check logs and retry after startup.

```bash
docker compose -f docker_compose.yml logs --tail=100 kafka
```

## Create and inspect the topic

```bash
docker compose -f docker_compose.yml exec kafka \
  /opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server localhost:9092 \
  --create --if-not-exists \
  --topic orders \
  --partitions 3 \
  --replication-factor 1
```

`--if-not-exists` lets you repeat creation without failing merely because the topic exists; it does not change an existing topic's configuration.

List topics:

```bash
docker compose -f docker_compose.yml exec kafka \
  /opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server localhost:9092 --list
```

Inspect this topic:

```bash
docker compose -f docker_compose.yml exec kafka \
  /opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server localhost:9092 --describe --topic orders
```

Expect three partitions and replication factor one.

## Send a message

Start the interactive producer:

```bash
docker compose -f docker_compose.yml exec kafka \
  /opt/kafka/bin/kafka-console-producer.sh \
  --bootstrap-server localhost:9092 --topic orders
```

Paste this at its prompt and press Enter:

```json
{"orderId":"44f551b5-0c28-4132-bcd4-d09b048dfe61","customerId":"d8f3b2a1-0000-4a8a-8e2b-123456789abc"}
```

Each line becomes one message. This is a demonstration payload, not a finalized event contract. The console producer sends text; it does not call your Go API or validate the JSON. Press **Ctrl+C** to exit the producer; Kafka keeps running.

## Read messages

```bash
docker compose -f docker_compose.yml exec kafka \
  /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic orders --from-beginning
```

This console consumer reads available retained messages from the beginning and then waits for new ones. Leave it open and send another message from a second terminal. Press **Ctrl+C** when finished.

To include partition and offset information:

```bash
docker compose -f docker_compose.yml exec kafka \
  /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic orders --from-beginning \
  --property print.partition=true \
  --property print.offset=true
```

These topic/producer/consumer commands follow the [Apache Kafka quickstart](https://kafka.apache.org/quickstart/). The image includes the scripts under `/opt/kafka/bin`, as described in [Docker's Kafka guide](https://docs.docker.com/guides/kafka/).

## Understand the command pieces

| Piece | Purpose |
| --- | --- |
| `docker compose -f docker_compose.yml` | Select this project's Compose file. |
| `exec kafka` | Run a command inside the running Kafka service. |
| `/opt/kafka/bin/...` | Select the Kafka command-line tool. |
| `--bootstrap-server localhost:9092` | Give the tool its initial broker address. Here the tool runs inside Kafka's own container. |
| `--topic orders` | Select the topic. |
| `--from-beginning` | Start at the earliest retained messages when there is no saved group position. |

## Consumer groups and saved progress

For a separate experiment with saved progress, add a group name:

```bash
docker compose -f docker_compose.yml exec kafka \
  /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic orders --group reference-demo --from-beginning
```

After the consumer commits progress, restarting with the same group resumes from that position. `--from-beginning` does not override existing committed offsets. For a fresh replay, use the earlier command without an explicit group.

Use `reference-demo` for CLI practice rather than `order-workers`: consumers in the same group share partitions and can take work from one another.

Inspect your Go consumer's group:

```bash
docker compose -f docker_compose.yml exec kafka \
  /opt/kafka/bin/kafka-consumer-groups.sh \
  --bootstrap-server localhost:9092 \
  --describe --group order-workers
```

The group may not exist until your Go consumer has actually run. For group and offset details, see the [Kafka operations documentation](https://kafka.apache.org/operations/).

## How this connects to the Go files

- [`config.go`](../orderApiGateway/kafka/config/config.go) defines topic/group settings. `KAFKA_BROKER` overrides the default broker address.
- [`order_created.go`](../orderApiGateway/kafka/events/order_created.go) defines the shared demo JSON event.
- [`producer.go`](../orderApiGateway/kafka/producer/producer.go) creates UUIDs, encodes events, and sends ten messages. Run `go run ./cmd/producer` from `orderApiGateway`.
- [`consumer.go`](../orderApiGateway/kafka/consumer/consumer.go) decodes and processes messages. Run `go run ./cmd/consumer` from `orderApiGateway`.
- [`docker_compose.yml`](../docker_compose.yml) starts the Kafka server, not the Go programs. Topic creation uses the CLI commands in this guide.

The demo consumer still accepts nonempty sample IDs such as `order-123`; the actual Order API requires UUIDs. Creating an HTTP order does not yet publish a Kafka event.

## Dead letters and acknowledgement

Create the dead-letter topic before running the Go consumer:

```bash
docker compose -f docker_compose.yml exec kafka \
  /opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server localhost:9092 \
  --create --if-not-exists --topic orders.dlq \
  --partitions 1 --replication-factor 1
```

The Go consumer uses `FetchMessage` and explicit `CommitMessages`. Unlike `ReadMessage`, fetching does not automatically commit group progress. See the [kafka-go documentation](https://github.com/segmentio/kafka-go#explicit-commits).

Policy:

- Valid events are processed, then committed. The current processor only logs.
- Processing failures get three total attempts, waiting one second and then two seconds between attempts.
- Invalid JSON, missing IDs, and exhausted processing failures are published to `orders.dlq`. Only after that write is confirmed does the consumer commit the source offset.
- If fetching, dead-letter publishing, or committing fails, the command exits instead of fetching later records. Restart it after resolving the issue.
- Ctrl+C cancels work and closes connections. An interrupted uncommitted record can be delivered again.

Inspect dead letters:

```bash
docker compose -f docker_compose.yml exec kafka \
  /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 --topic orders.dlq --from-beginning
```

Each record contains the original topic, partition, offset, error reason, key, and value. Key/value bytes are base64-encoded by Go's JSON encoder, preserving even malformed JSON. Decode the value when inspecting it (for example with Python's `base64.b64decode`). Replay is manual: fix the message and publish it to `orders` after understanding the original failure.

This provides an explicit at-least-once processing policy, not exactly-once effects. A crash after processing or DLQ publication but before commit can cause duplicates. Future inventory/payment handlers must use idempotency before performing business side effects.

## Troubleshooting

| Symptom | Check |
| --- | --- |
| Compose reports an unknown property | Check spelling: `container_name`, not `contianer_name`. |
| Undefined volume | The service mount uses `kafka_data`, matching the top-level declaration. |
| Connection refused | Check `ps kafka` and Kafka logs; the broker may still be starting. |
| Topic is missing | Run the topic creation command. |
| Consumer waits with no output | Confirm producer and consumer use `orders`; send a new message. For old messages, check the group's saved position. |
| Consumer reads only some messages | Another consumer in the same group may own other partitions. |
| Go cannot connect | Go running on your computer uses `localhost:9092`. |
| A different container cannot connect | The current advertised address is `localhost:9092`. Other containers need an internal advertised listener; changing only their bootstrap address to `kafka:9092` is insufficient. |

Kafka currently uses `latest`, so pulling the image later can change the version. Record or pin the working version when you want a reproducible setup.

## Stop for the day

Exit console tools with **Ctrl+C**, then stop only Kafka:

```bash
docker compose -f docker_compose.yml stop kafka
```

This preserves the container. To resume, use `up -d kafka` again.

The Compose file mounts a volume at `/var/lib/kafka/data`, but does not explicitly set `KAFKA_LOG_DIRS`. Verify or configure the broker's actual log directory to use that mount before relying on data surviving container replacement. Stopping and starting the same container is different from replacing it.
