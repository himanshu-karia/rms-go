# Deterministic Codegen for Unified IoT

This proposes a schema-in → code/migrations-out workflow. Everything is deterministic and template-driven; no AI generation.

## Flow at a Glance
1) **Define schema/contract**: Edit a declarative spec (e.g., DNA YAML/JSON or OpenAPI/GraphQL-like DSL) describing entities, fields, relations, indexes, auth scopes, and API shapes (CRUD, filters, pagination).
2) **Run CLI**: `go run ./cmd/codegen --schema ./dna/schema.yaml --out ./generated`
   - Emits DB migrations (up/down) for Postgres.
   - Emits Go types, validators, repos, services, controllers/routes, and wire-up hooks.
   - Emits OpenAPI + TS SDK/types for the frontend, and client hooks.
   - Optionally seeds/fixtures and sample requests.
3) **Integrate/build**: Generated code lives in `generated/`; main wiring registers routes (auto-wire or small mount). Frontend consumes the generated SDK/types.
4) **Publish/guardrails**: CI checks artifacts are current; preview/diff before applying migrations; feature-flag per project; reversible migrations.

## References / Prior Art (Deterministic)
- Prisma / Ent / Hasura codegen for DB+API.
- OpenAPI-driven server/client generation (oapi-codegen, openapi-generator).
- GraphQL codegen (schema → resolvers/types).
- tRPC / Buf (Protobuf) typed stubs.

## User Responsibilities When Enabled
- Author/update the schema spec.
- Run the codegen CLI (or let CI run it).
- Apply migrations (approved/reversible).
- Redeploy backend/frontend with generated artifacts.

## Detailed Plan
- **Schema model**: Define a DNA extension for entities, fields (type, nullability, defaults), relations, indexes, auth scopes, validation rules, and API surface (CRUD, filters, pagination).
- **CLI stages**:
  - Parse/validate schema (lint: reserved names, breaking changes, auth scope coverage).
  - Generate SQL migrations (up/down) with idempotent guards and comments.
  - Generate Go domain structs, validators, repos, services, controllers, route registration stubs.
  - Generate OpenAPI spec + TS SDK/types; emit React hooks/client helpers.
  - Emit seed/fixture and example HTTP requests.
- **Safety rails**:
  - Diff/preview mode for migrations and code.
  - Compatibility checks for destructive changes; require overrides or backfill steps.
  - Feature flags per project; allow staging-only publish.
  - CI gate to enforce regenerated artifacts are committed.
- **Runtime wiring**:
  - Keep generated code in `generated/`; expose a single mount in `cmd/server/main.go` to register new routes.
  - Config sync/DNA readers can optionally surface generated entities if needed for UI navigation.
- **Rollout path**:
  - Phase 1: JSONB-backed dynamic collections (CRUD + validation, no schema DDL).
  - Phase 2: Full DDL + codegen with migrations and auth scopes.
  - Phase 3: UI builder integration (form/table auto-render from schema), SDK auto-bump, and migration previews in the UI.
