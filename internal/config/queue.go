package config

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

// QueueWorkers returns the configured number of concurrent delivery workers.
// Defaults to the number of logical CPUs when unset or invalid.
func QueueWorkers() int {
	value := strings.TrimSpace(os.Getenv("SMTP_QUEUE_WORKERS"))
	if value == "" {
		return runtime.NumCPU()
	}
	workers, err := strconv.Atoi(value)
	if err != nil || workers < 1 {
		return runtime.NumCPU()
	}
	return workers
}
