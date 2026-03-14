#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="${SCRIPT_DIR}/.."
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"
TEST_NAME="${TEST_NAME:-TestDeviceLifecycle}"
SKIP_COMPOSE="false"
KEEP_UP="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-compose)
      SKIP_COMPOSE="true"
      shift
      ;;
    --keep-up)
      KEEP_UP="true"
      shift
      ;;
    --test)
      TEST_NAME="${2:-}"
      if [[ -z "${TEST_NAME}" ]]; then
        echo "[run-integration] --test requires a value"
        exit 1
      fi
      shift 2
      ;;
    --compose-file)
      COMPOSE_FILE="${2:-}"
      if [[ -z "${COMPOSE_FILE}" ]]; then
        echo "[run-integration] --compose-file requires a value"
        exit 1
      fi
      shift 2
      ;;
    *)
      echo "[run-integration] unknown option: $1"
      echo "usage: ./scripts/run-integration.sh [--skip-compose] [--keep-up] [--test <regex>] [--compose-file <file>]"
      exit 1
      ;;
  esac
done

cd "${ROOT_DIR}"

compose_down() {
  if [[ "${KEEP_UP}" == "true" || "${SKIP_COMPOSE}" == "true" ]]; then
    return
  fi
  echo "[run-integration] docker compose -f ${COMPOSE_FILE} down"
  docker compose -f "${COMPOSE_FILE}" down
}

if [[ "${SKIP_COMPOSE}" != "true" ]]; then
  echo "[run-integration] docker compose -f ${COMPOSE_FILE} up -d --build --remove-orphans redis timescaledb emqx emqx-bootstrapper server nginx"
  docker compose -f "${COMPOSE_FILE}" up -d --build --remove-orphans redis timescaledb emqx emqx-bootstrapper server nginx
fi

trap compose_down EXIT

echo "[run-integration] waiting for timescaledb readiness"
for _ in $(seq 1 30); do
  if docker compose -f "${COMPOSE_FILE}" exec -T timescaledb pg_isready -U postgres -d telemetry >/dev/null 2>&1; then
    echo "[run-integration] timescaledb ready"
    break
  fi
  sleep 2
done

echo "[run-integration] waiting for emqx-bootstrapper one-shot completion"
for _ in $(seq 1 90); do
  id="$(docker compose -f "${COMPOSE_FILE}" ps -a -q emqx-bootstrapper 2>/dev/null | head -n 1 || true)"
  if [[ -n "${id}" ]]; then
    status="$(docker inspect -f '{{.State.Status}}' "${id}" 2>/dev/null || true)"
    if [[ "${status}" == "exited" ]]; then
      code="$(docker inspect -f '{{.State.ExitCode}}' "${id}" 2>/dev/null || true)"
      if [[ "${code}" == "0" ]]; then
        echo "[run-integration] emqx-bootstrapper exited(0): expected for one-shot provisioning"
        break
      fi
      echo "[run-integration] emqx-bootstrapper failed with exit code ${code}"
      docker compose -f "${COMPOSE_FILE}" logs --tail 120 emqx-bootstrapper || true
      exit 1
    fi
  fi
  sleep 2
done

echo "[run-integration] running integration tests: ${TEST_NAME}"
if docker compose -f "${COMPOSE_FILE}" config --services | grep -qx "test-runner"; then
  docker compose -f "${COMPOSE_FILE}" run --rm test-runner go test -tags=integration ./tests/e2e -run "${TEST_NAME}" -count=1
elif docker compose -f "${COMPOSE_FILE}" exec -T ingestion-go sh -lc "command -v go >/dev/null 2>&1"; then
  docker compose -f "${COMPOSE_FILE}" exec -T ingestion-go go test -tags=integration ./tests/e2e -run "${TEST_NAME}" -count=1
else
  echo "[run-integration] go toolchain not available in ingestion-go; running integration tests on host"
  compose_name="$(basename "${COMPOSE_FILE}")"
  if [[ -z "${BASE_URL:-}" ]]; then
    if [[ "${compose_name}" == "docker-compose.integration.yml" ]]; then
      export BASE_URL="http://localhost:8081"
    else
      export BASE_URL="https://rms-iot.local:7443"
    fi
  fi
  export BOOTSTRAP_URL="${BOOTSTRAP_URL:-${BASE_URL%/}/api/bootstrap}"
  export HTTP_TLS_INSECURE="${HTTP_TLS_INSECURE:-true}"
  export MQTT_TLS_INSECURE="${MQTT_TLS_INSECURE:-true}"
  export MQTT_BROKER="${MQTT_BROKER:-mqtts://rms-iot.local:18883}"
  export TIMESCALE_URI="${TIMESCALE_URI:-postgres://postgres:password@localhost:5433/telemetry?sslmode=disable}"

  docker compose -f "${COMPOSE_FILE}" exec -T timescaledb psql -U postgres -d telemetry <<'SQL'
insert into projects (id, name, config)
values ('test-project','Test Project','{}')
on conflict (id) do nothing;
insert into command_catalog (name, scope, project_id, payload_schema, transport)
select 'E2E_Set','project','test-project','{}'::jsonb,'mqtt'
where not exists (
  select 1 from command_catalog where project_id='test-project' and name='E2E_Set'
);
SQL

  go test -tags=integration ./tests/e2e -run "${TEST_NAME}" -count=1
fi

echo "[run-integration] done"
