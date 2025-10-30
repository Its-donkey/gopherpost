Contributing to GopherPost

Thank you for your interest in contributing! This guide explains our workflow, conventions, and how to get set up locally.

Branching
- Name branches as `type/section/kebab-feature`.
  - Examples: `feature/queue/retry-jitter`, `bugfix/storage/permissions`, `doc/readme/quickstart`.
- Allowed types: feature, bugfix, hotfix, design, refactor, test, doc.

Commits
- Subject format: `<type-short> (section): <message>`
  - type-short: feat, fix, hotfix, design, refactor, test, doc
  - Example: `feat (delivery): opportunistic STARTTLS`
- Push after each discrete change.
- If unrelated files fail lint, you may push with `--no-verify` but keep the scope limited.

Pull Requests
- One PR per feature; open/update as needed.
- PR description must include:
  - `Type: <feature|bugfix|hotfix|design|refactor|test|doc>`
  - `Section: <area/owner>`
- Update `CHANGELOG.md` for every PR unless a maintainer agrees to label it `no-changelog`.
- Use labels to aid triage (e.g., `refactor`, `docs`, `packaging`, `ci`).

Local Git Hooks
- This repository includes hooks under `.githooks/` to help enforce conventions locally.
- Enable them with:
  - `git config core.hooksPath .githooks`
- Hooks provided:
  - `pre-commit`: requires `CHANGELOG.md` be included when other files are staged, and checks branch naming.
  - `commit-msg`: enforces commit subject format.
- Bypass changelog requirement when appropriate by setting `NO_CHANGELOG=1` in your environment for that commit.

CI Enforcement
- All PRs are checked for:
  - CHANGELOG update (unless labeled `no-changelog`).
  - Branch naming convention.
  - Commit subject format.
  - PR body fields `Type:` and `Section:`.

Development Setup
- Go 1.21+
- Run tests locally:
  - `go test ./...`
- Build locally:
  - `go build -o gopherpost ./...`

Code Style
- Prefer small, focused changes.
- Match the existing code style of the files you touch.
- Add/adjust tests when changing behavior.

Security & Operations
- Do not include secrets in commits or PR bodies.
- Follow least-privilege defaults (e.g., file permissions, service users).

Thanks for contributing!

