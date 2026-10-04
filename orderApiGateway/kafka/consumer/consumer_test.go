// Verifies processing, retry, dead-letter, and commit ordering without a broker.
package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/wbrooks8/go_order_api_gateway/kafka/events"
)

type fakeReader struct {
	messages  []kafka.Message
	fetched   int
	committed []kafka.Message
	commitErr error
	actions   *[]string
}

func (r *fakeReader) FetchMessage(context.Context) (kafka.Message, error) {
	if r.fetched == len(r.messages) {
		return kafka.Message{}, io.EOF
	}
	msg := r.messages[r.fetched]
	r.fetched++
	return msg, nil
}
func (r *fakeReader) CommitMessages(_ context.Context, messages ...kafka.Message) error {
	*r.actions = append(*r.actions, "commit")
	if r.commitErr != nil {
		return r.commitErr
	}
	r.committed = append(r.committed, messages...)
	return nil
}

type fakeWriter struct {
	messages []kafka.Message
	err      error
	actions  *[]string
}

func (w *fakeWriter) WriteMessages(_ context.Context, messages ...kafka.Message) error {
	*w.actions = append(*w.actions, "dlq")
	if w.err != nil {
		return w.err
	}
	w.messages = append(w.messages, messages...)
	return nil
}

func TestConsumeAcknowledgementPolicy(t *testing.T) {
	valid := `{"orderId":"order-1","customerId":"customer-1"}`
	failure := errors.New("processing failed")
	for _, test := range []struct {
		name, payload                   string
		failAttempts                    int
		dlqErr, commitErr               error
		wantCalls, wantDLQ, wantCommits int
	}{
		{name: "success", payload: valid, wantCalls: 1, wantCommits: 1},
		{name: "retry succeeds", payload: valid, failAttempts: 2, wantCalls: 3, wantCommits: 1},
		{name: "exhausted", payload: valid, failAttempts: 3, wantCalls: 3, wantDLQ: 1, wantCommits: 1},
		{name: "bad JSON", payload: "not json", wantDLQ: 1, wantCommits: 1},
		{name: "missing ID", payload: `{"orderId":"a"}`, wantDLQ: 1, wantCommits: 1},
		{name: "DLQ unavailable", payload: "not json", dlqErr: failure},
		{name: "commit unavailable", payload: valid, commitErr: failure, wantCalls: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			actions := []string{}
			source := kafka.Message{Topic: "orders", Partition: 1, Offset: 42, Value: []byte(test.payload)}
			r := &fakeReader{messages: []kafka.Message{source}, commitErr: test.commitErr, actions: &actions}
			w := &fakeWriter{err: test.dlqErr, actions: &actions}
			calls := 0
			err := consume(context.Background(), r, w, func(context.Context, events.OrderCreatedEvent, kafka.Message) error {
				actions = append(actions, "process")
				calls++
				if calls <= test.failAttempts {
					return failure
				}
				return nil
			}, 0)
			if test.dlqErr != nil || test.commitErr != nil {
				if !errors.Is(err, failure) {
					t.Fatalf("error = %v", err)
				}
			} else if !errors.Is(err, io.EOF) {
				t.Fatalf("error = %v, want EOF", err)
			}
			if calls != test.wantCalls || len(w.messages) != test.wantDLQ || len(r.committed) != test.wantCommits {
				t.Fatalf("process/DLQ/commits = %d/%d/%d", calls, len(w.messages), len(r.committed))
			}
			if test.wantCommits > 0 && actions[len(actions)-1] != "commit" {
				t.Fatal("committed before processing or DLQ")
			}
			if len(w.messages) > 0 {
				var record DeadLetter
				if err := json.Unmarshal(w.messages[0].Value, &record); err != nil {
					t.Fatal(err)
				}
				if string(record.Value) != test.payload || record.Offset != 42 || record.Topic != "orders" || record.Reason == "" {
					t.Fatalf("incomplete dead letter: %+v", record)
				}
			}
		})
	}
}

func TestConsumeStopsBeforeLaterOffsetsOnFailure(t *testing.T) {
	actions := []string{}
	failure := errors.New("DLQ offline")
	r := &fakeReader{messages: []kafka.Message{{Offset: 1, Value: []byte("bad")}, {Offset: 2}}, actions: &actions}
	w := &fakeWriter{err: failure, actions: &actions}
	err := consume(context.Background(), r, w, nil, 0)
	if !errors.Is(err, failure) || r.fetched != 1 || len(r.committed) != 0 {
		t.Fatalf("fetched=%d committed=%d error=%v", r.fetched, len(r.committed), err)
	}
}

func TestConsumeCancellationDuringRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	actions := []string{}
	r := &fakeReader{messages: []kafka.Message{{Value: []byte(`{"orderId":"a","customerId":"b"}`)}}, actions: &actions}
	w := &fakeWriter{actions: &actions}
	err := consume(ctx, r, w, func(context.Context, events.OrderCreatedEvent, kafka.Message) error {
		cancel()
		return errors.New("temporary failure")
	}, time.Hour)
	if err == nil || len(w.messages) != 0 || len(r.committed) != 0 {
		t.Fatalf("cancellation acknowledged record: %v", err)
	}
}
