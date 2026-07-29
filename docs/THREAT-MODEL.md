# Threat model review

## Assets and boundaries

Pelican API credentials, bootstrap tokens, routing topology, and management
authority are sensitive. The service trusts only mounted secret files, verified
OIDC tokens where enabled, the local SQLite volume, the Pelican API, and a
private routing-backend control plane.

## Controls

- Bootstrap setup is limited to an uninitialized database and mounted bearer
  token; subsequent management access requires configured authorization.
- New Pelican credentials are file references, not request-body secrets.
- Operational endpoints are unauthenticated by design and must be protected by
  deployment network boundaries.
- API responses, logs, metrics, history, fixtures, and alerts exclude token
  values, secret names, raw errors, URLs, hostnames, and server identifiers.
- One active writer per managed route boundary is supported; active-active
  control planes are not.
- Containers run non-root with a read-only filesystem, dropped capabilities,
  and `no-new-privileges`.

## Residual risks

The operator remains responsible for reverse-proxy exposure, OIDC issuer and
role configuration, secret-file permissions, backup encryption/storage, and
keeping the management network private. Validate every upgrade with the
disposable lifecycle matrix before production use.
