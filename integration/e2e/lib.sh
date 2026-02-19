#!/usr/bin/env bash
set -euo pipefail

LIB_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd -- "$LIB_DIR/../.." && pwd)
BASE_DIR="${E2E_BASE_DIR:-/var/lib/atomic-e2e}"

TMP_ROOT=""
ATOMIC_BIN=""
ATOMICD_BIN=""
SOCKET_PATH=""
STATE_DIR=""
WORK_DIR=""
JOURNAL_DIR=""
CASES_DIR=""
DAEMON_LOG=""
DAEMON_PID_FILE=""
DAEMON_PID=""

print_step() {
  echo "[e2e] $*"
}

e2e_fail() {
  echo "[e2e][FAIL] $*" >&2
  if [[ -n "$DAEMON_LOG" && -f "$DAEMON_LOG" ]]; then
    echo "[e2e] daemon log:" >&2
    sed -n '1,200p' "$DAEMON_LOG" >&2 || true
  fi
  exit 1
}

e2e_cleanup() {
  if [[ -n "$DAEMON_PID" ]]; then
    if [[ $(id -u) -eq 0 ]]; then
      kill "$DAEMON_PID" >/dev/null 2>&1 || true
      wait "$DAEMON_PID" >/dev/null 2>&1 || true
    else
      sudo kill "$DAEMON_PID" >/dev/null 2>&1 || true
    fi
  fi

  if [[ -n "$TMP_ROOT" ]]; then
    if [[ $(id -u) -eq 0 ]]; then
      rm -rf "$TMP_ROOT"
    else
      sudo rm -rf "$TMP_ROOT"
    fi
  fi
}

e2e_require_linux() {
  [[ $(uname -s) == "Linux" ]] || e2e_fail "integration tests require Linux"
}

e2e_require_commands() {
  command -v go >/dev/null 2>&1 || e2e_fail "go is required"
  command -v unshare >/dev/null 2>&1 || e2e_fail "unshare is required"
  command -v chroot >/dev/null 2>&1 || e2e_fail "chroot is required"
  command -v setpriv >/dev/null 2>&1 || e2e_fail "setpriv is required"
}

setup_base_dir() {
  if [[ $(id -u) -eq 0 ]]; then
    mkdir -p "$BASE_DIR"
    chmod 0755 "$BASE_DIR"
  else
    sudo mkdir -p "$BASE_DIR"
    sudo chown "$(id -u):$(id -g)" "$BASE_DIR"
  fi
}

e2e_build_binaries() {
  print_step "building atomic"
  (cd "$REPO_ROOT" && go build -o "$ATOMIC_BIN" ./cmd/atomic)
  print_step "building atomicd"
  (cd "$REPO_ROOT" && go build -o "$ATOMICD_BIN" ./cmd/atomicd)
}

wait_for_socket() {
  local deadline=$((SECONDS + 20))
  while (( SECONDS < deadline )); do
    if [[ -S "$SOCKET_PATH" ]]; then
      return 0
    fi
    sleep 0.1
  done
  return 1
}

start_daemon() {
  mkdir -p "$STATE_DIR" "$WORK_DIR" "$JOURNAL_DIR" "$CASES_DIR"
  chmod 0777 "$CASES_DIR"
  print_step "starting daemon"

  if [[ $(id -u) -eq 0 ]]; then
    "$ATOMICD_BIN" \
      --socket "$SOCKET_PATH" \
      --state-dir "$STATE_DIR" \
      --work-dir "$WORK_DIR" \
      --journal-dir "$JOURNAL_DIR" \
      >"$DAEMON_LOG" 2>&1 &
    DAEMON_PID=$!

    wait_for_socket || e2e_fail "timed out waiting for daemon socket"
    chmod 0666 "$SOCKET_PATH"
    return
  fi

  sudo env \
    ATOMICD_BIN="$ATOMICD_BIN" \
    SOCKET_PATH="$SOCKET_PATH" \
    STATE_DIR="$STATE_DIR" \
    WORK_DIR="$WORK_DIR" \
    JOURNAL_DIR="$JOURNAL_DIR" \
    DAEMON_LOG="$DAEMON_LOG" \
    DAEMON_PID_FILE="$DAEMON_PID_FILE" \
    bash -lc '"$ATOMICD_BIN" --socket "$SOCKET_PATH" --state-dir "$STATE_DIR" --work-dir "$WORK_DIR" --journal-dir "$JOURNAL_DIR" >"$DAEMON_LOG" 2>&1 & echo $! >"$DAEMON_PID_FILE"'

  [[ -f "$DAEMON_PID_FILE" ]] || e2e_fail "daemon pid file missing"
  DAEMON_PID=$(<"$DAEMON_PID_FILE")
  [[ -n "$DAEMON_PID" ]] || e2e_fail "failed to read daemon pid"

  wait_for_socket || e2e_fail "timed out waiting for daemon socket"
  sudo chmod 0666 "$SOCKET_PATH"
}

e2e_setup_case() {
  local case_name=$1
  setup_base_dir
  TMP_ROOT=$(mktemp -d "$BASE_DIR/${case_name}.XXXXXX")
  chmod 0755 "$TMP_ROOT"

  ATOMIC_BIN="${E2E_ATOMIC_BIN:-$TMP_ROOT/atomic}"
  ATOMICD_BIN="${E2E_ATOMICD_BIN:-$TMP_ROOT/atomicd}"
  SOCKET_PATH="$TMP_ROOT/atomicd.sock"
  STATE_DIR="$TMP_ROOT/state"
  WORK_DIR="$TMP_ROOT/work"
  JOURNAL_DIR="$TMP_ROOT/journal"
  CASES_DIR="$TMP_ROOT/cases"
  DAEMON_LOG="$TMP_ROOT/atomicd.log"
  DAEMON_PID_FILE="$TMP_ROOT/atomicd.pid"

  if [[ ! -x "$ATOMIC_BIN" || ! -x "$ATOMICD_BIN" ]]; then
    e2e_build_binaries
  fi
  start_daemon
}

current_user_uid() {
  if [[ $(id -u) -eq 0 ]] && getent passwd nobody >/dev/null 2>&1; then
    id -u nobody
  else
    id -u
  fi
}

run_atomic_user() {
  local script_path=$1
  if [[ $(id -u) -eq 0 ]] && getent passwd nobody >/dev/null 2>&1; then
    su -s /bin/bash nobody -c "ATOMIC_SOCKET='$SOCKET_PATH' '$ATOMIC_BIN' '$script_path'"
  else
    ATOMIC_SOCKET="$SOCKET_PATH" "$ATOMIC_BIN" "$script_path"
  fi
}

run_atomic_root() {
  local script_path=$1
  if [[ $(id -u) -eq 0 ]]; then
    ATOMIC_SOCKET="$SOCKET_PATH" "$ATOMIC_BIN" "$script_path"
  else
    sudo ATOMIC_SOCKET="$SOCKET_PATH" "$ATOMIC_BIN" "$script_path"
  fi
}

e2e_expect_exit() {
  local expected=$1
  shift
  set +e
  "$@"
  local rc=$?
  set -e
  [[ $rc -eq $expected ]] || e2e_fail "expected exit $expected, got $rc for command: $*"
}

e2e_new_case_dir() {
  local name=$1
  local dir="$CASES_DIR/$name"
  mkdir -p "$dir"
  chmod 0777 "$dir"
  echo "$dir"
}

write_script() {
  local path=$1
  shift
  cat >"$path" <<EOF_SCRIPT
#!/usr/bin/env bash
set -euo pipefail
$*
EOF_SCRIPT
  chmod 0755 "$path"
}
