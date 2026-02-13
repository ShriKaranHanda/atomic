# Contributing

## Local Development
- Build: `make build`
- Unit tests: `make test-unit`
- Full tests (Linux + root needed for integration): `make test`

## Linux VM Flow (macOS host)
1. Create/start VM: `limactl start --name=atomic-ubuntu template:ubuntu`
2. Bootstrap dependencies: `make vm-bootstrap`
3. Run full suite in VM: `make test-vm`

## TDD Policy
All behavioral changes require tests first. PRs without tests for changed behavior are not accepted.

## Review Expectations
PR descriptions must include:
- behavior change summary,
- atomicity impact,
- rollback/recovery impact,
- exact test commands run.
