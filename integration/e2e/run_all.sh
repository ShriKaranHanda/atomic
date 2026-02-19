#!/usr/bin/env bash
set -uo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd -- "$SCRIPT_DIR/../.." && pwd)

CASES=(
  "$SCRIPT_DIR/cases/01_success_commit.sh"
  "$SCRIPT_DIR/cases/02_failure_rollback.sh"
  "$SCRIPT_DIR/cases/03_delete_commit.sh"
  "$SCRIPT_DIR/cases/04_conflict_rejected.sh"
  "$SCRIPT_DIR/cases/05_user_identity.sh"
  "$SCRIPT_DIR/cases/06_root_identity.sh"
  "$SCRIPT_DIR/cases/07_recover_command.sh"
)

BUILD_ROOT=$(mktemp -d /tmp/atomic-e2e-build.XXXXXX)
ATOMIC_BIN="$BUILD_ROOT/atomic"
ATOMICD_BIN="$BUILD_ROOT/atomicd"

cleanup() {
  rm -rf "$BUILD_ROOT"
}
trap cleanup EXIT

echo "[e2e] building shared binaries"
(cd "$REPO_ROOT" && go build -o "$ATOMIC_BIN" ./cmd/atomic)
(cd "$REPO_ROOT" && go build -o "$ATOMICD_BIN" ./cmd/atomicd)

export E2E_ATOMIC_BIN="$ATOMIC_BIN"
export E2E_ATOMICD_BIN="$ATOMICD_BIN"

total=0
failed=0

for case_script in "${CASES[@]}"; do
  total=$((total + 1))
  echo "[e2e] running $(basename "$case_script")"
  if bash "$case_script"; then
    echo "[e2e] PASS $(basename "$case_script")"
  else
    rc=$?
    failed=$((failed + 1))
    echo "[e2e] FAIL $(basename "$case_script") (exit $rc)"
  fi
  echo
done

passed=$((total - failed))
echo "[e2e] summary: passed=$passed failed=$failed total=$total"

if [[ $failed -ne 0 ]]; then
  exit 1
fi
