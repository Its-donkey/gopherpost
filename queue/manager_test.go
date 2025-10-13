package queue

import (
	"errors"
	"testing"
	"time"

	"smtpserver/internal/metrics"
)

func TestManagerProcessQueueSuccess(t *testing.T) {
	metrics.ResetForTests()

	originalDeliver := deliverFunc
	defer func() { deliverFunc = originalDeliver }()

	var delivered [][]byte
	deliverFunc = func(from, to string, data []byte) error {
		if from != "sender@example.com" || to != "rcpt@example.net" {
			t.Fatalf("unexpected envelope %s -> %s", from, to)
		}
		delivered = append(delivered, append([]byte(nil), data...))
		return nil
	}

	m := NewManager()
	msg := QueuedMessage{
		ID:        "msg-1",
		From:      "sender@example.com",
		To:        "rcpt@example.net",
		Payload:   NewPayload([]byte("body")),
		Attempts:  1,
		NextRetry: time.Now().Add(-time.Second),
	}
	m.Enqueue(msg)

	if got := m.Depth(); got != 1 {
		t.Fatalf("expected depth 1, got %d", got)
	}
	if metrics.MessagesQueued.Value() != 1 {
		t.Fatalf("expected MessagesQueued=1, got %d", metrics.MessagesQueued.Value())
	}

	m.processQueue()

	if got := m.Depth(); got != 0 {
		t.Fatalf("expected depth 0 after delivery, got %d", got)
	}
	if len(delivered) != 1 {
		t.Fatalf("expected one delivery attempt, got %d", len(delivered))
	}
	if string(delivered[0]) != "body" {
		t.Fatalf("unexpected payload: %q", string(delivered[0]))
	}
	if metrics.MessagesDelivered.Value() != 1 {
		t.Fatalf("expected MessagesDelivered=1, got %d", metrics.MessagesDelivered.Value())
	}
	if metrics.DeliveryFailures.Value() != 0 {
		t.Fatalf("expected DeliveryFailures=0, got %d", metrics.DeliveryFailures.Value())
	}
}

func TestManagerProcessQueueFailure(t *testing.T) {
	metrics.ResetForTests()

	originalDeliver := deliverFunc
	defer func() { deliverFunc = originalDeliver }()

	deliverFunc = func(from, to string, data []byte) error {
		return errors.New("smtp unavailable")
	}

	m := NewManager()
	msg := QueuedMessage{
		ID:        "msg-2",
		From:      "sender@example.com",
		To:        "rcpt@example.net",
		Payload:   NewPayload([]byte("body")),
		Attempts:  1,
		NextRetry: time.Now().Add(-time.Second),
	}
	m.Enqueue(msg)

	m.processQueue()

	if got := m.Depth(); got != 1 {
		t.Fatalf("expected depth 1 after retry, got %d", got)
	}
	if metrics.DeliveryFailures.Value() != 1 {
		t.Fatalf("expected DeliveryFailures=1, got %d", metrics.DeliveryFailures.Value())
	}
	if m.queue[0].Attempts != 2 {
		t.Fatalf("expected Attempts=2, got %d", m.queue[0].Attempts)
	}
	if m.queue[0].LastError == "" {
		t.Fatalf("expected LastError to be recorded")
	}
}
