# Changelog

All notable changes to Pelican MC Router are documented in this file.

The project follows Semantic Versioning. Until version `1.0.0`, configuration,
API behavior, and deployment details may change between minor releases.

## 0.1.1

Patch release fixing Pelican wildcard allocation routing.

### Fixed

- Resolve Pelican allocation addresses `0.0.0.0` and `::` through the
  configurable `discovery.wildcard_backend_host` before route generation.
- Return a clear discovery error when a wildcard allocation has no configured
  fallback instead of programming an invalid routing destination.
- Preserve routable allocation IP addresses unchanged.

### Added

- Add the
  `PELICAN_MC_ROUTER_DISCOVERY_WILDCARD_BACKEND_HOST` environment variable and
  matching YAML configuration.
- Document how to choose a Pelican node address or Docker bridge gateway that
  is reachable from the routing backend.

## 0.1.0

First public development release.

### Added

- Automatic discovery of Minecraft servers through the Pelican application API.
- Deterministic hostname generation from Pelican server names and a configured
  base domain.
- Backend-independent route discovery and reconciliation.
- Default `mc-router` backend integration through its private HTTP API.
- Optional Infrared backend using managed proxy configuration files.
- SQLite persistence for application configuration and runtime state.
- REST endpoints for:
  - application health;
  - initial setup;
  - discovered servers;
  - generated routes.
- Production-oriented Docker image running as a non-root user.
- Docker Compose deployment with:
  - a private routing control network;
  - persistent application data;
  - read-only container filesystems;
  - dropped Linux capabilities;
  - `no-new-privileges`;
  - no Docker socket access.
- Reusable Docker Compose and Podman Compose smoke testing.
- Automated Go tests, race detection, vetting, binary builds, image builds,
  Compose validation, and runtime smoke testing in GitHub Actions.
- Tag-driven multi-platform container publishing for:
  - `linux/amd64`;
  - `linux/arm64`.
- GHCR image metadata, SBOM generation, and signed build provenance.

### Security

- The `mc-router` management API is not published outside the private Compose
  network.
- The Pelican MC Router management API binds to `127.0.0.1` by default.
- Application containers run without root privileges.
- Docker socket access is not required.

### Known limitations

- This is a pre-stable release. Configuration formats and API behavior may
  change before `1.0.0`.
- Only one routing backend can be active in a deployment.
- The web dashboard is not yet implemented.
- Initial configuration is performed through the REST API.
- The default deployment currently targets `mc-router` version `1.44.0`.
- Dynamic route updates for `mc-router` require a temporary writable route file
  because of upstream behavior in version `1.44.0`.
- The temporary `mc-router` route file is intentionally not persisted; Pelican
  MC Router remains the authoritative source and restores desired routes
  through reconciliation.
