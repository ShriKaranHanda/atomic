# atomic

`atomic` runs bash scripts with all-or-nothing filesystem commits through a privileged daemon (`atomicd`).

## Usage

```bash
atomic ./script.sh
sudo atomic ./script.sh
```

- `atomic ./script.sh` runs the script as the calling user.
- `sudo atomic ./script.sh` runs the script as root.
- Filesystem changes are committed only if the script exits `0`.

## Commands
- `atomic <script_path> [script_args...]`
- `atomic recover`

## Exit Codes
- `0` success and committed
- `10` script failed, no commit
- `20` preflight/unsupported environment/daemon unavailable
- `21` conflict detected, commit aborted
- `30` recovery/commit failure

## Requirements
- Linux
- overlayfs kernel support
- `atomicd` running (recommended via socket activation)

## Daemon Model
- Client socket path default: `/run/atomicd.sock`
- Override socket path: `ATOMIC_SOCKET=/path/to.sock atomic ...`
- For packaged installs, use systemd socket activation with `packaging/systemd/atomicd.socket`.

## Development
- Build client: `make build`
- Build daemon: `make build-daemon`
- Unit tests: `make test-unit`
- Full tests: `make test`
- Create VM (macOS): `make vm-create`
- Bootstrap VM deps: `make vm-bootstrap`
- VM tests from macOS: `make test-vm`
- Delete VM: `make vm-delete`

See `/Users/karanhanda/atomic/AGENTS.md` for agent workflow and `/Users/karanhanda/atomic/docs/ARCHITECTURE.md` for internals.
