#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=../lib.sh
source "$SCRIPT_DIR/../lib.sh"

trap e2e_cleanup EXIT

e2e_require_linux
e2e_require_commands
e2e_setup_case "delete-commit"

dir=$(e2e_new_case_dir "delete")
target="$dir/delete-me.txt"
script="$dir/delete.sh"
echo "bye" >"$target"
chmod 0666 "$target"
write_script "$script" "rm -f '$target'"

e2e_expect_exit 0 run_atomic_user "$script"
[[ ! -e "$target" ]] || e2e_fail "delete did not commit"

print_step "pass: delete commit"
