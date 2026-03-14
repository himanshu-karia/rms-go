# Security Review Report - RMS-GO

Date: 2026-03-14  
Scope: `rms-go` only (`go-kusumc`, `ui-kusumc`, `Docs`, app docs touchpoints)

## 1) Current state summary

### What is good already
- Backend route protection chain exists (`ApiKeyMiddleware -> AuthMiddleware -> AuditMiddleware`) for protected API group.
- Capability-based checks are used on sensitive routes.
- Timescale setup includes hypertable + compression policy.
- Automated tests exist in both backend and frontend.

### High-risk findings (must fix before internet-facing production)
1. Private keys are present in repo paths under nginx cert folders.
2. Runtime defaults include weak placeholder secrets in compose/env files.
3. No visible CI security gate set for `rms-go` (secret scan, vuln scan, policy checks).
4. API compatibility aliases are extensive; this helps migration but increases long-term drift risk.

## 2) Comparison requested by you

## A) Keep security aside for now, run live quickly, debug runtime bugs, keep simple

### Benefits
- Fastest path to visible runtime progress.
- Lowest cognitive load for current team.
- Best for finding real integration bugs early.

### Risks
- Secret/key hygiene can accidentally leak into deployment.
- Quick fixes may create debt that becomes expensive later.
- Harder to retro-fit deployment automation cleanly if environment drift grows.

### Missing if you choose A
- No formal security gate before release.
- No deploy-ready secrets discipline.
- No repeatable automation baseline for CI/CD.
- Weak rollback/audit trail if changes are ad-hoc.

### Good fit when
- Strictly controlled local/dev/test environment.
- No public exposure yet.
- You accept rework cost later.

## B) Tighten security now + learn deployment + build automation during development

### Benefits
- Lower production risk from day one.
- Cleaner release process and fewer “works on my machine” failures.
- Easier team onboarding and future scaling.

### Risks
- Slower immediate feature velocity.
- Requires some upfront process/tooling effort.

### Good fit when
- You plan external exposure soon.
- Multiple contributors are active.
- You want repeatable deployments and confidence.

## Recommended path: Hybrid (A-fast with B-baseline)

Do not choose pure A or pure B. Use this practical mix:
- Keep runtime/debug speed from A.
- Enforce a minimal non-negotiable security+automation baseline from B.

### Non-negotiable baseline (start now)
1. Remove/rotate all key material from repo paths.
2. Block placeholder secrets in non-dev startup.
3. Add one local automation script that runs tests + smoke checks consistently.
4. Define one deployment runbook for repeatability.

This gives fast progress without unsafe chaos.

## 3) Your git concern: many repos/docs/artifacts outside `rms-go`

You want only clean sub-repo `rms-go` on your git with automations.

## Safest approach (recommended)
Create a standalone repo from `rms-go` only, separate from parent root workflow.

### Why this is safest
- Parent folder clutter cannot pollute `rms-go` git history.
- Build artifacts outside `rms-go` are naturally ignored.
- CI automations can target only `rms-go` paths.

### Practical setup
1. Copy `rms-go` to a separate workspace path (example: `C:\Repos\rms-go`).
2. Initialize git there only.
3. Add a strict `.gitignore` for logs, env files, certs, binaries, and build outputs.
4. Connect this repo to your remote and use it as the canonical VCS source.

## If you keep `rms-go` inside current parent folder
It can still work, but be careful:
- Never run git commands from parent root for `rms-go` changes.
- Always run git from inside `rms-go` path.
- Nested repo behavior can confuse tooling and humans if parent repo also tracks the folder.

### Direct answer to your question
- Yes. If you never use git in the outer repo, you can use git only inside `rms-go`.
- Yes. Even if outer git is used later, you can keep `rms-go` out of outer tracking by adding `/rms-go/` in outer `.gitignore`.
- Important caveat: if outer repo already tracked `rms-go`, `.gitignore` alone is not enough. You must untrack it once from outer git index (`git rm -r --cached rms-go`) and commit that in the outer repo.

### Recommended interim model (before cut-out)
- Keep `rms-go` physically inside the parent workspace for reference reuse.
- Keep independent git history only in `rms-go`.
- Keep outer repo from tracking `rms-go` at all.
- After stabilization, move to `C:\Repos\rms-go` with the same git history.

## 4) “No git, local PC only for v1” option

Yes, possible. But use disciplined local automation so it does not become fragile.

### Minimum local-only controls
1. Daily snapshot script (zip source + docs + schema).
2. Standard run script for backend/frontend/tests/smoke checks.
3. Changelog file updated per run.
4. Secret files excluded from snapshots.

### Trade-off
- Pro: simplest operationally today.
- Con: weak rollback, weak collaboration, weak auditability.

If this is truly temporary for v1, keep it short-lived and migrate to clean git repo early.

## 5) Decision matrix

- Need fastest debugging only (internal/local): choose Hybrid leaning A.
- Need stable repeatable release in near term: choose Hybrid leaning B.
- Need long-term maintainability: choose B baseline immediately.

## 6) 14-day practical plan (recommended)

1. Day 1-2
- Create clean standalone `rms-go` repo.
- Add `.gitignore` and secret exclusions.
- Remove tracked key material and rotate secrets.

2. Day 3-5
- Add one command for backend checks (`go test`, integration smoke).
- Add one command for frontend checks (`npm test`, build).
- Document exact local deployment workflow.

3. Day 6-10
- Run live stack repeatedly, capture runtime bugs, fix high-impact issues.
- Keep compatibility routes, but mark deprecation owners/timeline.

4. Day 11-14
- Add lightweight automation entrypoint for repeat checks.
- Freeze a release checklist for v1 readiness.

## 6A) What to implement now from A+B hybrid (before moving out)

### A-speed items (runtime-first)
1. Keep local dev fast loop:
- `docker compose up --build` for backend stack.
- frontend `npm run dev` against same `/api` proxy.
2. Run story/integration checks daily and log failures as runtime bugs.
3. Maintain compatibility routes until runtime stability is achieved.

### B-baseline items (must not defer)
1. Secrets and key hygiene:
- remove repo-stored private keys and rotate.
- use `.env.local` for dev secrets, never commit secrets.
- publish deployment secrets to Google Secret Manager using `scripts/secrets/publish-gsm-secrets.ps1`.
- render runtime env from Google Secret Manager using `scripts/secrets/render-env-from-gsm.ps1`.
2. Repeatable automation:
- one script for backend checks (unit + integration smoke).
- one script for frontend checks (test + build).
- one combined script to run all checks with pass/fail exit code.
3. Deployment/runbook baseline:
- single operator runbook for start/stop/recover/rollback.
- explicit env variable contract (required vs optional).
4. Pre-cutover packaging:
- clean `.gitignore` in `rms-go`.
- remove logs/build artifacts/certs from git tracking.
- validate fresh clone can run all scripts on a new machine.

### Exit criteria before cut-out to `C:\Repos\rms-go`
1. 7 consecutive days of stable runtime smoke checks.
2. No critical secrets in repo scan.
3. Single-command automated checks pass locally.
4. Runbook tested by someone other than primary developer.

## 7) Final recommendation to you

Given your priorities, use a clean standalone `rms-go` repo plus Hybrid execution:
- Fast runtime debugging now.
- Minimal security/deploy baseline now.
- Full hardening incrementally.

This avoids getting blocked, while preventing hidden security/deployment debt from exploding later.

## 8) Traceable execution reference

Detailed GCP + GitHub + secret-management execution steps and observed machine-state trace are documented in:
- `Docs/gcp-github-deployment-runbook-2026-03-15.md`
