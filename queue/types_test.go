package queue

import "testing"

func TestPayload(t *testing.T) {
	payload := NewPayload([]byte("data"))
	if string(payload.Bytes()) != "data" {
		t.Fatalf("unexpected payload bytes")
	}
	if NewPayload(nil).Bytes() != nil {
		t.Fatalf("expected nil bytes when constructed with nil")
	}
}
