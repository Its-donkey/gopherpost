package delivery

import (
	"errors"
	"net"
	"testing"
)

func TestDeliverMessageNoMX(t *testing.T) {
	originalLookup := mxLookup
	originalDeliver := deliverFunc
	defer func() {
		mxLookup = originalLookup
		deliverFunc = originalDeliver
	}()

	mxLookup = func(domain string) ([]*net.MX, error) {
		return nil, nil
	}

	err := DeliverMessage("sender@example.com", "rcpt@example.com", []byte("body"))
	if err == nil || err.Error() != "MX lookup failed for example.com: no MX records" {
		t.Fatalf("expected no MX error, got %v", err)
	}
}

func TestDeliverMessageSuccess(t *testing.T) {
	originalLookup := mxLookup
	originalDeliver := deliverFunc
	defer func() {
		mxLookup = originalLookup
		deliverFunc = originalDeliver
	}()

	mxLookup = func(domain string) ([]*net.MX, error) {
		return []*net.MX{
			{Host: "mx1.example.com", Pref: 10},
		}, nil
	}

	var delivered bool
	deliverFunc = func(host, from, to string, data []byte) error {
		if host != "mx1.example.com" {
			t.Fatalf("unexpected host %s", host)
		}
		if from != "sender@example.com" || to != "rcpt@example.com" {
			t.Fatalf("unexpected envelope %s -> %s", from, to)
		}
		if string(data) != "payload" {
			t.Fatalf("unexpected payload %q", string(data))
		}
		delivered = true
		return nil
	}

	if err := DeliverMessage("sender@example.com", "rcpt@example.com", []byte("payload")); err != nil {
		t.Fatalf("DeliverMessage error: %v", err)
	}
	if !delivered {
		t.Fatalf("expected deliverFunc to be invoked")
	}
}

func TestDeliverMessageFailure(t *testing.T) {
	originalLookup := mxLookup
	originalDeliver := deliverFunc
	defer func() {
		mxLookup = originalLookup
		deliverFunc = originalDeliver
	}()

	mxLookup = func(domain string) ([]*net.MX, error) {
		return []*net.MX{
			{Host: "mx1.example.com", Pref: 10},
			{Host: "mx2.example.com", Pref: 20},
		}, nil
	}

	attempts := 0
	deliverFunc = func(host, from, to string, data []byte) error {
		attempts++
		return errors.New("smtp error")
	}

	err := DeliverMessage("sender@example.com", "rcpt@example.com", []byte("payload"))
	if err == nil {
		t.Fatalf("expected error")
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}
