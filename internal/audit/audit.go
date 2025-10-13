package audit

import (
	"log"
	"os"
)

var debug = os.Getenv("SMTP_DEBUG") == "1"

// Log prints debug audit messages if SMTP_DEBUG=1 is set.
func Log(format string, args ...any) {
	if debug {
		log.Printf("[AUDIT] "+format, args...)
	}
}
