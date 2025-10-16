# Simple Go SMTP Server

This standalone SMTP server is implemented entirely with the Go standard library. It provides a minimal feature set suitable for local testing or development environments where an external mail transfer agent is not available.

## Features

- RFC 5321 inspired command handling (HELO/EHLO, MAIL FROM, RCPT TO, DATA, RSET, NOOP, QUIT).  
- Support for accepting multiple recipients per message.
- Dot-stuffing handling inside the DATA section.
- Connection deadline enforcement to mitigate idle clients.
- Automatic message persistence with recipient hashing to protect sensitive metadata.
- Outbound delivery with MX lookup, opportunistic STARTTLS, and jittered retry queue.
- Optional DKIM signing for outbound messages.
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

This server is configured entirely through environment variables (automatically loaded from a local `.env` file when present). Boolean values must be specified as `true` or `false`.

#### Core runtime
```yml
SMTP_PORT # TCP port to bind for the SMTP listener (default 2525).  
SMTP_HOSTNAME # Hostname advertised in SMTP banners and HELO/EHLO (default system hostname).  
SMTP_BANNER # Custom greeting appended to the initial 220 response (default smtpserver ready).  
SMTP_DEBUG # Enable verbose audit logging when `true` (default `false`).  
SMTP_HEALTH_ADDR # Listen address for the health server (default :8080).  
SMTP_HEALTH_PORT # Override only the port component of the health address (e.g. 9090).  
SMTP_HEALTH_DISABLE # Disable the health endpoint when `true` (default `false`).  
SMTP_QUEUE_PATH # Directory used to persist inbound messages (default ./data/spool).  
```
#### Access control

```yml
SMTP_ALLOW_NETWORKS # Comma-separated CIDR blocks/IPs allowed to connect (e.g. 192.0.2.0/24,203.0.113.5). When unset, all connections are rejected.
SMTP_ALLOW_HOSTS # Comma-separated hostnames allowed to connect (e.g. mail.example.com). When unset alongside networks, all connections are rejected.
SMTP_REQUIRE_LOCAL_DOMAIN # Require `MAIL FROM` senders to match `SMTP_HOSTNAME` when `true` (default `true`).  
```
#### TLS

```yml
SMTP_TLS_DISABLE # Skip loading TLS certificates when `true` (default `false`).  
SMTP_TLS_CERT # Path to the PEM certificate served for STARTTLS (e.g. /etc/ssl/certs/smtp.crt).  
SMTP_TLS_KEY # Path to the PEM private key matching the TLS cert (e.g. /etc/ssl/private/smtp.key).  
```
#### DKIM

```yml
SMTP_DKIM_SELECTOR # DKIM selector published in DNS (e.g. mail1).  
SMTP_DKIM_KEY_PATH # Filesystem path to the DKIM private key (e.g. /etc/dkim/mail1.key).  
SMTP_DKIM_PRIVATE_KEY # Inline PEM-formatted DKIM private key (e.g. -----BEGIN RSA PRIVATE KEY-----).  
SMTP_DKIM_DOMAIN # Domain to sign messages as when overriding the sender domain (e.g. example.com).  
```
**Security note:** configure `SMTP_ALLOW_NETWORKS`, `SMTP_ALLOW_HOSTS`, and `SMTP_REQUIRE_LOCAL_DOMAIN` to enforce ingress and sender restrictions. The server lacks authentication, so deploy behind firewalls or proxies and run as a non-root service account.

## Retrieving Stored Messages

Messages are persisted automatically. On-disk filenames include a message identifier and a hash of the recipient address so personally identifiable information is not exposed through filenames.

## Example Session

```bash
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
Incoming messages are saved to disk under `./data/spool/YYYY-MM-DD/` by default. Override the directory by setting `SMTP_QUEUE_PATH` or by calling `storage.SetBaseDir` before accepting traffic (useful for tests or containerised deployments).  

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
An HTTP endpoint is available at `:8080/healthz` (override via `SMTP_HEALTH_ADDR`) for readiness/liveness probes. If the configured port cannot be bound the SMTP server continues without the health listener.
Structured metrics are exported at `/metrics` in expvar JSON format whenever the health server is running.
