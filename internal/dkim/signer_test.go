package dkim

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
)

func TestLoadFromEnvDisabled(t *testing.T) {
	t.Setenv("SMTP_DKIM_SELECTOR", "")
	t.Setenv("SMTP_DKIM_KEY_PATH", "")
	t.Setenv("SMTP_DKIM_PRIVATE_KEY", "")
	t.Setenv("SMTP_DKIM_DOMAIN", "")

	signer, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv returned error: %v", err)
	}
	if signer != nil {
		t.Fatalf("expected nil signer when no env values are set")
	}
}

func TestLoadFromEnvInlineKey(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	pemData := pem.EncodeToMemory(block)

	t.Setenv("SMTP_DKIM_SELECTOR", "test")
	t.Setenv("SMTP_DKIM_PRIVATE_KEY", string(pemData))
	t.Setenv("SMTP_DKIM_DOMAIN", "example.com")

	signer, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv returned error: %v", err)
	}
	if signer == nil {
		t.Fatalf("expected signer when env values are set")
	}
}

func TestSignerSignAddsHeader(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer := &Signer{
		selector:   "test",
		key:        key,
		headerKeys: []string{"from", "subject"},
	}

	raw := "From: sender@example.com\nSubject: Test\n\nBody\n"
	signed, err := signer.Sign([]byte(raw), "sender@example.com")
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}
	payload := string(signed)
	if !strings.Contains(payload, "DKIM-Signature:") {
		t.Fatalf("expected DKIM-Signature header, got %q", payload)
	}
	if !strings.Contains(payload, "\r\nFrom: sender@example.com") {
		t.Fatalf("expected CRLF normalized output, got %q", payload)
	}
}

func TestSignerSkipsWhenHeaderPresent(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer := &Signer{
		selector:   "test",
		key:        key,
		headerKeys: []string{"from"},
	}

	raw := "DKIM-Signature: existing\r\nFrom: sender@example.com\r\n\r\nBody\r\n"
	signed, err := signer.Sign([]byte(raw), "sender@example.com")
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}
	if string(signed) != raw {
		t.Fatalf("expected message to remain unchanged when signature exists")
	}
}
