// Package config holds connection settings for the standalone Kafka demos.
// Topics are created with the commands in docs/kafka-reference.md.
package config

import "os"

const OrdersTopic = "orders"
const DeadLetterTopic = "orders.dlq"
const ConsumerGroup = "order-workers"

func Broker() string {
	if broker := os.Getenv("KAFKA_BROKER"); broker != "" {
		return broker
	}
	return "localhost:9092"
}
