// file name — /install.sh
#!/usr/bin/env bash
set -euo pipefail

# --- root guard ---
if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
	echo "Please run as root (sudo)." >&2
	exit 1
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

VERSION="0.4.0"
NAME="GopherPost SMTP Server"
URL="gopherpost.io"

# Derive lowercase + slug from NAME (portable on macOS Bash 3.2 and Linux)
NAME_LOWER="$(printf '%s' "${NAME%%[[:space:]]*}" | tr '[:upper:]' '[:lower:]')"
NAME_SLUG="$(printf '%s' "$NAME_LOWER" | tr ' ' '-' | tr -cd 'a-z0-9-')"

# Binary, user, group, paths derived from slug
BINARY_NAME="${BINARY_NAME:-$NAME_SLUG}"
APP_USER="${APP_USER:-$NAME_SLUG}"
APP_GROUP="${APP_GROUP:-$APP_USER}"
OPT_DIR="/opt/${NAME_SLUG}"
VAR_LIB_DIR="/var/lib/${NAME_SLUG}"
ETC_DIR="/etc/${NAME_SLUG}"
DKIM_DIR="${ETC_DIR}/dkim"
DEFAULT_QUEUE_DIR="/var/spool/${NAME_SLUG}/"
SYSTEMD_UNIT="${NAME_SLUG}.service"

# --- Go presence & version check ---
if ! command -v go >/dev/null 2>&1; then
	echo "go command not found; please install Go 1.21 or newer." >&2
	exit 1
fi
REQUIRED_MAJOR=1 REQUIRED_MINOR=21
GO_VERSION="$(go env GOVERSION 2>/dev/null || true)"
if [[ -z "$GO_VERSION" ]]; then
	# fallback: parse `go version` output like: "go version go1.22.3 linux/amd64"
	GO_VERSION="$(go version | awk '{print $3}')" || true
fi
if [[ -n "$GO_VERSION" ]]; then
	V="${GO_VERSION#go}"
	MAJOR="${V%%.*}"
	MINOR="${V#*.}"; MINOR="${MINOR%%.*}"
	if (( MAJOR < REQUIRED_MAJOR || (MAJOR == REQUIRED_MAJOR && MINOR < REQUIRED_MINOR) )); then
		echo "Go ${REQUIRED_MAJOR}.${REQUIRED_MINOR} or newer required, but ${GO_VERSION} found." >&2
		exit 1
	fi
fi

# --- system user, dirs ---
if ! id -u "$APP_USER" >/dev/null 2>&1; then
	useradd --system --home "$OPT_DIR" --shell /usr/sbin/nologin "$APP_USER"
fi
mkdir -p "$OPT_DIR" "$VAR_LIB_DIR" "$DKIM_DIR"
chown -R "$APP_USER:$APP_GROUP" "$OPT_DIR" "$VAR_LIB_DIR" "$ETC_DIR" 2>/dev/null || true

OUT="${1:-./${SYSTEMD_UNIT}}"

# ---------- prompt helpers ----------
ask() {
	local var="$1" prompt="$2" default="${3:-}" input=""
	if [[ -n "$default" ]]; then
		read -r -p "$prompt [$default]: " input || true
		printf '%s' "${input:-$default}"
	else
		read -r -p "$prompt (leave blank to skip): " input || true
		printf '%s' "$input"
	fi
}
esc_env_val() {
	local s="$1"
	s="${s//\\/\\\\}"; s="${s//\"/\\\"}"
	printf '%s' "$s"
}

build_health_url() {
	local addr="$1"
	local port="$2"
	local host="$addr"

	if [[ -z "$host" ]]; then
		host="localhost"
	fi

	if [[ "$host" == :* ]]; then
		if [[ -z "$port" ]]; then
			port="${host#:}"
		fi
		host="localhost"
	fi

	if [[ "$host" == *:* ]]; then
		local maybe_host="${host%:*}"
		local maybe_port="${host##*:}"
		if [[ "$maybe_host" != "$host" ]]; then
			host="${maybe_host:-localhost}"
			if [[ -z "$port" && -n "$maybe_port" ]]; then
				port="$maybe_port"
			fi
		fi
	fi

	if [[ -z "$port" ]]; then
		port="8080"
	fi

	printf 'http://%s:%s/healthz' "$host" "$port"
}

echo "== ${NAME} v${VERSION} service generator ==" 

# ---------- [Unit] & [Service] headers ----------
UNIT_SECTION=$'[Unit]\nDescription='${NAME}$'\nAfter=network-online.target\nWants=network-online.target\n\n'
SERVICE_SECTION=$'[Service]\nType=simple'

# ---------- Inputs ----------
SMTP_PORT="$(ask SMTP_PORT 'SMTP_PORT' '2525')"
SMTP_HOSTNAME="$(ask SMTP_HOSTNAME 'SMTP_HOSTNAME' "${URL}")"
SMTP_BANNER="$(ask SMTP_BANNER 'SMTP_BANNER' 'GopherPost ready')"
SMTP_DEBUG="$(ask SMTP_DEBUG 'SMTP_DEBUG (true/false)' 'false')"

SMTP_HEALTH_ADDR="$(ask SMTP_HEALTH_ADDR 'SMTP_HEALTH_ADDR' '')"
SMTP_HEALTH_PORT="$(ask SMTP_HEALTH_PORT 'SMTP_HEALTH_PORT' '8877')"
SMTP_HEALTH_DISABLE="$(ask SMTP_HEALTH_DISABLE 'SMTP_HEALTH_DISABLE (true/false)' 'false')"

SMTP_QUEUE_PATH="$(ask SMTP_QUEUE_PATH 'SMTP_QUEUE_PATH' "${DEFAULT_QUEUE_DIR}")"
SMTP_QUEUE_WORKERS="$(ask SMTP_QUEUE_WORKERS 'SMTP_QUEUE_WORKERS (concurrent workers, blank for auto)' '')"
SMTP_ALLOW_NETWORKS="$(ask SMTP_ALLOW_NETWORKS 'SMTP_ALLOW_NETWORKS (CIDR list)' '')"
SMTP_ALLOW_HOSTS="$(ask SMTP_ALLOW_HOSTS 'SMTP_ALLOW_HOSTS (comma-separated)' '127.0.0.1')"
SMTP_REQUIRE_LOCAL_DOMAIN="$(ask SMTP_REQUIRE_LOCAL_DOMAIN 'SMTP_REQUIRE_LOCAL_DOMAIN (true/false)' 'true')"

SMTP_TLS_DISABLE="$(ask SMTP_TLS_DISABLE 'SMTP_TLS_DISABLE (true/false)' 'false')"
SMTP_TLS_CERT=""; SMTP_TLS_KEY=""
if [[ "$SMTP_TLS_DISABLE" == "false" ]]; then
	SMTP_TLS_CERT="$(ask SMTP_TLS_CERT 'SMTP_TLS_CERT (path to cert)' '')"
	SMTP_TLS_KEY="$(ask SMTP_TLS_KEY 'SMTP_TLS_KEY (path to key)' '')"
fi

SMTP_DKIM_SELECTOR="$(ask SMTP_DKIM_SELECTOR 'SMTP_DKIM_SELECTOR' 's1')"
SMTP_DKIM_DOMAIN="$(ask SMTP_DKIM_DOMAIN 'SMTP_DKIM_DOMAIN' "${URL}")"
SMTP_DKIM_KEY_PATH="$(ask SMTP_DKIM_KEY_PATH 'SMTP_DKIM_KEY_PATH (path to PEM)' "${DKIM_DIR}/s1.key")"
SMTP_DKIM_PRIVATE_KEY="$(ask SMTP_DKIM_PRIVATE_KEY 'SMTP_DKIM_PRIVATE_KEY (single-line; prefer KEY_PATH)' '')"

WORKING_DIR="$(ask WorkingDirectory 'WorkingDirectory (absolute path)' "${OPT_DIR}")"
EXEC_DIR="$(ask ExecutableDirectory 'ExecutableDirectory (binary path)' '/usr/local/bin')"

# Ensure DKIM dir exists and perms are safe if path provided
if [[ -n "$SMTP_DKIM_KEY_PATH" ]]; then
    install -d -m 0750 -o "$APP_USER" -g "$APP_GROUP" "$(dirname "$SMTP_DKIM_KEY_PATH")"
	if [[ -f "$SMTP_DKIM_KEY_PATH" ]]; then
        chown "$APP_USER:$APP_GROUP" "$SMTP_DKIM_KEY_PATH"
		chmod 0600 "$SMTP_DKIM_KEY_PATH"
	fi
fi

# Ensure queue directory exists with appropriate permissions
if [[ -n "$SMTP_QUEUE_PATH" ]]; then
    install -d -m 0750 -o "$APP_USER" -g "$APP_GROUP" "$SMTP_QUEUE_PATH"
fi

# ---------- Build Environment= lines ----------
env_lines=()
env_lines+=("Environment=\"SMTP_PORT=$(esc_env_val "$SMTP_PORT")\"")
env_lines+=("Environment=\"SMTP_HOSTNAME=$(esc_env_val "$SMTP_HOSTNAME")\"")
env_lines+=("Environment=\"SMTP_BANNER=$(esc_env_val "$SMTP_BANNER")\"")
env_lines+=("Environment=\"SMTP_DEBUG=$(esc_env_val "$SMTP_DEBUG")\"")
[[ -n "$SMTP_HEALTH_ADDR" ]] && env_lines+=("Environment=\"SMTP_HEALTH_ADDR=$(esc_env_val "$SMTP_HEALTH_ADDR")\"")
env_lines+=("Environment=\"SMTP_HEALTH_PORT=$(esc_env_val "$SMTP_HEALTH_PORT")\"")
env_lines+=("Environment=\"SMTP_HEALTH_DISABLE=$(esc_env_val "$SMTP_HEALTH_DISABLE")\"")
env_lines+=("Environment=\"SMTP_QUEUE_PATH=$(esc_env_val "$SMTP_QUEUE_PATH")\"")
[[ -n "$SMTP_QUEUE_WORKERS" ]] && env_lines+=("Environment=\"SMTP_QUEUE_WORKERS=$(esc_env_val "$SMTP_QUEUE_WORKERS")\"")
env_lines+=("Environment=\"SMTP_ALLOW_NETWORKS=$(esc_env_val "$SMTP_ALLOW_NETWORKS")\"")
env_lines+=("Environment=\"SMTP_ALLOW_HOSTS=$(esc_env_val "$SMTP_ALLOW_HOSTS")\"")
env_lines+=("Environment=\"SMTP_REQUIRE_LOCAL_DOMAIN=$(esc_env_val "$SMTP_REQUIRE_LOCAL_DOMAIN")\"")
env_lines+=("Environment=\"SMTP_TLS_DISABLE=$(esc_env_val "$SMTP_TLS_DISABLE")\"")
if [[ "$SMTP_TLS_DISABLE" == "false" ]]; then
	[[ -n "$SMTP_TLS_CERT" ]] && env_lines+=("Environment=\"SMTP_TLS_CERT=$(esc_env_val "$SMTP_TLS_CERT")\"")
	[[ -n "$SMTP_TLS_KEY"  ]] && env_lines+=("Environment=\"SMTP_TLS_KEY=$(esc_env_val "$SMTP_TLS_KEY")\"")
fi
env_lines+=("Environment=\"SMTP_DKIM_SELECTOR=$(esc_env_val "$SMTP_DKIM_SELECTOR")\"")
env_lines+=("Environment=\"SMTP_DKIM_DOMAIN=$(esc_env_val "$SMTP_DKIM_DOMAIN")\"")
[[ -n "$SMTP_DKIM_KEY_PATH"    ]] && env_lines+=("Environment=\"SMTP_DKIM_KEY_PATH=$(esc_env_val "$SMTP_DKIM_KEY_PATH")\"")
[[ -n "$SMTP_DKIM_PRIVATE_KEY" ]] && env_lines+=("Environment=\"SMTP_DKIM_PRIVATE_KEY=$(esc_env_val "$SMTP_DKIM_PRIVATE_KEY")\"")
ENV_BLOCK=$(printf '%s\n' "${env_lines[@]}")

# ---------- Compose final unit ----------
{
	printf '%s\n' "$UNIT_SECTION"
	printf '%s\n' "$SERVICE_SECTION"
    printf '%s\n' "$ENV_BLOCK"
    printf 'User=%s\n' "$APP_USER"
    printf 'Group=%s\n' "$APP_GROUP"
    printf 'WorkingDirectory=%s\n' "$WORKING_DIR"
    printf 'ExecStart=%s/%s\n' "$EXEC_DIR" "$BINARY_NAME"
	printf 'Restart=on-failure\nRestartSec=2\n'
	printf 'StandardOutput=journal\nStandardError=journal\n'
	# Hardening (adjust if your binary needs extra writes):
	printf 'NoNewPrivileges=true\n'
	printf 'PrivateTmp=true\n'
	printf 'ProtectSystem=strict\n'
	printf 'ProtectHome=true\n'
    printf 'ReadWritePaths=%s %s %s\n' "$VAR_LIB_DIR" "$SMTP_QUEUE_PATH" "$ETC_DIR"
    printf 'RuntimeDirectory=%s\n' "$NAME_SLUG"
    printf 'StateDirectory=%s\n' "$NAME_SLUG"
	printf 'LimitNOFILE=65536\n'
	printf '\n[Install]\nWantedBy=multi-user.target\n'
} > "$OUT"

# Make sure unit file is world-readable
chmod 0644 "$OUT"

echo "Tidying module dependencies…"
go mod tidy

OUTPUT_DIR="${OUTPUT_DIR:-"$ROOT_DIR/bin"}"
mkdir -p "$OUTPUT_DIR"

echo "Building ${NAME} v${VERSION} binary as '${BINARY_NAME}'…"
GO111MODULE=on go build -o "$OUTPUT_DIR/${BINARY_NAME}" .

# Install binary to EXEC_DIR
install -d -m 0755 "$EXEC_DIR"
install -o root -g root -m 0755 "$OUTPUT_DIR/${BINARY_NAME}" "$EXEC_DIR/${BINARY_NAME}"

# Ensure app-owned directories exist with correct ownership
install -d -m 0750 -o "$APP_USER" -g "$APP_GROUP" "$OPT_DIR" "$VAR_LIB_DIR" "$DKIM_DIR" "$SMTP_QUEUE_PATH"

# Install unit and enable
systemctl stop "$SYSTEMD_UNIT" 2>/dev/null || true
mv "$OUT" "/etc/systemd/system/${SYSTEMD_UNIT}"
systemctl daemon-reload
systemctl enable --now "$SYSTEMD_UNIT"
systemctl --no-pager --full status "$SYSTEMD_UNIT" || true

echo "[STARTING]::${NAME}"
sleep 5
# Optional: health check
if [[ "$SMTP_HEALTH_DISABLE" == "false" ]]; then
	if command -v curl >/dev/null 2>/dev/null; then
		HEALTH_URL="$(build_health_url "$SMTP_HEALTH_ADDR" "$SMTP_HEALTH_PORT")"
		if curl --silent --show-error --output /dev/null "$HEALTH_URL"; then
        	echo "[SERVER STARTED]::${NAME}"
		else
		 	echo "Health check failed for ${HEALTH_URL}; inspect systemctl status." >&2
		fi
	else
		echo "curl not installed; skipping health check." >&2
	fi
fi
