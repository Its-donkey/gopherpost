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
- Repo: Update .gitignore to ignore VS Code settings (`.vscode/*`) and workspace files (`*.code-workspace`).
- Installer: Improve slug derivation for NAME_LOWER to avoid spaces in slug.
- Packaging: Update systemd unit description to "GopherPost SMTP Server".
- Website: Add GitHub Pages ready static site under `docs/` (logo, styles, landing page).
- Docs: README now links to the hosted site and documents Pages configuration.
- Website: Adjust logo dimensions on landing page for better visibility.
- Website: Embed Google Analytics (G-DWM55QD5ZH) in the static site.
- Website: Configure custom domain via `docs/CNAME` and document at gopherpost.io.
- Website: Publish contact email `gday@gopherpost.io` on site and README.
- Website: Add favicon asset (`docs/assets/favicon.svg`) and wire it into the page.
- Website: Add dedicated contact section with email and GitHub links.
- Docs: Consolidate feature list in README (single "Features" section).
- Website: Mirror unified feature list on the marketing page.
- SMTP: Send a 554 5.7.1 rejection prior to closing unauthorized sessions so operators see explicit failures when hosts are misconfigured.
- Storage: Roll back partially persisted messages when any recipient write fails to keep the spool aligned with the delivery queue.
- Queue: Introduce a configurable worker pool with idempotent shutdown semantics and tests that verify real concurrency instead of timing heuristics.
- Config: Document and surface the new `SMTP_QUEUE_WORKERS` environment variable across README, `.env.example`, install tooling, and the marketing site.
- Brand: Refresh GopherPost logo and favicon with updated gopher-and-envelope concept; refine site header hover styling and add ringed-logo hover treatment.

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
