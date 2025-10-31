package health

import (
	"errors"
	"expvar"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

    "gopherpost/internal/audit"
)

// StartHealthServer launches a lightweight HTTP server that exposes /healthz and /metrics.
// It returns the server and listener so callers can manage shutdowns.
func StartHealthServer(addr string) (*http.Server, net.Listener, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler)
	mux.Handle("/metrics", expvar.Handler())

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, err
	}

	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("[health] server error: %v", err)
		}
	}()

	return srv, ln, nil
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	if !audit.Enabled() {
		if _, err := io.WriteString(w, "OK"); err != nil {
			log.Printf("[health] write failed: %v", err)
		}
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		if _, err := io.WriteString(w, "OK\n(flushing unsupported)"); err != nil {
			log.Printf("[health] write failed: %v", err)
		}
		return
	}

	ctx := r.Context()
	if _, err := io.WriteString(w, "OK\n"); err != nil {
		log.Printf("[health] write failed: %v", err)
		return
	}
	flusher.Flush()

	lines := audit.Subscribe(ctx, 64)
	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-lines:
			if !ok {
				return
			}
			if _, err := fmt.Fprintf(w, "%s\n", line); err != nil {
				log.Printf("[health] write failed: %v", err)
				return
			}
			flusher.Flush()
		}
	}
}
