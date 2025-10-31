package config

import (
	"runtime"
	"testing"
)

func TestBool(t *testing.T) {
	t.Setenv("BOOL_TRUE", "true")
	t.Setenv("BOOL_FALSE", "false")
	t.Setenv("BOOL_NOISE", "yes")

	if !Bool("BOOL_TRUE", false) {
		t.Fatalf("expected true")
	}
	if Bool("BOOL_FALSE", true) {
		t.Fatalf("expected false override")
	}
	if !Bool("BOOL_MISSING", true) {
		t.Fatalf("expected default true for missing key")
	}
	if Bool("BOOL_NOISE", true) != true {
		t.Fatalf("unexpected override for unsupported values")
	}
}

func TestQueueWorkers(t *testing.T) {
	t.Setenv("SMTP_QUEUE_WORKERS", "")
	expectedDefault := runtime.NumCPU()
	if got := QueueWorkers(); got != expectedDefault {
		t.Fatalf("expected default workers %d, got %d", expectedDefault, got)
	}

	t.Setenv("SMTP_QUEUE_WORKERS", "3")
	if got := QueueWorkers(); got != 3 {
		t.Fatalf("expected configured workers 3, got %d", got)
	}

	t.Setenv("SMTP_QUEUE_WORKERS", "-5")
	if got := QueueWorkers(); got != expectedDefault {
		t.Fatalf("expected fallback to default for negative value, got %d", got)
	}

	t.Setenv("SMTP_QUEUE_WORKERS", "noise")
	if got := QueueWorkers(); got != expectedDefault {
		t.Fatalf("expected fallback to default for invalid value, got %d", got)
	}
}
