# Secrets Scripts (Google Secret Manager)

These scripts let you clean secrets from repo files while keeping local development runnable.

## What this solves
- You can remove committed secret values from `.env*` and compose defaults.
- Developers still run locally using local-only env files.
- Deployment/runtime can load secrets from Google Secret Manager (GSM).

## Prerequisites
1. `gcloud` CLI installed.
2. Logged in: `gcloud auth login`
3. Project selected or passed explicitly.
4. Access roles (minimum):
- `roles/secretmanager.admin` for publishing
- `roles/secretmanager.secretAccessor` for rendering runtime env

## Naming convention
Secrets are stored as:
- `<prefix><env_var_lowercase_with_dashes>`
- Example: `AUTH_SECRET` with prefix `rms-go-` => `rms-go-auth-secret`

## Publish secrets to GSM
Script: `publish-gsm-secrets.ps1`

Example (required keys only):
```powershell
cd rms-go\go-kusumc
.\scripts\secrets\publish-gsm-secrets.ps1 -ProjectId my-gcp-project
```

Example including optional keys:
```powershell
.\scripts\secrets\publish-gsm-secrets.ps1 -ProjectId my-gcp-project -IncludeOptional
```

Dry run (no write):
```powershell
.\scripts\secrets\publish-gsm-secrets.ps1 -ProjectId my-gcp-project -DryRun
```

Default source env file is `go-kusumc/.env.local`.
Override with `-EnvFile` when needed.

## Render runtime env file from GSM
Script: `render-env-from-gsm.ps1`

Example:
```powershell
cd rms-go\go-kusumc
.\scripts\secrets\render-env-from-gsm.ps1 -ProjectId my-gcp-project -IncludeOptional -OutputFile .env.runtime
```

Stdout mode:
```powershell
.\scripts\secrets\render-env-from-gsm.ps1 -ProjectId my-gcp-project -StdoutOnly
```

## Suggested local flow
1. Keep `.env.local` untracked for local development.
2. Use `.env.example` as template for key names only.
3. Publish local secrets to GSM when preparing shared deployment.
4. Render `.env.runtime` from GSM on deployment machine.

## Suggested deployment flow
1. CI/CD identity reads GSM secrets.
2. Render env file or inject env vars into runtime.
3. Start stack (`docker compose up -d`).
4. Never print secret values in logs.

## Bootstrap new GCP project and IAM baseline
Script: `bootstrap-gcp-rms-go.ps1`

Dry run first:
```powershell
cd rms-go\go-kusumc
.\scripts\secrets\bootstrap-gcp-rms-go.ps1 -ProjectId rms-go-prod -BillingAccountId <BILLING_ACCOUNT_ID> -All -DryRun
```

Apply:
```powershell
.\scripts\secrets\bootstrap-gcp-rms-go.ps1 -ProjectId rms-go-prod -BillingAccountId <BILLING_ACCOUNT_ID> -All
```

This script writes a timestamped command log file for traceability and repeatability.

## Check certificate validity (local files)
Script: `check-certificate-status.ps1`

```powershell
cd rms-go\go-kusumc
.\scripts\secrets\check-certificate-status.ps1 -WarnDays 45
```

Use this before deployment to decide whether existing wildcard cert material can be reused.

## Important safety notes
- Never commit `.env.local` or `.env.runtime`.
- Never commit private key files under cert folders.
- Rotate any secrets previously committed to git history.
