package email

import "testing"

func TestParseCommandAddress(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "mail from",
			input: "MAIL FROM:<USER@example.com>",
			want:  "user@example.com",
		},
		{
			name:  "rcpt to",
			input: "RCPT TO: <recipient@domain.test>",
			want:  "recipient@domain.test",
		},
		{
			name:    "missing colon",
			input:   "MAIL FROM user@example.com",
			wantErr: true,
		},
		{
			name:    "invalid address",
			input:   "MAIL FROM:<invalid>",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseCommandAddress(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestDomain(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "basic",
			input: "user@example.com",
			want:  "example.com",
		},
		{
			name:  "trailing dot removed",
			input: "user@example.com.",
			want:  "example.com",
		},
		{
			name:    "missing at",
			input:   "userexample.com",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := Domain(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}
