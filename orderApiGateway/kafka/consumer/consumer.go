// Processes demo records sequentially and commits only after success or DLQ delivery.
package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/wbrooks8/go_order_api_gateway/kafka/config"
	"github.com/wbrooks8/go_order_api_gateway/kafka/events"
)

// Small interfaces let tests verify acknowledgement order without a Kafka broker.
type messageReader interface {
	FetchMessage(context.Context) (kafka.Message, error)
	CommitMessages(context.Context, ...kafka.Message) error
}
type messageWriter interface {
	WriteMessages(context.Context, ...kafka.Message) error
}

// DeadLetter preserves the original bytes (base64 in JSON), source position,
// and failure reason so a rejected record can be inspected and replayed deliberately.
type DeadLetter struct {
	Topic     string `json:"topic"`
	Partition int    `json:"partition"`
	Offset    int64  `json:"offset"`
	Key       []byte `json:"key"`
	Value     []byte `json:"value"`
	Reason    string `json:"reason"`
}

func RunConsumer(ctx context.Context) (err error) {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{config.Broker()}, Topic: config.OrdersTopic,
		GroupID: config.ConsumerGroup, MinBytes: 1, MaxBytes: 10e6,
		CommitInterval: 0, // Synchronous commits after processing, never on fetch.
	})
	defer func() { err = errors.Join(err, r.Close()) }()
	dlq := &kafka.Writer{
		Addr: kafka.TCP(config.Broker()), Topic: config.DeadLetterTopic,
		RequiredAcks: kafka.RequireAll, MaxAttempts: 3, WriteTimeout: 10 * time.Second,
	}
	defer func() { err = errors.Join(err, dlq.Close()) }()
	return consume(ctx, r, dlq, func(_ context.Context, event events.OrderCreatedEvent, msg kafka.Message) error {
		log.Printf("order=%s customer=%s partition=%d offset=%d", event.OrderID, event.CustomerID, msg.Partition, msg.Offset)
		return nil
	}, time.Second)
}

func consume(ctx context.Context, r messageReader, dlq messageWriter,
	process func(context.Context, events.OrderCreatedEvent, kafka.Message) error, backoff time.Duration) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		msg, err := r.FetchMessage(ctx)
		if err != nil {
			return fmt.Errorf("fetch: %w", err)
		}
		var event events.OrderCreatedEvent
		failure := json.Unmarshal(msg.Value, &event)
		if failure == nil && (event.OrderID == "" || event.CustomerID == "") {
			failure = errors.New("missing required order or customer ID")
		}
		if failure == nil {
			for attempt := 0; attempt < 3; attempt++ {
				if err := ctx.Err(); err != nil {
					return err
				}
				failure = process(ctx, event, msg)
				if failure == nil {
					break
				}
				if err := ctx.Err(); err != nil {
					return err
				}
				if errors.Is(failure, context.Canceled) {
					return failure
				}
				if attempt < 2 {
					timer := time.NewTimer(backoff * time.Duration(1<<attempt))
					select {
					case <-ctx.Done():
						timer.Stop()
						return ctx.Err()
					case <-timer.C:
					}
				}
			}
		}
		if failure != nil {
			payload, err := json.Marshal(DeadLetter{msg.Topic, msg.Partition, msg.Offset, msg.Key, msg.Value, failure.Error()})
			if err != nil {
				return err
			}
			writeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			err = dlq.WriteMessages(writeCtx, kafka.Message{Key: msg.Key, Value: payload})
			cancel()
			if err != nil {
				return fmt.Errorf("dead-letter delivery failed; source uncommitted: %w", err)
			}
			log.Printf("dead-lettered topic=%s partition=%d offset=%d: %v", msg.Topic, msg.Partition, msg.Offset, failure)
		}
		commitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err = r.CommitMessages(commitCtx, msg)
		cancel()
		if err != nil {
			return fmt.Errorf("commit: %w", err)
		}
		// Never fetch past a failed DLQ write or commit. A later commit could otherwise
		// skip failed work in the same partition. Replays remain possible after a crash.
	}
}
