// Sends ten sample order events to Kafka for the standalone producer demo.
package producer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/wbrooks8/go_order_api_gateway/kafka/config"
	"github.com/wbrooks8/go_order_api_gateway/kafka/events"
)

func RunProducer(ctx context.Context) (err error) {
	// A Writer is the producer-side connection to a Kafka topic.
	w := &kafka.Writer{
		Addr:         kafka.TCP(config.Broker()),
		Topic:        config.OrdersTopic,
		Balancer:     &kafka.Hash{},
		RequiredAcks: kafka.RequireAll,
		MaxAttempts:  3,
		WriteTimeout: 10 * time.Second,
	}

	defer func() { err = errors.Join(err, w.Close()) }()

	// Each loop iteration creates one independent Kafka record. Kafka keeps
	// records in the topic even if no consumer is currently running.
	for i := 0; i < 10; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		msg, err := newOrderMessage()
		if err != nil {
			return err
		}
		// WriteMessages waits for Kafka to acknowledge the write. An error here
		// means delivery was not confirmed; the broker may still have received it.
		writeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		writeErr := w.WriteMessages(writeCtx, msg)
		cancel()
		if writeErr != nil {
			return fmt.Errorf("publish sample: %w", writeErr)
		}
		log.Printf("sent message %d", i)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return nil
}

func newOrderMessage() (kafka.Message, error) {
	o := events.OrderCreatedEvent{OrderID: uuid.NewString(), CustomerID: uuid.NewString()}

	// JSON turns the Go struct into bytes, which is the value format Kafka
	// transports. A consumer can reconstruct the struct with json.Unmarshal.
	value, err := json.Marshal(o)

	if err != nil {
		return kafka.Message{}, err
	}

	// Time is metadata on the Kafka record, separate from the JSON payload.
	return kafka.Message{Key: []byte(o.OrderID), Value: value, Time: time.Now()}, nil
}
