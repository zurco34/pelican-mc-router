# Deployment support policy

## Supported topology

Run one active Pelican MC Router writer for each managed routing boundary. The
service is a control plane and does not proxy Minecraft traffic. Deploy it with
a private management bind or authenticated reverse proxy; direct public
management API and dashboard exposure are unsupported. Keep routing-backend
control APIs private.

Containers must retain non-root execution, read-only filesystems, dropped
capabilities, and `no-new-privileges`. Production examples use pinned release
images, never `latest`.

## Database guarantees

SQLite is a single-node local state store. Migrations are forward-only and
checksum-verified. Take an offline backup before an upgrade, restore only to a
new offline target, and use the release lifecycle matrix for verification.
Operational history and Prometheus process metrics are bounded/process-local;
they are not durable historical telemetry.

## Security and support boundary

Use mounted file-backed secrets and OIDC management authorization after
bootstrap. Do not place tokens, secret names, URLs, hostnames, server IDs, or
raw errors in automation output, metrics labels, fixtures, or alert labels.
Supported issue reports include the release tag, sanitized configuration shape,
and non-sensitive health/readiness/status outcomes.
