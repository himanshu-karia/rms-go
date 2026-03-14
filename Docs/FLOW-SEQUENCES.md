# Flow Sequences (Operational order)

## A. Bring-up sequence (integration profile)
1. Start compose stack (`docker-compose.integration.yml`).
2. Wait for DB + broker + API readiness.
3. Seed required integration fixtures.
4. Run targeted integration tests.
5. Run ordered E2E suite when validating full chain.

## B. Release readiness sequence
1. Build backend binary.
2. Run backend unit tests.
3. Run integration harness (targeted + ordered).
4. Verify docs alignment (firmware contract + status snapshot).
5. Tag report under `Docs/reports/`.

## C. Incident response sequence (ingest/backlog)
1. Check dead-letter queue length and replay counters.
2. Validate downstream dependencies (DB latency, broker health).
3. Trigger manual replay endpoint if backlog persists.
4. Tune `INGEST_DEADLETTER_REPLAY_*` settings.
5. Capture postmortem note in `Docs/reports/`.
