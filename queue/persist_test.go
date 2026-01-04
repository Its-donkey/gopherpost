package queue

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestQueuePersistence(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	SetPersistDir(tmpDir)

	// Create a test message file
	msgPath := filepath.Join(tmpDir, "test_msg.eml")
	if err := os.WriteFile(msgPath, []byte("test message body"), 0o600); err != nil {
		t.Fatalf("failed to create test message: %v", err)
	}

	// Create manager and enqueue a message
	m1 := NewManager()
	msg := QueuedMessage{
		ID:        "test-123",
		From:      "sender@example.com",
		To:        "rcpt@example.net",
		FilePath:  msgPath,
		Payload:   NewPayload([]byte("test message body")),
		Attempts:  2,
		NextRetry: time.Now().Add(time.Hour),
		LastError: "temporary failure",
	}
	m1.Enqueue(msg)

	if m1.Depth() != 1 {
		t.Fatalf("expected depth 1, got %d", m1.Depth())
	}

	// Verify queue file exists
	queueFile := filepath.Join(tmpDir, "queue.json")
	if _, err := os.Stat(queueFile); os.IsNotExist(err) {
		t.Fatalf("queue.json was not created")
	}

	// Create new manager and load queue
	m2 := NewManager()
	if err := m2.LoadQueue(); err != nil {
		t.Fatalf("LoadQueue failed: %v", err)
	}

	if m2.Depth() != 1 {
		t.Fatalf("expected restored depth 1, got %d", m2.Depth())
	}

	// Verify message fields were restored
	m2.mu.Lock()
	restored := m2.queue[0]
	m2.mu.Unlock()

	if restored.ID != "test-123" {
		t.Errorf("ID mismatch: got %s", restored.ID)
	}
	if restored.From != "sender@example.com" {
		t.Errorf("From mismatch: got %s", restored.From)
	}
	if restored.To != "rcpt@example.net" {
		t.Errorf("To mismatch: got %s", restored.To)
	}
	if restored.Attempts != 2 {
		t.Errorf("Attempts mismatch: got %d", restored.Attempts)
	}
	if restored.LastError != "temporary failure" {
		t.Errorf("LastError mismatch: got %s", restored.LastError)
	}
	if string(restored.Payload.Bytes()) != "test message body" {
		t.Errorf("Payload mismatch: got %s", string(restored.Payload.Bytes()))
	}
}

func TestQueuePersistenceSkipsMissingFiles(t *testing.T) {
	tmpDir := t.TempDir()
	SetPersistDir(tmpDir)

	// Create a manager and enqueue a message
	m1 := NewManager()
	msg := QueuedMessage{
		ID:       "missing-file",
		From:     "sender@example.com",
		To:       "rcpt@example.net",
		FilePath: filepath.Join(tmpDir, "nonexistent.eml"),
		Payload:  NewPayload([]byte("body")),
	}

	// Manually save queue state without the actual file
	m1.mu.Lock()
	m1.queue = append(m1.queue, msg)
	m1.mu.Unlock()
	if err := m1.SaveQueue(); err != nil {
		t.Fatalf("SaveQueue failed: %v", err)
	}

	// Load into new manager - should skip the missing file entry
	m2 := NewManager()
	if err := m2.LoadQueue(); err != nil {
		t.Fatalf("LoadQueue failed: %v", err)
	}

	if m2.Depth() != 0 {
		t.Errorf("expected depth 0 (missing file skipped), got %d", m2.Depth())
	}
}

func TestLoadQueueNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	SetPersistDir(tmpDir)

	m := NewManager()
	if err := m.LoadQueue(); err != nil {
		t.Errorf("LoadQueue should not error on missing file: %v", err)
	}
	if m.Depth() != 0 {
		t.Errorf("expected empty queue, got depth %d", m.Depth())
	}
}
