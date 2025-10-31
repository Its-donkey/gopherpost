package delivery

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

    "gopherpost/internal/config"
)

func TestDeliverSuccess(t *testing.T) {
    t.Setenv("SMTP_HOSTNAME", "gopherpost.test")
	expectedHello := config.Hostname()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen error: %v", err)
	}
	defer ln.Close()

	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	oldPort := smtpPort
	smtpPort = port
	defer func() { smtpPort = oldPort }()

	dataCh := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			t.Errorf("accept error: %v", err)
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		br := bufio.NewReader(conn)
		bw := bufio.NewWriter(conn)

		fmt.Fprint(bw, "220 test ESMTP\r\n")
		bw.Flush()

		expectCommand(t, br, "EHLO "+expectedHello, "HELO "+expectedHello)
		fmt.Fprint(bw, "250 OK\r\n")
		bw.Flush()

		expectCommand(t, br, "MAIL FROM:<sender@example.com>")
		fmt.Fprint(bw, "250 OK\r\n")
		bw.Flush()

		expectCommand(t, br, "RCPT TO:<rcpt@example.com>")
		fmt.Fprint(bw, "250 OK\r\n")
		bw.Flush()

		expectCommand(t, br, "DATA")
		fmt.Fprint(bw, "354 End data with <CR><LF>.<CR><LF>\r\n")
		bw.Flush()

		var lines []string
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				t.Errorf("read data error: %v", err)
				return
			}
			if line == ".\r\n" {
				break
			}
			lines = append(lines, line)
		}
		dataCh <- strings.Join(lines, "")
		fmt.Fprint(bw, "250 OK\r\n")
		bw.Flush()

		expectCommand(t, br, "QUIT")
		fmt.Fprint(bw, "221 Bye\r\n")
		bw.Flush()
	}()

	if err := Deliver("127.0.0.1", "sender@example.com", "rcpt@example.com", []byte("Subject: Test\r\n\r\nBody")); err != nil {
		t.Fatalf("Deliver returned error: %v", err)
	}

	select {
	case body := <-dataCh:
		if !strings.Contains(body, "Subject: Test") || !strings.Contains(body, "Body") {
			t.Fatalf("unexpected body %q", body)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for SMTP data")
	}
}

func TestDeliverDialError(t *testing.T) {
	oldPort := smtpPort
	smtpPort = "9" // typically closed
	defer func() { smtpPort = oldPort }()

	err := Deliver("127.0.0.1", "sender@example.com", "rcpt@example.com", []byte("Body"))
	if err == nil {
		t.Fatalf("expected dial error")
	}
}

func expectCommand(t *testing.T, br *bufio.Reader, allowed ...string) {
	t.Helper()
	line, err := br.ReadString('\n')
	if err != nil {
		t.Fatalf("read command error: %v", err)
	}
	line = strings.TrimRight(line, "\r\n")
	for _, option := range allowed {
		if line == option {
			return
		}
	}
	t.Fatalf("unexpected command %q", line)
}
