package health

import (
	"errors"
	"expvar"
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

// StartHealthServer launches a lightweight HTTP server that exposes /healthz and /metrics.
// It returns the server and listener so callers can manage shutdowns.
func StartHealthServer(addr string) (*http.Server, net.Listener, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if _, err := io.WriteString(w, "OK"); err != nil {
			log.Printf("[health] write failed: %v", err)
		}
	})
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
