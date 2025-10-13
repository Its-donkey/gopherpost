package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var baseDir = "./data/spool"

// SaveMessage stores the message on disk for inspection or retry persistence.
func SaveMessage(id string, from string, to string, data []byte) error {
	safeID, err := sanitizeComponent(id)
	if err != nil {
		return err
	}
	recipientToken := hashRecipient(to)

	dir := filepath.Join(baseDir, time.Now().UTC().Format("2006-01-02"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	filename := filepath.Join(dir, fmt.Sprintf("%s_%s.eml", safeID, recipientToken))
	payload := append([]byte(nil), data...)
	return os.WriteFile(filename, payload, 0o600)
}

// SetBaseDir allows overriding the storage location (useful for tests or configuration).
func SetBaseDir(dir string) {
	baseDir = dir
}

func sanitizeComponent(v string) (string, error) {
	if strings.ContainsAny(v, "/\\") || strings.Contains(v, "..") {
		return "", errors.New("invalid identifier")
	}
	v = strings.TrimSpace(v)
	if v == "" {
		return "", errors.New("empty identifier")
	}
	return v, nil
}

func hashRecipient(addr string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(addr))))
	return hex.EncodeToString(sum[:8])
}
