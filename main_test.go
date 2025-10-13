package main

import (
	"net"
	"testing"
)

func TestIsLocalhost(t *testing.T) {
	if !isLocalhost(&net.TCPAddr{IP: net.ParseIP("127.0.0.1")}) {
		t.Fatalf("expected loopback to be accepted")
	}
	if isLocalhost(&net.TCPAddr{IP: net.ParseIP("10.0.0.1")}) {
		t.Fatalf("expected non-loopback to be rejected")
	}
	if isLocalhost(&net.IPAddr{IP: net.ParseIP("127.0.0.1")}) {
		t.Fatalf("expected non-TCP addr to be rejected")
	}
}

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
