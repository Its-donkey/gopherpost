package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveMessage(t *testing.T) {
	tmp := t.TempDir()
	SetBaseDir(tmp)
	t.Cleanup(func() { SetBaseDir("./data/spool") })

	if err := SaveMessage("abc123", "from@example.com", "recipient@example.com", []byte("body")); err != nil {
		t.Fatalf("SaveMessage returned error: %v", err)
	}

	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 directory, got %d", len(entries))
	}

	dayDir := filepath.Join(tmp, entries[0].Name())
	files, err := os.ReadDir(dayDir)
	if err != nil {
		t.Fatalf("ReadDir day: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	name := files[0].Name()
	if strings.Contains(name, "recipient@example.com") {
		t.Fatalf("expected recipient to be hashed, got %q", name)
	}

	data, err := os.ReadFile(filepath.Join(dayDir, name))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "body" {
		t.Fatalf("expected message body, got %q", string(data))
	}
}

func TestSaveMessageSanitizesID(t *testing.T) {
	tmp := t.TempDir()
	SetBaseDir(tmp)
	t.Cleanup(func() { SetBaseDir("./data/spool") })

	if err := SaveMessage("../bad", "from@example.com", "recipient@example.com", []byte("body")); err == nil {
		t.Fatalf("expected error for invalid identifier")
	}
}
