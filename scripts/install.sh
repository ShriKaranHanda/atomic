#!/usr/bin/env bash
set -euo pipefail

BIN_DIR="/usr/local/bin"
SYSTEMD_DIR="/etc/systemd/system"
SOCKET_USER=""
SOCKET_GROUP=""
SOCKET_MODE=""
BUILD_DIR=""

cleanup() {
  if [[ -n "$BUILD_DIR" && -d "$BUILD_DIR" ]]; then
    rm -rf "$BUILD_DIR"
  fi
}

usage() {
  cat <<USAGE
Usage: sudo ./scripts/install.sh [options]

Options:
  --bin-dir <path>       Install binary directory (default: /usr/local/bin)
  --systemd-dir <path>   Systemd unit directory (default: /etc/systemd/system)
  --socket-user <user>   Socket owner user override
  --socket-group <group> Socket owner group override
  --socket-mode <mode>   Socket mode override (default single-user: 0600)
  -h, --help             Show this help
USAGE
}

require_root() {
  if [[ $(id -u) -ne 0 ]]; then
    echo "error: install must run as root (try sudo)" >&2
    exit 1
  fi
}

require_linux() {
  if [[ $(uname -s) != "Linux" ]]; then
    echo "error: install is supported on Linux only" >&2
    exit 1
  fi
}

require_commands() {
  local cmds=(go systemctl install id)
  local missing=()
  for cmd in "${cmds[@]}"; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
      missing+=("$cmd")
    fi
  done
  if [[ ${#missing[@]} -ne 0 ]]; then
    echo "error: missing required commands: ${missing[*]}" >&2
    exit 1
  fi
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --bin-dir)
        BIN_DIR="$2"
        shift 2
        ;;
      --systemd-dir)
        SYSTEMD_DIR="$2"
        shift 2
        ;;
      --socket-group)
        SOCKET_GROUP="$2"
        shift 2
        ;;
      --socket-user)
        SOCKET_USER="$2"
        shift 2
        ;;
      --socket-mode)
        SOCKET_MODE="$2"
        shift 2
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        echo "error: unknown argument: $1" >&2
        usage
        exit 1
        ;;
    esac
  done
}

resolve_socket_access() {
  if [[ -n "$SOCKET_USER" && -n "$SOCKET_GROUP" && -n "$SOCKET_MODE" ]]; then
    return
  fi

  if [[ -n "${SUDO_USER:-}" && "${SUDO_USER}" != "root" ]]; then
    SOCKET_USER="${SOCKET_USER:-$SUDO_USER}"
    SOCKET_GROUP="${SOCKET_GROUP:-$(id -gn "$SUDO_USER")}"
    SOCKET_MODE="${SOCKET_MODE:-0600}"
    return
  fi

  SOCKET_USER="${SOCKET_USER:-root}"
  SOCKET_GROUP="${SOCKET_GROUP:-root}"
  SOCKET_MODE="${SOCKET_MODE:-0600}"
}

build_binaries() {
  local repo_root="$1"
  local build_dir="$2"

  echo "building atomic"
  (cd "$repo_root" && go build -o "$build_dir/atomic" ./cmd/atomic)
  echo "building atomicd"
  (cd "$repo_root" && go build -o "$build_dir/atomicd" ./cmd/atomicd)
}

install_binaries() {
  local build_dir="$1"
  mkdir -p "$BIN_DIR"
  install -m 0755 "$build_dir/atomic" "$BIN_DIR/atomic"
  install -m 0755 "$build_dir/atomicd" "$BIN_DIR/atomicd"
}

install_units() {
  local repo_root="$1"
  local service_src="$repo_root/packaging/systemd/atomicd.service"
  local socket_src="$repo_root/packaging/systemd/atomicd.socket"
  local service_dst="$SYSTEMD_DIR/atomicd.service"
  local socket_dst="$SYSTEMD_DIR/atomicd.socket"
  local tmp_service
  local tmp_socket

  if [[ ! -f "$service_src" || ! -f "$socket_src" ]]; then
    echo "error: systemd unit templates not found under packaging/systemd" >&2
    exit 1
  fi

  mkdir -p "$SYSTEMD_DIR"

  tmp_service=$(mktemp)
  tmp_socket=$(mktemp)

  sed "s|^ExecStart=.*|ExecStart=$BIN_DIR/atomicd|" "$service_src" > "$tmp_service"
  sed \
    -e "s|^SocketUser=.*|SocketUser=$SOCKET_USER|" \
    -e "s|^SocketGroup=.*|SocketGroup=$SOCKET_GROUP|" \
    -e "s|^SocketMode=.*|SocketMode=$SOCKET_MODE|" \
    "$socket_src" > "$tmp_socket"

  install -m 0644 "$tmp_service" "$service_dst"
  install -m 0644 "$tmp_socket" "$socket_dst"

  rm -f "$tmp_service" "$tmp_socket"
}

enable_socket() {
  systemctl daemon-reload
  systemctl enable --now atomicd.socket
}

main() {
  parse_args "$@"
  require_root
  require_linux
  require_commands
  resolve_socket_access

  local script_dir
  script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
  local repo_root
  repo_root=$(cd -- "$script_dir/.." && pwd)
  BUILD_DIR=$(mktemp -d)
  trap cleanup EXIT

  build_binaries "$repo_root" "$BUILD_DIR"
  install_binaries "$BUILD_DIR"
  install_units "$repo_root"
  enable_socket

  echo
  echo "atomic installed successfully"
  echo "- binary: $BIN_DIR/atomic"
  echo "- daemon: $BIN_DIR/atomicd"
  echo "- socket: /run/atomicd.sock"
  echo "- socket access: ${SOCKET_USER}:${SOCKET_GROUP} (${SOCKET_MODE})"
  echo
  echo "sanity check:"
  echo "  systemctl status atomicd.socket"
  echo "  atomic recover"
}

main "$@"
