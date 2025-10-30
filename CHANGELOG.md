# Changelog

## Unreleased
- Rename project to GopherPost.
  - Module path changed to `gopherpost`; updated all internal imports.
  - Binary renamed to `gopherpost` and default banner to "GopherPost ready".
  - Systemd unit renamed to `gopherpost.service`; updated user/group to `gopherpost` and paths under `/opt/gopherpost`, `/var/lib/gopherpost`, `/var/spool/gopherpost`, `/etc/gopherpost`.
  - Installer `install.sh` updated to build/install `gopherpost` and generate the new unit and directories.
  - Documentation, examples, and `.env.example` updated accordingly.
  - TLS certificate test subjects now use `gopherpost.*` names; ephemeral cert CN/Org align with new name.
- .gitignore includes `gopherpost` binaries.
- No changes to `SMTP_*` environment variable names or behavior.

- CI: Add GitHub Action to enforce CHANGELOG updates on every PR, branch naming (`type/section/kebab-feature`), and commit style (`<type-short> (section): message`).
- PRs: Add pull request template with required fields and checklist.
- CI: Validate PR body includes required 'Type:' and 'Section:' fields from the template.
- DevX: Add in-repo Git hooks under `.githooks/` (`pre-commit` to require CHANGELOG and branch pattern, `commit-msg` to enforce commit subject). Enable with: `git config core.hooksPath .githooks`.
- Docs: Add CONTRIBUTING.md and README Contributing section with examples.

## v0.4.0
- Added subscription-based audit fan-out so `/healthz` can stream live debug logs when `SMTP_DEBUG=true`.
- Updated health endpoint to flush `OK` and continuous audit output for real-time monitoring.
- Expanded installer to validate queue directories, generate bespoke systemd units, and perform health probing.
- Documented manual and automated deployment flows, including debug streaming notes.

## v0.3.0
- Hardened outbound delivery with opportunistic STARTTLS, HELO retries, and MX sorting.
- Introduced reusable email parsing helpers with comprehensive tests.
- Added secure on-disk storage with hashed recipient filenames and identifier sanitisation.
- Improved retry queue with jittered exponential backoff and non-blocking delivery execution.
- Enforced SMTP session state machine, configurable banner, and connection deadline refresh.
- Added TLS defaults (TLS 1.2+) and safer listener initialisation.
- Updated health server to use dedicated mux, timeouts, and surfaced errors.
- Documented configuration, security considerations, and environment variables.

## v0.2.0
- Added modular outbound SMTP delivery with MX lookup and STARTTLS support.
- Added retry queue system with exponential backoff for failed deliveries.
- Added storage module to persist messages to disk.
- Integrated delivery, queue, and storage into SMTP Server.
- Added audit logging with `SMTP_DEBUG` toggle.
- Added systemd service file for deployment.
- Added optional TLS via SMTP_TLS_CERT and SMTP_TLS_KEY.
- Added /healthz HTTP endpoint for liveness checks.

## v0.1.0
- Initial version with local message storage.
