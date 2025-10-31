package tlsconfig

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"math/big"
	"os"
	"time"

    "gopherpost/internal/config"
)

// ErrTLSDisabled is returned when TLS configuration is missing.
var ErrTLSDisabled = errors.New("smtp tls disabled: certificate not configured")

func LoadTLSConfig() (*tls.Config, error) {
	if config.Bool("SMTP_TLS_DISABLE", false) {
		return nil, ErrTLSDisabled
	}

	certFile := os.Getenv("SMTP_TLS_CERT")
	keyFile := os.Getenv("SMTP_TLS_KEY")
	if certFile == "" || keyFile == "" {
		cert, err := generateEphemeralCertificate()
		if err != nil {
			return nil, fmt.Errorf("generate ephemeral certificate: %w", err)
		}
		return tlsConfigFromCertificate(cert, false), nil
	}

	loader := func() (*tls.Certificate, error) {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("load x509 key pair: %w", err)
		}
		return &cert, nil
	}

	initialCert, err := loader()
	if err != nil {
		return nil, err
	}

	conf := tlsConfigFromCertificate(*initialCert, true)
	conf.GetCertificate = func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		return loader()
	}

	return conf, nil
}

func tlsConfigFromCertificate(cert tls.Certificate, allowReload bool) *tls.Config {
	cfg := &tls.Config{
		Certificates:             []tls.Certificate{cert},
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
	}
	if !allowReload {
		captured := cert
		cfg.GetCertificate = func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			return &captured, nil
		}
	}
	return cfg
}

func generateEphemeralCertificate() (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, big.NewInt(1<<48))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("serial: %w", err)
	}

    tmpl := &x509.Certificate{
        SerialNumber: serial,
        Subject: pkix.Name{
            CommonName:   "gopherpost.local",
            Organization: []string{"gopherpost"},
        },
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create certificate: %w", err)
	}

	cert := tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  priv,
	}

	return cert, nil
}
