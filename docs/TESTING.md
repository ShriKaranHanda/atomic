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
Run:

```bash
make test-vm
```

This executes `make test` inside the `atomic-ubuntu` Lima VM.
