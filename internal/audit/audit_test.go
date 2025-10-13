package audit

import "testing"

func TestSetEnabled(t *testing.T) {
	Set(false)
	if Enabled() {
		t.Fatalf("expected disabled after Set(false)")
	}
	Set(true)
	if !Enabled() {
		t.Fatalf("expected enabled after Set(true)")
	}
}

func TestRefreshFromEnv(t *testing.T) {
	t.Setenv("SMTP_DEBUG", "1")
	Set(false)
	RefreshFromEnv()
	if !Enabled() {
		t.Fatalf("expected enabled after RefreshFromEnv")
	}
	t.Setenv("SMTP_DEBUG", "0")
	RefreshFromEnv()
	if Enabled() {
		t.Fatalf("expected disabled when SMTP_DEBUG=0")
	}
}
