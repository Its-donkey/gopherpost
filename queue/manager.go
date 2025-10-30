package queue

import (
	"log"
	"math/rand"
	"sync"
	"time"

    "gopherpost/delivery"
    audit "gopherpost/internal/audit"
    "gopherpost/internal/metrics"
)

var deliverFunc = delivery.DeliverMessage

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Manager manages a queue of outgoing messages with retry logic.
type Manager struct {
	queue []QueuedMessage
	mu    sync.Mutex
	quit  chan struct{}
	once  sync.Once
}

// NewManager creates a new delivery queue manager.
func NewManager() *Manager {
	return &Manager{
		queue: make([]QueuedMessage, 0),
		quit:  make(chan struct{}),
	}
}

// Enqueue adds a message to the queue.
func (m *Manager) Enqueue(msg QueuedMessage) {
	if msg.Payload == nil || len(msg.Payload.Bytes()) == 0 {
		log.Printf("Discarding message %s for %s: missing payload", msg.ID, msg.To)
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if msg.Attempts == 0 {
		msg.NextRetry = time.Now().Add(initialBackoff())
	}
	m.queue = append(m.queue, msg)
	log.Printf("Queued message %s for %s (attempt %d)", msg.ID, msg.To, msg.Attempts)
	audit.Log("queue enqueue %s -> %s attempt %d next %s", msg.ID, msg.To, msg.Attempts, msg.NextRetry.Format(time.RFC3339))
	metrics.MessagesQueued.Add(1)
	metrics.SetQueueDepth(len(m.queue))
}

// Start starts the queue processor in a background goroutine.
func (m *Manager) Start() {
	m.once.Do(func() {
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			m.processQueue()
			for {
				select {
				case <-m.quit:
					return
				case <-ticker.C:
					m.processQueue()
				}
			}
		}()
	})
}

// Stop shuts down the queue processor.
func (m *Manager) Stop() {
	close(m.quit)
}

// processQueue attempts to deliver messages that are due.
func (m *Manager) processQueue() {
	now := time.Now()

	due := make([]QueuedMessage, 0)

	m.mu.Lock()
	remaining := m.queue[:0]
	for _, msg := range m.queue {
		if now.Before(msg.NextRetry) {
			remaining = append(remaining, msg)
			continue
		}
		due = append(due, msg)
	}
	m.queue = remaining
	metrics.SetQueueDepth(len(m.queue))
	m.mu.Unlock()

	for _, msg := range due {
		payload := msg.Payload
		if payload == nil {
			log.Printf("Skipping message %s for %s: missing payload", msg.ID, msg.To)
			audit.Log("queue skip %s -> %s missing payload", msg.ID, msg.To)
			continue
		}

		err := deliverFunc(msg.From, msg.To, payload.Bytes())
		if err != nil {
			msg.Attempts++
			msg.NextRetry = time.Now().Add(backoffDuration(msg.Attempts))
			msg.LastError = err.Error()
			log.Printf("Retry %d for %s in %v (message %s): %v", msg.Attempts, msg.To, time.Until(msg.NextRetry), msg.ID, err)
			metrics.DeliveryFailures.Add(1)
			audit.Log("queue retry %s -> %s attempt %d next %s error %v", msg.ID, msg.To, msg.Attempts, msg.NextRetry.Format(time.RFC3339), err)

			m.mu.Lock()
			m.queue = append(m.queue, msg)
			metrics.SetQueueDepth(len(m.queue))
			m.mu.Unlock()
			continue
		}

		msg.LastError = ""
		log.Printf("Delivered message %s to %s", msg.ID, msg.To)
		metrics.MessagesDelivered.Add(1)
		audit.Log("queue delivered %s -> %s attempts %d", msg.ID, msg.To, msg.Attempts)
	}
}

// Depth returns the current queue length.
func (m *Manager) Depth() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.queue)
}

func backoffDuration(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	base := time.Minute * time.Duration(1<<uint(min(attempts-1, 6)))
	jitter := time.Duration(rand.Int63n(int64(base / 4)))
	return base + jitter
}

func initialBackoff() time.Duration {
	return time.Second
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
