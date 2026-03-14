#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="${SCRIPT_DIR}/.."
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"

cd "${ROOT_DIR}"

echo "[down-lorawan] docker compose -f ${COMPOSE_FILE} --profile lorawan down --remove-orphans"
docker compose -f "${COMPOSE_FILE}" --profile lorawan down --remove-orphans
