# ADR-0008: Stable Route Policy and Ownership Model

## Status

Accepted

## Date

2026-07-29

## Context

Generated hostnames currently derive from mutable Pelican server names. That
works for the default route set, but it cannot safely support renamed servers,
operator-selected primary names, aliases, exclusions, or a preview API. Route
policy must also not widen the routing backend's authority: in particular, an
mc-router reconciliation must never delete a route owned by another control
plane.

The v0.3.0 management-plane contract already requires verified viewer and
operator authorization. This decision defines the route-policy boundary that
subsequent v0.4.0 storage, planner, backend, and API changes implement.

## Decision

### Stable policy identity

- Persist route policy records by the immutable Pelican server UUID. A server
  name, identifier, allocation, hostname, or backend address is never a policy
  key.
- A server with no policy retains the existing generated hostname as its
  default primary route. Renaming a server therefore changes only that
  unconfigured default; stored policy remains attached to its UUID.
- A policy may select one DNS-valid primary hostname, zero or more DNS-valid
  aliases, or exclusion from routing. It cannot select an arbitrary backend
  address, allocation, or Pelican server identity.
- Policies for currently undiscovered or deleted servers remain stored. They
  are visible only through authorized management views and are not applied
  until their UUID is discovered again.

### Pure planning and validation

- Route planning is a backend-independent, side-effect-free operation. It
  consumes discovered server facts, persisted policy, and the configured base
  domain, then returns a complete desired route set or a bounded validation
  result.
- The planner normalizes hostnames and rejects invalid DNS names, duplicate
  normalized names, duplicate aliases, collisions between defaults and policy
  names, and invalid policy combinations before any backend call.
- The same planner powers preview and reconciliation. A successful preview
  therefore describes the desired state that reconciliation will submit,
  subject only to later discovery or backend changes.
- Preview never performs Pelican refreshes or routing-backend calls. It uses a
  caller-supplied/snapshotted inventory and returns only explicit, sanitized
  route-policy schema fields and bounded counts.

### Managed-route ownership and topology

- Each backend adapter reconciles only routes inside an explicit Pelican MC
  Router managed boundary. For mc-router, this boundary must be represented by
  a stable, adapter-recognized ownership marker or namespace; routes outside it
  are read-only and are never deleted or overwritten.
- A deployment has exactly one active Pelican MC Router writer for a database
  and managed backend boundary. Active-active control planes are unsupported
  until a separately designed lease and fencing mechanism exists.
- The service must fail safely before a backend mutation when it cannot
  establish its managed boundary or single-writer assumption. It preserves the
  last known good runtime route set on reconciliation failure.
- Infrared continues to manage only its own explicitly prefixed rendered files;
  it must not remove unrelated files.

### API authorization and concurrency

- Authorized viewers may read sanitized route policy and preview results.
  Authorized operators may create, update, or delete policies.
- Policy writes use an explicit monotonically increasing revision. A write with
  a stale revision fails with a conflict and does not trigger a partial update.
- Policy management does not expose backend control APIs, backend addresses,
  credentials, raw errors, or arbitrary backend overrides.

## Consequences

v0.4.0 introduces additive policy storage keyed by UUID and leaves the default
generated-hostname behavior intact for servers without policy. The planner is
the single collision-validation point for all adapters, which avoids the
current inconsistent duplicate handling.

Until the managed-route boundary and single-writer contract are implemented,
policy writes that could expand backend mutation authority must not be enabled.
Operators using multiple instances against the same database or backend are
unsupported and must use one active writer.

These additions do not provision DNS, support arbitrary destination overrides,
or provide active-active reconciliation. Route names and any returned route
facts remain management-plane data and require the v0.3.0 authorization
boundary.
