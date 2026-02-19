#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=../lib.sh
source "$SCRIPT_DIR/../lib.sh"

trap e2e_cleanup EXIT

e2e_require_linux
e2e_require_commands
e2e_setup_case "failure-rollback"

dir=$(e2e_new_case_dir "failure")
target="$dir/target.txt"
script="$dir/fail.sh"
echo "original" >"$target"
chmod 0666 "$target"
write_script "$script" "echo transient > '$target'; exit 9"

e2e_expect_exit 10 run_atomic_user "$script"
[[ $(<"$target") == "original" ]] || e2e_fail "target changed despite script failure"

print_step "pass: failure rollback"
