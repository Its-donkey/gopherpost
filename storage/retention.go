package storage

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	retentionDays   = 7 // default 7 days
	retentionMu     sync.RWMutex
	cleanupInterval = 1 * time.Hour
)

// SetRetentionDays configures how many days to retain stored messages.
// Messages older than this are eligible for cleanup.
func SetRetentionDays(days int) {
	retentionMu.Lock()
	defer retentionMu.Unlock()
	if days > 0 {
		retentionDays = days
	}
}

// RetentionDays returns the current retention period in days.
func RetentionDays() int {
	retentionMu.RLock()
	defer retentionMu.RUnlock()
	return retentionDays
}

// LoadRetentionFromEnv loads retention settings from environment variables.
func LoadRetentionFromEnv() {
	if val := strings.TrimSpace(os.Getenv("SMTP_RETENTION_DAYS")); val != "" {
		if days, err := strconv.Atoi(val); err == nil && days > 0 {
			SetRetentionDays(days)
		}
	}
}

// CleanupOldMessages removes message files older than the retention period.
// Returns the number of files removed and any error encountered.
func CleanupOldMessages() (int, error) {
	retentionMu.RLock()
	days := retentionDays
	retentionMu.RUnlock()

	// Use start of today to ensure consistent date comparison
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	cutoff := today.AddDate(0, 0, -days)
	removed := 0

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Directory names are YYYY-MM-DD
		dirDate, err := time.Parse("2006-01-02", entry.Name())
		if err != nil {
			continue // Skip non-date directories
		}

		if dirDate.Before(cutoff) {
			dirPath := filepath.Join(baseDir, entry.Name())
			count, err := removeDir(dirPath)
			if err != nil {
				log.Printf("Failed to remove old spool directory %s: %v", dirPath, err)
				continue
			}
			removed += count
		}
	}

	return removed, nil
}

// removeDir removes a directory and returns the count of files removed.
func removeDir(path string) (int, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0, err
	}

	count := len(entries)
	if err := os.RemoveAll(path); err != nil {
		return 0, err
	}

	return count, nil
}

// SpoolStats returns statistics about the current spool.
type SpoolStats struct {
	TotalFiles   int
	TotalBytes   int64
	OldestDate   string
	NewestDate   string
	DirCount     int
	RetentionDay int
}

// GetSpoolStats returns statistics about the message spool.
func GetSpoolStats() (SpoolStats, error) {
	stats := SpoolStats{
		RetentionDay: RetentionDays(),
	}

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return stats, nil
		}
		return stats, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Validate it's a date directory
		if _, err := time.Parse("2006-01-02", entry.Name()); err != nil {
			continue
		}

		stats.DirCount++
		if stats.OldestDate == "" || entry.Name() < stats.OldestDate {
			stats.OldestDate = entry.Name()
		}
		if entry.Name() > stats.NewestDate {
			stats.NewestDate = entry.Name()
		}

		dirPath := filepath.Join(baseDir, entry.Name())
		files, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}

		for _, f := range files {
			if f.IsDir() {
				continue
			}
			stats.TotalFiles++
			if info, err := f.Info(); err == nil {
				stats.TotalBytes += info.Size()
			}
		}
	}

	return stats, nil
}

// StartRetentionCleanup starts a background goroutine that periodically
// cleans up old messages. Returns a channel that can be closed to stop cleanup.
func StartRetentionCleanup() chan struct{} {
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()

		// Run initial cleanup
		if count, err := CleanupOldMessages(); err != nil {
			log.Printf("Retention cleanup error: %v", err)
		} else if count > 0 {
			log.Printf("Retention cleanup removed %d old messages", count)
		}

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if count, err := CleanupOldMessages(); err != nil {
					log.Printf("Retention cleanup error: %v", err)
				} else if count > 0 {
					log.Printf("Retention cleanup removed %d old messages", count)
				}
			}
		}
	}()
	return stop
}

// ListMessages returns message files in the spool directory.
// If date is empty, lists all dates. Otherwise filters to specific date.
func ListMessages(date string) ([]string, error) {
	var messages []string

	if date != "" {
		// List specific date
		dirPath := filepath.Join(baseDir, date)
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			if os.IsNotExist(err) {
				return messages, nil
			}
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".eml") {
				messages = append(messages, filepath.Join(date, entry.Name()))
			}
		}
		return messages, nil
	}

	// List all dates
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return messages, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := time.Parse("2006-01-02", entry.Name()); err != nil {
			continue
		}
		subEntries, err := os.ReadDir(filepath.Join(baseDir, entry.Name()))
		if err != nil {
			continue
		}
		for _, f := range subEntries {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".eml") {
				messages = append(messages, filepath.Join(entry.Name(), f.Name()))
			}
		}
	}

	return messages, nil
}
