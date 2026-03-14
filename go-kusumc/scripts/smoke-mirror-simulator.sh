#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8081}"
IMEI="${IMEI:-359760000000001}"
TOPIC="${TOPIC:-heartbeat}"
MIRROR_USER="${MIRROR_USER:-}"
MIRROR_PASS="${MIRROR_PASS:-}"
MIRROR_CLIENT_ID="${MIRROR_CLIENT_ID:-}"
SIMULATOR_DEVICE_UUID="${SIMULATOR_DEVICE_UUID:-}"
BEARER_TOKEN="${BEARER_TOKEN:-}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --base-url) BASE_URL="${2:-}"; shift 2 ;;
    --imei) IMEI="${2:-}"; shift 2 ;;
    --topic) TOPIC="${2:-}"; shift 2 ;;
    --mirror-user) MIRROR_USER="${2:-}"; shift 2 ;;
    --mirror-pass) MIRROR_PASS="${2:-}"; shift 2 ;;
    --mirror-client-id) MIRROR_CLIENT_ID="${2:-}"; shift 2 ;;
    --simulator-device-uuid) SIMULATOR_DEVICE_UUID="${2:-}"; shift 2 ;;
    --bearer-token) BEARER_TOKEN="${2:-}"; shift 2 ;;
    *)
      echo "[smoke-mirror-simulator] unknown option: $1"
      echo "usage: ./scripts/smoke-mirror-simulator.sh [--base-url <url>] [--imei <imei>] [--topic <topic>] [--mirror-user <u>] [--mirror-pass <p>] [--mirror-client-id <id>] [--simulator-device-uuid <uuid>] [--bearer-token <jwt>]"
      exit 1
      ;;
  esac
done

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "[smoke-mirror-simulator] required command not found: $1"
    exit 1
  }
}

require_cmd curl
require_cmd sed

echo "== Mirror Ingest =="
if [[ -z "${MIRROR_USER}" || -z "${MIRROR_PASS}" || -z "${MIRROR_CLIENT_ID}" ]]; then
  echo "[smoke-mirror-simulator] mirror credentials missing; provide --mirror-user --mirror-pass --mirror-client-id"
else
  msgid="smoke-$(date +%Y%m%d%H%M%S)"
  ts="$(date +%s)"
  code=$(curl -sS -o /tmp/smoke_mirror_ingest.json -w '%{http_code}' -X POST "${BASE_URL}/api/telemetry/mirror/${TOPIC}" \
    -u "${MIRROR_USER}:${MIRROR_PASS}" \
    -H "X-RMS-IMEI: ${IMEI}" \
    -H "X-RMS-ClientId: ${MIRROR_CLIENT_ID}" \
    -H "X-RMS-MsgId: ${msgid}" \
    -H 'Content-Type: application/json' \
    -d "{\"packet_type\":\"${TOPIC}\",\"imei\":\"${IMEI}\",\"timestamp\":${ts},\"data\":{\"ping\":\"ok\"}}")

  if [[ "${code}" != "200" && "${code}" != "201" && "${code}" != "202" ]]; then
    echo "[smoke-mirror-simulator] mirror ingest failed: HTTP ${code}"
    cat /tmp/smoke_mirror_ingest.json
  else
    echo "[smoke-mirror-simulator] mirror response:"
    cat /tmp/smoke_mirror_ingest.json
  fi
fi

echo "== Simulator Sessions =="
if [[ -z "${SIMULATOR_DEVICE_UUID}" || -z "${BEARER_TOKEN}" ]]; then
  echo "[smoke-mirror-simulator] simulator inputs missing; provide --simulator-device-uuid and --bearer-token"
  exit 0
fi

create_code=$(curl -sS -o /tmp/smoke_sim_create.json -w '%{http_code}' -X POST "${BASE_URL}/api/simulator/sessions" \
  -H "Authorization: Bearer ${BEARER_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "{\"deviceUuid\":\"${SIMULATOR_DEVICE_UUID}\",\"expiresInMinutes\":60}")
if [[ "${create_code}" != "200" && "${create_code}" != "201" ]]; then
  echo "[smoke-mirror-simulator] session create failed: HTTP ${create_code}"
  cat /tmp/smoke_sim_create.json
  exit 1
fi

session_id=$(sed -n 's/.*"id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' /tmp/smoke_sim_create.json | head -n1)
if [[ -z "${session_id}" ]]; then
  echo "[smoke-mirror-simulator] session create succeeded but id missing"
  cat /tmp/smoke_sim_create.json
  exit 1
fi

echo "[smoke-mirror-simulator] session created: ${session_id}"

list_code=$(curl -sS -o /tmp/smoke_sim_list.json -w '%{http_code}' "${BASE_URL}/api/simulator/sessions" \
  -H "Authorization: Bearer ${BEARER_TOKEN}")
if [[ "${list_code}" != "200" ]]; then
  echo "[smoke-mirror-simulator] session list failed: HTTP ${list_code}"
  cat /tmp/smoke_sim_list.json
  exit 1
fi

echo "[smoke-mirror-simulator] sessions list response:"
cat /tmp/smoke_sim_list.json

revoke_code=$(curl -sS -o /tmp/smoke_sim_revoke.json -w '%{http_code}' -X DELETE "${BASE_URL}/api/simulator/sessions/${session_id}" \
  -H "Authorization: Bearer ${BEARER_TOKEN}")
if [[ "${revoke_code}" != "200" && "${revoke_code}" != "204" ]]; then
  echo "[smoke-mirror-simulator] session revoke failed: HTTP ${revoke_code}"
  cat /tmp/smoke_sim_revoke.json
  exit 1
fi

echo "[smoke-mirror-simulator] session revoked: ${session_id}"
echo "[smoke-mirror-simulator] ✅ completed"
