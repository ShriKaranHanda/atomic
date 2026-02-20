#!/usr/bin/env bash
set -euo pipefail

BIN_DIR="/usr/local/bin"
SYSTEMD_DIR="/etc/systemd/system"
PURGE_STATE="false"
REMOVE_GROUP="false"
SOCKET_GROUP="atomic"

usage() {
  cat <<USAGE
Usage: sudo ./scripts/uninstall.sh [options]

Options:
  --bin-dir <path>       Installed binary directory (default: /usr/local/bin)
  --systemd-dir <path>   Systemd unit directory (default: /etc/systemd/system)
  --purge-state          Remove /var/lib/atomic
  --remove-group         Remove socket group (default: atomic)
  --socket-group <name>  Group name used by socket (default: atomic)
  -h, --help             Show this help
USAGE
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
      --purge-state)
        PURGE_STATE="true"
        shift
        ;;
      --remove-group)
        REMOVE_GROUP="true"
        shift
        ;;
      --socket-group)
        SOCKET_GROUP="$2"
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

require_root() {
  if [[ $(id -u) -ne 0 ]]; then
    echo "error: uninstall must run as root (try sudo)" >&2
    exit 1
  fi
}

require_linux() {
  if [[ $(uname -s) != "Linux" ]]; then
    echo "error: uninstall is supported on Linux only" >&2
    exit 1
  fi
}

require_commands() {
  local cmds=(systemctl)
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

disable_units() {
  systemctl disable --now atomicd.socket >/dev/null 2>&1 || true
  systemctl disable --now atomicd.service >/dev/null 2>&1 || true
}

remove_units() {
  rm -f "$SYSTEMD_DIR/atomicd.socket"
  rm -f "$SYSTEMD_DIR/atomicd.service"
  systemctl daemon-reload
}

remove_binaries() {
  rm -f "$BIN_DIR/atomic"
  rm -f "$BIN_DIR/atomicd"
}

purge_state_if_requested() {
  if [[ "$PURGE_STATE" == "true" ]]; then
    rm -rf /var/lib/atomic
  fi
}

remove_group_if_requested() {
  if [[ "$REMOVE_GROUP" != "true" ]]; then
    return
  fi
  if command -v getent >/dev/null 2>&1 && getent group "$SOCKET_GROUP" >/dev/null 2>&1; then
    groupdel "$SOCKET_GROUP" || true
  fi
}

main() {
  parse_args "$@"
  require_root
  require_linux
  require_commands

  disable_units
  remove_units
  remove_binaries
  purge_state_if_requested
  remove_group_if_requested

  echo "atomic uninstalled"
}

main "$@"
