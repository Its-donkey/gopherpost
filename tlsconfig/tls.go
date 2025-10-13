package tlsconfig

import (
	"crypto/tls"
	"log"
	"os"
)

func LoadTLSConfig() (*tls.Config, error) {
	certFile := os.Getenv("SMTP_TLS_CERT")
	keyFile := os.Getenv("SMTP_TLS_KEY")
	if certFile == "" || keyFile == "" {
		log.Println("[TLS] SMTP_TLS_CERT or SMTP_TLS_KEY not set; TLS disabled")
		return nil, nil
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates:             []tls.Certificate{cert},
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
	}, nil
}
