package audit

import (
	"log"
	"sync/atomic"

	"smtpserver/internal/config"
)

var debug atomic.Bool

func init() {
	RefreshFromEnv()
}

// RefreshFromEnv reloads the SMTP_DEBUG flag from the environment.
func RefreshFromEnv() {
	debug.Store(config.Bool("SMTP_DEBUG", false))
}

// Set enables or disables audit logging programmatically.
func Set(enabled bool) {
	debug.Store(enabled)
}

// Enabled reports the current audit logging state.
func Enabled() bool {
	return debug.Load()
}

// Log prints debug audit messages if SMTP_DEBUG=true is set.
func Log(format string, args ...any) {
	if !Enabled() {
		return
	}
	log.Printf("[AUDIT] "+format, args...)
}
