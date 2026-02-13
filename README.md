# atomic

`atomic` runs bash scripts with all-or-nothing filesystem commits.

## Usage

<!-- TODO: Make sure it doesn't run the script as root -->
```bash
sudo atomic ./script.sh
```

Host filesystem changes are committed only if the script exits `0`.

## Commands
- `atomic <script_path> [script_args...]`
- `atomic recover`

## Exit Codes
- `0` success and committed
- `10` script failed, no commit
- `20` preflight/unsupported environment
- `21` conflict detected, commit aborted
- `30` recovery/commit failure

## Requirements
- Linux
- root privileges
- overlayfs kernel support

## Development
- Build: `make build`
- Unit tests: `make test-unit`
- Full tests: `make test`
- VM tests from macOS: `make test-vm`

See `/Users/karanhanda/atomic/AGENTS.md` for agent workflow and `/Users/karanhanda/atomic/docs/ARCHITECTURE.md` for internals.
