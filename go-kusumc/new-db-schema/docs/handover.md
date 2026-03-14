# Handover: Modular DB schema (core + verticals)

This folder is a **staging area** for reshaping DB initialization into composable bundles.
It is intentionally isolated from `unified-go/schemas/` so we can iterate safely.

## What you get in this folder

- `sql/core.sql` — required platform tables
- `sql/verticals/*.sql` — optional vertical bundles
- `bundles.yaml` — bundle manifest (paths, dependencies, table inventory)
- `diagrams/*.mmd` and `diagrams/*.svg` — ER diagrams (core / pm_kusum / combined)

## Non-negotiable rules

### 1) FK boundary rule (composability)

- Vertical tables may reference core tables (**vertical → core**).
- Core tables must never reference vertical tables (**core → vertical** is forbidden).

Rationale: If core references a vertical table, core can no longer be installed alone.

If you need cross-vertical relationships:
- Prefer a **generic core association table** (core owns the relationship), or
- Use a soft reference (`uuid` + `type`) as a temporary bridge until the concept is promoted into core.

### 2) One database, many projects

- Do not create per-project schemas/databases.
- Use `projects` + Project DNA to represent project-specific differences.

## How to apply bundles (fresh DB only)

Postgres init scripts under `/docker-entrypoint-initdb.d/` run once per data volume.
To try this schema you must use a **fresh** Timescale/Postgres volume.

Example init order:

- `01_core.sql`  (mount `sql/core.sql`)
- `02_pm_kusum.sql` (mount `sql/verticals/pm_kusum.sql`)

Key point: **order matters**. Always load `core` first.

## Adding a new vertical bundle

Create a new file under `sql/verticals/<name>.sql`.

Checklist:
- Every table should be explicitly **scoped** (usually `project_id`) unless it is truly global master data.
- Use consistent key style (match the repo’s current patterns: `uuid` PKs, timestamps).
- All foreign keys must point to core tables only.
- Avoid triggers/functions unless absolutely required.
- Add the bundle to `bundles.yaml` (tables + `depends_on: [core]`).
- Add or update the ER diagram(s) if the bundle is important.

## What belongs in core vs vertical

Put in **core** when:
- It is required for every deployment.
- It represents platform primitives (auth/tenancy, projects, devices, telemetry, rules/alerts, provisioning).
- Multiple verticals will depend on it.

Put in a **vertical** when:
- It models domain-specific workflows/entities.
- It is only needed by a subset of deployments.

## Testing matrix (recommended)

Do these tests on a fresh DB volume each time:

1. **core only**
   - Mount only `sql/core.sql`
   - Verify app boot, basic auth, project creation, device registration, telemetry ingestion.

2. **core + pm_kusum**
   - Mount `sql/core.sql` then `sql/verticals/pm_kusum.sql`
   - Verify PM-KUSUM flows (beneficiary/installation) and VFD catalog/assignments.

3. **core + each other vertical** (optional)
   - Mount `sql/core.sql` then one vertical at a time
   - Verify the service boots and migrations apply cleanly.

4. **core + multiple verticals** (only where expected)
   - Only combine verticals if the product explicitly needs it.

## Operational notes

- Timescale-specific objects (hypertables, compression policies) should remain in core where applicable.
- If a vertical needs high-volume time series data, prefer writing into the core `telemetry` store and model the vertical-specific meaning in DNA/config.

## Where to look

- Bundle definitions: `../bundles.yaml`
- Bundle overview: `./bundles.md`
- ER diagrams: `../diagrams/`
