#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8081}"
API_KEY="${API_KEY:-}"
DEVICE_UUID="${DEVICE_UUID:-}"
IMEI="${IMEI:-}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --base-url) BASE_URL="${2:-}"; shift 2 ;;
    --api-key) API_KEY="${2:-}"; shift 2 ;;
    --device-uuid) DEVICE_UUID="${2:-}"; shift 2 ;;
    --imei) IMEI="${2:-}"; shift 2 ;;
    *)
      echo "[smoke-device-open-broker] unknown option: $1"
      echo "usage: ./scripts/smoke-device-open-broker.sh [--base-url <url>] [--api-key <key>] [--device-uuid <uuid>] [--imei <imei>]"
      exit 1
      ;;
  esac
done

if [[ -z "${DEVICE_UUID}" && -z "${IMEI}" ]]; then
  echo "[smoke-device-open-broker] provide --device-uuid or --imei"
  exit 1
fi

DEVICE_REF="${DEVICE_UUID}"
if [[ -z "${DEVICE_REF}" ]]; then
  DEVICE_REF="${IMEI}"
fi

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "[smoke-device-open-broker] required command not found: $1"
    exit 1
  }
}

require_cmd curl

HDRS=()
if [[ -n "${API_KEY}" ]]; then
  HDRS+=( -H "x-api-key: ${API_KEY}" )
fi

echo "[1/4] GET /api/device-open/credentials/local"
code=$(curl -sS -o /tmp/smoke_devopen_local.json -w '%{http_code}' "${BASE_URL}/api/device-open/credentials/local?imei=${IMEI}" "${HDRS[@]}")
if [[ "${code}" != "200" ]]; then
  echo "[smoke-device-open-broker] local credentials failed: HTTP ${code}"
  cat /tmp/smoke_devopen_local.json
  exit 1
fi
cat /tmp/smoke_devopen_local.json

echo "[2/4] GET /api/device-open/credentials/government"
code=$(curl -sS -o /tmp/smoke_devopen_govt.json -w '%{http_code}' "${BASE_URL}/api/device-open/credentials/government?imei=${IMEI}" "${HDRS[@]}")
if [[ "${code}" != "200" ]]; then
  echo "[smoke-device-open-broker] government credentials failed: HTTP ${code}"
  cat /tmp/smoke_devopen_govt.json
  exit 1
fi
cat /tmp/smoke_devopen_govt.json

echo "[3/4] GET /api/device-open/vfd"
code=$(curl -sS -o /tmp/smoke_devopen_vfd.json -w '%{http_code}' "${BASE_URL}/api/device-open/vfd?deviceUuid=${DEVICE_REF}" "${HDRS[@]}")
if [[ "${code}" != "200" ]]; then
  echo "[smoke-device-open-broker] vfd fetch failed: HTTP ${code}"
  cat /tmp/smoke_devopen_vfd.json
  exit 1
fi
cat /tmp/smoke_devopen_vfd.json

echo "[4/4] POST /api/broker/sync"
code=$(curl -sS -o /tmp/smoke_broker_sync.json -w '%{http_code}' -X POST "${BASE_URL}/api/broker/sync" \
  "${HDRS[@]}" \
  -H 'Content-Type: application/json' \
  -d "{\"deviceUuid\":\"${DEVICE_REF}\",\"reason\":\"smoke-check\"}")
if [[ "${code}" != "200" ]]; then
  echo "[smoke-device-open-broker] broker sync failed: HTTP ${code}"
  cat /tmp/smoke_broker_sync.json
  exit 1
fi
cat /tmp/smoke_broker_sync.json

echo "[smoke-device-open-broker] ✅ completed"
