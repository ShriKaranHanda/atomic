#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=../lib.sh
source "$SCRIPT_DIR/../lib.sh"

trap e2e_cleanup EXIT

e2e_require_linux
e2e_require_commands
e2e_setup_case "success-commit"

dir=$(e2e_new_case_dir "success")
target="$dir/target.txt"
script="$dir/success.sh"
write_script "$script" "echo committed > '$target'"

e2e_expect_exit 0 run_atomic_user "$script"
[[ -f "$target" ]] || e2e_fail "target missing after success"
[[ $(<"$target") == "committed" ]] || e2e_fail "unexpected success content"

print_step "pass: success commit"
