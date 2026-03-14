#!/usr/bin/env bash
set -euo pipefail

# Purpose: Spin up infra, apply schema + seeds, and leave services running for manual command publish/ingest checks.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="${SCRIPT_DIR}/.."
SKIP_COMPOSE="false"
SKIP_DOWN="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-compose)
      SKIP_COMPOSE="true"
      shift
      ;;
    --skip-down)
      SKIP_DOWN="true"
      shift
      ;;
    --project-root)
      PROJECT_ROOT="$2"
      shift 2
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

cd "${PROJECT_ROOT}"

ensure_compose_up() {
  if [[ "${SKIP_COMPOSE}" == "true" ]]; then
    return
  fi
  echo "[info] docker compose up --build"
  docker compose up -d --build --remove-orphans >/dev/null
}

wait_timescale() {
  echo "[info] waiting for timescaledb..."
  for _ in $(seq 1 30); do
    if docker compose exec -T timescaledb pg_isready -U postgres -d telemetry >/dev/null 2>&1; then
      echo "[info] timescaledb ready"
      return
    fi
    sleep 2
  done
  echo "timescaledb not ready" >&2
  exit 1
}

apply_sql() {
  local path="$1"
  echo "[info] applying ${path}"
  cat "${path}" | docker compose exec -T timescaledb psql -U postgres -d telemetry >/dev/null
}

main() {
  ensure_compose_up
  wait_timescale
  apply_sql "${PROJECT_ROOT}/schemas/v1_init.sql"
  if [[ -f "${PROJECT_ROOT}/test-fixtures/seed_commands.sql" ]]; then
    apply_sql "${PROJECT_ROOT}/test-fixtures/seed_commands.sql"
  fi

  echo "[done] infra ready. Start server: GO_PORT=8081 go run ./cmd/server"
  echo "[tip] use mqtt-cli or mosquitto_sub to watch: channels/<project>/commands/<imei>/resp"
}

main

if [[ "${SKIP_DOWN}" != "true" ]]; then
  echo "[note] keeping stack running; pass --skip-down to leave up"
fi
