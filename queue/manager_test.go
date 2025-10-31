package queue

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"gopherpost/internal/metrics"
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

func TestManagerStopIdempotent(t *testing.T) {
	m := NewManager()
	m.Start()
	m.Stop()
	// second stop should not panic
	m.Stop()
}

func TestManagerProcessQueueWorkerConcurrency(t *testing.T) {
	metrics.ResetForTests()

	originalDeliver := deliverFunc
	defer func() { deliverFunc = originalDeliver }()

	var mu sync.Mutex
	current := 0
	max := 0

	deliverFunc = func(from, to string, data []byte) error {
		mu.Lock()
		current++
		if current > max {
			max = current
		}
		mu.Unlock()

		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		current--
		mu.Unlock()

		return nil
	}

	makeMessage := func(id int) QueuedMessage {
		return QueuedMessage{
			ID:        fmt.Sprintf("msg-%d", id),
			From:      "sender@example.com",
			To:        fmt.Sprintf("rcpt-%d@example.net", id),
			Payload:   NewPayload([]byte("body")),
			Attempts:  1,
			NextRetry: time.Now().Add(-time.Second),
		}
	}

	measure := func(workers int) int {
		m := NewManager(WithWorkers(workers))
		for i := 0; i < 6; i++ {
			m.Enqueue(makeMessage(workers*100 + i))
		}

		mu.Lock()
		current = 0
		max = 0
		mu.Unlock()

		m.processQueue()

		mu.Lock()
		defer mu.Unlock()
		return max
	}

	if serialMax := measure(1); serialMax != 1 {
		t.Fatalf("expected serial worker to max at 1 concurrent delivery, got %d", serialMax)
	}

	if parallelMax := measure(4); parallelMax <= 1 {
		t.Fatalf("expected parallel workers to exceed 1 concurrent delivery, got %d", parallelMax)
	}
}
