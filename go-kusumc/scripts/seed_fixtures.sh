#!/usr/bin/env bash
set -euo pipefail
# Run from repo root or this script will cd into the script's parent
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="${SCRIPT_DIR}/.."
SQL_FILE="${ROOT_DIR}/test-fixtures/seed_commands.sql"

if [ ! -f "${SQL_FILE}" ]; then
  echo "Seed SQL not found: ${SQL_FILE}" >&2
  exit 1
fi

echo "Applying seed fixtures to timescaledb (database: telemetry) using docker compose"
cd "${ROOT_DIR}"

docker compose exec -T timescaledb psql -U postgres -d telemetry < "${SQL_FILE}"

echo "Seeding completed."
