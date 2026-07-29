# ADR-0005: Dashboard Security and Exposure Model

## Status

Accepted

## Date

2026-07-29

## Context

The dashboard exposes operational information about the Pelican MC Router
control plane. Although it must not expose credentials, it can reveal
deployment state and routing activity. The current Compose deployment binds the
management API to loopback by default and keeps the routing-backend API private.

Dashboard work must preserve those boundaries before adding UI code or operator
actions.

## Decision

- The dashboard will be served same-origin by Pelican MC Router and will remain
  fully usable through the existing headless API.
- The dashboard is read-only apart from the separately authorized manual
  reconciliation action. It may display only data already safe for the
  versioned API: build identity, readiness, cached reconciliation state, and
  bounded diagnostics.
- Production access must remain private or be protected by an authenticated
  reverse proxy/SSO layer. Direct public exposure of the dashboard or
  management API is unsupported.
- `/health`, `/ready`, and `/metrics` retain their existing operational roles;
  they are not dashboard authentication mechanisms.
- The mc-router control API remains on the private Compose network and is never
  exposed through the dashboard.
- Credentials, API keys, sensitive URLs, raw backend errors, hostnames, and
  server identifiers must not be rendered, logged, or added to metrics for the
  dashboard.
- Dashboard actions require application-level authentication and authorization.
  ADR-0006 defines the OIDC roles, CSRF header, denial behavior, and generic
  action logging for manual reconciliation. Additional actions require a new
  decision before implementation.

## Consequences

The dashboard can use the existing deployment model without adding a frontend
authentication system. Operators who need remote access must place the
application behind their authenticated proxy or SSO boundary.

The existing manual reconciliation control does not make the dashboard a
general management surface. Its authorization is intentionally narrow and the
least-privilege Docker topology remains unchanged.
