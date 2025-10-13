package delivery

import (
	"net"
	"testing"
)

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		addr   string
		domain string
		ok     bool
	}{
		{"user@example.com", "example.com", true},
		{"USER@EXAMPLE.COM.", "example.com", true},
		{"invalid", "", false},
	}

	for _, tt := range tests {
		domain, err := ExtractDomain(tt.addr)
		if tt.ok && err != nil {
			t.Fatalf("expected domain for %q, got err %v", tt.addr, err)
		}
		if !tt.ok && err == nil {
			t.Fatalf("expected error for %q", tt.addr)
		}
		if domain != tt.domain {
			t.Fatalf("expected %q, got %q", tt.domain, domain)
		}
	}
}

func TestResolveMX(t *testing.T) {
	original := mxLookup
	defer func() { mxLookup = original }()

	mxLookup = func(domain string) ([]*net.MX, error) {
		return []*net.MX{
			{Host: "slow.example.com.", Pref: 20},
			{Host: "fast.example.com.", Pref: 5},
			{Host: "backup.example.com.", Pref: 20},
		}, nil
	}

	records, err := ResolveMX("example.com")
	if err != nil {
		t.Fatalf("ResolveMX error: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}
	if records[0].Pref != 5 || records[0].Host != "fast.example.com" {
		t.Fatalf("expected primary record to be fast.example.com, got %+v", records[0])
	}
	for _, r := range records {
		if r.Host[len(r.Host)-1] == '.' {
			t.Fatalf("expected trailing dot trimmed, got %q", r.Host)
		}
	}
}
