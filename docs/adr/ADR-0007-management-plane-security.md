# ADR-0007: Management Plane Security Contract

## Status

Accepted

## Date

2026-07-29

## Context

The service currently exposes setup, settings, inventory, route, and status
endpoints without a consistent application-level access policy. Setup and
settings can accept Pelican credentials, so a deployment network boundary
alone is not a sufficient management-plane control.

The dashboard OIDC design in ADR-0006 provides a verified-token foundation,
but it applies only to dashboard routes. The next security release needs one
explicit contract before changing endpoint behavior, credential handling, or
deployment configuration.

## Decision

### Endpoint policy

- Public operational endpoints are `GET /health`, `GET /ready`, and
  `GET /metrics`. They remain unauthenticated so container and monitoring
  systems can use them, but deployment network controls remain mandatory.
- Viewer access is required for read-only management endpoints:
  `GET /dashboard`, `GET /api/v1/status`, `GET /api/v1/servers`, and
  `GET /api/v1/routes`.
- Operator access is required for management mutations, initially
  `PUT /api/v1/settings` and `POST /api/v1/dashboard/reconcile`. The operator
  role inherits viewer access.
- Setup is bootstrap-only. `GET` and `POST /api/v1/setup` require a bootstrap
  bearer token while the database is uninitialized. After successful setup,
  setup routes are no longer available and do not provide a way to replay or
  inspect bootstrap state.

### Authentication and proxy boundary

- OIDC is required for normal management-plane operation after bootstrap.
  The application verifies a bearer token forwarded in the `Authorization`
  header; it does not trust a proxy assertion of identity.
- The configured issuer, audience, role claim, viewer role, and operator role
  remain explicit application configuration. Missing or invalid OIDC
  configuration fails closed rather than exposing management routes.
- The application does not trust `X-Forwarded-For`, `Forwarded`, or similar
  client-identity headers by default. There is no proxy-derived client-IP
  authorization or per-client rate-limit key until a separate proxy trust
  decision is made.
- Management access is supported only behind a private bind or an authenticated
  reverse proxy/SSO deployment boundary. Direct public exposure is unsupported.

### Bootstrap and Pelican credentials

- Bootstrap uses a random bearer token stored in a mounted secret file. It is
  valid only for an uninitialized database and is never persisted, returned,
  logged, or added to metrics.
- New setup and credential rotation accept a validated secret reference, not a
  raw Pelican API key. References resolve only within a configured secret
  directory; API input cannot select an arbitrary filesystem path.
- Existing plaintext SQLite API-key settings remain an upgrade-only legacy
  state until a successful file-backed rotation. A successful rotation makes
  the file reference authoritative and removes reliance on the legacy value.
  No endpoint exposes either credential form.

### Sensitive actions

- Bootstrap, setup, settings updates, and manual reconciliation use bounded,
  action-class rate limits. Limits are not keyed by user, token, hostname, or
  IP address.
- These actions produce generic, allowlisted audit events. Events, logs,
  responses, and metrics must not include identities, tokens, secret names,
  hostnames, server identifiers, URLs, or raw errors.

## Consequences

This is a pre-1.0 access-policy change. Operators upgrading to the secure
release must provision OIDC configuration and a bootstrap secret file before
initial setup. Existing configured installations retain legacy credential
compatibility only until they complete an explicit file-backed rotation.

`/health`, `/ready`, and `/metrics` are intentionally not authorization
endpoints. Operators must continue to restrict their network exposure. The
contract does not add sessions, user persistence, arbitrary proxy trust,
encryption-at-rest, or multi-instance ownership; those require separate
decisions.
