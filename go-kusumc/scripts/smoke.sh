#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="${SCRIPT_DIR}/.."
BASE_URL="${BASE_URL:-https://rms-iot.local:7443}"
PROJECT_ID="${PROJECT_ID:-smoke_proj_$(date +%s)}"
PASSWORD="${PASSWORD:-SmokePassword123!}"
ROLE="${ROLE:-manager}"
WAIT_SECONDS="${WAIT_SECONDS:-120}"
SKIP_COMPOSE_UP="false"
SKIP_COMPOSE_DOWN="false"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"

EMAIL="${EMAIL:-smoke_manager_$(date +%s)@test.com}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --base-url)
      BASE_URL="${2:-}"; shift 2 ;;
    --project-id)
      PROJECT_ID="${2:-}"; shift 2 ;;
    --email)
      EMAIL="${2:-}"; shift 2 ;;
    --password)
      PASSWORD="${2:-}"; shift 2 ;;
    --role)
      ROLE="${2:-}"; shift 2 ;;
    --wait-seconds)
      WAIT_SECONDS="${2:-}"; shift 2 ;;
    --skip-compose-up)
      SKIP_COMPOSE_UP="true"; shift ;;
    --skip-compose-down)
      SKIP_COMPOSE_DOWN="true"; shift ;;
    --compose-file)
      COMPOSE_FILE="${2:-}"; shift 2 ;;
    *)
      echo "[smoke] unknown option: $1"
      echo "usage: ./scripts/smoke.sh [--base-url <url>] [--project-id <id>] [--email <email>] [--password <pwd>] [--role <role>] [--wait-seconds <sec>] [--skip-compose-up] [--skip-compose-down] [--compose-file <file>]"
      exit 1 ;;
  esac
done

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "[smoke] required command not found: $1"
    exit 1
  }
}

require_cmd curl
require_cmd docker

cd "${ROOT_DIR}"

if [[ "${SKIP_COMPOSE_UP}" != "true" ]]; then
  echo "[smoke] docker compose -f ${COMPOSE_FILE} up -d --build nginx ingestion-go timescaledb redis emqx db-migrations"
  docker compose -f "${COMPOSE_FILE}" up -d --build nginx ingestion-go timescaledb redis emqx db-migrations
fi

cleanup() {
  if [[ "${SKIP_COMPOSE_DOWN}" == "true" ]]; then
    return
  fi
  echo "[smoke] docker compose -f ${COMPOSE_FILE} down"
  docker compose -f "${COMPOSE_FILE}" down || true
}
trap cleanup EXIT

echo "[smoke] waiting for API readiness at ${BASE_URL}"
end_ts=$(( $(date +%s) + WAIT_SECONDS ))
while [[ $(date +%s) -lt ${end_ts} ]]; do
  code=$(curl -sk -o /dev/null -w '%{http_code}' -X POST "${BASE_URL}/api/auth/login" -H 'Content-Type: application/json' -d '{}') || true
  if [[ "${code}" == "200" || "${code}" == "400" || "${code}" == "401" ]]; then
    break
  fi
  sleep 2
done

if [[ $(date +%s) -ge ${end_ts} ]]; then
  echo "[smoke] API not ready after ${WAIT_SECONDS}s"
  exit 1
fi

echo "[smoke] register/login (${ROLE})"
reg_code=$(curl -sk -o /tmp/smoke_register.json -w '%{http_code}' -X POST "${BASE_URL}/api/auth/register" \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"${EMAIL}\",\"password\":\"${PASSWORD}\",\"role\":\"${ROLE}\"}")
if [[ "${reg_code}" != "200" && "${reg_code}" != "201" && "${reg_code}" != "409" ]]; then
  if [[ "${reg_code}" != "500" ]] || ! grep -qi 'users_username_key' /tmp/smoke_register.json; then
    echo "[smoke] register failed: HTTP ${reg_code}"
    cat /tmp/smoke_register.json
    exit 1
  fi
fi

login_code=$(curl -sk -o /tmp/smoke_login.json -w '%{http_code}' -X POST "${BASE_URL}/api/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}")
if [[ "${login_code}" != "200" ]]; then
  echo "[smoke] login failed: HTTP ${login_code}"
  cat /tmp/smoke_login.json
  exit 1
fi

TOKEN=$(sed -n 's/.*"token"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' /tmp/smoke_login.json | head -n1)
if [[ -z "${TOKEN}" ]]; then
  echo "[smoke] login succeeded but no token found"
  cat /tmp/smoke_login.json
  exit 1
fi

IMEI="SMOKE_$(date +%s)"
MSGID="smoke_$(date +%s)_$RANDOM"
TS_MS=$(( $(date +%s) * 1000 ))

echo "[smoke] create project ${PROJECT_ID}"
project_code=$(curl -sk -o /tmp/smoke_project.json -w '%{http_code}' -X POST "${BASE_URL}/api/projects" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"id\":\"${PROJECT_ID}\",\"name\":\"Smoke Project\",\"config\":{\"generated\":true}}")
if [[ "${project_code}" != "200" && "${project_code}" != "201" ]]; then
  echo "[smoke] project create failed: HTTP ${project_code}"
  cat /tmp/smoke_project.json
  exit 1
fi

echo "[smoke] create device IMEI=${IMEI}"
device_code=$(curl -sk -o /tmp/smoke_device.json -w '%{http_code}' -X POST "${BASE_URL}/api/devices" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"Smoke Device\",\"imei\":\"${IMEI}\",\"projectId\":\"${PROJECT_ID}\",\"attributes\":{}}")
if [[ "${device_code}" != "200" ]]; then
  echo "[smoke] device create failed: HTTP ${device_code}"
  cat /tmp/smoke_device.json
  exit 1
fi

DEVICE_ID=$(sed -n 's/.*"device_id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' /tmp/smoke_device.json | head -n1)
if [[ -z "${DEVICE_ID}" ]]; then
  echo "[smoke] device create succeeded but device_id missing"
  cat /tmp/smoke_device.json
  exit 1
fi

echo "[smoke] ingest telemetry"
ingest_code=$(curl -sk -o /tmp/smoke_ingest.json -w '%{http_code}' -X POST "${BASE_URL}/api/ingest" \
  -H 'Content-Type: application/json' \
  -d "{\"imei\":\"${IMEI}\",\"device_id\":\"${DEVICE_ID}\",\"project_id\":\"${PROJECT_ID}\",\"packet_type\":\"telemetry\",\"msgid\":\"${MSGID}\",\"ts\":${TS_MS},\"temp\":99.9,\"smoke\":true}")
if [[ "${ingest_code}" != "200" ]]; then
  echo "[smoke] ingest failed: HTTP ${ingest_code}"
  cat /tmp/smoke_ingest.json
  exit 1
fi

echo "[smoke] verify telemetry history"
found="false"
for _ in $(seq 1 15); do
  history_code=$(curl -sk -o /tmp/smoke_history.json -w '%{http_code}' "${BASE_URL}/api/telemetry/history?imei=${IMEI}" \
    -H "Authorization: Bearer ${TOKEN}")
  if [[ "${history_code}" == "200" ]] && grep -q '99.9' /tmp/smoke_history.json; then
    found="true"
    break
  fi
  sleep 2
done

if [[ "${found}" != "true" ]]; then
  echo "[smoke] telemetry history missing expected sentinel value"
  cat /tmp/smoke_history.json
  exit 1
fi

echo "[smoke] ✅ PASSED"
