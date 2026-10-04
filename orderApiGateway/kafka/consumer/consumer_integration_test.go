// Opt-in broker test using temporary topics and an isolated consumer group.
package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/wbrooks8/go_order_api_gateway/kafka/events"
)

type stoppingReader struct {
	*kafka.Reader
	cancel  context.CancelFunc
	commits int
}

func (r *stoppingReader) CommitMessages(ctx context.Context, messages ...kafka.Message) error {
	if err := r.Reader.CommitMessages(ctx, messages...); err != nil {
		return err
	}
	r.commits += len(messages)
	if r.commits == 2 {
		r.cancel()
	}
	return nil
}

func TestKafkaIntegrationProcessAndDeadLetter(t *testing.T) {
	broker := os.Getenv("TEST_KAFKA_BROKER")
	if broker == "" {
		t.Skip("set TEST_KAFKA_BROKER to run against a local single-broker Kafka")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	conn, err := kafka.DialContext(ctx, "tcp", broker)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(45 * time.Second)); err != nil {
		t.Fatal(err)
	}
	topic := "cleanup-test-" + uuid.NewString()
	deadTopic := topic + "-dlq"
	if err := conn.CreateTopics(
		kafka.TopicConfig{Topic: topic, NumPartitions: 1, ReplicationFactor: 1},
		kafka.TopicConfig{Topic: deadTopic, NumPartitions: 1, ReplicationFactor: 1},
	); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.DeleteTopics(topic, deadTopic); err != nil {
			t.Errorf("remove temporary topics: %v", err)
		}
	}()
	writer := &kafka.Writer{Addr: kafka.TCP(broker), Topic: topic, RequiredAcks: kafka.RequireAll, MaxAttempts: 5, WriteTimeout: 5 * time.Second}
	defer writer.Close()
	valid := []byte(`{"orderId":"a","customerId":"b"}`)
	if err := writer.WriteMessages(ctx, kafka.Message{Value: valid}, kafka.Message{Value: []byte("bad JSON")}); err != nil {
		t.Fatal(err)
	}
	reader := kafka.NewReader(kafka.ReaderConfig{Brokers: []string{broker}, Topic: topic, GroupID: topic, StartOffset: kafka.FirstOffset, MinBytes: 1, MaxBytes: 1e6})
	defer reader.Close()
	runCtx, stop := context.WithCancel(ctx)
	defer stop()
	tracked := &stoppingReader{Reader: reader, cancel: stop}
	dlq := &kafka.Writer{Addr: kafka.TCP(broker), Topic: deadTopic, RequiredAcks: kafka.RequireAll, WriteTimeout: 5 * time.Second}
	defer dlq.Close()
	processed := 0
	err = consume(runCtx, tracked, dlq, func(context.Context, events.OrderCreatedEvent, kafka.Message) error { processed++; return nil }, 0)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("consume: %v", err)
	}
	if processed != 1 || tracked.commits != 2 {
		t.Fatalf("processed=%d commits=%d", processed, tracked.commits)
	}
	deadReader := kafka.NewReader(kafka.ReaderConfig{Brokers: []string{broker}, Topic: deadTopic, Partition: 0, MinBytes: 1, MaxBytes: 1e6})
	defer deadReader.Close()
	msg, err := deadReader.ReadMessage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var dead DeadLetter
	if err := json.Unmarshal(msg.Value, &dead); err != nil {
		t.Fatal(err)
	}
	if dead.Topic != topic || string(dead.Value) != "bad JSON" || dead.Offset != 1 {
		t.Fatalf("bad dead-letter record: %+v", dead)
	}
}
