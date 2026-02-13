# Architecture

## Overview
`atomic` runs a script inside isolated overlayfs mount namespaces, captures filesystem changes in overlay upperdirs, checks for host-side conflicts, and commits changes to the host filesystem only on successful script exit.

## Runtime Pipeline
1. Preflight
- Linux + root required.
- overlayfs support required.
2. Recovery
- Scan journal directory and finish/rollback interrupted commits.
3. Isolated execution
- Create run workspace.
- Use `unshare --mount` to isolate mount namespace.
- Mount root overlay and per-mount overlays for writable real mounts.
- `chroot` into merged root and execute script.
4. Diff capture
- Parse overlay upperdirs into operation list.
- Support upsert + delete operations, including whiteouts and opaque directory markers.
5. Conflict checks
- Fail commit if touched paths/parents changed after transaction start.
6. Commit with journal
- Persist journal before and during apply.
- Backup each target path before mutation.
- Apply operations deterministically.
- Roll back from backups on failure.

## Scope Guarantees
- Filesystem changes are transactional.
- Non-filesystem side effects (network/services/db) are out of scope.

## Portability
- Supported target: modern Linux systems with overlayfs.
