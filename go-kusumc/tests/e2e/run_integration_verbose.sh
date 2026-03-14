#!/usr/bin/env bash
set -euo pipefail

export PATH="/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

# Tools needed by the test runner script.
apt-get update -qq
apt-get install -y -qq netcat-openbsd postgresql-client >/dev/null

until pg_isready -h timescaledb -p 5432 -U postgres >/dev/null 2>&1; do
  echo "waiting_for_db"
  sleep 2
done

psql "postgres://postgres:password@timescaledb:5432/telemetry?sslmode=disable" \
  -c "insert into projects (id, name, config) values ('test-project','Test Project','{}') on conflict (id) do nothing;" \
  >/dev/null

psql "postgres://postgres:password@timescaledb:5432/telemetry?sslmode=disable" \
  -c "insert into command_catalog (name, scope, project_id, payload_schema, transport)
      select 'E2E_Set','project','test-project','{}'::jsonb,'mqtt'
      where not exists (select 1 from command_catalog where project_id='test-project' and name='E2E_Set');" \
  >/dev/null

until getent hosts server >/dev/null 2>&1 && nc -z server 8081; do
  echo "waiting_for_server"
  sleep 2
done

go test -tags=integration ./tests/e2e -count=1 -v
