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