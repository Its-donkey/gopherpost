package queue

import "time"

// QueuedMessage represents a message waiting to be delivered.
type QueuedMessage struct {
	From      string
	To        string
	Data      []byte
	Attempts  int
	NextRetry time.Time
}
