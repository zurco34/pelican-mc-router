# Pelican MC Router

Automatic Minecraft hostname routing for servers managed by Pelican.

Pelican MC Router discovers Minecraft servers through the Pelican API,
generates hostname routes, and synchronizes them with a selected routing
backend.

## Features

- Automatic discovery of Pelican-managed Minecraft servers
- Hostname generation using a configurable base domain
- Backend-independent route discovery and synchronization
- mc-router integration through its HTTP API
- Optional Infrared configuration generation
- Deterministic and convergent route reconciliation
- Docker-first deployment model
- REST API and web dashboard under development

## Supported routing backends

### mc-router

`mc-router` is the default and recommended routing backend.

Pelican MC Router manages its routes through the private mc-router HTTP API.
The API should only be exposed on a trusted internal Docker network.

```yaml
router:
  backend: "mc-router"
  domain: "mc.example.com"

mcrouter:
  api_url: "http://mc-router:8080"
```

### Infrared

Infrared remains available as an optional backend. Pelican MC Router writes
managed proxy configuration files and updates a reload marker after changes.

```yaml
router:
  backend: "infrared"
  domain: "mc.example.com"

infrared:
  proxies_path: "/etc/infrared/proxies"
  reload_marker_path: "/etc/infrared/control/infrared.reload"
```

## Routing flow

```text
Minecraft client
        |
        v
Selected routing backend
  |-- mc-router
  `-- Infrared
        |
        v
Pelican-managed Minecraft server
```

For example, a Pelican server named `Techopolis` can be exposed as:

```text
techopolis.mc.example.com
```

## Wildcard Pelican allocations

Pelican nodes may publish allocations with `0.0.0.0` or `::`. These values are
listener bind addresses and cannot be used as remote routing destinations.

Configure a reachable fallback destination when a node uses wildcard
allocations:

```dotenv
PELICAN_MC_ROUTER_DISCOVERY_WILDCARD_BACKEND_HOST=192.168.1.10
```

The correct address depends on the deployment network and must be reachable
from the routing backend. Routable allocation IPs returned by Pelican continue
to be used unchanged.

## Architecture

```text
Pelican API
    |
    v
Route discovery
    |
    v
Generic route synchronizer
    |
    v
Selected backend adapter
  |-- mc-router HTTP API
  `-- Infrared configuration files
```

Only one routing backend is active for a deployment.

## Deployment

The production-oriented Docker Compose stack includes Pelican MC Router,
mc-router, private control-plane networking, persistent SQLite storage, and
least-privilege container settings.

Versioned multi-platform container images are published to:

```text
ghcr.io/zurco34/pelican-mc-router
```

For the `v0.1.1` release, create the environment file and start the pinned
production images:

```bash
cp .env.example .env

docker compose pull
docker compose up --detach --no-build
```

Podman Compose is also supported:

```bash
cp .env.example .env

podman-compose pull
podman-compose up --detach
```

Building from source remains available for development:

```bash
PELICAN_MC_ROUTER_IMAGE=pelican-mc-router:local \
  docker compose up --detach --build
```

See [Docker Compose deployment](docs/DEPLOYMENT.md) for initial setup,
networking, security, source builds, and upgrade instructions.

## Releases

See the [changelog](CHANGELOG.md) for release contents, known limitations, and
security-related deployment details.

## Project status

Under active development. Configuration formats and deployment behavior may
change before the first stable release.
