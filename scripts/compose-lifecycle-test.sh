#!/usr/bin/env bash
set -Eeuo pipefail

readonly ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"

if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
  compose=(docker compose)
  engine=docker
elif command -v podman-compose >/dev/null 2>&1; then
  compose=(podman-compose)
  engine=podman
elif command -v podman >/dev/null 2>&1 && podman compose version >/dev/null 2>&1; then
  compose=(podman compose)
  engine=podman
else
  echo 'Compose is required' >&2
  exit 1
fi

project="pelican-mc-router-lifecycle-${GITHUB_RUN_ID:-$$}"
secrets="$(mktemp -d /tmp/pelican-mc-router-lifecycle.XXXXXX)"
cleanup() {
  "${compose[@]}" --project-name "${project}" --file docker-compose.yml --file docker-compose.lifecycle.yml down --volumes --remove-orphans || true
  if [[ "${engine}" = podman ]]; then
    podman unshare chown 0:0 "${secrets}" "${secrets}"/* 2>/dev/null || true
  fi
  rm -rf -- "${secrets}"
}
trap cleanup EXIT

printf '%s\n' lifecycle-bootstrap-token >"${secrets}/bootstrap-token"
printf '%s\n' lifecycle-pelican-key >"${secrets}/pelican-api-key"
chmod 700 "${secrets}"
chmod 600 "${secrets}"/*
if [[ "${engine}" = podman ]]; then
  podman unshare chown 10001:10001 "${secrets}" "${secrets}"/*
else
  sudo chown 10001:10001 "${secrets}" "${secrets}"/*
fi

export PELICAN_MC_ROUTER_SECRETS_HOST_DIR="${secrets}"
export PELICAN_MC_ROUTER_IMAGE="pelican-mc-router:lifecycle-${project}"
export PELICAN_MC_ROUTER_HTTP_PORT=38081
export PELICAN_MC_ROUTER_BIND_ADDRESS=127.0.0.1
export PELICAN_MC_ROUTER_VERSION=lifecycle
export PELICAN_MC_ROUTER_REVISION=disposable

"${compose[@]}" --project-name "${project}" --file docker-compose.yml --file docker-compose.lifecycle.yml up --detach --build
for _ in $(seq 1 60); do
  if curl --fail --silent http://127.0.0.1:38081/health >/dev/null; then break; fi
  sleep 1
done
curl --fail --silent http://127.0.0.1:38081/health >/dev/null
status="$(curl --silent --output /dev/null --write-out '%{http_code}' http://127.0.0.1:38081/ready)"
test "${status}" = 503

curl --fail --silent --show-error \
  --header 'Authorization: Bearer lifecycle-bootstrap-token' \
  --header 'Content-Type: application/json' \
  --data '{"pelican_url":"http://fake-pelican:8081/api/application","pelican_secret_name":"pelican-api-key","router_domain":"mc.example.test"}' \
  http://127.0.0.1:38081/api/v1/setup >/dev/null

for _ in $(seq 1 30); do
  if curl --fail --silent http://127.0.0.1:38081/ready >/dev/null; then break; fi
  sleep 1
done
curl --fail --silent http://127.0.0.1:38081/ready >/dev/null

# The route must have been created by the application through the Compose
# network; this test never writes directly to mc-router.
routes="$("${compose[@]}" --project-name "${project}" --file docker-compose.yml --file docker-compose.lifecycle.yml exec -T pelican-mc-router wget -qO- http://mc-router:8080/routes)"
grep -Fq 'lifecycle.mc.example.test' <<<"${routes}"
