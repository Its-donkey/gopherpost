package health

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestStartHealthServer(t *testing.T) {
	server, listener, err := StartHealthServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("StartHealthServer returned error: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		_ = listener.Close()
	}()

	baseURL := "http://" + listener.Addr().String()

	resp, err := http.Get(baseURL + "/healthz")
	if err != nil {
		t.Fatalf("health request error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	resp, err = http.Get(baseURL + "/metrics")
	if err != nil {
		t.Fatalf("metrics request error: %v", err)
	}
	defer resp.Body.Close()
	var metrics map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
		t.Fatalf("decoding metrics failed: %v", err)
	}
	if len(metrics) == 0 {
		t.Fatalf("expected metrics payload")
	}
}
