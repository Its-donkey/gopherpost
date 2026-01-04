package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanupOldMessages(t *testing.T) {
	tmpDir := t.TempDir()
	originalBaseDir := baseDir
	baseDir = tmpDir
	defer func() { baseDir = originalBaseDir }()

	SetRetentionDays(3)

	// Create directories for different dates
	today := time.Now().UTC()
	dirs := []struct {
		date    time.Time
		files   int
		deleted bool
	}{
		{today, 2, false},                     // today - keep
		{today.AddDate(0, 0, -1), 1, false},   // yesterday - keep
		{today.AddDate(0, 0, -2), 1, false},   // 2 days ago - keep
		{today.AddDate(0, 0, -3), 3, false},   // 3 days ago - keep (boundary)
		{today.AddDate(0, 0, -4), 2, true},    // 4 days ago - delete
		{today.AddDate(0, 0, -10), 5, true},   // 10 days ago - delete
	}

	for _, d := range dirs {
		dirPath := filepath.Join(tmpDir, d.date.Format("2006-01-02"))
		if err := os.MkdirAll(dirPath, 0o755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
		for i := 0; i < d.files; i++ {
			fpath := filepath.Join(dirPath, "msg"+string(rune('a'+i))+".eml")
			if err := os.WriteFile(fpath, []byte("test"), 0o600); err != nil {
				t.Fatalf("failed to create file: %v", err)
			}
		}
	}

	removed, err := CleanupOldMessages()
	if err != nil {
		t.Fatalf("CleanupOldMessages failed: %v", err)
	}

	expectedRemoved := 2 + 5 // 4 days ago + 10 days ago files
	if removed != expectedRemoved {
		t.Errorf("expected %d removed, got %d", expectedRemoved, removed)
	}

	// Verify directories
	for _, d := range dirs {
		dirPath := filepath.Join(tmpDir, d.date.Format("2006-01-02"))
		_, err := os.Stat(dirPath)
		if d.deleted {
			if !os.IsNotExist(err) {
				t.Errorf("directory %s should have been deleted", d.date.Format("2006-01-02"))
			}
		} else {
			if err != nil {
				t.Errorf("directory %s should exist: %v", d.date.Format("2006-01-02"), err)
			}
		}
	}
}

func TestGetSpoolStats(t *testing.T) {
	tmpDir := t.TempDir()
	originalBaseDir := baseDir
	baseDir = tmpDir
	defer func() { baseDir = originalBaseDir }()

	// Create some test data
	dates := []string{"2024-01-01", "2024-01-02", "2024-01-03"}
	for _, date := range dates {
		dirPath := filepath.Join(tmpDir, date)
		if err := os.MkdirAll(dirPath, 0o755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
		for i := 0; i < 3; i++ {
			fpath := filepath.Join(dirPath, "msg"+string(rune('a'+i))+".eml")
			if err := os.WriteFile(fpath, []byte("test content"), 0o600); err != nil {
				t.Fatalf("failed to create file: %v", err)
			}
		}
	}

	stats, err := GetSpoolStats()
	if err != nil {
		t.Fatalf("GetSpoolStats failed: %v", err)
	}

	if stats.DirCount != 3 {
		t.Errorf("expected 3 dirs, got %d", stats.DirCount)
	}
	if stats.TotalFiles != 9 {
		t.Errorf("expected 9 files, got %d", stats.TotalFiles)
	}
	if stats.OldestDate != "2024-01-01" {
		t.Errorf("expected oldest 2024-01-01, got %s", stats.OldestDate)
	}
	if stats.NewestDate != "2024-01-03" {
		t.Errorf("expected newest 2024-01-03, got %s", stats.NewestDate)
	}
}

func TestListMessages(t *testing.T) {
	tmpDir := t.TempDir()
	originalBaseDir := baseDir
	baseDir = tmpDir
	defer func() { baseDir = originalBaseDir }()

	// Create test messages
	dirPath := filepath.Join(tmpDir, "2024-01-15")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	for _, name := range []string{"msg1.eml", "msg2.eml"} {
		if err := os.WriteFile(filepath.Join(dirPath, name), []byte("test"), 0o600); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}

	// List specific date
	msgs, err := ListMessages("2024-01-15")
	if err != nil {
		t.Fatalf("ListMessages failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages, got %d", len(msgs))
	}

	// List all
	msgs, err = ListMessages("")
	if err != nil {
		t.Fatalf("ListMessages failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages, got %d", len(msgs))
	}

	// List nonexistent date
	msgs, err = ListMessages("2024-12-31")
	if err != nil {
		t.Fatalf("ListMessages failed: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

func TestRetentionDays(t *testing.T) {
	original := RetentionDays()
	defer SetRetentionDays(original)

	SetRetentionDays(14)
	if got := RetentionDays(); got != 14 {
		t.Errorf("expected 14, got %d", got)
	}

	SetRetentionDays(0) // Should not change
	if got := RetentionDays(); got != 14 {
		t.Errorf("expected 14 (unchanged), got %d", got)
	}
}
