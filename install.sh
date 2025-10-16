#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

if ! command -v go >/dev/null 2>&1; then
  echo "go command not found; please install Go 1.21 or newer." >&2
  exit 1
fi

GO_VERSION="$(go env GOVERSION 2>/dev/null || true)"
REQUIRED_MAJOR=1
REQUIRED_MINOR=21

if [[ -n "$GO_VERSION" ]]; then
  VERSION="${GO_VERSION#go}"
  MAJOR="${VERSION%%.*}"
  MINOR="${VERSION#*.}"
  MINOR="${MINOR%%.*}"
  if (( MAJOR < REQUIRED_MAJOR || (MAJOR == REQUIRED_MAJOR && MINOR < REQUIRED_MINOR) )); then
    echo "Go $REQUIRED_MAJOR.$REQUIRED_MINOR or newer required, but $GO_VERSION found." >&2
    exit 1
  fi
fi

echo "Tidying module dependencies…"
go mod tidy

OUTPUT_DIR="${OUTPUT_DIR:-"$ROOT_DIR/bin"}"
mkdir -p "$OUTPUT_DIR"

echo "Building smtpserver binary…"
GO111MODULE=on go build -o "$OUTPUT_DIR/smtpserver" .

cat <<EOF

Installation complete.
- Binary: $OUTPUT_DIR/smtpserver
- Health endpoint listens via SMTP_HEALTH_ADDR (default :8080)
- Run with: $OUTPUT_DIR/smtpserver

EOF


