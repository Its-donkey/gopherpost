package email

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
)

var (
	// ErrInvalidCommand indicates the SMTP command lacks an address portion.
	ErrInvalidCommand = errors.New("invalid SMTP command")
	// ErrInvalidAddress indicates the address failed validation.
	ErrInvalidAddress = errors.New("invalid email address")
)

// ParseCommandAddress extracts and normalises the address portion from a SMTP command line.
// It accepts commands such as "MAIL FROM:<user@example.com>" and "RCPT TO:<user@example.com>".
func ParseCommandAddress(line string) (string, error) {
	if strings.ContainsAny(line, "\r\n") {
		return "", fmt.Errorf("%w: unexpected newline", ErrInvalidCommand)
	}

	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("%w: missing ':' separator", ErrInvalidCommand)
	}

	addr := strings.TrimSpace(parts[1])
	addr = strings.Trim(addr, "<>")
	if addr == "" {
		return "", fmt.Errorf("%w: empty address", ErrInvalidAddress)
	}

	parsed, err := mail.ParseAddress(addr)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidAddress, err)
	}

	return strings.ToLower(parsed.Address), nil
}

// Domain returns the domain component of a validated email address.
func Domain(address string) (string, error) {
	at := strings.LastIndex(address, "@")
	if at == -1 || at == len(address)-1 {
		return "", fmt.Errorf("%w: missing domain", ErrInvalidAddress)
	}

	domain := address[at+1:]
	domain = strings.TrimSuffix(domain, ".")
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return "", fmt.Errorf("%w: empty domain", ErrInvalidAddress)
	}
	if strings.ContainsAny(domain, " \t") {
		return "", fmt.Errorf("%w: whitespace in domain", ErrInvalidAddress)
	}

	return domain, nil
}
