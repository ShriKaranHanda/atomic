# Architecture

## Overview
`atomic` is an unprivileged client that sends run requests to `atomicd` over a Unix socket.
`atomicd` performs mount/overlay execution, conflict checks, and commit/rollback.

## Identity Model
- Script identity comes from Unix peer credentials on the socket (`SO_PEERCRED`).
- `atomic` (non-root caller) runs scripts as caller UID/GID.
- `sudo atomic` runs scripts as root UID/GID.
- Client input never overrides run identity in v1.

## Runtime Pipeline
1. Client request
- `atomic` sends a JSON request to `atomicd`.
2. Daemon auth + scheduling
- Daemon reads peer credentials.
- Daemon enforces single active transaction in v1.
3. Recovery
- Daemon runs journal recovery before processing requests.
4. Isolated execution
- Daemon creates run workspace.
- Daemon launches runner in an isolated mount namespace (`unshare --mount`).
- Runner mounts root + writable mount overlays.
- Runner executes script in chroot as caller UID/GID.
5. Diff capture
- Parse upperdirs into upsert/delete operations (whiteouts + opaque dirs handled).
6. Conflict checks
- Reject commit if touched paths/parents changed after txn start.
7. Commit with journal
- Persist journal before and during apply.
- Backup each target path before mutation.
- Roll back from backups on failure.

## Scope Guarantees
- Filesystem changes are transactional.
- Non-filesystem side effects (network/services/db) are out of scope.

## Portability
- Supported target: modern Linux systems with overlayfs.
