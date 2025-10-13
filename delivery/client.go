package delivery

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"time"
)

const heloName = "smtpserver.local"

// Deliver attempts SMTP delivery to a given host with raw message data.
func Deliver(host string, from string, to string, data []byte) error {
	addr := net.JoinHostPort(host, "25")
	dialer := &net.Dialer{Timeout: 30 * time.Second}
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(2 * time.Minute)); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("new client: %w", err)
	}
	defer client.Close()

	if err := client.Hello(heloName); err != nil {
		return fmt.Errorf("helo: %w", err)
	}

	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsConf := &tls.Config{
			ServerName:               host,
			MinVersion:               tls.VersionTLS12,
			PreferServerCipherSuites: true,
		}
		if err := client.StartTLS(tlsConf); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
		if err := client.Hello(heloName); err != nil {
			return fmt.Errorf("post-starttls helo: %w", err)
		}
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("rcpt to: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data start: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("data write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("data close: %w", err)
	}

	if err := client.Quit(); err != nil {
		return fmt.Errorf("quit: %w", err)
	}

	return nil
}
