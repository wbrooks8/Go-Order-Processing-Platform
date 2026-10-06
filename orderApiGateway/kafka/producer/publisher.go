package producer

import (
	"context"
	"encoding/json"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/wbrooks8/go_order_api_gateway/kafka/config"
	"github.com/wbrooks8/go_order_api_gateway/kafka/events"
)

type Publisher struct {
	writer *kafka.Writer
}

func NewPublisher() *Publisher {
	return &Publisher{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(config.Broker()),
			Topic:        config.OrdersTopic,
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireAll,
			MaxAttempts:  3,
			WriteTimeout: 10 * time.Second,
		},
	}
}

func (p *Publisher) PublishOrderCreated(ctx context.Context, orderID, customerID string) error {
	event := events.OrderCreatedEvent{
		OrderID:    orderID,
		CustomerID: customerID,
	}

	value, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(orderID),
		Value: value,
		Time:  time.Now(),
	})
}

func (p *Publisher) Close() error {
	return p.writer.Close()
}
