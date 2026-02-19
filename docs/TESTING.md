# Testing

## Unit Tests
Run:

```bash
make test-unit
```

Coverage focus:
- mountinfo parsing/filtering,
- overlay diff scanning,
- operation ordering,
- journal persistence,
- conflict detection,
- commit/rollback behavior,
- daemon IPC framing.

## Integration Tests (Linux)
Run:

```bash
make test-integration
```

`make test-integration` runs the bash end-to-end suite at `integration/e2e/run_all.sh`.
The runner executes `integration/e2e/cases/*.sh` independently, so one failing case does not block the others.
Each case starts `atomicd` the same way users do (root daemon + unix socket) and then drives `atomic` commands directly.

Coverage:
- successful commit,
- failed script no commit,
- delete commit,
- conflict rejection,
- caller identity execution (`atomic` as user, `sudo atomic` as root),
- explicit `atomic recover`.

## VM Tests (macOS host)
Initial setup:

```bash
make vm-create
make vm-bootstrap
```

Then run:

```bash
make test-vm
```

Delete the VM when you no longer need it:

```bash
make vm-delete
```

The VM is named `atomic-ubuntu`.

`make test-vm` executes `make test` inside that VM.
