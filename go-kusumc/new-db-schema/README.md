# New DB Schema (Draft / Staging)

This folder is a **safe staging area** for redesigning the DB schema into a modular layout:

- `sql/core.sql` — tables needed by *any* project (platform fundamentals)
- `sql/verticals/*.sql` — optional domain bundles that can be composed with core

This does **not** change the currently used schema in `unified-go/schemas/`.

## Composition model

- Always apply: `sql/core.sql`
- Optionally apply one or more vertical bundles:
  - `sql/verticals/pm_kusum.sql`
  - `sql/verticals/healthcare.sql`
  - `sql/verticals/gis.sql`
  - `sql/verticals/traffic.sql`
  - `sql/verticals/logistics.sql`
  - `sql/verticals/maintenance.sql`

See `bundles.yaml` for the authoritative bundle list.

### FK boundary rule

- Vertical tables may reference core tables (vertical → core).
- Core tables must never reference vertical tables.

This keeps bundles optional and composable.

## Try it locally (manual)

Use these files by mounting them into `/docker-entrypoint-initdb.d/` on a **fresh** DB volume.
Example (order matters):

- `01_core.sql`
- `02_pm_kusum.sql`

## Docs & artifacts

- Bundle overview: `docs/bundles.md`
- Bundle manifest: `bundles.yaml`
- Handover for next teams: `docs/handover.md`
- Diagrams (Mermaid + SVG exports): `diagrams/`

See `docs/bundles.md` for details.
