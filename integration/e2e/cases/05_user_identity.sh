#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=../lib.sh
source "$SCRIPT_DIR/../lib.sh"

trap e2e_cleanup EXIT

e2e_require_linux
e2e_require_commands
e2e_setup_case "user-identity"

dir=$(e2e_new_case_dir "identity-user")
target="$dir/uid.txt"
script="$dir/user.sh"
want_uid=$(current_user_uid)
write_script "$script" "id -u > '$target'"

e2e_expect_exit 0 run_atomic_user "$script"
[[ $(<"$target") == "$want_uid" ]] || e2e_fail "expected user uid $want_uid, got $(<"$target")"

print_step "pass: user identity"
