package tlsconfig

import (
	"crypto/tls"
	"errors"
	"fmt"
	"os"
)

// ErrTLSDisabled is returned when TLS configuration is missing.
var ErrTLSDisabled = errors.New("smtp tls disabled: certificate not configured")

func LoadTLSConfig() (*tls.Config, error) {
	certFile := os.Getenv("SMTP_TLS_CERT")
	keyFile := os.Getenv("SMTP_TLS_KEY")
	if certFile == "" || keyFile == "" {
		return nil, ErrTLSDisabled
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

	conf := &tls.Config{
		Certificates:             []tls.Certificate{*initialCert},
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
	}

	conf.GetCertificate = func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		return loader()
	}

	return conf, nil
}
