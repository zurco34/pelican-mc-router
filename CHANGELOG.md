# Changelog

## 1.1.0 - 2026-08-11

### Added

- The operations dashboard now supports light and dark themes. When no explicit
  preference has been stored, it follows the browser or operating-system color
  scheme. An explicit choice is persisted locally in the browser.

### Changed

- Authorized `GET /` requests now return `307 Temporary Redirect` to
  `/dashboard` under the existing management viewer authorization.
- The stable OpenAPI document now includes the authenticated root redirect.

### Upgrade notes

- No database migration or configuration change is required. Existing OIDC
  viewer/operator authorization and manual-reconciliation CSRF behavior are
  unchanged.
- Dashboard theme preference is browser-local presentation state stored in
  `localStorage`; it is not sent to the service.
- Back up the SQLite volume before upgrade. Migrations are forward-only; use a
  verified pre-upgrade backup when a rollback requires an older binary.

## 1.0.7 - 2026-08-10

### Fixed

- Management OIDC configuration now loads issuer and audience values supplied
  through the documented environment variables, allowing OIDC-enabled startup
  while preserving existing fail-closed validation.

### Upgrade notes

- Upgrade to v1.0.7 before enabling management OIDC through environment-only
  configuration. No database migration or API contract change is introduced.
- Back up the SQLite volume before upgrade. Migrations are forward-only; use a
  verified pre-upgrade backup when a rollback requires an older binary.

## 1.0.6 - 2026-08-09

### Fixed

- Management API routes now fail closed when an authorizer is unavailable;
  public liveness, readiness, and metrics endpoints remain operational.
- Periodic reconciliation failures preserve the last-known-good runtime, mark
  readiness unhealthy, and retry at the configured interval instead of
  terminating the process.
- Route-policy requests now distinguish omitted required fields from explicit
  zero values, matching the stable OpenAPI contract.

### Changed

- OpenAPI documentation now records management-auth unavailability and the
  dashboard reconciliation CSRF header.
- Corrected the historical v1.0.5 release notes: pending-setup generation
  restaging shipped in v1.0.5. The immutable v1.0.5 tag is unchanged.

### Upgrade notes

- Back up the SQLite volume before upgrade. Migrations are forward-only; use a
  verified pre-upgrade backup when a rollback requires an older binary.

## 1.0.5 - 2026-08-09

### Fixed

- HTTP control-plane logs now use stable failure categories and do not include
  raw external error text.
- The versioned API contract distinguishes route-policy create and update
  requests, documents settings-before-setup conflicts, and preserves additive
  response compatibility.
- Pending setup restaging replaces the candidate generation, preventing a
  stale candidate from being promoted during retry.

### Added

- Authorization-class and lifecycle coverage for public operational,
  bootstrap-only, viewer, and operator paths, including fail-closed management
  wiring when OIDC is disabled.
- Disposable validation covers safe error logging, setup-state behavior,
  recovery-binary availability, non-root operation, and private routing
  backend connectivity.

### Upgrade notes

- Back up the SQLite volume before upgrade. Migrations are forward-only; use a
  verified pre-upgrade backup when a rollback requires an older binary.

## 1.0.4 - 2026-08-08

### Fixed

- Candidate setup activation records its completed reconciliation state, so
  readiness becomes healthy after a successful initial setup.

### Added

- A disposable application lifecycle test covering bootstrap setup, file-backed
  credentials, reconciliation, restart closure, and graceful shutdown.
- A dedicated race-enabled Backend CI gate for the disposable lifecycle test.

### Changed

- Deployment guidance now requires OIDC configuration before normal management
  use and provides explicit rootful Docker and rootless Podman secret ownership
  steps.

### Upgrade notes

- Back up the SQLite volume before upgrade. v1.0.3 remains the immediate
  rollback image only while the database schema remains compatible.

## 1.0.3 - 2026-08-07

### Fixed

- Candidate activation now compensates failed or canceled route synchronization
  before releasing the serialized runtime owner lock.
- `/api/v1/status` now returns the documented sanitized `503` response when
  setup state is unavailable.

### Added

- Explicit bounded OpenAPI schemas and route-contract checks for operation IDs.
- Bounded authenticated sensitive-action history at `/api/v1/action-history`.

### Changed

- Recovery instructions resolve the actual Compose database volume and stable
  deployment guidance documents owner-only file-backed secret mounts.

### Upgrade notes

- Back up the SQLite volume before upgrade. v1.0.2 remains the immediate
  rollback image only while the database schema remains compatible.

## 1.0.2 - 2026-08-07

### Fixed

- Candidate runtime activation remains serialized through reconciliation,
  persistence, compensation, and publication.
- Startup refuses databases containing migrations unsupported by the running
  server binary, protecting rollback safety.
- Completed setup routes now use the sanitized versioned JSON error envelope.

### Changed

- Deployment guidance now requires a real Pelican panel-issued Application API
  key and documents named-volume offline recovery.
- OpenAPI documents manual-reconciliation cancellation and unavailable
  outcomes; disposable Compose smoke verifies setup-incomplete readiness.
- Sensitive-action persistence uses bounded cancellation-safe recording and
  records rate-limit denials.

### Upgrade notes

- Back up the SQLite volume before upgrade. Migrations are forward-only; use
  the pre-upgrade backup when returning to an older binary is required.

## 1.0.1 - 2026-08-06

### Fixed

- Setup routes remain closed after activation, including after restart.
- Initial setup is promoted only after candidate runtime activation succeeds;
  failed activation remains bootstrap-retryable.

### Added

- Complete OpenAPI route-contract validation and a production-image offline
  `sqlite-recovery` binary.
- Bounded allowlisted sensitive-action history foundation and corrected v1
  security/deployment guidance.

## 1.0.0 - 2026-07-29

### Added

- Stable `/api/v1` OpenAPI source contract and compatibility/deprecation policy.
- Supported deployment topology, database, security, recovery, and lifecycle
  support policies.
- Documented threat model and release verification evidence.

### Changed

- The project now declares its v1 stable operational and API contract.

## 0.6.0 - 2026-07-29

### Added

- Migration checksum verification, including safe backfill for existing
  databases.
- Offline SQLite integrity, backup, restore, and compaction utility.
- Deterministic fake mc-router reconciliation coverage and a disposable release
  lifecycle verification matrix.

### Changed

- Recovery operations are explicitly offline-only and use distinct,
  non-overwriting restore targets with documented rollback precautions.

## 0.5.0 - 2026-07-29

### Added

- Bounded, persisted operational history containing only fixed reconciliation
  outcomes and numeric route-change diagnostics.
- Viewer-authorized recent-event API and dashboard activity view.
- Safe Prometheus alert-rule examples for reconciliation failures.

### Changed

- Operational history is retained locally with bounded size and excludes errors,
  credentials, topology, identities, and unbounded payloads.

## 0.4.0 - 2026-07-29

### Added

- UUID-keyed route policies with primary hostnames, aliases, exclusions, and
  optimistic revisions.
- Viewer-authorized route preview and policy reads, plus operator-authorized
  policy mutations.
- Cached reconciliation inventory for read-only route views without
  request-time Pelican discovery.

### Changed

- Reconciliation plans the complete policy-aware route set before backend
  mutation and uses immutable Pelican UUIDs as route identity.
- mc-router only deletes stale routes within its configured managed domain;
  deployments support one active control-plane writer per managed boundary.

All notable changes to Pelican MC Router are documented in this file.

The project follows Semantic Versioning. The stable v1 API, configuration, and
deployment compatibility commitments are defined in
[the API compatibility policy](docs/API-COMPATIBILITY.md).

## 0.3.0 - 2026-07-29

### Added

- Add a bounded reader contract and read-only Compose mount for future
  file-backed bootstrap and Pelican credential secrets. Secret values remain
  outside application configuration and `.env` files.
- Require the mounted bootstrap bearer token for setup while the database is
  uninitialized. Setup routes disappear after successful setup; the token is
  never persisted, returned, logged, or added to metrics.
- Require OIDC authorization for management routes and add bounded generic
  throttling for sensitive actions.
- Add file-backed Pelican credential references. Legacy SQLite credentials stay
  readable for upgrade compatibility until a successful reference rotation.

## 0.2.0 - 2026-07-29

### Added

- A same-origin, read-only `/dashboard` page for cached operational status. It
  does not trigger reconciliation and must remain private or protected by an
  authenticated reverse proxy.
- Optional OIDC JWT verification for `/dashboard`, with configured issuer,
  audience, and read-only role authorization. Invalid identities fail closed;
  identity data is never exposed.
- An OIDC-operator-authorized manual reconciliation action. It uses the
  existing serialized refresh path and returns only safe cached status.

### Security

- Dashboard OIDC protection is opt-in. When enabled, dashboard access requires
  a verified issuer, audience, and role; the manual reconciliation action also
  requires the configured `operator` role and a same-origin request header.

### Upgrade notes

- Dashboard OIDC settings are optional and default to disabled. Existing
  private, read-only dashboard deployments continue to work without them.
- Reconciliation and Prometheus metric history remain process-local. After an
  upgrade or restart, startup reconciliation repopulates current status while
  counters and histogram history start again from zero.

## 0.1.3

### Added

- Add coordinated graceful shutdown for SIGINT and SIGTERM, including the HTTP
  server and runtime scheduler.
- Add configurable, bounded HTTP server header, request, response, and idle
  timeouts.
- Add configurable, bounded retries for transient Pelican and mc-router read
  failures without retrying route mutations.
- Add build version and revision to startup logs and `GET /api/v1/status`.
- Add bounded reconciliation route diagnostics to status responses and success
  logs without exposing routing identities or backend details.

## 0.1.2

Patch release adding reconciliation readiness, cached status reporting, and
Prometheus metrics.

### Added

- Add process-local, race-safe reconciliation status tracking with sanitized
  public error summaries. Reconciliation and metric state reset on restart;
  startup reconciliation repopulates current status and gauges, while
  reconciliation counters and histogram history start over from zero.
- Add `GET /api/v1/status` for cached reconciliation state and `GET /ready`
  with stable reasons: `setup_incomplete`, `reconciliation_pending`,
  `reconciliation_failed`, `status_unavailable`, and `ready`.
- Preserve `GET /health` as unchanged process liveness and preserve
  last-known-good runtime services after failed reconciliation.
- Add `GET /metrics` with the reconciliation counter, duration histogram, and
  last-success, consecutive-failures, and in-progress gauges.
- Add standard Prometheus Go runtime and process collectors.
- Use fixed, low-cardinality reconciliation result labels only:
  `not_configured`, `success`, and `failure`.

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
