# Testing

## Unit Tests
Run:

```bash
make test-unit
```

Coverage focus:
- mountinfo parsing/filtering,
- overlay diff scanning (whiteout/opaque),
- operation ordering,
- journal persistence,
- conflict detection,
- commit/rollback behavior.

## Integration Tests (Linux root)
Run:

```bash
make test-integration
```

Scenarios:
- successful commit,
- failed script rollback,
- delete commit,
- conflict rejection.

## VM Tests (macOS host)
Create and bootstrap the Lima VM once:

```bash
make vm-create
make vm-bootstrap
```

Run tests inside the VM:

```bash
make test-vm
```

Delete the VM when you no longer need it:

```bash
make vm-delete
```

The VM is named `atomic-ubuntu`.

`make test-vm` executes `make test` inside that VM.
