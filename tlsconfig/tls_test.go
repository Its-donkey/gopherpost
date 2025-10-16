package tlsconfig

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadTLSConfigDisabled(t *testing.T) {
	t.Setenv("SMTP_TLS_DISABLE", "true")
	t.Setenv("SMTP_TLS_CERT", "")
	t.Setenv("SMTP_TLS_KEY", "")

	conf, err := LoadTLSConfig()
	if !errors.Is(err, ErrTLSDisabled) {
		t.Fatalf("expected ErrTLSDisabled, got %v", err)
	}
	if conf != nil {
		t.Fatalf("expected nil config when disabled")
	}
}

func TestLoadTLSConfigEphemeral(t *testing.T) {
	t.Setenv("SMTP_TLS_DISABLE", "false")
	t.Setenv("SMTP_TLS_CERT", "")
	t.Setenv("SMTP_TLS_KEY", "")

	conf, err := LoadTLSConfig()
	if err != nil {
		t.Fatalf("LoadTLSConfig error: %v", err)
	}
	if conf == nil {
		t.Fatalf("expected config, got nil")
	}
	if len(conf.Certificates) != 1 {
		t.Fatalf("expected one certificate, got %d", len(conf.Certificates))
	}
	if _, err := conf.GetCertificate(&tls.ClientHelloInfo{}); err != nil {
		t.Fatalf("GetCertificate error: %v", err)
	}
}

func TestLoadTLSConfigSuccess(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := generateSelfSignedCert(t, dir)

	t.Setenv("SMTP_TLS_CERT", certPath)
	t.Setenv("SMTP_TLS_KEY", keyPath)

	conf, err := LoadTLSConfig()
	if err != nil {
		t.Fatalf("LoadTLSConfig error: %v", err)
	}
	if conf == nil {
		t.Fatalf("expected config, got nil")
	}
	if conf.MinVersion != tls.VersionTLS12 {
		t.Fatalf("expected MinVersion TLS1.2, got %d", conf.MinVersion)
	}
	if _, err := conf.GetCertificate(&tls.ClientHelloInfo{}); err != nil {
		t.Fatalf("GetCertificate error: %v", err)
	}
}

func generateSelfSignedCert(t *testing.T, dir string) (string, string) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName:   "smtpserver.test",
			Organization: []string{"smtpserver"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}

	certOut, err := os.Create(filepath.Join(dir, "cert.pem"))
	if err != nil {
		t.Fatalf("Create cert file: %v", err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		t.Fatalf("Encode cert: %v", err)
	}
	if err := certOut.Close(); err != nil {
		t.Fatalf("Close cert file: %v", err)
	}

	keyBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("MarshalECPrivateKey: %v", err)
	}

	keyOut, err := os.Create(filepath.Join(dir, "key.pem"))
	if err != nil {
		t.Fatalf("Create key file: %v", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}); err != nil {
		t.Fatalf("Encode key: %v", err)
	}
	if err := keyOut.Close(); err != nil {
		t.Fatalf("Close key file: %v", err)
	}

	return certOut.Name(), keyOut.Name()
}
