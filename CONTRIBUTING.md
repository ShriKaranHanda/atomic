# Contributing

## Local Development
- Build client: `make build`
- Build daemon: `make build-daemon`
- Unit tests: `make test-unit`
- Integration tests (bash e2e): `make test-integration`
- Full tests: `make test`

## Linux VM Flow (macOS host)
1. Create/start VM: `limactl start --name=atomic-ubuntu --mount-writable template:ubuntu`
2. Bootstrap dependencies: `make vm-bootstrap`
3. Run full suite in VM: `make test-vm`

## TDD Policy
All behavioral changes require tests first. PRs without tests for changed behavior are not accepted.

## Daemon Notes
- Client talks to daemon socket (`/run/atomicd.sock` by default).
- For local testing use `ATOMIC_SOCKET=/tmp/atomicd.sock`.
- Packaged deployments should use systemd socket activation.

## Review Expectations
PR descriptions must include:
- behavior change summary,
- atomicity impact,
- rollback/recovery impact,
- daemon/protocol impact (if any),
- exact test commands run.
