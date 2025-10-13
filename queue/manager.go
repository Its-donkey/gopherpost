package queue

import (
	"log"
	"math/rand"
	"sync"
	"time"

	"smtpserver/delivery"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Manager manages a queue of outgoing messages with retry logic.
type Manager struct {
	queue []QueuedMessage
	mu    sync.Mutex
	quit  chan struct{}
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
	m.mu.Lock()
	defer m.mu.Unlock()
	if msg.Attempts == 0 {
		msg.NextRetry = time.Now().Add(initialBackoff())
	}
	m.queue = append(m.queue, msg)
	log.Printf("Queued message for %s (attempt %d)", msg.To, msg.Attempts)
}

// Start starts the queue processor in a background goroutine.
func (m *Manager) Start() {
	go func() {
		for {
			select {
			case <-m.quit:
				return
			default:
				m.processQueue()
				time.Sleep(5 * time.Second)
			}
		}
	}()
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
	m.mu.Unlock()

	for _, msg := range due {
		err := delivery.DeliverMessage(msg.From, msg.To, msg.Data)
		if err != nil {
			msg.Attempts++
			msg.NextRetry = time.Now().Add(backoffDuration(msg.Attempts))
			log.Printf("Retry %d for %s in %v: %v", msg.Attempts, msg.To, time.Until(msg.NextRetry), err)

			m.mu.Lock()
			m.queue = append(m.queue, msg)
			m.mu.Unlock()
			continue
		}

		log.Printf("Delivered message to %s", msg.To)
	}
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
