# Simple Go SMTP Server

This standalone SMTP server is implemented entirely with the Go standard library. It provides a minimal feature set suitable for local testing or development environments where an external mail transfer agent is not available.

## Features

- RFC 5321 inspired command handling (HELO/EHLO, MAIL FROM, RCPT TO, DATA, RSET, NOOP, QUIT).
- Support for accepting multiple recipients per message.
- Dot-stuffing handling inside the DATA section.
- Connection deadline enforcement to mitigate idle clients.
- Automatic message persistence with recipient hashing to protect sensitive metadata.
- Outbound delivery with MX lookup, opportunistic STARTTLS, and jittered retry queue.
- Server-side message size limit (10 MiB) to guard against resource exhaustion.
- Built-in instrumentation exposed via `/metrics` (expvar format).
- Access restricted to localhost clients to prevent unintended relaying.

## Usage

```bash
cd smtpserver
# Build the binary
go build
# Or run directly
go run ./...
```

### Configuration

This server is configured entirely through environment variables:

- `SMTP_PORT` – TCP address to bind to (default `2525`).
- `SMTP_HEALTH_ADDR` – Listen address for the health server (default `:8080`).
- `SMTP_BANNER` – Custom greeting banner returned in the `220` response (default `smtpserver ready`).
- `SMTP_DEBUG` – Set to `1` to enable verbose audit logging.
- `SMTP_TLS_CERT` / `SMTP_TLS_KEY` – Enable TLS when both are provided.

**Security note:** the server only accepts SMTP sessions from `localhost` and does not implement authentication. It is intended for controlled environments (local testing, integration pipelines). Run as a non-root service account and avoid exposing it to the public internet.

## Retrieving Stored Messages

Messages are persisted automatically. On-disk filenames include a message identifier and a hash of the recipient address so personally identifiable information is not exposed through filenames.

## Example Session

```
$ telnet localhost 2525
Trying 127.0.0.1...
Connected to localhost.
Escape character is '^]'.
220 localhost Simple Go SMTP Server
HELO example.com
250 localhost greets example.com
MAIL FROM:<alice@example.com>
250 Sender <alice@example.com> OK
RCPT TO:<bob@example.net>
250 Recipient <bob@example.net> OK
DATA
354 End data with <CR><LF>.<CR><LF>
Subject: Hello

This is a test email.
.
250 Message accepted for delivery
QUIT
221 localhost signing off
Connection closed by foreign host.
```


## Outbound Delivery
This server now supports direct delivery to recipient domains via MX record resolution.
It performs MX preference sorting, randomises equal-priority records, and upgrades to STARTTLS only when advertised by the remote host.

## Delivery Queue
Messages that fail to deliver are automatically retried with capped exponential backoff and jitter to avoid thundering herd effects.

## Message Persistence
Incoming messages are saved to disk under `./data/spool/YYYY-MM-DD/`. Override the directory by calling `storage.SetBaseDir` before accepting traffic (useful for tests or containerised deployments).

## Running the Server
Set `SMTP_PORT=2525` (or any open port) and run:
```
go run .
```

## Debugging and Auditing
Set `SMTP_DEBUG=1` to enable verbose delivery logs.

## Systemd Integration
Copy `smtpserver.service` to `/etc/systemd/system/` and run:
```bash
sudo systemctl enable smtpserver
sudo systemctl start smtpserver
```

## TLS Support
Set `SMTP_TLS_CERT` and `SMTP_TLS_KEY` to enable STARTTLS. Certificates are served with a minimum TLS version of 1.2.
The outbound client upgrades to TLS when the remote server advertises the capability, but it never accepts invalid certificates.

## Health Checks & Metrics
An HTTP endpoint is available at `:8080/healthz` (override via `SMTP_HEALTH_ADDR`) for readiness/liveness probes.
Structured metrics are exported at `/metrics` in expvar JSON format.
