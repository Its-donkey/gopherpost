// Package auth provides SMTP authentication support.
package auth

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"sync"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAuthNotConfigured  = errors.New("authentication not configured")
	ErrMalformedAuth      = errors.New("malformed authentication data")
)

// Credentials holds configured authentication credentials.
type Credentials struct {
	mu       sync.RWMutex
	users    map[string]string // username -> password
	enabled  bool
	insecure bool // allow AUTH without TLS
}

var creds = &Credentials{
	users: make(map[string]string),
}

// LoadFromEnv loads authentication configuration from environment variables.
// SMTP_AUTH_USERS: comma-separated list of user:password pairs
// SMTP_AUTH_INSECURE: allow AUTH without TLS (default false)
func LoadFromEnv() {
	creds.mu.Lock()
	defer creds.mu.Unlock()

	creds.users = make(map[string]string)
	creds.enabled = false

	usersEnv := strings.TrimSpace(os.Getenv("SMTP_AUTH_USERS"))
	if usersEnv == "" {
		return
	}

	pairs := strings.Split(usersEnv, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			continue
		}
		username := strings.TrimSpace(parts[0])
		password := parts[1] // Don't trim password - spaces may be intentional
		if username != "" && password != "" {
			creds.users[username] = password
		}
	}

	creds.enabled = len(creds.users) > 0
	creds.insecure = strings.EqualFold(os.Getenv("SMTP_AUTH_INSECURE"), "true")
}

// Enabled returns true if authentication is configured.
func Enabled() bool {
	creds.mu.RLock()
	defer creds.mu.RUnlock()
	return creds.enabled
}

// AllowInsecure returns true if AUTH is permitted without TLS.
func AllowInsecure() bool {
	creds.mu.RLock()
	defer creds.mu.RUnlock()
	return creds.insecure
}

// Validate checks if the given credentials are valid.
func Validate(username, password string) error {
	creds.mu.RLock()
	defer creds.mu.RUnlock()

	if !creds.enabled {
		return ErrAuthNotConfigured
	}

	expected, ok := creds.users[username]
	if !ok {
		return ErrInvalidCredentials
	}

	if subtle.ConstantTimeCompare([]byte(password), []byte(expected)) != 1 {
		return ErrInvalidCredentials
	}

	return nil
}

// DecodePlain decodes SASL PLAIN authentication data.
// Format: base64(authzid\0authcid\0password) or base64(\0username\0password)
func DecodePlain(encoded string) (username, password string, err error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", ErrMalformedAuth
	}

	// PLAIN format: authzid\0authcid\0password
	// authzid is optional, authcid is the username
	parts := strings.Split(string(decoded), "\x00")
	if len(parts) != 3 {
		return "", "", ErrMalformedAuth
	}

	// Use authcid (second part) as username
	username = parts[1]
	password = parts[2]

	if username == "" {
		return "", "", ErrMalformedAuth
	}

	return username, password, nil
}

// DecodeLogin decodes SASL LOGIN username or password.
// Each is simply base64 encoded.
func DecodeLogin(encoded string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", ErrMalformedAuth
	}
	return string(decoded), nil
}

// EncodeChallenge encodes a LOGIN challenge (Username: or Password:).
func EncodeChallenge(text string) string {
	return base64.StdEncoding.EncodeToString([]byte(text))
}
