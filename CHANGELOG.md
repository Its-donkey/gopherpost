# Changelog

## v0.1.0
- Initial version with local message storage.

## v0.2.0
- Added modular outbound SMTP delivery with MX lookup and STARTTLS support.
- Added retry queue system with exponential backoff for failed deliveries.
- Added storage module to persist messages to disk.
- Integrated delivery, queue, and storage into SMTP Server.
- Added audit logging with `SMTP_DEBUG` toggle.
- Added systemd service file for deployment.
- Added optional TLS via SMTP_TLS_CERT and SMTP_TLS_KEY.
- Added /healthz HTTP endpoint for liveness checks.

## v0.3.0
- Hardened outbound delivery with opportunistic STARTTLS, HELO retries, and MX sorting.
- Introduced reusable email parsing helpers with comprehensive tests.
- Added secure on-disk storage with hashed recipient filenames and identifier sanitisation.
- Improved retry queue with jittered exponential backoff and non-blocking delivery execution.
- Enforced SMTP session state machine, configurable banner, and connection deadline refresh.
- Added TLS defaults (TLS 1.2+) and safer listener initialisation.
- Updated health server to use dedicated mux, timeouts, and surfaced errors.
- Documented configuration, security considerations, and environment variables.
