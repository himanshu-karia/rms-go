# go-kusumc Extra Features (Canonical)

This file lists implemented `go-kusumc` capabilities that are not fully consumed by current firmware docs or active frontend flows.

| Feature | Primary sources | API/Runtime surface | UI consumption | Firmware-doc consumption | Classification |
|---|---|---|---|---|---|
| Dead-letter replay worker | `go-kusumc/internal/core/workers/deadletter_replay_worker.go`, `go-kusumc/internal/adapters/secondary/redis_store.go` | Worker runtime + queue `ingest:deadletter` | Partial (ops-only) | No | Platform-internal reliability |
| Dead-letter diagnostics endpoints | `go-kusumc/internal/adapters/primary/http/diagnostics_controller.go`, `go-kusumc/cmd/server/main.go` | `GET /api/diagnostics/ingest/deadletter`, `POST /api/diagnostics/ingest/deadletter/replay` | Partial (ops/admin paths) | No | Ops endpoint surface |
| Ingest overflow dead-letter write path | `go-kusumc/internal/core/services/ingestion_service.go` | Overflow handling + dead-letter counters/queue writes | No direct | No | Platform safety net |
| Topic profile gating | `go-kusumc/internal/adapters/primary/mqtt_handler.go`, `go-kusumc/internal/core/services/device_service.go` | `MQTT_TOPIC_PROFILE`, `MQTT_COMPAT_TOPICS_ENABLED` | N/A | Partial (documented in compact/compat track) | Migration-control |
| Live telemetry ticket persistence/validation | `go-kusumc/internal/adapters/primary/http/telemetry_live_controller.go`, `go-kusumc/cmd/server/main.go` | `POST /api/telemetry/devices/:device_uuid/live-token` + Redis ticket keys | Partial (live telemetry consumers) | No | Security/session hardening |
| Command retry worker | `go-kusumc/internal/core/workers/command_retry_worker.go`, `go-kusumc/cmd/server/main.go` | Background retry/timeouts for pending commands | Indirect only | No | Platform-internal orchestration |
| Archiver + restore path | `go-kusumc/internal/core/services/archiver_service.go`, `go-kusumc/internal/core/services/analytics_service.go`, `go-kusumc/cmd/server/main.go` | Daily archive job + restore-backed analytics windows | Indirect/partial | No | Data lifecycle capability |

## Related comparison workspace
- `../../legacy-node-vs-fresh-go/04-go-extra-features-catalog.md`
