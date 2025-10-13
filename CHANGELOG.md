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
