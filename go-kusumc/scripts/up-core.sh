#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="${SCRIPT_DIR}/.."
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"
CLEAN="${CLEAN:-0}"
BUILD="${BUILD:-0}"
WAIT_SECONDS="${WAIT_SECONDS:-180}"

cd "${ROOT_DIR}"

compose() {
	echo "[up-core] docker compose -f ${COMPOSE_FILE} $*"
	docker compose -f "${COMPOSE_FILE}" "$@"
}

wait_service() {
	local service="$1"
	local deadline=$((SECONDS + WAIT_SECONDS))
	while (( SECONDS < deadline )); do
		local id status
		id="$(docker compose -f "${COMPOSE_FILE}" ps -q "${service}" 2>/dev/null | head -n 1 || true)"
		if [[ -n "${id}" ]]; then
			status="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "${id}" 2>/dev/null || true)"
			if [[ "${status}" == "healthy" || "${status}" == "running" ]]; then
				echo "[up-core] [ok] ${service} is ${status}"
				return 0
			fi
		fi
		sleep 2
	done
	echo "[up-core] [error] ${service} did not become healthy/running within ${WAIT_SECONDS}s" >&2
	return 1
}

if [[ "${CLEAN}" == "1" || "${CLEAN}" == "true" ]]; then
	compose down --volumes --remove-orphans
fi

if [[ "${BUILD}" == "1" || "${BUILD}" == "true" ]]; then
	compose build
fi

# Stage 1: Core infra first
compose up -d --remove-orphans redis timescaledb emqx
wait_service redis
wait_service timescaledb
wait_service emqx

# Stage 2: DB migrations + one-shot EMQX bootstrap
compose up -d --remove-orphans db-migrations
echo "[up-core] starting emqx-bootstrapper one-shot service"
compose up -d --remove-orphans emqx-bootstrapper

deadline=$((SECONDS + WAIT_SECONDS))
while (( SECONDS < deadline )); do
	id="$(docker compose -f "${COMPOSE_FILE}" ps -a -q emqx-bootstrapper 2>/dev/null | head -n 1 || true)"
	if [[ -n "${id}" ]]; then
		state="$(docker inspect -f '{{.State.Status}}' "${id}" 2>/dev/null || true)"
		if [[ "${state}" == "exited" ]]; then
			exit_code="$(docker inspect -f '{{.State.ExitCode}}' "${id}" 2>/dev/null || true)"
			if [[ "${exit_code}" == "0" ]]; then
				echo "[up-core] [ok] emqx-bootstrapper exited(0): expected for one-shot provisioning"
				break
			fi
			echo "[up-core] [error] emqx-bootstrapper failed with exit code ${exit_code}" >&2
			compose logs --tail 120 emqx-bootstrapper || true
			exit 1
		fi
	fi
	sleep 2
done

if (( SECONDS >= deadline )); then
	echo "[up-core] [error] emqx-bootstrapper did not finish within ${WAIT_SECONDS}s" >&2
	compose logs --tail 120 emqx-bootstrapper || true
	exit 1
fi

# Stage 3: API + edge services
compose up -d --remove-orphans ingestion-go nginx prometheus
wait_service ingestion-go
wait_service nginx

echo "[up-core] done: staged startup complete"
