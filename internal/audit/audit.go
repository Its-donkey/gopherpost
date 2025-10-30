package audit

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"

    "gopherpost/internal/config"
)

var debug atomic.Bool

func init() {
	RefreshFromEnv()
}

// RefreshFromEnv reloads the SMTP_DEBUG flag from the environment.
func RefreshFromEnv() {
	debug.Store(config.Bool("SMTP_DEBUG", false))
}

// Set enables or disables audit logging programmatically.
func Set(enabled bool) {
	debug.Store(enabled)
}

// Enabled reports the current audit logging state.
func Enabled() bool {
	return debug.Load()
}

var (
	subscribersMu sync.RWMutex
	subscribers   = make(map[uint64]chan string)
	nextSubID     atomic.Uint64
)

// Subscribe returns a channel that receives audit log lines while the context is alive.
// The returned channel is closed when ctx.Done fires.
func Subscribe(ctx context.Context, buffer int) <-chan string {
	if buffer <= 0 {
		buffer = 32
	}
	ch := make(chan string, buffer)
	id := nextSubID.Add(1)

	subscribersMu.Lock()
	subscribers[id] = ch
	subscribersMu.Unlock()

	go func() {
		<-ctx.Done()
		subscribersMu.Lock()
		delete(subscribers, id)
		close(ch)
		subscribersMu.Unlock()
	}()

	return ch
}

func broadcast(msg string) {
	subscribersMu.RLock()
	subs := make([]chan string, 0, len(subscribers))
	for _, ch := range subscribers {
		subs = append(subs, ch)
	}
	subscribersMu.RUnlock()

	for _, ch := range subs {
		func() {
			defer func() {
				recover()
			}()
			select {
			case ch <- msg:
			default:
			}
		}()
	}
}

// Log prints debug audit messages if SMTP_DEBUG=true is set.
func Log(format string, args ...any) {
	if !Enabled() {
		return
	}
	msg := fmt.Sprintf("[AUDIT] "+format, args...)
	log.Print(msg)
	broadcast(msg)
}
