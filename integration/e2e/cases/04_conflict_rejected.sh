#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=../lib.sh
source "$SCRIPT_DIR/../lib.sh"

trap e2e_cleanup EXIT

e2e_require_linux
e2e_require_commands
e2e_setup_case "conflict-rejected"

dir=$(e2e_new_case_dir "conflict")
target="$dir/race.txt"
script="$dir/conflict.sh"
write_script "$script" "echo atomic > '$target'; sleep 2"

run_atomic_user "$script" &
tx_pid=$!

sleep 0.6
echo "external" >"$target"
chmod 0666 "$target"

set +e
wait "$tx_pid"
rc=$?
set -e

[[ $rc -eq 21 ]] || e2e_fail "expected conflict exit 21, got $rc"
[[ $(<"$target") == "external" ]] || e2e_fail "conflict case overwrote external change"

print_step "pass: conflict rejected"
