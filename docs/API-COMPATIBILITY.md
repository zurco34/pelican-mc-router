# API compatibility policy

`/api/v1` is the stable public management contract as of v1.0.0. Additive
fields and endpoints may be introduced in v1 without changing existing field
meaning, authorization class, status-code semantics, or security guarantees.

Breaking changes require `/api/v2` and a documented migration path. A v1
deprecation will be documented in the changelog and deployment guide for at
least one minor release before removal. Public operational endpoints
(`/health`, `/ready`, `/metrics`) are outside the versioned API but retain
their documented liveness, readiness, and Prometheus semantics.

The source contract is [`api/openapi.yaml`](api/openapi.yaml). Consumers must
ignore unknown JSON fields and must not depend on response key ordering.
