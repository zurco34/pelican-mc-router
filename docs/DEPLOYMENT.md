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

# Required when Pelican allocations use 0.0.0.0 or ::.
PELICAN_MC_ROUTER_DISCOVERY_WILDCARD_BACKEND_HOST=192.168.1.10

MC_ROUTER_IMAGE=docker.io/itzg/mc-router:1.44.0
PELICAN_MC_ROUTER_IMAGE=ghcr.io/zurco34/pelican-mc-router:0.1.1
```

The `.env` file is ignored by Git and must not be committed.

Keep `PELICAN_MC_ROUTER_IMAGE` pinned to an exact release version in production.
This makes upgrades explicit and prevents an unrelated moving tag from changing
the deployed application unexpectedly.

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

Expected response:

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
- a Pelican application API key;
- the base Minecraft routing domain.

Read the API key without placing it directly in shell history:

```bash
read -rsp "Pelican API key: " PELICAN_API_KEY
printf '\n'
```

Submit the initial setup:

```bash
curl \
  --fail \
  --show-error \
  --silent \
  --request POST \
  --header 'Content-Type: application/json' \
  --data-binary @- \
  http://127.0.0.1:8080/api/v1/setup <<EOF
{
  "pelican_url": "https://panel.example.com",
  "pelican_api_key": "${PELICAN_API_KEY}",
  "router_domain": "mc.example.com"
}
EOF

unset PELICAN_API_KEY
```

A successful request returns HTTP `204 No Content`.

Confirm setup status:

```bash
curl \
  --fail \
  --silent \
  http://127.0.0.1:8080/api/v1/setup
```

Expected response:

```json
{"completed":true}
```

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
PELICAN_MC_ROUTER_IMAGE=ghcr.io/zurco34/pelican-mc-router:0.2.0
```

Replace `0.2.0` with the release being installed.

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

Verify health after the upgrade:

```bash
curl --fail http://127.0.0.1:8080/health
```
