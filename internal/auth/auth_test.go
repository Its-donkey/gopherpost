package auth

import (
	"encoding/base64"
	"os"
	"testing"
)

func TestLoadFromEnv(t *testing.T) {
	// Save and restore env
	origUsers := os.Getenv("SMTP_AUTH_USERS")
	origInsecure := os.Getenv("SMTP_AUTH_INSECURE")
	defer func() {
		os.Setenv("SMTP_AUTH_USERS", origUsers)
		os.Setenv("SMTP_AUTH_INSECURE", origInsecure)
	}()

	// Test with users configured
	os.Setenv("SMTP_AUTH_USERS", "alice:secret123,bob:password456")
	os.Setenv("SMTP_AUTH_INSECURE", "false")
	LoadFromEnv()

	if !Enabled() {
		t.Error("expected auth to be enabled")
	}
	if AllowInsecure() {
		t.Error("expected insecure to be false")
	}

	// Validate correct credentials
	if err := Validate("alice", "secret123"); err != nil {
		t.Errorf("valid credentials rejected: %v", err)
	}
	if err := Validate("bob", "password456"); err != nil {
		t.Errorf("valid credentials rejected: %v", err)
	}

	// Validate incorrect credentials
	if err := Validate("alice", "wrong"); err != ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
	if err := Validate("unknown", "secret123"); err != ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}

	// Test insecure mode
	os.Setenv("SMTP_AUTH_INSECURE", "true")
	LoadFromEnv()
	if !AllowInsecure() {
		t.Error("expected insecure to be true")
	}

	// Test empty config
	os.Setenv("SMTP_AUTH_USERS", "")
	LoadFromEnv()
	if Enabled() {
		t.Error("expected auth to be disabled with empty users")
	}
}

func TestDecodePlain(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantUser string
		wantPass string
		wantErr  bool
	}{
		{
			name:     "valid with authzid",
			input:    base64.StdEncoding.EncodeToString([]byte("authzid\x00username\x00password")),
			wantUser: "username",
			wantPass: "password",
		},
		{
			name:     "valid without authzid",
			input:    base64.StdEncoding.EncodeToString([]byte("\x00username\x00password")),
			wantUser: "username",
			wantPass: "password",
		},
		{
			name:    "invalid base64",
			input:   "not-valid-base64!!!",
			wantErr: true,
		},
		{
			name:    "wrong format",
			input:   base64.StdEncoding.EncodeToString([]byte("just-one-part")),
			wantErr: true,
		},
		{
			name:    "empty username",
			input:   base64.StdEncoding.EncodeToString([]byte("authzid\x00\x00password")),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, pass, err := DecodePlain(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if user != tt.wantUser {
				t.Errorf("user = %q, want %q", user, tt.wantUser)
			}
			if pass != tt.wantPass {
				t.Errorf("pass = %q, want %q", pass, tt.wantPass)
			}
		})
	}
}

func TestDecodeLogin(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("testuser"))
	decoded, err := DecodeLogin(encoded)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if decoded != "testuser" {
		t.Errorf("got %q, want %q", decoded, "testuser")
	}

	_, err = DecodeLogin("invalid!!!")
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestEncodeChallenge(t *testing.T) {
	result := EncodeChallenge("Username:")
	expected := base64.StdEncoding.EncodeToString([]byte("Username:"))
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}
