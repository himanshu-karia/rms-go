#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="${SCRIPT_DIR}/.."
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"

cd "${ROOT_DIR}"

# Bring up core stack with staged sequencing first.
if [[ "${BUILD:-0}" == "1" || "${BUILD:-0}" == "true" ]]; then
	BUILD=1 COMPOSE_FILE="${COMPOSE_FILE}" "${SCRIPT_DIR}/up-core.sh"
else
	COMPOSE_FILE="${COMPOSE_FILE}" "${SCRIPT_DIR}/up-core.sh"
fi

echo "[up-lorawan] docker compose -f ${COMPOSE_FILE} --profile lorawan up -d --remove-orphans chirpstack-postgres chirpstack-redis chirpstack chirpstack-gateway-bridge"
docker compose -f "${COMPOSE_FILE}" --profile lorawan up -d --remove-orphans chirpstack-postgres chirpstack-redis chirpstack chirpstack-gateway-bridge
