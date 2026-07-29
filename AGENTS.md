# Project Instructions

## Project

- Pelican MC Router is a Go control-plane service.
- GitHub is the source of truth.
- It discovers Pelican-managed Minecraft servers and reconciles hostname routes into configurable routing backends.
- The service does not proxy Minecraft traffic itself.

## Architecture

- Preserve clean package boundaries and dependency direction.
- Keep `cmd/server` limited to process startup.
- Keep wiring and lifecycle in `internal/app`.
- Keep Pelican API concerns in `internal/pelican`.
- Keep discovery rules in `internal/discovery`.
- Keep backend-independent routing logic in `internal/router`.
- Keep backend adapters under `internal/router/<backend>`.
- Keep persisted runtime settings and migrations in their existing packages.
- Avoid global mutable state.
- Prefer narrow interfaces owned by the consuming package.
- Preserve last-known-good runtime behavior when reconciliation fails.

## Go Quality

- Target the Go version declared in `backend/go.mod`.
- Use production-quality, idiomatic Go.
- Run `gofmt` on changed Go files.
- Add table-driven unit tests where appropriate.
- Avoid unnecessary dependencies and abstractions.
- Wrap errors with useful operation context.
- Never include credentials, API keys, or sensitive URLs in errors, logs, metrics, fixtures, or test output.
- Ensure concurrent state is race-safe.

## HTTP and Observability

- Keep `/health` as liveness.
- Expose readiness separately; it must not replace Docker liveness checks.
- Keep API schemas explicit and backward-compatible within `/api/v1`.
- Prometheus labels must remain low-cardinality.
- Never use hostnames, server IDs, URLs, or error strings as metric labels.

## Docker

- Preserve non-root execution, read-only filesystems, dropped capabilities, and `no-new-privileges`.
- Keep containers Docker-first and Compose-compatible.
- Do not expose private routing-backend control APIs publicly.
- Pin release images rather than relying on `latest` in production examples.

## Testing

Required validation for backend changes:

```bash
cd backend && gofmt -w <changed-go-files>
cd backend && go mod verify
cd backend && go test ./...
cd backend && go test -race -count=1 ./...
cd backend && go vet ./...
cd backend && go build -trimpath -o /tmp/pelican-mc-router ./cmd/server
```

Validate Compose changes with:

```bash
docker compose --file docker-compose.yml config
```

- Run relevant repository smoke tests when affected.
- Do not claim a command passed unless it was actually run successfully.

## Git Workflow

- Work on a focused branch created from an up-to-date `main`.
- Use Conventional Commits.
- Keep commits and pull requests narrowly scoped.
- Do not commit, push, tag, merge, or create releases unless explicitly instructed.
- Never modify production deployment files outside this repository.

## Documentation

- Update README, deployment documentation, configuration examples, and CHANGELOG when behavior or configuration changes.
- Document operational consequences and rollback considerations for runtime changes.
