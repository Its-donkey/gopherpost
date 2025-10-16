package config

import "testing"

func TestBool(t *testing.T) {
	t.Setenv("BOOL_TRUE", "true")
	t.Setenv("BOOL_FALSE", "false")
	t.Setenv("BOOL_NOISE", "yes")

	if !Bool("BOOL_TRUE", false) {
		t.Fatalf("expected true")
	}
	if Bool("BOOL_FALSE", true) {
		t.Fatalf("expected false override")
	}
	if !Bool("BOOL_MISSING", true) {
		t.Fatalf("expected default true for missing key")
	}
	if Bool("BOOL_NOISE", true) != true {
		t.Fatalf("unexpected override for unsupported values")
	}
}
