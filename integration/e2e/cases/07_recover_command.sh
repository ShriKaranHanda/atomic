#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=../lib.sh
source "$SCRIPT_DIR/../lib.sh"

trap e2e_cleanup EXIT

e2e_require_linux
e2e_require_commands
e2e_setup_case "recover-command"

e2e_expect_exit 0 env ATOMIC_SOCKET="$SOCKET_PATH" "$ATOMIC_BIN" recover

print_step "pass: recover command"
