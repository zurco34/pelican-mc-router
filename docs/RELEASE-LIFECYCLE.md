# Release lifecycle verification

Run this matrix against disposable local state only. Never use production
credentials, databases, or routing backends for release verification.

1. Start a clean Compose stack and confirm `/health` is live while `/ready`
   reports setup incomplete.
2. Complete setup with a mounted test secret, then confirm `/ready`, status,
   metrics, and one successful reconciliation.
3. Stop the stack gracefully, restart it, and confirm the current state is
   repopulated by startup reconciliation. Metrics and in-memory history are
   process-local and start again from zero.
4. Stop the service and run `sqlite-recovery -operation integrity` followed by
   a backup. Restore only to a new path and verify that restored copy before
   replacing an offline database.
5. Upgrade a disposable v0.5 database to the candidate. Confirm migration
   checksums are recorded and startup succeeds. Roll back only after retaining
   the original backup; older images cannot consume future schema semantics.

Use Podman Compose in this environment, or Docker Compose where supported.
