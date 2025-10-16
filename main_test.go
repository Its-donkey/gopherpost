package main

import (
	"net"
	"strings"
	"testing"
)

func TestShortID(t *testing.T) {
	id1 := shortID()
	id2 := shortID()
	if len(id1) != 16 || len(id2) != 16 {
		t.Fatalf("expected 16 hex chars, got %d and %d", len(id1), len(id2))
	}
	if id1 == id2 {
		t.Fatalf("expected unique ids, got %s twice", id1)
	}
}

func TestOverridePort(t *testing.T) {
	tests := []struct {
		addr     string
		port     string
		expected string
	}{
		{":8080", "9090", ":9090"},
		{"127.0.0.1:8080", "9090", "127.0.0.1:9090"},
		{"0.0.0.0:8080", ":9090", "0.0.0.0:9090"},
		{"localhost", "9090", "localhost:9090"},
		{"[::1]:8080", "9090", "[::1]:9090"},
	}

	for _, tt := range tests {
		if got := overridePort(tt.addr, tt.port); got != tt.expected {
			t.Fatalf("overridePort(%q, %q) = %q, want %q", tt.addr, tt.port, got, tt.expected)
		}
	}
}

func TestSummarizeCommand(t *testing.T) {
	long := strings.Repeat("A", 200)
	if got := summarizeCommand("  " + long + "  "); len(got) != 120 || !strings.HasSuffix(got, "...") {
		t.Fatalf("expected truncated summary, got %q (len=%d)", got, len(got))
	}
	if got := summarizeCommand("NOOP"); got != "NOOP" {
		t.Fatalf("expected NOOP, got %q", got)
	}
}

func TestConnAllowed(t *testing.T) {
	t.Setenv("SMTP_ALLOW_HOSTS", "example.com")
	t.Setenv("SMTP_ALLOW_NETWORKS", "")
	allowed := connAllowed(&net.TCPAddr{IP: net.ParseIP("203.0.113.10"), Port: 25, Zone: ""})
	if allowed {
		t.Fatalf("expected connection to be blocked without matching host")
	}
	t.Setenv("SMTP_ALLOW_NETWORKS", "203.0.113.0/24")
	if !connAllowed(&net.TCPAddr{IP: net.ParseIP("203.0.113.10")}) {
		t.Fatalf("expected connection within network to be allowed")
	}
}
