package queue

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// persistedEntry represents a queue entry stored on disk.
type persistedEntry struct {
	ID        string    `json:"id"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	FilePath  string    `json:"file_path"`
	Attempts  int       `json:"attempts"`
	NextRetry time.Time `json:"next_retry"`
	LastError string    `json:"last_error,omitempty"`
}

// persistedQueue represents the entire queue state on disk.
type persistedQueue struct {
	Entries []persistedEntry `json:"entries"`
}

var (
	persistDir  = "./data/spool"
	persistFile = "queue.json"
	persistMu   sync.Mutex
)

// SetPersistDir overrides the directory used for queue persistence.
func SetPersistDir(dir string) {
	persistMu.Lock()
	defer persistMu.Unlock()
	persistDir = dir
}

// persistPath returns the full path to the queue state file.
func persistPath() string {
	persistMu.Lock()
	defer persistMu.Unlock()
	return filepath.Join(persistDir, persistFile)
}

// SaveQueue persists the current queue state to disk.
func (m *Manager) SaveQueue() error {
	m.mu.Lock()
	entries := make([]persistedEntry, 0, len(m.queue))
	for _, msg := range m.queue {
		entries = append(entries, persistedEntry{
			ID:        msg.ID,
			From:      msg.From,
			To:        msg.To,
			FilePath:  msg.FilePath,
			Attempts:  msg.Attempts,
			NextRetry: msg.NextRetry,
			LastError: msg.LastError,
		})
	}
	m.mu.Unlock()

	pq := persistedQueue{Entries: entries}
	data, err := json.MarshalIndent(pq, "", "  ")
	if err != nil {
		return err
	}

	path := persistPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// Write atomically via temp file
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadQueue restores the queue state from disk.
// Messages are rehydrated from their stored .eml files.
func (m *Manager) LoadQueue() error {
	path := persistPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No persisted state
		}
		return err
	}

	var pq persistedQueue
	if err := json.Unmarshal(data, &pq); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, entry := range pq.Entries {
		// Read message content from stored file
		content, err := os.ReadFile(entry.FilePath)
		if err != nil {
			// File missing - skip this entry (already delivered or cleaned up)
			continue
		}

		msg := QueuedMessage{
			ID:        entry.ID,
			From:      entry.From,
			To:        entry.To,
			FilePath:  entry.FilePath,
			Payload:   NewPayload(content),
			Attempts:  entry.Attempts,
			NextRetry: entry.NextRetry,
			LastError: entry.LastError,
		}
		m.queue = append(m.queue, msg)
	}

	return nil
}

// ClearPersistedQueue removes the persisted queue file.
func ClearPersistedQueue() error {
	path := persistPath()
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
