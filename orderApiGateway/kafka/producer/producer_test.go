// Tests producer payloads and stopping before any network work is attempted.
package producer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/wbrooks8/go_order_api_gateway/kafka/events"
)

func TestNewOrderMessage(t *testing.T) {
	msg, err := newOrderMessage()
	if err != nil {
		t.Fatal(err)
	}
	var event events.OrderCreatedEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		t.Fatal(err)
	}
	if uuid.Validate(event.OrderID) != nil || uuid.Validate(event.CustomerID) != nil {
		t.Fatal("demo IDs must be UUIDs")
	}
	if string(msg.Key) != event.OrderID {
		t.Fatal("message key must be order ID")
	}
	if msg.Time.IsZero() {
		t.Fatal("missing message time")
	}
}

func TestProducerHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := RunProducer(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}
