package health

import (
	"errors"
	"expvar"
	"io"
	"log"
	"net/http"
	"time"
)

// StartHealthServer launches a lightweight HTTP server that exposes /healthz.
// The returned server can be gracefully shutdown by the caller.
func StartHealthServer(addr string) *http.Server {
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

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("[health] server error: %v", err)
		}
	}()

	return srv
}
