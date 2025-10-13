package metrics

import "expvar"

var (
	MessagesQueued    = expvar.NewInt("smtp_messages_queued_total")
	MessagesDelivered = expvar.NewInt("smtp_messages_delivered_total")
	DeliveryFailures  = expvar.NewInt("smtp_delivery_failures_total")
	queueDepth        = expvar.NewInt("smtp_queue_depth")
	sessionsActive    = expvar.NewInt("smtp_sessions_active")
)

// SetQueueDepth records the current queue depth.
func SetQueueDepth(n int) {
	queueDepth.Set(int64(n))
}

// IncSessions increments the active session count.
func IncSessions() {
	sessionsActive.Add(1)
}

// DecSessions decrements the active session count.
func DecSessions() {
	sessionsActive.Add(-1)
}

// ResetForTests clears counters; intended for use in tests only.
func ResetForTests() {
	MessagesQueued.Set(0)
	MessagesDelivered.Set(0)
	DeliveryFailures.Set(0)
	queueDepth.Set(0)
	sessionsActive.Set(0)
}
