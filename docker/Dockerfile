# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.26.5
ARG ALPINE_VERSION=3.24

FROM golang:${GO_VERSION}-bookworm AS build

ARG VERSION=dev
ARG REVISION=unknown

WORKDIR /src/backend

COPY backend/go.mod backend/go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY backend/ ./

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 \
    GOOS=linux \
    go build \
        -trimpath \
        -ldflags="-s -w -X github.com/zurco34/pelican-mc-router/pkg/buildinfo.Version=${VERSION} -X github.com/zurco34/pelican-mc-router/pkg/buildinfo.Revision=${REVISION}" \
        -o /out/pelican-mc-router \
        ./cmd/server

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 \
    GOOS=linux \
    go build \
        -trimpath \
        -ldflags="-s -w -X github.com/zurco34/pelican-mc-router/pkg/buildinfo.Version=${VERSION} -X github.com/zurco34/pelican-mc-router/pkg/buildinfo.Revision=${REVISION}" \
        -o /out/sqlite-recovery \
        ./cmd/sqlite-recovery

FROM alpine:${ALPINE_VERSION} AS runtime

ARG VERSION=dev
ARG REVISION=unknown

LABEL org.opencontainers.image.title="Pelican MC Router" \
      org.opencontainers.image.description="Automatic Minecraft hostname routing for Pelican-managed servers" \
      org.opencontainers.image.source="https://github.com/zurco34/pelican-mc-router" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}"

RUN apk add --no-cache \
        ca-certificates \
        tzdata \
    && addgroup \
        -S \
        -g 10001 \
        pelican \
    && adduser \
        -S \
        -D \
        -H \
        -u 10001 \
        -G pelican \
        pelican \
    && mkdir -p /app/data \
    && chown -R pelican:pelican /app

WORKDIR /app

COPY --from=build \
    --chown=pelican:pelican \
    /out/pelican-mc-router \
    /usr/local/bin/pelican-mc-router

COPY --from=build \
    --chown=pelican:pelican \
    /out/sqlite-recovery \
    /usr/local/bin/sqlite-recovery

ENV PELICAN_MC_ROUTER_SERVER_PORT=8080 \
    PELICAN_MC_ROUTER_DATABASE_PATH=/app/data/pelican-mc-router.db

USER 10001:10001

EXPOSE 8080

HEALTHCHECK \
    --interval=30s \
    --timeout=3s \
    --start-period=10s \
    --retries=3 \
    CMD wget \
        -q \
        -O /dev/null \
        "http://127.0.0.1:${PELICAN_MC_ROUTER_SERVER_PORT}/health" \
        || exit 1

ENTRYPOINT ["/usr/local/bin/pelican-mc-router"]
