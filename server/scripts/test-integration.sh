#!/usr/bin/env bash

set -Eeuo pipefail

readonly script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
readonly server_dir="$(cd -- "${script_dir}/.." && pwd)"
readonly postgres_image="postgres:17-alpine"
readonly database_name="settled_test"
readonly database_user="settled"
readonly database_password="settled_test_password"

for required_command in docker psql go; do
  if ! command -v "${required_command}" >/dev/null 2>&1; then
    echo "error: required command not found: ${required_command}" >&2
    exit 1
  fi
done

if ! docker info >/dev/null 2>&1; then
  echo "error: Docker daemon is unavailable" >&2
  exit 1
fi

readonly temporary_dir="$(mktemp -d)"
readonly container_id_file="${temporary_dir}/container.cid"
container_id=""

cleanup() {
  local exit_status=$?
  trap - EXIT INT TERM

  if [[ ! "${container_id}" =~ ^[0-9a-f]{64}$ ]] &&
    [[ -f "${container_id_file}" ]]; then
    IFS= read -r container_id <"${container_id_file}" || true
  fi

  if [[ "${container_id}" =~ ^[0-9a-f]{64}$ ]]; then
    docker rm --force "${container_id}" >/dev/null 2>&1 || true
  fi

  rm -f -- "${container_id_file}"
  rmdir -- "${temporary_dir}" 2>/dev/null || true

  exit "${exit_status}"
}

trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

readonly container_name="settled-postgres-test-$(date +%s)-$$-${RANDOM}"

container_id="$(
  docker run \
    --detach \
    --rm \
    --cidfile "${container_id_file}" \
    --name "${container_name}" \
    --publish 127.0.0.1::5432 \
    --env "POSTGRES_DB=${database_name}" \
    --env "POSTGRES_USER=${database_user}" \
    --env "POSTGRES_PASSWORD=${database_password}" \
    "${postgres_image}"
)"

if [[ ! "${container_id}" =~ ^[0-9a-f]{64}$ ]]; then
  echo "error: Docker returned an invalid container ID" >&2
  exit 1
fi

readonly readiness_deadline=$((SECONDS + 30))
until docker exec "${container_id}" \
  pg_isready \
  --username "${database_user}" \
  --dbname "${database_name}" >/dev/null 2>&1; do
  if ((SECONDS >= readiness_deadline)); then
    echo "error: PostgreSQL did not become ready within 30 seconds" >&2
    docker logs "${container_id}" >&2 || true
    exit 1
  fi
  sleep 1
done

port_binding="$(docker port "${container_id}" 5432/tcp)"
host_port="${port_binding##*:}"
if [[ ! "${host_port}" =~ ^[0-9]+$ ]] ||
  ((host_port < 1 || host_port > 65535)); then
  echo "error: Docker returned an invalid PostgreSQL host port" >&2
  exit 1
fi
readonly host_port

readonly test_database_url="postgres://${database_user}:${database_password}@127.0.0.1:${host_port}/${database_name}?sslmode=disable"

readonly host_readiness_deadline=$((SECONDS + 30))
until psql "${test_database_url}" -X -qAtc "SELECT 1" >/dev/null 2>&1; do
  if ((SECONDS >= host_readiness_deadline)); then
    echo "error: PostgreSQL did not accept host connections within 30 seconds" >&2
    exit 1
  fi
  sleep 1
done

psql \
  "${test_database_url}" \
  -X \
  -v ON_ERROR_STOP=1 \
  -f "${server_dir}/sql/schema.sql"

(
  cd "${server_dir}"
  TEST_DATABASE_URL="${test_database_url}" \
    go test -count=1 -tags=integration ./...
)
