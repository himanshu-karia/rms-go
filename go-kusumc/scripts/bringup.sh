#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="${SCRIPT_DIR}/.."
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"

cd "${ROOT_DIR}"

echo "[bringup] docker compose -f ${COMPOSE_FILE} up -d --build --remove-orphans"
docker compose -f "${COMPOSE_FILE}" up -d --build --remove-orphans

echo "[bringup] stack is starting. Run ./scripts/status.sh for health snapshot."
