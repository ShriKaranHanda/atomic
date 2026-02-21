# atomic

`atomic` runs bash scripts transactionally.

Note: requires Linux kernel `5.4+`.
Warning: currently tested only on Ubuntu `25.10`.

## Quick Start
Clone and install:

```bash
git clone https://github.com/ShriKaranHanda/atomic.git
cd atomic
sudo ./scripts/install.sh
```

Try it out:

```bash
mkdir -p ~/atomic-demo
cd ~/atomic-demo

echo "keep-original" > keep.txt
echo "delete-me" > remove.txt
```

Create a script that fails after changing files:

```bash
cat > fail.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
echo "changed" > keep.txt
rm -f remove.txt
echo "new-file" > create.txt
exit 1
EOF
chmod +x fail.sh
```

Run with `atomic`:

```bash
atomic ./fail.sh; echo "exit=$?"
```

Expected after failure:
- `exit=10`
- `keep.txt` is still `keep-original`
- `remove.txt` still exists
- `create.txt` does not exist

Now run a success script:

```bash
cat > ok.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
echo "changed" > keep.txt
rm -f remove.txt
echo "new-file" > create.txt
EOF
chmod +x ok.sh

atomic ./ok.sh; echo "exit=$?"
```

Expected:
- first run exits `10` and leaves files unchanged
- second run exits `0` and commits changes

`install.sh` builds and installs `atomic` + `atomicd`, installs systemd units, and enables socket activation automatically.
By default, `install.sh` grants socket access to the invoking sudo user (single-user v1 flow), so plain `atomic ...` works without extra setup.

## Usage

```bash
atomic ./script.sh
sudo atomic ./script.sh
```

- `atomic ./script.sh` runs the script as the calling user.
- `sudo atomic ./script.sh` runs the script as root.
- Filesystem changes are committed only if the script exits `0`.

### Commands
- `atomic <script_path> [script_args...]`
- `atomic recover`

### Exit Codes
- `0` success and committed
- `10` script failed, no commit
- `20` preflight/unsupported environment/daemon unavailable
- `21` conflict detected, commit aborted
- `30` recovery/commit failure

### Expected After Success Run
- `exit=0`
- `keep.txt` is `changed`
- `remove.txt` is gone
- `create.txt` exists

### Socket Activation (If You Did Not Use `install.sh`)
You do not need this when using `scripts/install.sh`.

`atomic` defaults to `/run/atomicd.sock`. Only for manual/non-script installs:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now atomicd.socket
```

If `atomicd.service` does not point to your installed daemon path, update `ExecStart` first.

## Limitations (v0.1.0)
- Linux only (`kernel 5.4+`).
- Requires overlayfs support enabled in the running kernel.
- Transactional guarantees apply to filesystem changes only.
- Non-filesystem side effects (network calls, service mutations, database writes) are not rolled back.
- One active transaction at a time (`atomicd` is single-runner in v1).
- Focused on regular files/directories/symlinks; unsupported special node types fail the transaction.

## Uninstall
From repo root:

```bash
sudo ./scripts/uninstall.sh
```

Optional destructive cleanup:
- remove `/var/lib/atomic`: `--purge-state`
- remove socket group: `--remove-group`

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
