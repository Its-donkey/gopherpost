package queue

import "time"

// Payload wraps immutable SMTP message data shared across recipients.
type Payload struct {
	Data []byte
}

// NewPayload creates an immutable payload from raw message bytes.
func NewPayload(data []byte) *Payload {
	return &Payload{Data: data}
}

// Bytes returns the underlying immutable message.
func (p *Payload) Bytes() []byte {
	if p == nil {
		return nil
	}
	return p.Data
}

// QueuedMessage represents a message waiting to be delivered to a single recipient.
// Payload must never be mutated after enqueueing.
type QueuedMessage struct {
	ID        string
	From      string
	To        string
	Payload   *Payload
	Attempts  int
	NextRetry time.Time
	LastError string
}
