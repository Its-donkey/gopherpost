package config

import (
	"os"
	"strings"
)

// Bool reads an environment variable and returns a boolean value.
// Only "true" or "false" (case-insensitive) are recognised; any other
// value results in the provided default.
func Bool(key string, defaultValue bool) bool {
	val := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	switch val {
	case "":
		return defaultValue
	case "true":
		return true
	case "false":
		return false
	default:
		return defaultValue
	}
}
