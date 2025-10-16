package config

import (
	"os"
)

const defaultHostname = "localhost"

// Hostname returns the hostname the SMTP server should identify as.
// Preference order: SMTP_HOSTNAME env var, system hostname, fallback.
func Hostname() string {
	if env := os.Getenv("SMTP_HOSTNAME"); env != "" {
		return env
	}
	if host, err := os.Hostname(); err == nil && host != "" {
		return host
	}
	return defaultHostname
}
