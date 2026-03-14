# Bootstrap optimisation: what is stored vs derived

This document answers: “On provision, what all do we create/store, and what does `GET /api/bootstrap?imei=...` return?”

The guiding principle is:
- Persist *stable identity + per-device secrets*.
- Derive *environment routing* and *project-wide configuration* at runtime.

## Entities created / stored on provision

Provisioning happens via `POST /api/devices` or CSV import (`POST /api/devices/import`).

### A) Per-device (always stored)
| Item | Stored where | Keyed by | Example fields | Used by | Notes |
|---|---|---|---|---|---|
| Device identity | `devices` | `device_id` (uuid), `imei` | `id`, `imei`, `project_id`, `auth` | bootstrap, ingestion, provisioning | This is the canonical “device exists” record. |
| Unified MQTT auth (internal EMQX) | `credential_history.bundle` (latest) | `device_id` | `client_id`, `username`, `password` | EMQX provisioning worker + bootstrap | Only auth material is persisted; topics/endpoints are not stored. |
| Provisioning state | `credential_history` + `mqtt_jobs` | `device_id` | `lifecycle`, `applied`, `attempts`, `last_error` | bootstrap, ops visibility | Bootstrap exposes status in `provisioning`. |

### B) Per-device (optional stored)
| Item | Stored where | Keyed by | Example fields | Used by | Notes |
|---|---|---|---|---|---|
| Govt/vendor broker auth | govt creds storage (`GovtCredsService`) | `device_id` + `protocol_id` | `client_id`, `username`, `password` | bootstrap (as `govt_broker`) | This is *only* credentials. No endpoint/topics stored here. |

### C) Per-project (stored)
| Item | Stored where | Keyed by | Example fields | Used by | Notes |
|---|---|---|---|---|---|
| Project record | `projects` | `project_id` | `id`, `name`, `config` | UI, bootstrap context | Bootstrap returns `{project:{id,name}}` (name may be mocked if not joined). |
| Protocol profiles | `protocols` | `project_id` + `kind` | `kind=primary/govt`, `protocol`, `host`, `port`, `publish_topics`, `subscribe_topics`, `metadata` | bootstrap; (primary topics also used for EMQX ACL derivation) | Think of this as “broker profile definitions”, not per-device secrets. |
| Project DNA | `project_dna` | `project_id` | payload schema rows, rules, virtual sensors, metadata | bootstrap `dna`, envelope-required keys, ingestion strictness/transforms | DNA governs payload validation/transform/rules, not broker connectivity. |

### D) Per-device context links (optional stored)
| Item | Stored where | Keyed by | Example fields | Used by | Notes |
|---|---|---|---|---|---|
| Domain links/context | installation/beneficiary/vfd tables | `device_id` (via installation) | location, beneficiary, VFD model + assignment | bootstrap `context` | Per-device and optional. Often null initially; can be linked later and will appear in bootstrap once present. |

## Runtime-derived fields (not stored per device)

### D) Unified / internal broker routing (primary_broker)
| Field returned in bootstrap | Derivation source | Default | Notes |
|---|---|---|---|
| `primary_broker.protocol/host/port/endpoints[]` | env: `MQTT_PUBLIC_PROTOCOL/HOST/PORT` | `mqtts://<MQTT_HOST>:8883` (with dev fallbacks) | This prevents docker-internal hostnames from leaking to real devices/host-run tests. |
| `primary_broker.publish_topics[]` | project-scoped defaults OR `protocols.kind=primary` templates | `channels/{project_id}/messages/{imei}` | Topics are project-scoped; templates may use `{project_id}`/`{imei}`. |
| `primary_broker.subscribe_topics[]` | project-scoped defaults OR `protocols.kind=primary` templates | `channels/{project_id}/commands/{imei}` | Same derivation as publish topics. |
| `primary_broker.username/password/client_id` | latest `credential_history.bundle` | n/a | This is the only per-device part for primary broker. |

### E) Govt/vendor broker routing (govt_broker)
| Field returned in bootstrap | Derivation source | Default | Notes |
|---|---|---|---|
| `govt_broker.protocol/endpoints[]` | `protocols.kind=govt` profile | none | Govt broker must come from the govt profile; it must not fall back to Unified routing. |
| `govt_broker.publish_topics[]`, `subscribe_topics[]` | `protocols.kind=govt` templates | none | Templates may use `{project_id}`/`{imei}`; if a govt profile isn’t configured, `govt_broker` is omitted. |
| `govt_broker.username/password/client_id` | per-device govt creds bundle | none | Credentials are per-device; protocol profile is per-project. |

## Bootstrap response: matrix (what you see on device)

| Bootstrap section | What it contains | Source of truth |
|---|---|---|
| `identity` | `{imei, uuid, lifecycle}` | `devices` + latest `credential_history.lifecycle` |
| `primary_broker` | Unified broker connectivity + topics + per-device auth | endpoints from env; topics from project scope / primary profile; creds from `credential_history.bundle` |
| `govt_broker` (optional) | Govt/vendor broker connectivity + topics + per-device auth | endpoints/topics from govt profile; creds from govt creds storage |
| `provisioning` | status + credential_history_id + error/attempts | `credential_history` |
| `envelope.required` | base keys + DNA required keys | base list + `project_dna.rows[].envelopeRequired` + `project_dna.metadata.envelope_required` |
| `dna` (optional) | project DNA schema/config block | `project_dna` |
| `context` | project + optional location/beneficiary/vfd | project + installation/beneficiary/vfd repos (via device installation link) |

## How it works end-to-end (happy path)
1) Create device (`POST /api/devices` / bulk import) → a `devices` row exists.
2) Server generates/stores Unified MQTT auth in `credential_history.bundle` and enqueues a provisioning job.
3) Background worker provisions EMQX user + ACL using runtime-derived topics.
4) Device calls `GET /api/bootstrap?imei=...` and receives:
   - `primary_broker`: public endpoint + topics + latest creds
   - `govt_broker` only if govt profile + per-device govt creds exist
   - envelope keys + optional DNA + context links

## Bootstrap optimisation recommendations
- Keep `credential_history.bundle` “auth-only” (already done): secrets rotate cleanly without migrating routing fields.
- Keep endpoints env-derived: avoids “emqx:1883” leaks outside docker.
- Treat topics as project-scoped routing; only allow per-project overrides via protocol profiles.
- Keep govt broker config strictly profile-driven; never fall back to internal settings.
