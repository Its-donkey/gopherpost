package metrics

import "testing"

func TestMetricsCounters(t *testing.T) {
	ResetForTests()
	if MessagesQueued.Value() != 0 {
		t.Fatalf("expected zero initial MessagesQueued")
	}

	MessagesQueued.Add(2)
	SetQueueDepth(5)
	IncSessions()
	DecSessions()
	if MessagesQueued.Value() != 2 {
		t.Fatalf("expected MessagesQueued=2, got %d", MessagesQueued.Value())
	}
	if sessionsActive.Value() != 0 {
		t.Fatalf("expected sessionsActive=0, got %d", sessionsActive.Value())
	}

	ResetForTests()
	if queueDepth.Value() != 0 {
		t.Fatalf("expected queueDepth reset to 0")
	}
}
