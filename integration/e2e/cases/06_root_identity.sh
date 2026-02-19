#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=../lib.sh
source "$SCRIPT_DIR/../lib.sh"

trap e2e_cleanup EXIT

e2e_require_linux
e2e_require_commands
e2e_setup_case "root-identity"

dir=$(e2e_new_case_dir "identity-root")
target="$dir/uid.txt"
script="$dir/root.sh"
write_script "$script" "id -u > '$target'"

e2e_expect_exit 0 run_atomic_root "$script"
[[ $(<"$target") == "0" ]] || e2e_fail "expected root uid 0, got $(<"$target")"

print_step "pass: root identity"
