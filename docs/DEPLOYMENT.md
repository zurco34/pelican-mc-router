# Docker Compose deployment

This guide deploys Pelican MC Router with the default and recommended
`mc-router` backend.

## Architecture

```text
Minecraft client
       |
       | TCP 25565
       v
   mc-router
       |
       | Pelican allocation IP and port
       v
Minecraft server managed by Pelican

Pelican API
       |
       v
Pelican MC Router
       |
       | Private HTTP API
       v
   mc-router
```

The Compose deployment:

- publishes the Minecraft listener on TCP port `25565`;
- exposes the Pelican MC Router HTTP API on `127.0.0.1:8080` by default;
- does not publish the mc-router management API;
- does not mount the Docker socket;
- runs both services as unprivileged users;
- uses read-only container root filesystems;
- stores application data in a named volume.

## Requirements

- Docker Engine with the Docker Compose plugin, or Podman with
  `podman-compose`
- A reachable Pelican panel
- A Pelican application API key
- TCP port `25565` forwarded to the deployment host
- DNS records pointing the Minecraft domain to the deployment host

A wildcard DNS record is recommended:

```text
*.mc.example.com -> deployment host
```

## Configure the deployment

Clone the repository and enter its directory:

```bash
git clone https://github.com/zurco34/pelican-mc-router.git
cd pelican-mc-router
```

Create the local environment file:

```bash
cp .env.example .env
```

Edit `.env` and review at least:

```dotenv
TZ=Europe/Oslo

MINECRAFT_BIND_ADDRESS=0.0.0.0
MINECRAFT_PORT=25565

PELICAN_MC_ROUTER_BIND_ADDRESS=127.0.0.1
PELICAN_MC_ROUTER_HTTP_PORT=8080

# Inbound HTTP API timeouts.
PELICAN_MC_ROUTER_SERVER_READ_HEADER_TIMEOUT=5s
PELICAN_MC_ROUTER_SERVER_READ_TIMEOUT=15s
PELICAN_MC_ROUTER_SERVER_WRITE_TIMEOUT=30s
PELICAN_MC_ROUTER_SERVER_IDLE_TIMEOUT=1m

# Bounded retries for transient Pelican and mc-router read failures.
PELICAN_MC_ROUTER_RETRY_ATTEMPTS=3
PELICAN_MC_ROUTER_RETRY_INITIAL_BACKOFF=200ms
PELICAN_MC_ROUTER_RETRY_MAX_BACKOFF=2s

# Directory mounted read-only at /run/secrets/pelican-mc-router.
# Keep files owner-readable only; do not store secret values in .env.
PELICAN_MC_ROUTER_SECRETS_HOST_DIR=./secrets
PELICAN_MC_ROUTER_SECRETS_BOOTSTRAP_TOKEN_NAME=bootstrap-token

# Required when Pelican allocations use 0.0.0.0 or ::.
PELICAN_MC_ROUTER_DISCOVERY_WILDCARD_BACKEND_HOST=192.168.1.10

MC_ROUTER_IMAGE=docker.io/itzg/mc-router:1.44.0
PELICAN_MC_ROUTER_IMAGE=ghcr.io/zurco34/pelican-mc-router:1.0.3
```

The `.env` file is ignored by Git and must not be committed.

## Mounted secret files

The Compose service mounts `PELICAN_MC_ROUTER_SECRETS_HOST_DIR` read-only at
`/run/secrets/pelican-mc-router`. Create the host directory with restrictive
permissions before starting the stack, and ensure the container's unprivileged
UID (`10001`) can read its files without making them group- or world-readable.
The application accepts only regular, owner-readable files with bounded names
from this directory; it refuses paths, symlinks, group-readable files, and
world-readable files.

This mount provides both the one-time bootstrap token and file-backed Pelican
credential references. For rootful Docker, use directory mode `0700`, file
mode `0600`, and ownership `10001:10001` so the non-root container can traverse
the directory and read owner-only files. For rootless Podman, set equivalent
mapped ownership with `podman unshare chown 10001:10001 ./secrets/*`; apply an
SELinux relabel option only when the host requires it. Do not put a token or
API key in `.env`, logs, or shell history.

For an uninitialized database, create the configured bootstrap-token file in
this directory before starting the service. Its contents are a random bearer
token generated and retained by the operator; use it only in the HTTP
`Authorization` header for `GET` or `POST /api/v1/setup`. Avoid clients, shell
history, or diagnostics that record request headers. After successful setup,
these routes return `404` and the bootstrap token cannot be replayed.

Keep `PELICAN_MC_ROUTER_IMAGE` pinned to an exact release version in production.
This makes upgrades explicit and prevents an unrelated moving tag from changing
the deployed application unexpectedly.

## HTTP API timeouts

The HTTP API uses bounded timeouts to protect the control plane from slow or
stalled clients. The defaults are 5 seconds for request headers, 15 seconds
for the complete request, 30 seconds for the response, and 1 minute for idle
keep-alive connections.

All timeout values must be positive Go durations. Increase them only when a
known API client needs longer requests or responses; excessively large values
can allow slow clients to hold connections and reduce control-plane capacity.

## Control-plane read retries

Pelican MC Router retries only idempotent control-plane reads after transport
errors and HTTP `429`, `502`, `503`, or `504` responses. The defaults permit
three total attempts with jittered exponential backoff from 200 milliseconds
to 2 seconds. Route create and delete operations are never retried, so the
next scheduled reconciliation remains the safe recovery path for mutations.

Keep retry values bounded: higher values increase the time before a failed
reconciliation is reported and can delay shutdown until request contexts are
cancelled.

## Start the stack

The recommended production deployment uses the versioned images configured in
`.env`.

Using Docker Compose:

```bash
docker compose pull
docker compose up --detach --no-build
```

Using Podman Compose:

```bash
podman-compose pull
podman-compose up --detach
```

Check the containers with Docker:

```bash
docker compose ps
```

Or with Podman:

```bash
podman-compose ps
```

Check application health:

```bash
curl --fail http://127.0.0.1:8080/health
```

`/health` verifies process liveness. Before initial setup, `/ready` correctly
returns HTTP 503 with reason `setup_incomplete`.

Expected `/health` response:

```text
OK
```

## Build from source

Building Pelican MC Router locally remains available for development and for
testing unreleased changes.

Using Docker Compose:

```bash
PELICAN_MC_ROUTER_IMAGE=pelican-mc-router:local \
  docker compose up --detach --build
```

Using Podman Compose:

```bash
PELICAN_MC_ROUTER_IMAGE=pelican-mc-router:local \
  podman-compose up --detach --build
```

The versioned GHCR image remains the recommended production deployment method.

## Initial application setup

Pelican MC Router stores its runtime settings in its SQLite database.

Initial setup requires:

- the Pelican panel URL;
- a mounted Pelican credential secret file and its bounded name;
- the base Minecraft routing domain.

Create a Pelican **Application API key** in the Pelican panel. Do not generate
this value locally. Write the issued key through stdin or a local editor so it
is not placed in a command argument or shell history; generate only the
operator-owned bootstrap token locally:

```bash
install -d -m 700 ./secrets
umask 077
cat > ./secrets/pelican-api-key
# paste the panel-issued Application API key, then press Ctrl-D
openssl rand -base64 48 > ./secrets/bootstrap-token
chown 10001:10001 ./secrets/pelican-api-key ./secrets/bootstrap-token
chmod 600 ./secrets/pelican-api-key ./secrets/bootstrap-token
```

Submit the initial setup:

```bash
curl \
  --fail \
  --show-error \
  --silent \
  --request POST \
  --header 'Content-Type: application/json' \
  --header "Authorization: Bearer $(cat ./secrets/bootstrap-token)" \
  --data-binary @- \
  http://127.0.0.1:8080/api/v1/setup <<EOF
{
  "pelican_url": "https://panel.example.com",
  "pelican_secret_name": "pelican-api-key",
  "router_domain": "mc.example.com"
}
EOF

```

A successful request returns HTTP `204 No Content`.

After successful setup, `/api/v1/setup` returns `404` permanently. Use an OIDC
viewer bearer token to query authenticated status instead.

After setup completes, verify observability endpoints:

```bash
curl --fail http://127.0.0.1:8080/ready
curl --fail --header "Authorization: Bearer $OIDC_TOKEN" http://127.0.0.1:8080/api/v1/status
curl --fail http://127.0.0.1:8080/metrics
```

`/ready` should return HTTP 200 after setup and successful reconciliation.
`/api/v1/status` shows cached reconciliation state and the running build
version/revision, and `/metrics` returns Prometheus text exposition.

The status response includes `reconciliation.route_changes` with bounded route
counts and a `changed` flag from the latest attempt. These diagnostics never
contain hostnames, server identifiers, URLs, or backend error text.

Inspect discovered servers:

```bash
curl \
  --fail \
  --silent \
  http://127.0.0.1:8080/api/v1/servers
```

Inspect generated routes:

```bash
curl \
  --fail \
  --silent \
  http://127.0.0.1:8080/api/v1/routes
```

## Backend reachability

Pelican MC Router uses each server's primary Pelican allocation as the
mc-router backend address.

The allocation IP and port must therefore be reachable from inside the
mc-router container.

Pelican may report `0.0.0.0` or `::` for an allocation. These are wildcard
listener bind addresses, not valid remote destinations. Configure a reachable
fallback host in `.env` when such allocations are present:

```dotenv
PELICAN_MC_ROUTER_DISCOVERY_WILDCARD_BACKEND_HOST=192.168.1.10
```

The correct value depends on the network layout. It may be the Pelican node's
LAN address or a Docker bridge gateway shared with mc-router. The address must
be reachable from inside the mc-router container.

Pelican MC Router only applies this fallback to `0.0.0.0` and `::`. Any
routable allocation IP returned by Pelican is preserved.

Do not use `127.0.0.1` for a server running outside the mc-router container.
Inside a container, `127.0.0.1` refers to that container itself.

## Network exposure

By default:

| Service | Host binding | Purpose |
| --- | --- | --- |
| mc-router | `0.0.0.0:25565` | Public Minecraft traffic |
| Pelican MC Router | `127.0.0.1:8080` | Local management API |
| mc-router API | Not published | Private route synchronization |

Keep the mc-router API private. Pelican MC Router needs access to it, but
external clients do not.

Change `PELICAN_MC_ROUTER_BIND_ADDRESS` only when remote API access is
required and protected by an authenticated reverse proxy or another trusted
access-control layer.

The read-only `/dashboard` follows the same boundary and must remain private or
proxy-authenticated. It displays cached status only and does not trigger
reconciliation. Direct public dashboard exposure is unsupported. See
[ADR-0005](adr/ADR-0005-dashboard-security-model.md) for the security model and
the prerequisites for future write actions.

### Offline SQLite recovery

Stop the service before using the offline recovery utility; it must never copy,
restore, or compact a live database. The command does not print database rows
or credentials. Create backups on storage protected by the operator and verify
them before a restore:

```bash
container_id="$(docker compose ps -q pelican-mc-router)"
volume_name="$(docker inspect "$container_id" --format '{{range .Mounts}}{{if eq .Destination "/app/data"}}{{.Name}}{{end}}{{end}}')"
test -n "$volume_name"
docker compose stop pelican-mc-router
docker run --rm --entrypoint sqlite-recovery \
  -v "$volume_name:/data:ro" \
  ghcr.io/zurco34/pelican-mc-router:1.0.3 \
  -operation integrity -source /data/pelican-mc-router.db

# Mount a separate writable, operator-protected backup directory for backup or
# restore. Restore only into a new non-existent target, then verify integrity.
# Resolve Podman Compose container mounts equivalently with `podman inspect`.
```

Restore targets must be distinct and must not already exist. Replace the
service database only while the service is stopped, retain the original until
startup and integrity verification succeed, and roll back by restoring that
original backup. `compact` runs SQLite `VACUUM` and likewise requires downtime.

### Required management OIDC authorization

Set `PELICAN_MC_ROUTER_DASHBOARD_AUTH_ENABLED=true` when an authenticated
reverse proxy or SSO forwards a verified OpenID Connect ID token as an
`Authorization: Bearer` header. Configure its HTTPS issuer, the expected
audience, the array-valued role claim, and the required role (normally
`viewer`). Configure a separate `operator` role for the manual reconciliation
control.
The service validates provider metadata during startup and fails
closed if that check cannot complete.

The application returns generic HTTP `401` for absent or invalid tokens and
HTTP `403` for authenticated identities without the role. It does not log,
return, or expose token contents, subjects, or roles in metrics. Keep the
existing private binding or proxy authentication boundary in place; OIDC does
not make direct public API exposure supported.

The manual reconciliation control requires the `operator` role and a
same-origin custom request header; it waits for the existing serialized refresh
and reports only safe cached reconciliation status. An interrupted request
cancels its refresh without changing the last-known-good runtime state. See
[ADR-0006](adr/ADR-0006-dashboard-oidc-authorization.md).

## Temporary mc-router route file

The deployment configures:

```text
ROUTES_CONFIG=/tmp/routes.json
```

mc-router `1.44.0` requires a configured route file when its dynamic route API
writes routes. Without it, `POST /routes` can trigger a nil-pointer panic.

The route file is stored on a small tmpfs because Pelican MC Router remains
the authoritative source of route state and reconciles the desired routes
through the private API.

This file is intentionally not persisted across mc-router container
recreation.

## View logs

Using Docker Compose:

```bash
docker compose logs --follow
```

Using Podman Compose:

```bash
podman-compose logs --follow
```

View only Pelican MC Router logs:

```bash
docker compose logs --follow pelican-mc-router
```

View only mc-router logs:

```bash
docker compose logs --follow mc-router
```

Podman equivalents:

```bash
podman-compose logs --follow pelican-mc-router
podman-compose logs --follow mc-router
```

## Stop the deployment

Stop and remove the containers while retaining application data:

```bash
docker compose down
```

The Podman equivalent is:

```bash
podman-compose down
```

## Remove application data

Removing the named volume permanently deletes the SQLite database and stored
application settings:

```bash
docker compose down --volumes
```

The Podman equivalent is:

```bash
podman-compose down --volumes
```

Do not use `--volumes` during routine restarts or upgrades.

## Upgrade

Pull the latest deployment files:

```bash
git pull --ff-only
```

Update the pinned Pelican MC Router image in `.env` to the desired release. For
example:

```dotenv
PELICAN_MC_ROUTER_IMAGE=ghcr.io/zurco34/pelican-mc-router:1.0.3
```

Change the tag to the exact release being installed.

Pull the configured images and recreate the containers:

```bash
docker compose pull
docker compose up --detach --no-build
```

For Podman:

```bash
podman-compose pull
podman-compose up --detach
```

Do not use `--volumes` during an upgrade. The named volume contains the SQLite
database and application configuration.

Take and verify an offline backup before upgrade. Migrations are forward-only;
retain the existing named volume and do not use `docker compose down --volumes`.

Verify the endpoints after the upgrade:

```bash
curl --fail http://127.0.0.1:8080/health
curl --fail http://127.0.0.1:8080/ready
curl --fail --header "Authorization: Bearer $OIDC_TOKEN" http://127.0.0.1:8080/api/v1/status
curl --fail http://127.0.0.1:8080/metrics
```

`/ready` may temporarily return HTTP 503 while startup reconciliation runs.
Retry after a short wait; if it remains unavailable, use its JSON reason and
`/api/v1/status` to diagnose the condition.

## Rollback

To roll back, set `PELICAN_MC_ROUTER_IMAGE` to the previously known-good
release tag only when that release supports the current schema. Restore the
verified offline backup when a schema downgrade is unsupported:

```dotenv
PELICAN_MC_ROUTER_IMAGE=ghcr.io/zurco34/pelican-mc-router:1.0.0
```

```bash
docker compose pull
docker compose up --detach --no-build
```

Retain the named volume and do not use `--volumes`. Do not assume an older image
can open a newer schema; restore an offline backup instead when required.
