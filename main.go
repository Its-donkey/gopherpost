package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/textproto"
	"os"
	"strings"
	"time"

	health "smtpserver/health"
	audit "smtpserver/internal/audit"
	"smtpserver/internal/email"
	"smtpserver/internal/metrics"
	"smtpserver/queue"
	"smtpserver/storage"
	tlsconfig "smtpserver/tlsconfig"
)

const (
	defaultSMTPPort   = "2525"
	defaultHealthAddr = ":8080"
	defaultBanner     = "smtpserver ready"
	maxMessageBytes   = 10 << 20 // 10 MiB
	commandDeadline   = 15 * time.Minute
)

func main() {
	audit.RefreshFromEnv()

	port := defaultSMTPPort
	if env := os.Getenv("SMTP_PORT"); env != "" {
		port = env
	}
	addr := ":" + port

	healthAddr := defaultHealthAddr
	if env := os.Getenv("SMTP_HEALTH_ADDR"); env != "" {
		healthAddr = env
	}
	banner := os.Getenv("SMTP_BANNER")
	if banner == "" {
		banner = defaultBanner
	}

	healthServer, healthListener, err := health.StartHealthServer(healthAddr)
	if err != nil {
		log.Fatalf("Failed to start health server: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = healthServer.Shutdown(ctx)
		_ = healthListener.Close()
	}()
	log.Printf("Health endpoint listening on %s/healthz", healthListener.Addr().String())

	q := queue.NewManager()
	q.Start()
	defer q.Stop()

	tlsConf, tlsErr := tlsconfig.LoadTLSConfig()
	if tlsErr != nil && !errors.Is(tlsErr, tlsconfig.ErrTLSDisabled) {
		log.Fatalf("Failed to load TLS: %v", tlsErr)
	}
	baseListener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	var ln net.Listener = baseListener
	if tlsConf != nil {
		ln = tls.NewListener(baseListener, tlsConf)
		audit.Log("SMTP TLS enabled on %s", addr)
		log.Printf("SMTP TLS enabled on %s", addr)
	} else {
		log.Printf("SMTP plaintext listening on %s", addr)
		if tlsErr != nil {
			log.Printf("TLS disabled: %v", tlsErr)
		}
	}

	audit.Log("SMTP server listening on %s", addr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}
		if !isLocalhost(conn.RemoteAddr()) {
			audit.Log("rejecting non-local connection from %s", conn.RemoteAddr())
			_ = conn.Close()
			continue
		}
		go handleSession(conn, q, banner)
	}
}

func handleSession(conn net.Conn, q *queue.Manager, banner string) {
	defer conn.Close()
	tp := textproto.NewConn(conn)
	defer tp.Close()

	remote := conn.RemoteAddr().String()
	audit.Log("session start %s", remote)
	metrics.IncSessions()
	defer metrics.DecSessions()
	defer audit.Log("session closed %s", remote)

	timeout := commandDeadline
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		log.Printf("failed to set initial deadline: %v", err)
		return
	}

	send := func(code int, msg string) bool {
		t := fmt.Sprintf("%d %s", code, msg)
		if err := tp.PrintfLine(t); err != nil {
			log.Printf("send error to %s: %v", remote, err)
			return false
		}
		return true
	}

	if !send(220, banner) {
		return
	}
	var from string
	var to []string
	var data bytes.Buffer

	reset := func() {
		from = ""
		to = nil
		data.Reset()
	}
	defer reset()

	for {
		if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
			log.Printf("failed to refresh deadline: %v", err)
			return
		}
		line, err := tp.ReadLine()
		if err != nil {
			log.Printf("session error: %v", err)
			return
		}
		cmd := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(cmd, "HELO") || strings.HasPrefix(cmd, "EHLO"):
			if !send(250, "Hello") {
				return
			}
		case strings.HasPrefix(cmd, "MAIL FROM:"):
			addr, err := email.ParseCommandAddress(line)
			if err != nil {
				if !send(501, "Invalid sender address") {
					return
				}
				continue
			}
			from = addr
			to = nil
			if !send(250, "Sender OK") {
				return
			}
		case strings.HasPrefix(cmd, "RCPT TO:"):
			if from == "" {
				if !send(503, "Need MAIL command first") {
					return
				}
				continue
			}
			addr, err := email.ParseCommandAddress(line)
			if err != nil {
				if !send(501, "Invalid recipient address") {
					return
				}
				continue
			}
			to = append(to, addr)
			if !send(250, "Recipient OK") {
				return
			}
		case strings.HasPrefix(cmd, "RSET"):
			reset()
			if !send(250, "State cleared") {
				return
			}
		case strings.HasPrefix(cmd, "NOOP"):
			if !send(250, "OK") {
				return
			}
		case strings.HasPrefix(cmd, "DATA"):
			if from == "" || len(to) == 0 {
				if !send(503, "Need sender and recipient before DATA") {
					return
				}
				continue
			}
			messageID := shortID()
			if !send(354, "End with <CR><LF>.<CR><LF>") {
				return
			}
			data.Reset()
			reader := tp.DotReader()
			limited := &io.LimitedReader{
				R: reader,
				N: maxMessageBytes + 1,
			}
			_, err := io.Copy(&data, limited)
			if err != nil {
				if !send(554, "Read error") {
					return
				}
				return
			}
			if limited.N <= 0 {
				if !send(552, "Message exceeds size limit") {
					return
				}
				reset()
				continue
			}

			messageBytes := append([]byte(nil), data.Bytes()...)
			payload := queue.NewPayload(messageBytes)
			var queued []queue.QueuedMessage
			var persistErr error

			for _, rcpt := range to {
				if err := storage.SaveMessage(messageID, from, rcpt, messageBytes); err != nil {
					log.Printf("failed to persist message for %s: %v", rcpt, err)
					persistErr = err
					break
				}
				queued = append(queued, queue.QueuedMessage{
					ID:      messageID,
					From:    from,
					To:      rcpt,
					Payload: payload,
				})
			}
			if persistErr != nil {
				if !send(451, "Requested action aborted: storage failure") {
					return
				}
				reset()
				continue
			}
			for _, msg := range queued {
				q.Enqueue(msg)
			}
			if !send(250, fmt.Sprintf("Message queued as %s", messageID)) {
				return
			}
			audit.Log("message %s queued from %s to %d recipients", messageID, from, len(to))
			reset()
		case strings.HasPrefix(cmd, "QUIT"):
			if !send(221, "Bye") {
				return
			}
			return
		default:
			if !send(502, "Command not implemented") {
				return
			}
		}
	}
}

func shortID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func isLocalhost(addr net.Addr) bool {
	tcp, ok := addr.(*net.TCPAddr)
	if !ok {
		return false
	}
	if tcp.IP == nil {
		return false
	}
	return tcp.IP.IsLoopback()
}
