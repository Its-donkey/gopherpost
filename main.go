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
	"net/netip"
	"net/textproto"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"

	health "gopherpost/health"
	audit "gopherpost/internal/audit"
	"gopherpost/internal/auth"
	"gopherpost/internal/config"
	"gopherpost/internal/dkim"
	"gopherpost/internal/email"
	"gopherpost/internal/metrics"
	"gopherpost/internal/version"
	"gopherpost/queue"
	"gopherpost/storage"
	tlsconfig "gopherpost/tlsconfig"
)

const (
	defaultSMTPPort   = "2525"
	defaultHealthAddr = ":8080"
	defaultBanner     = "GopherPost ready"
	maxMessageBytes   = 10 << 20 // 10 MiB
	commandDeadline   = 15 * time.Minute
)

func main() {
	_ = godotenv.Load()
	audit.RefreshFromEnv()
	auth.LoadFromEnv()
	log.Printf("GopherPost version %s starting", version.Number)
	if auth.Enabled() {
		if auth.AllowInsecure() {
			log.Printf("SMTP AUTH enabled (warning: insecure mode allows AUTH without TLS)")
		} else {
			log.Printf("SMTP AUTH enabled (requires TLS)")
		}
		audit.Log("auth enabled insecure=%v", auth.AllowInsecure())
	}
	audit.Log("version %s boot", version.Number)

	port := defaultSMTPPort
	if env := os.Getenv("SMTP_PORT"); env != "" {
		port = env
	}
	addr := ":" + port

	healthDisabled := config.Bool("SMTP_HEALTH_DISABLE", false)

	healthAddr := defaultHealthAddr
	if env := os.Getenv("SMTP_HEALTH_ADDR"); env != "" {
		healthAddr = env
	}
	if port := os.Getenv("SMTP_HEALTH_PORT"); port != "" {
		healthAddr = overridePort(healthAddr, port)
	}
	hostname := config.Hostname()
	dkimSigner, err := dkim.LoadFromEnv()
	if err != nil {
		log.Fatalf("Failed to initialize DKIM: %v", err)
	}
	if dkimSigner != nil {
		log.Printf("DKIM signing enabled (selector %s)", dkimSigner.Selector())
		audit.Log("DKIM signing enabled selector %s domain %s", dkimSigner.Selector(), dkimSigner.Domain())
	}
	banner := os.Getenv("SMTP_BANNER")
	if banner == "" {
		banner = defaultBanner
	}
	greeting := fmt.Sprintf("%s %s", hostname, banner)

	if healthDisabled {
		log.Printf("Health server disabled via SMTP_HEALTH_DISABLE")
	} else if healthServer, healthListener, err := health.StartHealthServer(healthAddr); err != nil {
		log.Printf("Health server disabled: %v", err)
	} else {
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = healthServer.Shutdown(ctx)
			_ = healthListener.Close()
		}()
		log.Printf("Health endpoint listening on %s/healthz", healthListener.Addr().String())
	}

	workerCount := config.QueueWorkers()
	q := queue.NewManager(queue.WithWorkers(workerCount))
	if dir := strings.TrimSpace(os.Getenv("SMTP_QUEUE_PATH")); dir != "" {
		storage.SetBaseDir(dir)
		queue.SetPersistDir(dir)
		log.Printf("Queue storage path set to %s", dir)
	}
	storage.LoadRetentionFromEnv()
	retentionStop := storage.StartRetentionCleanup()
	defer close(retentionStop)
	log.Printf("Message retention set to %d days", storage.RetentionDays())
	if err := q.LoadQueue(); err != nil {
		log.Printf("Warning: failed to load persisted queue: %v", err)
	} else if depth := q.Depth(); depth > 0 {
		log.Printf("Restored %d pending deliveries from disk", depth)
		audit.Log("queue restored %d entries", depth)
	}
	log.Printf("Queue workers configured: %d", workerCount)
	audit.Log("queue workers %d", workerCount)
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
		tlsActive := tlsConf != nil
		go handleSession(conn, q, greeting, hostname, dkimSigner, tlsActive)
	}
}

func handleSession(conn net.Conn, q *queue.Manager, greeting string, hostname string, signer *dkim.Signer, tlsActive bool) {
	defer conn.Close()
	tp := textproto.NewConn(conn)
	defer tp.Close()

	remoteAddr := conn.RemoteAddr()
	remote := remoteAddr.String()
	sessionID := shortID()
	audit.Log("session %s start %s", sessionID, remote)
	alog := func(format string, args ...any) {
		prefixArgs := append([]any{sessionID}, args...)
		audit.Log("session %s "+format, prefixArgs...)
	}
	send := func(code int, msg string) bool {
		t := fmt.Sprintf("%d %s", code, msg)
		if err := tp.PrintfLine(t); err != nil {
			log.Printf("send error to %s: %v", remote, err)
			alog("send error: %v", err)
			return false
		}
		alog("sent %d %s", code, msg)
		return true
	}
	if !connAllowed(remoteAddr) {
		_ = send(554, "5.7.1 Access denied")
		audit.Log("session %s rejected remote %s", sessionID, remote)
		return
	}
	metrics.IncSessions()
	defer metrics.DecSessions()
	defer audit.Log("session %s closed %s", sessionID, remote)

	requireLocalDomain := config.RequireSenderDomain()
	expectedDomain := strings.ToLower(hostname)

	timeout := commandDeadline
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		log.Printf("failed to set initial deadline: %v", err)
		alog("set deadline failed: %v", err)
		return
	}

	if !send(220, greeting) {
		return
	}
	var from string
	var to []string
	var data bytes.Buffer
	var authenticated bool
	var authUser string

	// Determine if AUTH should be offered
	authAvailable := auth.Enabled() && (tlsActive || auth.AllowInsecure())

	reset := func() {
		from = ""
		to = nil
		data.Reset()
	}
	defer reset()

	for {
		if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
			log.Printf("failed to refresh deadline: %v", err)
			alog("refresh deadline failed: %v", err)
			return
		}
		line, err := tp.ReadLine()
		if err != nil {
			log.Printf("session error: %v", err)
			alog("read error: %v", err)
			return
		}
		alog("recv %s", summarizeCommand(line))
		cmd := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(cmd, "HELO"):
			if !send(250, hostname) {
				return
			}
			alog("handshake HELO")
		case strings.HasPrefix(cmd, "EHLO"):
			// Send multi-line EHLO response
			lines := []string{hostname}
			if authAvailable {
				lines = append(lines, "AUTH PLAIN LOGIN")
			}
			for i, l := range lines {
				var prefix string
				if i == len(lines)-1 {
					prefix = "250 "
				} else {
					prefix = "250-"
				}
				if err := tp.PrintfLine("%s%s", prefix, l); err != nil {
					log.Printf("send error to %s: %v", remote, err)
					alog("send error: %v", err)
					return
				}
			}
			alog("handshake EHLO (auth_available=%v)", authAvailable)
		case strings.HasPrefix(cmd, "AUTH "):
			if !authAvailable {
				if !send(503, "5.5.1 AUTH not available") {
					return
				}
				alog("AUTH rejected: not available")
				continue
			}
			if authenticated {
				if !send(503, "5.5.1 Already authenticated") {
					return
				}
				alog("AUTH rejected: already authenticated")
				continue
			}
			authMethod := strings.TrimPrefix(cmd, "AUTH ")
			switch {
			case strings.HasPrefix(authMethod, "PLAIN"):
				// AUTH PLAIN may have credentials inline or require continuation
				parts := strings.SplitN(authMethod, " ", 2)
				var encodedCreds string
				if len(parts) == 2 && parts[1] != "" {
					encodedCreds = parts[1]
				} else {
					// Request credentials
					if err := tp.PrintfLine("334 "); err != nil {
						alog("send error: %v", err)
						return
					}
					creds, err := tp.ReadLine()
					if err != nil {
						alog("read error during AUTH PLAIN: %v", err)
						return
					}
					encodedCreds = creds
				}
				user, pass, err := auth.DecodePlain(encodedCreds)
				if err != nil {
					if !send(501, "5.5.4 Malformed authentication data") {
						return
					}
					alog("AUTH PLAIN decode error: %v", err)
					continue
				}
				if err := auth.Validate(user, pass); err != nil {
					if !send(535, "5.7.8 Authentication failed") {
						return
					}
					alog("AUTH PLAIN failed for user %s", user)
					continue
				}
				authenticated = true
				authUser = user
				if !send(235, "2.7.0 Authentication successful") {
					return
				}
				alog("AUTH PLAIN success user=%s", user)
			case strings.HasPrefix(authMethod, "LOGIN"):
				// AUTH LOGIN uses challenge-response
				// Send Username: challenge
				if err := tp.PrintfLine("334 %s", auth.EncodeChallenge("Username:")); err != nil {
					alog("send error: %v", err)
					return
				}
				encodedUser, err := tp.ReadLine()
				if err != nil {
					alog("read error during AUTH LOGIN: %v", err)
					return
				}
				user, err := auth.DecodeLogin(encodedUser)
				if err != nil {
					if !send(501, "5.5.4 Malformed authentication data") {
						return
					}
					alog("AUTH LOGIN decode error: %v", err)
					continue
				}
				// Send Password: challenge
				if err := tp.PrintfLine("334 %s", auth.EncodeChallenge("Password:")); err != nil {
					alog("send error: %v", err)
					return
				}
				encodedPass, err := tp.ReadLine()
				if err != nil {
					alog("read error during AUTH LOGIN: %v", err)
					return
				}
				pass, err := auth.DecodeLogin(encodedPass)
				if err != nil {
					if !send(501, "5.5.4 Malformed authentication data") {
						return
					}
					alog("AUTH LOGIN decode error: %v", err)
					continue
				}
				if err := auth.Validate(user, pass); err != nil {
					if !send(535, "5.7.8 Authentication failed") {
						return
					}
					alog("AUTH LOGIN failed for user %s", user)
					continue
				}
				authenticated = true
				authUser = user
				if !send(235, "2.7.0 Authentication successful") {
					return
				}
				alog("AUTH LOGIN success user=%s", user)
			default:
				if !send(504, "5.5.4 Unrecognized authentication mechanism") {
					return
				}
				alog("AUTH rejected: unknown mechanism")
			}
		case strings.HasPrefix(cmd, "MAIL FROM:"):
			addr, err := email.ParseCommandAddress(line)
			if err != nil {
				if !send(501, "Invalid sender address") {
					return
				}
				alog("invalid MAIL FROM: %v", err)
				continue
			}
			// Skip domain restriction for authenticated users
			if requireLocalDomain && !authenticated {
				domain, derr := email.Domain(addr)
				if derr != nil {
					if !send(501, "Invalid sender domain") {
						return
					}
					alog("invalid sender domain: %v", derr)
					continue
				}
				if expectedDomain != "" && !strings.EqualFold(domain, expectedDomain) {
					if !send(553, "Sender domain not permitted") {
						return
					}
					alog("sender domain %s rejected (expected %s)", domain, expectedDomain)
					continue
				}
			}
			from = addr
			to = nil
			if !send(250, "Sender OK") {
				return
			}
			alog("mail from %s", from)
		case strings.HasPrefix(cmd, "RCPT TO:"):
			if from == "" {
				if !send(503, "Need MAIL command first") {
					return
				}
				alog("RCPT before MAIL rejected")
				continue
			}
			addr, err := email.ParseCommandAddress(line)
			if err != nil {
				if !send(501, "Invalid recipient address") {
					return
				}
				alog("invalid RCPT TO: %v", err)
				continue
			}
			to = append(to, addr)
			if !send(250, "Recipient OK") {
				return
			}
			alog("rcpt add %s (total=%d)", addr, len(to))
		case strings.HasPrefix(cmd, "RSET"):
			reset()
			if !send(250, "State cleared") {
				return
			}
			alog("state reset")
		case strings.HasPrefix(cmd, "NOOP"):
			if !send(250, "OK") {
				return
			}
			alog("noop acknowledged")
		case strings.HasPrefix(cmd, "DATA"):
			if from == "" || len(to) == 0 {
				if !send(503, "Need sender and recipient before DATA") {
					return
				}
				alog("DATA before MAIL/RCPT rejected")
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
				alog("dot-reader copy error: %v", err)
				return
			}
			if limited.N <= 0 {
				if !send(552, "Message exceeds size limit") {
					return
				}
				alog("message exceeded max size (%d bytes)", maxMessageBytes)
				reset()
				continue
			}

			messageBytes := append([]byte(nil), data.Bytes()...)
			if signer != nil {
				signed, err := signer.Sign(messageBytes, from)
				if err != nil {
					if !send(451, "Requested action aborted: DKIM signing failure") {
						return
					}
					alog("dkim signing error: %v", err)
					reset()
					continue
				}
				messageBytes = signed
				alog("dkim signature applied")
			}
			payload := queue.NewPayload(messageBytes)
			var queued []queue.QueuedMessage
			var persistedPaths []string
			var persistErr error

			for _, rcpt := range to {
				path, err := storage.SaveMessage(messageID, from, rcpt, messageBytes)
				if err != nil {
					log.Printf("failed to persist message for %s: %v", rcpt, err)
					alog("storage error for %s: %v", rcpt, err)
					persistErr = err
					break
				}
				persistedPaths = append(persistedPaths, path)
				queued = append(queued, queue.QueuedMessage{
					ID:       messageID,
					From:     from,
					To:       rcpt,
					FilePath: path,
					Payload:  payload,
				})
			}
			if persistErr != nil {
				for _, path := range persistedPaths {
					if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
						log.Printf("failed to roll back persisted message %s: %v", path, err)
						alog("rollback error %s: %v", path, err)
					}
				}
				if !send(451, "Requested action aborted: storage failure") {
					return
				}
				alog("message %s aborted due to storage failure", messageID)
				reset()
				continue
			}
			for _, msg := range queued {
				q.Enqueue(msg)
			}
			if !send(250, fmt.Sprintf("Message queued as %s", messageID)) {
				return
			}
			if authenticated {
				alog("message %s queued (size=%d bytes, recipients=%d, auth_user=%s)", messageID, len(messageBytes), len(to), authUser)
			} else {
				alog("message %s queued (size=%d bytes, recipients=%d)", messageID, len(messageBytes), len(to))
			}
			reset()
		case strings.HasPrefix(cmd, "QUIT"):
			if !send(221, "Bye") {
				return
			}
			alog("quit requested")
			return
		default:
			if !send(502, "Command not implemented") {
				return
			}
			alog("unhandled command: %s", summarizeCommand(line))
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

func connAllowed(addr net.Addr) bool {
	if addr == nil {
		return false
	}
	untyped := addr.String()
	if untyped == "" {
		return false
	}
	allowedNets := config.AllowedNetworks()
	allowedHosts := config.AllowedHosts()
	if len(allowedNets) == 0 && len(allowedHosts) == 0 {
		return false
	}
	if len(allowedHosts) > 0 {
		host := hostFromAddr(untyped)
		hostLower := strings.ToLower(host)
		for _, allowed := range allowedHosts {
			if hostLower == allowed {
				return true
			}
		}
	}
	if len(allowedNets) > 0 {
		if ip := extractIP(addr); ip != nil {
			for _, network := range allowedNets {
				if network.Contains(ip) {
					return true
				}
			}
		}
	}
	return false
}

func extractIP(addr net.Addr) net.IP {
	switch v := addr.(type) {
	case *net.TCPAddr:
		return v.IP
	case *net.UDPAddr:
		return v.IP
	default:
		if host := hostFromAddr(addr.String()); host != "" {
			if ip, err := netip.ParseAddr(host); err == nil {
				return net.IP(ip.AsSlice())
			}
			return net.ParseIP(host)
		}
	}
	return nil
}

func hostFromAddr(addr string) string {
	if h, _, err := net.SplitHostPort(addr); err == nil {
		return h
	}
	return addr
}

func overridePort(addr, port string) string {
	port = strings.TrimSpace(port)
	port = strings.TrimPrefix(port, ":")
	if port == "" {
		return addr
	}

	switch {
	case addr == "", strings.HasPrefix(addr, ":"):
		return ":" + port
	case !strings.Contains(addr, ":"):
		return addr + ":" + port
	default:
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return addr
		}
		return net.JoinHostPort(host, port)
	}
}

func summarizeCommand(line string) string {
	line = strings.TrimSpace(line)
	if len(line) > 120 {
		return line[:117] + "..."
	}
	return line
}
