# Roadmap

Released work is documented in the [changelog](../CHANGELOG.md). This roadmap
describes planned work only; it does not change the behavior of a released
image.

## v0.3.0: Secure management plane

- [x] Require verified OIDC roles for management endpoints after bootstrap.
- [x] Protect one-time initial setup with a mounted bootstrap-token secret.
- [x] Accept file-backed Pelican credential references for new setup and
  rotation, while allowing a documented legacy upgrade transition.
- [x] Add bounded sensitive-action rate limits and generic audit events.
- [x] Add `govulncheck` to the backend CI gate.

See [ADR-0007](adr/ADR-0007-management-plane-security.md) for the approved
endpoint, trust-boundary, bootstrap, and credential-transition contract.

## v0.4.0: Route policies

- [x] Define the UUID-keyed route-policy, managed-route ownership, and preview
  contract in [ADR-0008](adr/ADR-0008-route-policy-model.md).
- [x] Persist UUID-keyed route policies.
- [x] Validate stable primary names, aliases, exclusions, and collisions before
  a backend mutation.
- [x] Establish a managed-route boundary for mc-router before expanding backend
  mutations; one active writer remains the supported topology.
- [x] Add authenticated route preview and policy-management APIs.

## v0.5.0: Operational history

- [x] Persist bounded, allowlisted reconciliation and action history.
- [x] Expose authenticated recent-event views and dashboard rendering.
- [x] Publish safe Prometheus alert examples.

## v0.6.0: Recovery and lifecycle

- [ ] Formalize migration compatibility and integrity validation.
- [ ] Add offline SQLite backup, restore, and recovery tools.
- [ ] Add fake-control-plane end-to-end and release lifecycle coverage.

## v1.0.0: Stable contract

- [ ] Publish an OpenAPI contract and compatibility/deprecation policy.
- [ ] Declare supported topology, database guarantees, and security model.
- [ ] Complete threat-model, performance, and lifecycle evidence.
