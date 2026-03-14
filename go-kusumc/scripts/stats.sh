#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="${SCRIPT_DIR}/.."
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"

cd "${ROOT_DIR}"

CONTAINER_IDS="$(docker compose -f "${COMPOSE_FILE}" ps -q)"
if [ -z "${CONTAINER_IDS}" ]; then
  echo "[stats] no running containers for compose file: ${COMPOSE_FILE}"
  exit 0
fi

echo "[stats] docker stats --no-stream"
docker stats --no-stream ${CONTAINER_IDS}
