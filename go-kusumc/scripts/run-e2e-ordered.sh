#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="${SCRIPT_DIR}/.."
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.integration.yml}"
SKIP_COMPOSE="false"
KEEP_UP="false"

ORDERED_TESTS=(
  TestBootstrapConnectPersist
  TestLiveBootstrapTLS
  TestDeviceOpenAliasCoverage
  TestDeviceConfigurationApply
  TestDeviceLifecycle
  TestDeviceCommandLifecycle
  TestMQTTCredRotation
  TestMQTTRotationForcesDisconnect
  TestKusumFullCycle
  TestSolarRMSFullCycle
  TestStory_FullCycle
  TestUIAndDeviceOpenFullCycle
  TestRMSMegaFlow
)

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
    --compose-file)
      COMPOSE_FILE="${2:-}"
      if [[ -z "${COMPOSE_FILE}" ]]; then
        echo "[ordered-e2e] --compose-file requires a value"
        exit 1
      fi
      shift 2
      ;;
    *)
      echo "[ordered-e2e] unknown option: $1"
      echo "usage: ./scripts/run-e2e-ordered.sh [--skip-compose] [--keep-up] [--compose-file <file>]"
      exit 1
      ;;
  esac
done

cd "${ROOT_DIR}"

compose_down() {
  if [[ "${KEEP_UP}" == "true" || "${SKIP_COMPOSE}" == "true" ]]; then
    return
  fi
  echo "[ordered-e2e] docker compose -f ${COMPOSE_FILE} down"
  docker compose -f "${COMPOSE_FILE}" down
}

if [[ "${SKIP_COMPOSE}" != "true" ]]; then
  echo "[ordered-e2e] docker compose -f ${COMPOSE_FILE} up -d --build --remove-orphans redis timescaledb emqx emqx-bootstrapper server nginx"
  docker compose -f "${COMPOSE_FILE}" up -d --build --remove-orphans redis timescaledb emqx emqx-bootstrapper server nginx
fi

trap compose_down EXIT

echo "[ordered-e2e] waiting for timescaledb"
for _ in $(seq 1 40); do
  if docker compose -f "${COMPOSE_FILE}" exec -T timescaledb pg_isready -U postgres -d telemetry >/dev/null 2>&1; then
    echo "[ordered-e2e] timescaledb ready"
    break
  fi
  sleep 2
done

echo "[ordered-e2e] waiting for emqx-bootstrapper one-shot completion"
for _ in $(seq 1 90); do
  id="$(docker compose -f "${COMPOSE_FILE}" ps -a -q emqx-bootstrapper 2>/dev/null | head -n 1 || true)"
  if [[ -n "${id}" ]]; then
    status="$(docker inspect -f '{{.State.Status}}' "${id}" 2>/dev/null || true)"
    if [[ "${status}" == "exited" ]]; then
      code="$(docker inspect -f '{{.State.ExitCode}}' "${id}" 2>/dev/null || true)"
      if [[ "${code}" == "0" ]]; then
        echo "[ordered-e2e] emqx-bootstrapper exited(0): expected for one-shot provisioning"
        break
      fi
      echo "[ordered-e2e] emqx-bootstrapper failed with exit code ${code}"
      docker compose -f "${COMPOSE_FILE}" logs --tail 120 emqx-bootstrapper || true
      exit 1
    fi
  fi
  sleep 2
done

export BASE_URL="${BASE_URL:-http://localhost:8081}"
export BOOTSTRAP_URL="${BOOTSTRAP_URL:-${BASE_URL%/}/api/bootstrap}"
export BOOTSTRAP_IMEI="${BOOTSTRAP_IMEI:-999$(printf '%011d' $((RANDOM%100000000000)))}"
export HTTP_TLS_INSECURE="${HTTP_TLS_INSECURE:-true}"
export MQTT_TLS_INSECURE="${MQTT_TLS_INSECURE:-true}"
export MQTT_BROKER="${MQTT_BROKER:-mqtts://rms-iot.local:18883}"
export TIMESCALE_URI="${TIMESCALE_URI:-postgres://postgres:password@localhost:5433/telemetry?sslmode=disable}"
export PROJECT_ID="${PROJECT_ID:-test-project}"

echo "[ordered-e2e] waiting for API readiness at ${BASE_URL}/api/auth/login"
for _ in $(seq 1 60); do
  code=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "${BASE_URL}/api/auth/login" -H 'Content-Type: application/json' -d '{}' || true)
  if [[ "${code}" == "200" || "${code}" == "400" || "${code}" == "401" ]]; then
    echo "[ordered-e2e] API ready"
    break
  fi
  sleep 2
done

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

idx=0
for test_name in "${ORDERED_TESTS[@]}"; do
  idx=$((idx+1))
  echo "[ordered-e2e] [${idx}/${#ORDERED_TESTS[@]}] running ${test_name}"
  go test -tags=integration ./tests/e2e -run "^${test_name}$" -count=1 -v
done

echo "[ordered-e2e] all ordered tests passed"
