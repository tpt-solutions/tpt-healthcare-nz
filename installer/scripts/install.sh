#!/usr/bin/env bash
# TPT Healthcare NZ — Linux/macOS installer
# Usage: curl -fsSL https://install.tpt.health/nz | bash
#        bash install.sh [--from-source] [--no-service] [--uninstall] [--help]

set -euo pipefail

# ---------------------------------------------------------------------------
# Colours
# ---------------------------------------------------------------------------
if [ -t 1 ] && command -v tput &>/dev/null; then
  RED=$(tput setaf 1)
  GREEN=$(tput setaf 2)
  YELLOW=$(tput setaf 3)
  CYAN=$(tput setaf 6)
  BOLD=$(tput bold)
  RESET=$(tput sgr0)
else
  RED="" GREEN="" YELLOW="" CYAN="" BOLD="" RESET=""
fi

info()    { printf "${CYAN}[INFO]${RESET}  %s\n" "$*"; }
ok()      { printf "${GREEN}[OK]${RESET}    %s\n" "$*"; }
warn()    { printf "${YELLOW}[WARN]${RESET}  %s\n" "$*"; }
error()   { printf "${RED}[ERROR]${RESET} %s\n" "$*" >&2; }
die()     { error "$*"; exit 1; }
header()  { printf "\n${BOLD}${CYAN}==> %s${RESET}\n" "$*"; }

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------
APP_NAME="tpt-health-interop"
GITHUB_ORG="PhillipC05"
GITHUB_REPO="tpt-healthcare-nz"
BINARY_VERSION="${TPT_VERSION:-latest}"
DOWNLOAD_BASE="https://github.com/${GITHUB_ORG}/${GITHUB_REPO}/releases"
SERVICE_USER="tpt-healthcare"
SERVICE_GROUP="tpt-healthcare"

# ---------------------------------------------------------------------------
# Flags
# ---------------------------------------------------------------------------
OPT_FROM_SOURCE=false
OPT_NO_SERVICE=false
OPT_UNINSTALL=false

usage() {
  cat <<EOF
${BOLD}TPT Healthcare NZ Installer${RESET}

Usage:
  install.sh [OPTIONS]

Options:
  --from-source   Build tpt-health-interop from source (requires Go 1.22+)
  --no-service    Skip systemd/LaunchAgent service installation
  --uninstall     Remove TPT Healthcare NZ from this machine
  --help          Show this help message

Environment variables:
  TPT_VERSION     Binary release tag to install (default: latest)
  TPT_INSTALL_DIR Override install directory (default: /opt/tpt-healthcare or
                  ~/.tpt-healthcare if sudo is unavailable)
EOF
}

for arg in "$@"; do
  case "$arg" in
    --from-source) OPT_FROM_SOURCE=true ;;
    --no-service)  OPT_NO_SERVICE=true ;;
    --uninstall)   OPT_UNINSTALL=true ;;
    --help|-h)     usage; exit 0 ;;
    *) die "Unknown option: $arg. Run with --help for usage." ;;
  esac
done

# ---------------------------------------------------------------------------
# OS / arch detection
# ---------------------------------------------------------------------------
detect_platform() {
  header "Detecting platform"

  OS="$(uname -s)"
  ARCH="$(uname -m)"

  case "$OS" in
    Linux)  OS_NAME="linux" ;;
    Darwin) OS_NAME="darwin" ;;
    *) die "Unsupported OS: $OS. This installer supports Linux and macOS only." ;;
  esac

  case "$ARCH" in
    x86_64|amd64) ARCH_NAME="amd64" ;;
    arm64|aarch64) ARCH_NAME="arm64" ;;
    *) die "Unsupported architecture: $ARCH. Supported: amd64, arm64." ;;
  esac

  ok "Detected: ${OS_NAME}/${ARCH_NAME}"
}

# ---------------------------------------------------------------------------
# Privilege detection
# ---------------------------------------------------------------------------
detect_privileges() {
  if [ "$(id -u)" -eq 0 ]; then
    HAS_SUDO=true
    SUDO=""
  elif command -v sudo &>/dev/null && sudo -n true 2>/dev/null; then
    HAS_SUDO=true
    SUDO="sudo"
  else
    HAS_SUDO=false
    SUDO=""
    warn "No root/sudo access. Installing to user directories."
  fi
}

# ---------------------------------------------------------------------------
# Directory layout
# ---------------------------------------------------------------------------
set_directories() {
  if [ -n "${TPT_INSTALL_DIR:-}" ]; then
    INSTALL_DIR="$TPT_INSTALL_DIR"
  elif $HAS_SUDO; then
    INSTALL_DIR="/opt/tpt-healthcare"
  else
    INSTALL_DIR="${HOME}/.tpt-healthcare"
  fi

  BIN_DIR="${INSTALL_DIR}/bin"
  DATA_DIR="${INSTALL_DIR}/data"

  if $HAS_SUDO; then
    CONFIG_DIR="/etc/tpt-healthcare"
    ENV_FILE="/etc/tpt-healthcare/environment"
    LOG_DIR="/var/log/tpt-healthcare"
    LIB_DIR="/var/lib/tpt-healthcare"
  else
    CONFIG_DIR="${HOME}/.config/tpt-healthcare"
    ENV_FILE="${HOME}/.config/tpt-healthcare/environment"
    LOG_DIR="${HOME}/.local/share/tpt-healthcare/logs"
    LIB_DIR="${HOME}/.local/share/tpt-healthcare/data"
  fi

  CONFIG_FILE="${CONFIG_DIR}/config.yaml"

  info "Install directory : ${INSTALL_DIR}"
  info "Config directory  : ${CONFIG_DIR}"
  info "Log directory     : ${LOG_DIR}"
}

# ---------------------------------------------------------------------------
# Prerequisite checks
# ---------------------------------------------------------------------------
check_prerequisites() {
  header "Checking prerequisites"

  local missing=()

  # Go 1.22+ (only required for --from-source)
  if $OPT_FROM_SOURCE; then
    if command -v go &>/dev/null; then
      GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
      GO_MAJOR=$(echo "$GO_VERSION" | cut -d. -f1)
      GO_MINOR=$(echo "$GO_VERSION" | cut -d. -f2)
      if [ "$GO_MAJOR" -lt 1 ] || { [ "$GO_MAJOR" -eq 1 ] && [ "$GO_MINOR" -lt 22 ]; }; then
        die "Go 1.22+ is required for --from-source (found go${GO_VERSION})."
      fi
      ok "Go ${GO_VERSION}"
    else
      missing+=("go (1.22+ required for --from-source; see https://go.dev/dl/)")
    fi
  fi

  # Docker
  if command -v docker &>/dev/null; then
    if docker info &>/dev/null 2>&1; then
      ok "Docker $(docker --version | awk '{print $3}' | tr -d ',')"
    else
      warn "Docker is installed but the daemon is not running. Start Docker before running migrations."
    fi
  else
    missing+=("docker (see https://docs.docker.com/engine/install/)")
  fi

  # pnpm (optional — only needed to build frontend)
  if command -v pnpm &>/dev/null; then
    ok "pnpm $(pnpm --version)"
  else
    warn "pnpm not found — frontend apps will not be built. Install via: npm install -g pnpm"
  fi

  # PostgreSQL client (psql)
  if command -v psql &>/dev/null; then
    ok "psql $(psql --version | awk '{print $3}')"
  else
    missing+=("postgresql-client (psql must be on PATH for migration verification)")
  fi

  # curl or wget for downloads
  if ! command -v curl &>/dev/null && ! command -v wget &>/dev/null; then
    missing+=("curl or wget (required for downloading the binary)")
  else
    ok "curl/wget present"
  fi

  if [ ${#missing[@]} -gt 0 ]; then
    error "Missing prerequisites:"
    for item in "${missing[@]}"; do
      error "  • ${item}"
    done
    die "Please install the above tools and re-run the installer."
  fi
}

# ---------------------------------------------------------------------------
# Download helpers
# ---------------------------------------------------------------------------
download() {
  local url="$1"
  local dest="$2"
  if command -v curl &>/dev/null; then
    curl -fsSL --retry 3 --retry-delay 2 -o "$dest" "$url"
  else
    wget -qO "$dest" "$url"
  fi
}

fetch_latest_version() {
  if command -v curl &>/dev/null; then
    curl -fsSL "https://api.github.com/repos/${GITHUB_ORG}/${GITHUB_REPO}/releases/latest" \
      | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/'
  else
    wget -qO- "https://api.github.com/repos/${GITHUB_ORG}/${GITHUB_REPO}/releases/latest" \
      | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/'
  fi
}

# ---------------------------------------------------------------------------
# Create directories
# ---------------------------------------------------------------------------
create_directories() {
  header "Creating directories"

  for dir in "$INSTALL_DIR" "$BIN_DIR" "$DATA_DIR" "$CONFIG_DIR" "$LOG_DIR" "$LIB_DIR"; do
    if [ ! -d "$dir" ]; then
      $SUDO mkdir -p "$dir"
      ok "Created ${dir}"
    else
      info "Exists: ${dir}"
    fi
  done

  # Restrict config directory permissions
  $SUDO chmod 750 "$CONFIG_DIR" || true
}

# ---------------------------------------------------------------------------
# Install binary
# ---------------------------------------------------------------------------
install_binary() {
  header "Installing ${APP_NAME} binary"

  if $OPT_FROM_SOURCE; then
    build_from_source
    return
  fi

  local version="$BINARY_VERSION"
  if [ "$version" = "latest" ]; then
    info "Fetching latest release version..."
    version=$(fetch_latest_version) || true
    if [ -z "$version" ]; then
      warn "Could not determine latest version; falling back to --from-source build."
      OPT_FROM_SOURCE=true
      build_from_source
      return
    fi
    info "Latest version: ${version}"
  fi

  local tarball="${APP_NAME}_${OS_NAME}_${ARCH_NAME}.tar.gz"
  local url="${DOWNLOAD_BASE}/download/${version}/${tarball}"
  local tmp_dir
  tmp_dir=$(mktemp -d)
  trap 'rm -rf "$tmp_dir"' EXIT

  info "Downloading ${url}"
  if ! download "$url" "${tmp_dir}/${tarball}"; then
    warn "Download failed — falling back to building from source."
    OPT_FROM_SOURCE=true
    build_from_source
    return
  fi

  tar -xzf "${tmp_dir}/${tarball}" -C "$tmp_dir"
  $SUDO install -m 0755 "${tmp_dir}/${APP_NAME}" "${BIN_DIR}/${APP_NAME}"
  ok "Installed ${BIN_DIR}/${APP_NAME}"
}

build_from_source() {
  info "Building ${APP_NAME} from source..."

  # Determine source root (assumes this script lives in installer/scripts/)
  SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  SOURCE_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

  if [ ! -f "${SOURCE_ROOT}/go.work" ]; then
    die "Source root not found at ${SOURCE_ROOT}. Cannot build from source."
  fi

  (
    cd "${SOURCE_ROOT}"
    go build \
      -ldflags="-s -w -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" \
      -o "${BIN_DIR}/${APP_NAME}" \
      ./interop/cmd/tpt-health-interop/...
  )
  ok "Built and installed ${BIN_DIR}/${APP_NAME}"
}

# ---------------------------------------------------------------------------
# Generate encryption key
# ---------------------------------------------------------------------------
generate_encryption_key() {
  # 32 random bytes hex-encoded = 64 hex chars
  if command -v openssl &>/dev/null; then
    openssl rand -hex 32
  elif [ -r /dev/urandom ]; then
    dd if=/dev/urandom bs=32 count=1 2>/dev/null | od -A n -t x1 | tr -d ' \n'
  else
    die "Cannot generate random bytes: neither openssl nor /dev/urandom is available."
  fi
}

# ---------------------------------------------------------------------------
# Write config files
# ---------------------------------------------------------------------------
write_config() {
  header "Writing configuration"

  local enc_key
  enc_key=$(generate_encryption_key)
  ok "Generated ENCRYPTION_KEY (32 bytes, hex)"

  # --- config.yaml ---
  $SUDO tee "$CONFIG_FILE" > /dev/null <<YAML
# TPT Healthcare NZ — main configuration
# Generated by installer on $(date -u +"%Y-%m-%dT%H:%M:%SZ")
# IMPORTANT: Keep this file private (mode 640, owned by ${SERVICE_USER}).

server:
  listen: "0.0.0.0:8080"
  tls_cert_file: ""
  tls_key_file: ""
  base_url: "http://localhost:8080"

database:
  # PostgreSQL DSN — override via POSTGRES_DSN environment variable
  dsn: "postgres://tpt_healthcare:changeme@localhost:5432/tpt_healthcare?sslmode=prefer"
  max_open_conns: 20
  max_idle_conns: 5
  conn_max_lifetime: "5m"

redis:
  addr: "localhost:6379"
  password: ""
  db: 0

auth:
  # mode: local_jwt | auth0 | tpt_identity
  mode: "local_jwt"
  jwt:
    # Ed25519 private key path (generated on first run if absent)
    private_key_file: "${CONFIG_DIR}/jwt_ed25519.pem"
    token_ttl: "1h"
    refresh_ttl: "24h"
  totp:
    issuer: "TPT Healthcare NZ"

logging:
  level: "info"        # debug | info | warn | error
  format: "json"       # json | text
  output: "${LOG_DIR}/interop.log"

migrations:
  auto_run: true

wizard:
  completed: false
YAML

  $SUDO chmod 640 "$CONFIG_FILE"
  ok "Config written to ${CONFIG_FILE}"

  # --- environment file ---
  $SUDO tee "$ENV_FILE" > /dev/null <<ENV
# TPT Healthcare NZ — environment variables for systemd/launchd
# Generated by installer on $(date -u +"%Y-%m-%dT%H:%M:%SZ")
# IMPORTANT: This file contains secrets. Mode 640, owned by ${SERVICE_USER}.

ENCRYPTION_KEY=${enc_key}
CONFIG_FILE=${CONFIG_FILE}
TPT_LOG_LEVEL=info
ENV

  $SUDO chmod 640 "$ENV_FILE"
  ok "Environment file written to ${ENV_FILE}"
}

# ---------------------------------------------------------------------------
# Service user (Linux only)
# ---------------------------------------------------------------------------
create_service_user() {
  if [ "$OS_NAME" != "linux" ] || ! $HAS_SUDO; then
    return
  fi

  if id "$SERVICE_USER" &>/dev/null; then
    info "Service user '${SERVICE_USER}' already exists."
    return
  fi

  $SUDO useradd \
    --system \
    --no-create-home \
    --shell /sbin/nologin \
    --comment "TPT Healthcare service account" \
    --group "$SERVICE_GROUP" \
    "$SERVICE_USER" 2>/dev/null || \
  $SUDO useradd \
    --system \
    --no-create-home \
    --shell /sbin/nologin \
    --comment "TPT Healthcare service account" \
    "$SERVICE_USER"

  ok "Created system user: ${SERVICE_USER}"

  # Ownership of directories
  for dir in "$LIB_DIR" "$LOG_DIR" "$CONFIG_DIR"; do
    $SUDO chown -R "${SERVICE_USER}:${SERVICE_GROUP}" "$dir" 2>/dev/null || \
    $SUDO chown -R "${SERVICE_USER}" "$dir" || true
  done
}

# ---------------------------------------------------------------------------
# Database migrations
# ---------------------------------------------------------------------------
run_migrations() {
  header "Running database migrations"

  if ! "${BIN_DIR}/${APP_NAME}" migrate --config "$CONFIG_FILE" 2>/dev/null; then
    warn "Automatic migration failed or binary is not yet available."
    warn "Run migrations manually after configuring the database:"
    warn "  ${BIN_DIR}/${APP_NAME} migrate --config ${CONFIG_FILE}"
  else
    ok "Migrations applied."
  fi
}

# ---------------------------------------------------------------------------
# Systemd service (Linux)
# ---------------------------------------------------------------------------
install_systemd_service() {
  if [ "$OS_NAME" != "linux" ] || ! $HAS_SUDO; then
    return
  fi
  if $OPT_NO_SERVICE; then
    info "Skipping systemd service installation (--no-service)."
    return
  fi

  header "Installing systemd service"

  local unit_src
  SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  unit_src="${SCRIPT_DIR}/../systemd/tpt-health-interop.service"

  if [ ! -f "$unit_src" ]; then
    warn "Unit file not found at ${unit_src}; writing inline unit."
    unit_src="/tmp/tpt-health-interop.service"
    cat > "$unit_src" <<UNIT
[Unit]
Description=TPT Health Interoperability Service
After=network.target postgresql.service redis.service

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_GROUP}
ExecStart=${BIN_DIR}/${APP_NAME} serve
EnvironmentFile=${ENV_FILE}
Restart=always
RestartSec=5s
StandardOutput=journal
StandardError=journal
SyslogIdentifier=tpt-health-interop
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=${LIB_DIR}
PrivateTmp=true

[Install]
WantedBy=multi-user.target
UNIT
  fi

  $SUDO cp "$unit_src" /etc/systemd/system/tpt-health-interop.service
  $SUDO systemctl daemon-reload
  $SUDO systemctl enable --now tpt-health-interop.service
  ok "systemd service enabled and started."
  info "Service status: systemctl status tpt-health-interop"
}

# ---------------------------------------------------------------------------
# LaunchAgent (macOS)
# ---------------------------------------------------------------------------
install_launchagent() {
  if [ "$OS_NAME" != "darwin" ]; then
    return
  fi
  if $OPT_NO_SERVICE; then
    info "Skipping LaunchAgent installation (--no-service)."
    return
  fi

  header "Installing macOS LaunchAgent"

  local plist_src
  SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  plist_src="${SCRIPT_DIR}/../launchagent/nz.co.tpt.health-interop.plist"
  local plist_dest="${HOME}/Library/LaunchAgents/nz.co.tpt.health-interop.plist"

  if [ ! -f "$plist_src" ]; then
    warn "Plist not found at ${plist_src}; skipping LaunchAgent install."
    return
  fi

  # Substitute paths in the plist
  sed \
    -e "s|{{BIN_DIR}}|${BIN_DIR}|g" \
    -e "s|{{CONFIG_FILE}}|${CONFIG_FILE}|g" \
    -e "s|{{ENV_FILE}}|${ENV_FILE}|g" \
    -e "s|{{LOG_DIR}}|${HOME}/Library/Logs/TPT|g" \
    "$plist_src" > "$plist_dest"

  launchctl load -w "$plist_dest"
  ok "LaunchAgent loaded: nz.co.tpt.health-interop"
  info "To stop: launchctl unload ${plist_dest}"
}

# ---------------------------------------------------------------------------
# Uninstall
# ---------------------------------------------------------------------------
uninstall() {
  header "Uninstalling TPT Healthcare NZ"
  detect_privileges

  if [ "$OS_NAME" = "linux" ] && $HAS_SUDO && systemctl list-units --full --all | grep -q "tpt-health-interop"; then
    $SUDO systemctl stop tpt-health-interop.service || true
    $SUDO systemctl disable tpt-health-interop.service || true
    $SUDO rm -f /etc/systemd/system/tpt-health-interop.service
    $SUDO systemctl daemon-reload
    ok "Removed systemd service."
  fi

  if [ "$OS_NAME" = "darwin" ]; then
    local plist="${HOME}/Library/LaunchAgents/nz.co.tpt.health-interop.plist"
    if [ -f "$plist" ]; then
      launchctl unload -w "$plist" || true
      rm -f "$plist"
      ok "Removed LaunchAgent."
    fi
  fi

  for dir in "/opt/tpt-healthcare" "${HOME}/.tpt-healthcare"; do
    if [ -d "$dir" ]; then
      $SUDO rm -rf "$dir"
      ok "Removed ${dir}"
    fi
  done

  for dir in "/etc/tpt-healthcare" "${HOME}/.config/tpt-healthcare"; do
    if [ -d "$dir" ]; then
      warn "Config directory ${dir} NOT removed (may contain secrets)."
      warn "Remove manually: rm -rf ${dir}"
    fi
  done

  ok "Uninstall complete."
  exit 0
}

# ---------------------------------------------------------------------------
# Success message
# ---------------------------------------------------------------------------
print_success() {
  cat <<EOF

${BOLD}${GREEN}TPT Healthcare NZ installed successfully!${RESET}

  Binary   : ${BIN_DIR}/${APP_NAME}
  Config   : ${CONFIG_FILE}
  Env file : ${ENV_FILE}
  Logs     : ${LOG_DIR}

${BOLD}Next steps:${RESET}

  1. Edit ${CONFIG_FILE} to set your PostgreSQL DSN.
  2. Edit ${ENV_FILE} to review your ENCRYPTION_KEY and any overrides.
  3. Open the first-run wizard: ${BOLD}http://localhost:8080/setup${RESET}

${BOLD}${YELLOW}Security reminder:${RESET}
  • Change the default database password immediately.
  • Store your ENCRYPTION_KEY in a secret manager; loss of this key means
    loss of all encrypted health data.
  • Ensure TLS is configured before accepting any patient data.
  • This system stores health information governed by the NZ Privacy Act 2020
    and the Health Information Privacy Code 2020.

EOF
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
  detect_platform
  detect_privileges
  set_directories

  if $OPT_UNINSTALL; then
    uninstall
  fi

  check_prerequisites
  create_directories
  install_binary
  write_config
  create_service_user
  run_migrations
  install_systemd_service
  install_launchagent
  print_success
}

main
