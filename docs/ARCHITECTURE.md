# Architecture

Pelican MC Router is a control-plane service for automatically maintaining
Minecraft hostname routes for servers managed by Pelican.

It does not proxy Minecraft traffic itself. Instead, it discovers servers,
generates the desired route state, and synchronizes that state with one
selected routing backend.

## System context

```text
Minecraft client
       |
       | Minecraft hostname
       v
Selected routing backend
  |-- mc-router
  `-- Infrared
       |
       | Pelican allocation IP and port
       v
Minecraft server managed by Pelican
```

The routing control plane is separate from Minecraft traffic:

```text
Pelican API
     |
     | Server and allocation discovery
     v
Pelican MC Router
     |
     | Desired route synchronization
     v
Selected routing backend
  |-- mc-router HTTP API
  `-- Infrared configuration files
```

Only one routing backend is active for a deployment.

## Responsibilities

Pelican MC Router is responsible for:

- connecting to the Pelican application API;
- discovering Pelican-managed Minecraft servers;
- reading each server's primary allocation;
- generating deterministic Minecraft hostnames;
- maintaining the desired route set;
- reconciling routes with the selected backend;
- exposing setup, server, route, and health HTTP endpoints;
- persisting application settings in SQLite;
- periodically refreshing discovered servers and routes.

Pelican MC Router is not responsible for:

- proxying Minecraft connections directly;
- starting or stopping Minecraft servers;
- managing Pelican Wings;
- modifying DNS records;
- dynamically creating Docker containers;
- accessing the Docker socket;
- requiring Traefik or another HTTP reverse proxy.

## Data plane

The data plane handles Minecraft client traffic.

```text
Minecraft client
       |
       | TCP 25565
       v
Routing backend
       |
       | Destination selected from requested hostname
       v
Pelican server allocation
```

The routing backend reads the hostname requested by the Minecraft client and
forwards the connection to the configured backend address.

For example:

```text
techopolis.mc.example.com -> 192.168.1.50:25566
```

The destination is derived from the server's primary Pelican allocation.

## Control plane

The control plane discovers and maintains routes.

```text
Pelican API
     |
     v
Pelican client
     |
     v
Server discovery
     |
     v
Route generation
     |
     v
Generic route synchronizer
     |
     v
Selected backend adapter
```

Route discovery and synchronization are independent of any specific routing
backend.

This separation allows additional backends to be implemented without
changing Pelican discovery or hostname generation.

## Route discovery

The discovery process:

1. Fetches servers from the Pelican application API.
2. Filters for supported Minecraft servers.
3. Reads each server's primary allocation.
4. Normalizes the server name into a hostname-safe label.
5. Appends the configured base routing domain.
6. Produces the desired hostname-to-backend mapping.

For example:

```text
Pelican server name: Techopolis
Routing domain:      mc.example.com
Generated hostname:  techopolis.mc.example.com
Backend allocation:  192.168.1.50:25566
```

The generated route set is deterministic. Identical Pelican state and
configuration produce identical routes.

## Route reconciliation

The synchronizer treats the discovered route set as the desired state.

```text
Desired routes
      |
      v
Generic synchronizer
      |
      | Create, update, or remove managed routes
      v
Backend adapter
```

Repeated synchronization converges toward the same result and must not create
duplicate or progressively changing route state.

The selected backend adapter translates generic route operations into the
backend-specific mechanism.

## Routing backends

### mc-router

`mc-router` is the default and recommended backend.

```text
Pelican MC Router
       |
       | Private HTTP API
       v
mc-router
```

Routes are managed through the mc-router HTTP API.

The API should remain private to the deployment network. Minecraft clients
only need access to the public Minecraft listener.

The production Compose deployment stores mc-router's temporary route file on
tmpfs. Pelican MC Router remains the authoritative source of desired route
state.

### Infrared

Infrared is supported as an optional backend.

```text
Pelican MC Router
       |
       | Managed proxy configuration files
       v
Infrared
```

The Infrared adapter:

- writes managed proxy configuration files;
- removes obsolete managed files;
- updates a reload marker after configuration changes.

Infrared file generation is isolated behind the same backend interface used
by mc-router.

## Application components

The Go backend is organized around explicit responsibilities:

```text
backend/
|-- cmd/server
|   `-- application entry point
|-- internal/app
|   `-- application construction and lifecycle
|-- internal/discovery
|   `-- Pelican server discovery and route generation
|-- internal/http
|   `-- HTTP handlers and API routing
|-- internal/pelican
|   `-- Pelican API client
|-- internal/router
|   |-- generic route synchronization
|   |-- mcrouter
|   |   `-- mc-router backend adapter
|   `-- infrared
|       `-- Infrared backend adapter
|-- internal/runtime
|   `-- runtime configuration and refresh behavior
|-- internal/scheduler
|   `-- periodic reconciliation
|-- internal/settings
|   `-- persisted application settings
|-- internal/storage
|   `-- SQLite storage and migrations
|-- pkg/config
|   `-- configuration loading and defaults
`-- pkg/models
    `-- shared domain models
```

Backend-specific details remain inside their adapters. Discovery and route
models do not depend on mc-router or Infrared implementations.

## Configuration and persistence

Configuration comes from:

- static configuration files;
- environment variables;
- runtime settings stored in SQLite.

Runtime settings include:

- the Pelican panel URL;
- the Pelican application API key;
- the base Minecraft routing domain.

The SQLite database contains persistent application state and should be
stored on durable storage.

Backend-generated temporary or derived files are not the authoritative
application state.

## Deployment boundaries

The production Compose deployment separates public and private interfaces:

```text
Host network
|-- TCP 25565
|   `-- public Minecraft listener
`-- TCP 127.0.0.1:8080
    `-- local Pelican MC Router management API

Private Compose network
|-- Pelican MC Router
`-- mc-router management API
```

Security properties of the default deployment include:

- no Docker socket mount;
- unprivileged container users;
- read-only root filesystems;
- dropped Linux capabilities;
- `no-new-privileges`;
- a private backend management API;
- a localhost-only application API by default.

Remote access to the application API should be placed behind an authenticated
reverse proxy or another trusted access-control layer.

## Extensibility

A new routing backend should implement the generic backend contract rather
than adding backend-specific behavior to discovery or scheduling.

A backend implementation should be responsible only for:

- translating generic routes into its native representation;
- reading or tracking managed route state when required;
- applying route additions, updates, and removals;
- reporting actionable synchronization errors.

This keeps the architecture open to additional routing engines while
preserving one shared discovery and reconciliation pipeline.
