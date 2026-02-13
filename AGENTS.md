# AGENTS

## Mission
Maintain `atomic` as a low-overhead, all-or-nothing Linux script runner with strict TDD discipline.

## Required Workflow (Non-Optional)
1. Write or update tests first (red).
2. Implement the minimum change to pass (green).
3. Refactor while keeping tests green.
4. Run `make test-unit` after each package-level change.
5. Run `make test` before opening/updating a PR.
6. For Linux behavior changes, also run `make test-vm`.

## Safety Rules
- Do not bypass conflict detection for touched paths.
- Do not remove recovery-journal writes before mutating host paths.
- Do not introduce non-filesystem rollback guarantees unless explicitly implemented.
- Keep `atomic` Linux-only unless product requirements change.

## Mistake Log
### 2026-02-13
- `limactl list` can appear broken under sandboxed status checks. Verify with an unsandboxed check before assuming VM corruption.
- Compound Lima shell commands should be wrapped with `sh -lc` to avoid argument parsing mistakes.
- Keep Linux-only syscalls behind build tags (`//go:build linux`) to prevent macOS compile failures.

## PR Checklist
- [ ] Tests added/updated first.
- [ ] `make test-unit` passed.
- [ ] `make test` passed.
- [ ] If filesystem transaction logic changed: `make test-vm` passed.
- [ ] Documentation updated (`docs/ARCHITECTURE.md`, `docs/TESTING.md`) if behavior changed.
