# GopherPost SMTP Server

This standalone SMTP server is implemented entirely with the Go standard library. It provides a minimal feature set suitable for local testing or development environments where an external mail transfer agent is not available.

Current release: `v0.4.0`

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
- Allow-list based access control; traffic is rejected until `SMTP_ALLOW_NETWORKS` or `SMTP_ALLOW_HOSTS` is configured.

## Current Capabilities
- Implements the core SMTP verbs (HELO/EHLO, MAIL FROM, RCPT TO, DATA, RSET, NOOP, QUIT) with multi-recipient support, dot-stuffing awareness, message-size limits, and per-command deadlines to keep sessions well-behaved.
- Enforces allow-listed access by IP/network or hostname before any SMTP banner is sent, providing a coarse ingress control layer for the unauthenticated listener.
- Persists every accepted message to disk with per-recipient hashing to avoid exposing raw addresses, supporting later inspection or reprocessing.
- Performs outbound delivery by resolving MX records, randomising equal-priority hosts, and attempting opportunistic STARTTLS upgrades before handing off the payload with retries managed by an in-memory queue and exponential backoff with jitter.
- Optionally signs outbound messages with DKIM when the necessary selector, key, and domain information are supplied via environment variables.
- Exposes a lightweight health server that serves readiness information, expvar-formatted metrics (queue depth, delivery counters, active sessions), and—when debug mode is enabled—a live audit log stream over HTTP.
- Provides audit logging that can be toggled via environment variables and fanned out to subscribers, which the health endpoint leverages for streaming diagnostics.
- Loads configuration strictly from environment variables (with .env support) for everything from ports and banner text to DKIM keys and queue location, simplifying containerised or systemd deployments.

## Project Website
- Live overview: https://gopherpost.io
- Fallback GitHub Pages URL: https://its-donkey.github.io/smtpserver/
- Source for the static site lives in `docs/` (GitHub Pages ready).
- GitHub Pages configuration: Settings → Pages → “Deploy from a branch” → select `main` and `/docs`; set custom domain to `gopherpost.io`.
- Contact: `gday@gopherpost.io`

## Usage

### Local quick start

```bash
cp .env.example .env
# Set SMTP_ALLOW_NETWORKS or SMTP_ALLOW_HOSTS before starting
go run ./...

# or build a local binary
go build -o gopherpost ./...
./gopherpost
```

### Configuration

This server is configured entirely through environment variables (automatically loaded from a local `.env` file when present). Boolean values must be specified as `true` or `false`.

#### Core runtime
```yml
SMTP_PORT # TCP port to bind for the SMTP listener (default 2525).  
SMTP_HOSTNAME # Hostname advertised in SMTP banners and HELO/EHLO (default system hostname).  
SMTP_BANNER # Custom greeting appended to the initial 220 response (default GopherPost ready).  
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

Use an absolute path for `SMTP_QUEUE_PATH` when running the daemon under systemd so that the service `ReadWritePaths` setting can be aligned.

## Deployment

### Manual systemd installation

Prerequisites: Linux host with systemd, Go 1.21 or newer, and root access.

```bash
# 1. Build the binary
go build -o gopherpost ./...

# 2. Create a dedicated user and runtime directories
sudo useradd --system --home /opt/gopherpost --shell /usr/sbin/nologin gopherpost
sudo install -d -o gopherpost -g gopherpost -m 0755 /opt/gopherpost /var/lib/gopherpost /var/spool/gopherpost
sudo install -d -m 0755 /etc/gopherpost

# 3. Install the binary and systemd unit
sudo install -o root -g root -m 0755 gopherpost /usr/local/bin/gopherpost
sudo install -m 0644 packaging/systemd/gopherpost.service /etc/systemd/system/gopherpost.service

# 4. Configure environment
sudo cp .env.example /etc/gopherpost/gopherpost.env
sudo ${EDITOR:-nano} /etc/gopherpost/gopherpost.env
#   - Set SMTP_ALLOW_NETWORKS or SMTP_ALLOW_HOSTS to the networks/hosts you trust
#   - Ensure SMTP_QUEUE_PATH=/var/spool/gopherpost (or adjust the unit's ReadWritePaths accordingly)

# 5. Activate the service
sudo systemctl daemon-reload
sudo systemctl enable --now gopherpost
```

### Automatic systemd installation

The `install.sh` helper automates the same steps, generates a tailored unit file, and performs an optional health check. Run it as root:

```bash
sudo ./install.sh
```

You will be prompted for the environment values (defaults are shown inline). The script will build the binary, create required directories, install the service to `/etc/systemd/system/gopherpost.service`, reload systemd, and start the daemon. Provide an absolute `SMTP_QUEUE_PATH` so the generated `ReadWritePaths` remain valid.

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

## Debugging and Auditing
Set `SMTP_DEBUG=true` to enable verbose delivery logs.

## TLS Support
Set `SMTP_TLS_CERT` and `SMTP_TLS_KEY` to enable STARTTLS. Certificates are served with a minimum TLS version of 1.2.
The outbound client upgrades to TLS when the remote server advertises the capability, but it never accepts invalid certificates.

## Health Checks & Metrics
An HTTP endpoint is available at `:8080/healthz` (override via `SMTP_HEALTH_ADDR`) for readiness/liveness probes. If the configured port cannot be bound the SMTP server continues without the health listener.
Structured metrics are exported at `/metrics` in expvar JSON format whenever the health server is running.
When `SMTP_DEBUG=true`, visiting the `/healthz` endpoint renders `OK` followed by a live stream of audit log entries so you can tail activity from a browser or `curl`.

## Contributing
- Enable local Git hooks for commit and branch checks:
  - `git config core.hooksPath .githooks`
- Branch naming: `type/section/kebab-feature` (e.g., `feature/queue/retry-jitter`).
- Commit style: `<type-short> (section): <message>`
  - type-short: feat, fix, hotfix, design, refactor, test, doc
- PR template fields: include `Type:` and `Section:` at the top of the description.
- Update `CHANGELOG.md` for every PR unless you apply the `no-changelog` label.

Example commit subject
`refactor (module): rename module and imports to gopherpost`

Example PR body
```
Type: refactor
Section: module
Summary:

Rename module path and all imports to `gopherpost`.

Changes
- Update `go.mod` module path
- Update internal imports
- Adjust tests

Checklist
- [x] CHANGELOG.md updated
- [x] Branch name follows convention
- [x] Commits follow convention
```
