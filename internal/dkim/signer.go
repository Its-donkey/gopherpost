package dkim

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"

	msgauthdkim "github.com/emersion/go-msgauth/dkim"
)

// Signer applies DKIM signatures to messages when configured.
type Signer struct {
	domain     string
	selector   string
	key        crypto.Signer
	headerKeys []string
}

// Selector returns the configured DKIM selector string.
func (s *Signer) Selector() string {
	if s == nil {
		return ""
	}
	return s.selector
}

// Domain returns the configured DKIM signing domain, if any.
func (s *Signer) Domain() string {
	if s == nil {
		return ""
	}
	return s.domain
}

// LoadFromEnv initializes a Signer using environment variables.
// Required env vars:
//
//	SMTP_DKIM_SELECTOR – DKIM selector string
//	SMTP_DKIM_KEY_PATH or SMTP_DKIM_PRIVATE_KEY – PEM encoded private key
//
// Optional:
//
//	SMTP_DKIM_DOMAIN – overrides the domain extracted from the sender address
func LoadFromEnv() (*Signer, error) {
	selector := strings.TrimSpace(os.Getenv("SMTP_DKIM_SELECTOR"))
	keyPath := strings.TrimSpace(os.Getenv("SMTP_DKIM_KEY_PATH"))
	inlineKey := os.Getenv("SMTP_DKIM_PRIVATE_KEY")
	domain := strings.TrimSpace(os.Getenv("SMTP_DKIM_DOMAIN"))

	if selector == "" && keyPath == "" && inlineKey == "" && domain == "" {
		return nil, nil
	}

	if selector == "" {
		return nil, fmt.Errorf("dkim: SMTP_DKIM_SELECTOR is required when enabling DKIM")
	}

	var pemData []byte
	switch {
	case inlineKey != "":
		pemData = []byte(inlineKey)
	case keyPath != "":
		data, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("dkim: read private key: %w", err)
		}
		pemData = data
	default:
		return nil, fmt.Errorf("dkim: provide SMTP_DKIM_KEY_PATH or SMTP_DKIM_PRIVATE_KEY")
	}

	key, err := parsePrivateKey(pemData)
	if err != nil {
		return nil, fmt.Errorf("dkim: parse private key: %w", err)
	}

	return &Signer{
		domain:   domain,
		selector: selector,
		key:      key,
		headerKeys: []string{
			"from",
			"to",
			"subject",
			"date",
			"mime-version",
			"content-type",
			"message-id",
		},
	}, nil
}

// Sign ensures the message carries a DKIM signature. When the message already includes
// a DKIM-Signature header it is left untouched.
func (s *Signer) Sign(message []byte, from string) ([]byte, error) {
	if s == nil || s.key == nil {
		return message, nil
	}
	if hasSignature(message) {
		return message, nil
	}

	domain := s.domain
	if domain == "" {
		domain = extractDomain(from)
	}
	if domain == "" {
		return nil, fmt.Errorf("dkim: unable to determine signing domain")
	}

	opts := &msgauthdkim.SignOptions{
		Domain:                 domain,
		Selector:               s.selector,
		Signer:                 s.key,
		HeaderCanonicalization: msgauthdkim.CanonicalizationRelaxed,
		BodyCanonicalization:   msgauthdkim.CanonicalizationRelaxed,
		HeaderKeys:             s.headerKeys,
	}

	var signed bytes.Buffer
	reader := bytes.NewReader(normalizeLineEndings(message))
	if err := msgauthdkim.Sign(&signed, reader, opts); err != nil {
		return nil, fmt.Errorf("dkim: signing failed: %w", err)
	}
	return signed.Bytes(), nil
}

func parsePrivateKey(pemData []byte) (crypto.Signer, error) {
	for {
		block, rest := pem.Decode(pemData)
		if block == nil {
			break
		}
		switch block.Type {
		case "RSA PRIVATE KEY":
			key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				return nil, err
			}
			return key, nil
		case "PRIVATE KEY":
			key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				return nil, err
			}
			if signer, ok := key.(crypto.Signer); ok {
				return signer, nil
			}
			return nil, fmt.Errorf("unsupported private key type in PKCS#8 container")
		}
		pemData = rest
	}
	return nil, fmt.Errorf("no private key found in PEM data")
}

func extractDomain(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}

	// Strip optional angle brackets.
	if strings.HasPrefix(address, "<") && strings.HasSuffix(address, ">") {
		address = address[1 : len(address)-1]
	}
	if i := strings.LastIndex(address, "@"); i >= 0 && i+1 < len(address) {
		return strings.ToLower(address[i+1:])
	}
	return ""
}

func hasSignature(message []byte) bool {
	upper := bytes.ToUpper(message)
	return bytes.Contains(upper, []byte("\nDKIM-SIGNATURE:")) || bytes.HasPrefix(upper, []byte("DKIM-SIGNATURE:"))
}

func normalizeLineEndings(data []byte) []byte {
	if bytes.Contains(data, []byte("\r\n")) || !bytes.Contains(data, []byte("\n")) {
		return data
	}
	lines := bytes.Split(data, []byte{'\n'})
	return bytes.Join(lines, []byte("\r\n"))
}
