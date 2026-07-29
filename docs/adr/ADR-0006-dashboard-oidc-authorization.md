# ADR-0006: Dashboard OIDC Authorization

## Status

Accepted

## Date

2026-07-29

## Context

ADR-0005 requires application-level authentication and authorization before
dashboard write actions can be considered. The dashboard is same-origin and
has one narrow operator action, but relying only on a network boundary cannot
authorize that action or produce consistent denial behavior.

## Decision

- Management and dashboard authentication use verified OpenID Connect ID tokens.
  An authenticated reverse proxy or SSO obtains the token and forwards it in
  the `Authorization: Bearer` request header.
- When enabled, the application discovers the configured HTTPS issuer during
  startup using a bounded timeout, verifies token signatures and issuer through
  the provider's published keys, and requires the configured audience.
- A token must contain a non-empty subject and an array-valued configured role
  claim. Access to `/dashboard` requires the configured `viewer` role by
  default; the configured `operator` role inherits read access. The subject and
  token are never logged, returned, or placed in metrics.
- Missing, malformed, expired, or unverifiable tokens return a generic `401`.
  Verified identities without the required role return a generic `403`.
- The only authorized dashboard mutation is a manual reconciliation request.
  It requires the `operator` role and the same-origin
  `X-Pelican-MC-Router-CSRF: 1` request header, uses the existing serialized
  refresh path with the request context, and records only generic action
  lifecycle events without an identity or token.
- `/health`, `/ready`, and `/metrics` remain public operational endpoints.
  Versioned management APIs require the viewer or operator role defined by
  ADR-0007; missing management authorization fails closed. The deployment's
  private binding or authenticated proxy boundary remains required.
- `viewer` is read-only. The configured `operator` role may authorize narrowly
  scoped mutations only after a separate design defines CSRF protection,
  request serialization, audit events, and cancellation behavior.

## Consequences

Management deployment requires an SSO or reverse proxy able
to forward a verified token, an HTTPS issuer URL, the expected audience, and a
role claim. If provider discovery fails at startup, the process fails closed.

This change intentionally does not add cookies, sessions, CSRF exemptions, or
user/audit persistence beyond the bounded generic action lifecycle events.
