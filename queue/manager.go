package queue

import (
	"log"
	"math/rand"
	"runtime"
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
	queue    []QueuedMessage
	mu       sync.Mutex
	quit     chan struct{}
	once     sync.Once
	stopOnce sync.Once
	workers  int
}

// Option configures a Manager.
type Option func(*Manager)

// WithWorkers overrides the number of concurrent delivery workers.
func WithWorkers(workers int) Option {
	return func(m *Manager) {
		if workers > 0 {
			m.workers = workers
		}
	}
}

// NewManager creates a new delivery queue manager.
func NewManager(opts ...Option) *Manager {
	m := &Manager{
		queue:   make([]QueuedMessage, 0),
		quit:    make(chan struct{}),
		workers: runtime.NumCPU(),
	}
	for _, opt := range opts {
		opt(m)
	}
	if m.workers < 1 {
		m.workers = 1
	}
	return m
}

// Enqueue adds a message to the queue.
func (m *Manager) Enqueue(msg QueuedMessage) {
	if msg.Payload == nil || len(msg.Payload.Bytes()) == 0 {
		log.Printf("Discarding message %s for %s: missing payload", msg.ID, msg.To)
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if msg.Attempts == 0 && msg.NextRetry.IsZero() {
		msg.NextRetry = time.Now()
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
	m.stopOnce.Do(func() {
		close(m.quit)
	})
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

	if len(due) == 0 {
		return
	}

	workerCount := m.workers
	if workerCount < 1 {
		workerCount = 1
	}
	sem := make(chan struct{}, workerCount)
	var wg sync.WaitGroup

	for _, msg := range due {
		msg := msg
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			payload := msg.Payload
			if payload == nil {
				log.Printf("Skipping message %s for %s: missing payload", msg.ID, msg.To)
				audit.Log("queue skip %s -> %s missing payload", msg.ID, msg.To)
				return
			}

			if err := deliverFunc(msg.From, msg.To, payload.Bytes()); err != nil {
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
				return
			}

			msg.LastError = ""
			log.Printf("Delivered message %s to %s", msg.ID, msg.To)
			metrics.MessagesDelivered.Add(1)
			audit.Log("queue delivered %s -> %s attempts %d", msg.ID, msg.To, msg.Attempts)
		}()
	}

	wg.Wait()
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
