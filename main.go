package main

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
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
	"smtpserver/queue"
	"smtpserver/storage"
	tlsconfig "smtpserver/tlsconfig"
)

func main() {
	port := "2525"
	if env := os.Getenv("SMTP_PORT"); env != "" {
		port = env
	}
	addr := ":" + port
	log.Printf("SMTP Server listening on %s", addr)

	health.StartHealthServer("8080")
	q := queue.NewManager()
	q.Start()
	defer q.Stop()

	tlsConf, err := tlsconfig.LoadTLSConfig()
	if err != nil {
		log.Fatalf("Failed to load TLS: %v", err)
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
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}
		go handleSession(conn, q)
	}
}

func handleSession(conn net.Conn, q *queue.Manager) {
	defer conn.Close()
	tp := textproto.NewConn(conn)
	defer tp.Close()

	timeout := 15 * time.Minute
	_ = conn.SetDeadline(time.Now().Add(timeout))

	send := func(code int, msg string) {
		t := fmt.Sprintf("%d %s", code, msg)
		_ = tp.PrintfLine(t)
	}

	send(220, "Ship Shape SHarpening SMTP Server Ready")
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
			send(250, "Hello")
		case strings.HasPrefix(cmd, "MAIL FROM:"):
			addr, err := email.ParseCommandAddress(line)
			if err != nil {
				send(501, "Invalid sender address")
				continue
			}
			from = addr
			to = nil
			send(250, "Sender OK")
		case strings.HasPrefix(cmd, "RCPT TO:"):
			if from == "" {
				send(503, "Need MAIL command first")
				continue
			}
			addr, err := email.ParseCommandAddress(line)
			if err != nil {
				send(501, "Invalid recipient address")
				continue
			}
			to = append(to, addr)
			send(250, "Recipient OK")
		case strings.HasPrefix(cmd, "DATA"):
			if from == "" || len(to) == 0 {
				send(503, "Need sender and recipient before DATA")
				continue
			}
			messageID := shortID()
			send(354, "End with <CR><LF>.<CR><LF>")
			data.Reset()
			reader := tp.DotReader()
			_, err := io.Copy(&data, reader)
			if err != nil {
				send(554, "Read error")
				return
			}
			send(250, "Message accepted")

			messageBytes := append([]byte(nil), data.Bytes()...)

			for _, rcpt := range to {
				if err := storage.SaveMessage(messageID, from, rcpt, messageBytes); err != nil {
					log.Printf("failed to persist message for %s: %v", rcpt, err)
				}
				msgCopy := append([]byte(nil), messageBytes...)
				q.Enqueue(queue.QueuedMessage{
					From:     from,
					To:       rcpt,
					Data:     msgCopy,
					Attempts: 0,
				})
			}
			reset()
		case strings.HasPrefix(cmd, "QUIT"):
			send(221, "Bye")
			return
		default:
			send(502, "Command not implemented")
		}
	}
}

func shortID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
