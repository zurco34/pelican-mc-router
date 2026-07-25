#!/usr/bin/env bash

set -Eeuo pipefail

readonly PROJECT_ROOT="$(
  cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."
  pwd
)"

cd "${PROJECT_ROOT}"

log() {
  printf '\n%s\n' "--- $* ---"
}

fail() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

detect_compose() {
  if command -v docker >/dev/null 2>&1 &&
    docker compose version >/dev/null 2>&1; then
    COMPOSE=(docker compose)
    ENGINE=(docker)
    return
  fi

  if command -v podman-compose >/dev/null 2>&1; then
    COMPOSE=(podman-compose)
    ENGINE=(podman)
    return
  fi

  if command -v podman >/dev/null 2>&1 &&
    podman compose version >/dev/null 2>&1; then
    COMPOSE=(podman compose)
    ENGINE=(podman)
    return
  fi

  fail "Docker Compose, podman-compose, or Podman Compose is required"
}

compose() {
  "${COMPOSE[@]}" \
    --project-name "${COMPOSE_PROJECT_NAME}" \
    --file docker-compose.yml \
    "$@"
}

container_id() {
  local service="$1"
  local id

  id="$(
    "${ENGINE[@]}" ps \
      --filter "label=com.docker.compose.project=${COMPOSE_PROJECT_NAME}" \
      --filter "label=com.docker.compose.service=${service}" \
      --format '{{.ID}}'
  )"

  [[ -n "${id}" ]] ||
    fail "container for service ${service} was not found"

  if [[ "${id}" == *$'\n'* ]]; then
    fail "multiple containers were found for service ${service}"
  fi

  printf '%s\n' "${id}"
}

container_exec() {
  local service="$1"
  local id

  shift

  if ! id="$(container_id "${service}")"; then
    return 1
  fi

  "${ENGINE[@]}" exec "${id}" "$@"
}

show_diagnostics() {
  log "Container status"
  compose ps || true

  log "Container logs"
  compose logs --no-color || true
}

cleanup() {
  local exit_code=$?

  trap - EXIT INT TERM

  if ((exit_code != 0)); then
    show_diagnostics
  fi

  if [[ "${SMOKE_KEEP_STACK:-0}" != "1" ]]; then
    log "Removing smoke-test stack"
    compose down \
      --volumes \
      --remove-orphans \
      || true
  else
    log "Keeping smoke-test stack"
    printf 'Project name: %s\n' "${COMPOSE_PROJECT_NAME}"
  fi

  exit "${exit_code}"
}

wait_for_health() {
  local attempts="${SMOKE_HEALTH_ATTEMPTS:-60}"
  local delay="${SMOKE_HEALTH_DELAY_SECONDS:-2}"
  local attempt

  for ((attempt = 1; attempt <= attempts; attempt++)); do
    if curl \
      --fail \
      --silent \
      --show-error \
      "http://127.0.0.1:${PELICAN_MC_ROUTER_HTTP_PORT}/health" \
      >/dev/null 2>&1; then
      return
    fi

    printf \
      'Waiting for application health (%d/%d)...\n' \
      "${attempt}" \
      "${attempts}"

    sleep "${delay}"
  done

  fail "application did not become healthy"
}

detect_compose

readonly SMOKE_ID="$(
  printf '%s' "${GITHUB_RUN_ID:-$$}" |
    tr -cd '[:alnum:]'
)"

export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-pelican-mc-router-smoke-${SMOKE_ID}}"

export TZ="${TZ:-UTC}"

export MINECRAFT_BIND_ADDRESS="127.0.0.1"
export MINECRAFT_PORT="${SMOKE_MINECRAFT_PORT:-32565}"

export PELICAN_MC_ROUTER_BIND_ADDRESS="127.0.0.1"
export PELICAN_MC_ROUTER_HTTP_PORT="${SMOKE_HTTP_PORT:-38080}"

export PELICAN_MC_ROUTER_IMAGE="${PELICAN_MC_ROUTER_IMAGE:-pelican-mc-router:smoke-${SMOKE_ID}}"

export PELICAN_MC_ROUTER_VERSION="smoke"
export PELICAN_MC_ROUTER_REVISION="$(
  git rev-parse HEAD 2>/dev/null || printf 'unknown'
)"

trap cleanup EXIT INT TERM

command -v curl >/dev/null 2>&1 ||
  fail "curl is required"

log "Smoke-test environment"
printf 'Compose command:'
printf ' %q' "${COMPOSE[@]}"
printf '\n'
printf 'Container engine:'
printf ' %q' "${ENGINE[@]}"
printf '\n'
printf 'Project: %s\n' "${COMPOSE_PROJECT_NAME}"
printf 'Application API: 127.0.0.1:%s\n' \
  "${PELICAN_MC_ROUTER_HTTP_PORT}"
printf 'Minecraft listener: 127.0.0.1:%s\n' \
  "${MINECRAFT_PORT}"

log "Validating Compose model"
compose config >/dev/null

log "Removing any previous stack with the same project name"
compose down \
  --volumes \
  --remove-orphans \
  >/dev/null 2>&1 ||
  true

log "Building and starting stack"
compose up \
  --detach \
  --build

log "Waiting for application health"
wait_for_health

health_response="$(
  curl \
    --fail \
    --silent \
    --show-error \
    "http://127.0.0.1:${PELICAN_MC_ROUTER_HTTP_PORT}/health"
)"

printf 'Health response: %s\n' "${health_response}"
[[ "${health_response}" = "OK" ]] ||
  fail "unexpected health response: ${health_response}"

log "Verifying application runtime user"
runtime_identity="$(
  container_exec \
    pelican-mc-router \
    sh -eu -c \
    'printf "%s:%s" "$(id -u)" "$(id -g)"'
)"

printf 'Runtime identity: %s\n' "${runtime_identity}"
[[ "${runtime_identity}" = "10001:10001" ]] ||
  fail "unexpected runtime identity: ${runtime_identity}"

log "Creating route through private mc-router API"

readonly ROUTE_HOSTNAME="smoke-test.mc.example.com"
readonly ROUTE_BACKEND="127.0.0.1:25565"

route_payload="$(
  printf \
    '{"serverAddress":"%s","backend":"%s"}' \
    "${ROUTE_HOSTNAME}" \
    "${ROUTE_BACKEND}"
)"

container_exec \
  pelican-mc-router \
  wget \
  -q \
  -O /dev/null \
  --header='Content-Type: application/json' \
  --post-data="${route_payload}" \
  http://mc-router:8080/routes

log "Reading routes through private mc-router API"

routes="$(
  container_exec \
    pelican-mc-router \
    wget \
    -q \
    -O - \
    http://mc-router:8080/routes
)"

printf '%s\n' "${routes}"

grep -Fq "${ROUTE_HOSTNAME}" <<<"${routes}" ||
  fail "created hostname was not returned by mc-router"

grep -Fq "${ROUTE_BACKEND}" <<<"${routes}" ||
  fail "created backend was not returned by mc-router"

log "Verifying application remains healthy"

curl \
  --fail \
  --silent \
  --show-error \
  "http://127.0.0.1:${PELICAN_MC_ROUTER_HTTP_PORT}/health" \
  >/dev/null

log "Smoke test completed successfully"
