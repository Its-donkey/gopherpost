# Repository Review Checklist

**.gitignore**
- [x] `.gitignore` – add generated artifacts such as `./data/spool/`, coverage files, and `gopherpost` variants to avoid unintentionally committing payloads or binaries.

**go.mod**
- [x] `go.mod:1` – module path is `github.com/example/gopherpost`, but all imports use `gopherpost/...`; update either the module path or every import so the project builds.
- [x] `go.mod` – pin toolchain or add `//go:build` for Go version guarantees if 1.21 features are required.

**CHANGELOG.md**
- [x] `CHANGELOG.md` – include notes about TLS, health checks, and queue persistence behaviour to match current code (helps operations/security review).

**README.md**
- [x] `README.md` – update storage instructions (code now persists automatically) and document new env vars (`SMTP_PORT`, TLS cert/key, debug) with accurate defaults.
- [x] `README.md` – add warning about outbound delivery requirements (network access, DNS) and recommend non-root service accounts for security.
- [x] `README.md` – mention health endpoint port configurability; currently hard-coded to `8080`.

**health/health.go**
- [x] `health/health.go:8-12` – construct a dedicated `http.Server` with read/write timeouts and context-aware shutdown; avoid using the default mux to prevent handler collisions.
- [x] `health/health.go:12` – handle the `ListenAndServe` error (log or expose) so startup failures aren’t silent.
- [x] `health/health.go` – allow the caller to provide address/handler so this service can be unit-tested and composed.

**internal/audit/audit.go**
- [x] `internal/audit/audit.go:8` – read `SMTP_DEBUG` dynamically (or expose setter) so toggling at runtime/tests works.
- [x] `internal/audit/audit.go:10-14` – integrate the audit logger in the SMTP session code; currently unused import in `main.go` hides issues.
- [x] `internal/audit/audit.go` – consider leveled logging or structured fields to avoid format-string misuse.

**storage/files.go**
- [x] `storage/files.go:18` – sanitize `to` before using it in a filename (reject `../`, whitespace, path separators) to prevent path traversal or invalid filenames.
- [x] `storage/files.go:18` – mask personally identifiable addresses in filenames (hash or encode) to reduce data leakage.
- [x] `storage/files.go:19` – use restrictive permissions (e.g. `0o600`) for stored messages.
- [x] `storage/files.go:10` – allow configuring `baseDir` via environment/flag to support containers and testing.
- [x] `storage/files.go:14` – use `time.Now().UTC()` so directory naming is stable across time zones.
- [x] `storage/files.go` – copy the message bytes before writing; callers pass shared buffers that may mutate.

**tlsconfig/tls.go**
- [x] `tlsconfig/tls.go:20` – enforce modern security defaults (`MinVersion: tls.VersionTLS12`, `PreferServerCipherSuites`, OCSP stapling if certificates rotate).
- [x] `tlsconfig/tls.go:9-20` – surface a warning instead of printing to stdout; return a sentinel error so callers can decide whether to run without TLS.
- [x] `tlsconfig/tls.go` – reload certificates automatically (via `GetCertificate`) for long-running services.
- [x] `tlsconfig/tls.go` – add unit tests for missing env vars vs. invalid key pairs.

**queue/types.go**
- [x] `queue/types.go:6-11` – track a message ID (and maybe recipients slice) instead of duplicating per-recipient data to support dedupe and audit trails.
- [x] `queue/types.go` – clarify ownership of `Data` slices (document that callers must provide immutable copies).

**queue/manager.go**
- [x] `queue/manager.go:57-79` – release `m.mu` before performing network delivery; holding the lock blocks new enqueue/dequeue operations and risks deadlocks if `DeliverMessage` requeues.
- [x] `queue/manager.go:68-73` – implement exponential backoff with jitter; current linear retry invites thundering herd issues.
- [x] `queue/manager.go:30-32` – set `NextRetry` to `time.Now()` plus an initial delay so immediate retries don’t hammer remote MX hosts when delivery fails instantly.
- [x] `queue/manager.go:70-73` – persist attempts count and last error for observability; surface metrics instead of plain logs.
- [x] `queue/manager.go:37-45` – use a `time.Ticker` or context instead of `time.Sleep` in a tight loop to reduce latency and improve shutdown responsiveness.
- [x] `queue/manager.go:32` – log structured fields (recipient, attempts, nextRetry) for easier parsing.
- [x] Duplicate handling – current queue stores a separate entry per recipient without dedupe; consider batching identical payloads.

**delivery/mx.go**
- [x] `delivery/mx.go:9-12` – comment promises sorted MX records but no sorting occurs; sort by `Pref` and randomize equal priorities per RFC 5321.
- [x] `delivery/mx.go:16-20` – use `strings.LastIndex` and validate both local part and domain; current split allows multiple `@` or empty sections.
- [x] `delivery/mx.go` – trim trailing dot from `mx.Host` before dialing.
- [x] Duplicate logic – extraction/sanitisation logic mirrors parts of SMTP command parsing (`main.go:135-142`); refactor into a reusable email parser.

**delivery/client.go**
- [x] `delivery/client.go:6` – remove unused `net` import; current code doesn’t compile.
- [x] `delivery/client.go:12-17` – wrap the dial with context/deadlines to avoid hangs; consider opportunistic TLS using `tls.DialWithDialer`.
- [x] `delivery/client.go:19-21` – check `c.StartTLS` error only after verifying server advertises the extension; otherwise log downgrade risk.
- [x] `delivery/client.go:22-27` – call `c.Hello`/`c.EHLO` before `MAIL FROM` to satisfy RFCs and STARTTLS negotiation.
- [x] `delivery/client.go:32-35` – capture and return any `Close` error to surface truncated transfers.
- [x] `delivery/client.go` – reuse SMTP connections for multiple recipients or commands to reduce duplicate code.
- [x] Security – add AUTH support or explicitly document unauthenticated relay restrictions.

**delivery/deliver.go**
- [x] `delivery/deliver.go:14-16` – when `len(mxRecords)==0` and `err==nil`, wrap a sentinel error instead of `%w` with `nil` (currently prints `<nil>`).
- [x] `delivery/deliver.go:18-24` – observe MX priority ordering; currently iterates in DNS response order.
- [x] `delivery/deliver.go` – implement per-MX attempt limits and backoff; now cycles endlessly with queue retries.
- [x] Duplicate effort – identical retry logic exists in `queue/manager.go`; consider centralizing backoff policy.

**main.go**
- [x] `main.go:3-23` – imports `bufio`, `audit`, and uses `tls.Listen` without importing `crypto/tls`; code fails to compile.
- [x] `main.go:51` – replace `tls.Listen` with `tls.NewListener` wrapping a TCP listener; ensure the tls package is imported.
- [x] `main.go:39-42` – make health port configurable and handle the returned `http.Server` for graceful shutdown.
- [x] `main.go:44-45` – remove unused `message` struct or use it to encapsulate session state; avoid dead code.
- [x] `main.go:52-55` – handle listener setup errors and log TLS status via `audit.Log`.
- [x] `main.go:62-68` – wrap connection handling with context/cancellation; audit log accepted clients.
- [x] `main.go:76-83` – refresh deadlines per command; ignoring `PrintfLine` errors hides I/O failures.
- [x] `main.go:85` – banner contains typo and reveals internal branding; consider configurable banner for security.
- [x] `main.go:86-125` – enforce SMTP state machine (HELO/EHLO before MAIL, MAIL before RCPT, at least one RCPT before DATA) and clear `from`/`to` after each DATA command.
- [x] `main.go:109-115` – handle `io.Copy` partial reads and limit message size to avoid memory exhaustion.
- [x] `main.go:118-124` – copy `data.Bytes()` per recipient before enqueueing; current slice references mutate on next message, corrupting stored mail.
- [x] `main.go:118-119` – check errors from `storage.SaveMessage` and surface to client/queue; otherwise delivery may retry without persistence.
- [x] `main.go:120-124` – when enqueueing, populate `Attempts` and message ID to support dedupe; consider queueing once per message with recipients list.
- [x] `main.go:135-142` – strengthen address parsing (strip comments, enforce angle brackets, reject newline injection).
- [x] `main.go:145-148` – handle `rand.Read` error and increase ID length to reduce collision risk.
- [x] Duplicate parsing – email address extraction logic overlaps `delivery.ExtractDomain`; centralize to avoid divergence.

**Other considerations**
- [x] Testing – add unit/integration tests for SMTP session handling, queue retries, TLS loader, and delivery client to prevent regressions.
- [x] Observability – expose metrics (queue depth, delivery latency, failure counts) and structured logs.
