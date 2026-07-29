# ADR-0006: Dashboard OIDC Authorization

## Status

Accepted

## Date

2026-07-29

## Context

ADR-0005 requires application-level authentication and authorization before
dashboard write actions can be considered. The dashboard is same-origin and
read-only today, but relying only on a network boundary cannot authorize future
operator actions or produce consistent denial behavior.

## Decision

- Dashboard authentication is opt-in and uses verified OpenID Connect ID tokens.
  An authenticated reverse proxy or SSO obtains the token and forwards it in
  the `Authorization: Bearer` request header.
- When enabled, the application discovers the configured HTTPS issuer during
  startup using a bounded timeout, verifies token signatures and issuer through
  the provider's published keys, and requires the configured audience.
- A token must contain a non-empty subject and an array-valued configured role
  claim. Access to `/dashboard` requires the configured `viewer` role by
  default. The subject and token are never logged, returned, or placed in
  metrics.
- Missing, malformed, expired, or unverifiable tokens return a generic `401`.
  Verified identities without the required role return a generic `403`.
- `/health`, `/ready`, `/metrics`, and the existing versioned API remain outside
  this middleware. The deployment's private binding or authenticated proxy
  boundary remains required.
- `viewer` is read-only. A future `operator` role may authorize narrowly scoped
  mutations only after a separate design defines CSRF protection, request
  serialization, audit events, and cancellation behavior.

## Consequences

Existing deployments retain their current read-only dashboard behavior unless
OIDC is explicitly enabled. Enabling it requires an SSO or reverse proxy able
to forward a verified token, an HTTPS issuer URL, the expected audience, and a
role claim. If provider discovery fails at startup, the process fails closed.

This change intentionally does not add dashboard actions, cookies, sessions,
CSRF exemptions, or user/audit persistence.
