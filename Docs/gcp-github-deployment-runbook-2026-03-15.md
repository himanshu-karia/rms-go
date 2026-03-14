# GCP + GitHub Deployment Runbook (Traceable)

Date: 2026-03-15  
Scope: `rms-go` repository, GCP project foundation, secret flow, hosting boundaries, DNS/cert decision flow.

## 1) What was attempted from this machine (trace)

### Commands attempted
1. `gcloud --version`
2. `gcloud auth list`
3. `gcloud config list --format="text(core.account,core.project,core.disable_usage_reporting)"`
4. Checked expected wildcard deploy files:
   - `go-kusumc/infra/nginx/certs/hkbase.in-certs/fullchain.pem`
   - `go-kusumc/infra/nginx/certs/hkbase.in-certs/privkey.pem`
5. Checked git remote from `rms-go` root.

### Observed results
- `gcloud` command not found in PATH on current machine.
- `openssl` command not found in PATH on current machine.
- wildcard deploy files (`fullchain.pem`, `privkey.pem`) not present at expected path.
- `rms-go` is currently not initialized as a git repository on this machine yet.
- certificate status check (`check-certificate-status.ps1`) results:
   - `hkbase.in.crt` expired on `2026-02-18`.
   - `hkbase.in-certs/hkbase.in.crt` expired on `2026-02-18`.
   - local dev cert `localhost-certs/rms-local.crt` valid until `2028-06-12`.

## 2) What can be done autonomously now

### Already implemented by automation
1. Google Secret Manager scripts:
- `go-kusumc/scripts/secrets/publish-gsm-secrets.ps1`
- `go-kusumc/scripts/secrets/render-env-from-gsm.ps1`

2. GCP foundation bootstrap script:
- `go-kusumc/scripts/secrets/bootstrap-gcp-rms-go.ps1`

3. Certificate validity check script (OpenSSL not required):
- `go-kusumc/scripts/secrets/check-certificate-status.ps1`

4. Local/Git separation baseline:
- `rms-go/.gitignore`

## 3) What requires your one-time interaction

1. Install or expose `gcloud` in PATH.
2. Run `gcloud auth login` with your intended account.
3. Confirm billing account id for new GCP project.
4. Confirm whether second user email needs IAM role binding.
5. Confirm GoDaddy DNS ownership flow for domain records.

## 4) GitHub repo onboarding process (`git@github.com:himanshu-karia/rms-go.git`)

From `rms-go` folder:

```powershell
cd C:\Project-Play\Unified-IoT-Portal-18-Jan\rms-go
git init
git branch -M main
git remote add origin git@github.com:himanshu-karia/rms-go.git
git add .
git commit -m "bootstrap rms-go repo"
git push -u origin main
```

If remote already exists:

```powershell
git remote set-url origin git@github.com:himanshu-karia/rms-go.git
```

## 5) GCP project bootstrap process (repeatable)

### Step A: Dry run
```powershell
cd C:\Project-Play\Unified-IoT-Portal-18-Jan\rms-go\go-kusumc
.\scripts\secrets\bootstrap-gcp-rms-go.ps1 -ProjectId rms-go-prod -BillingAccountId <BILLING_ACCOUNT_ID> -All -DryRun
```

### Step B: Apply
```powershell
.\scripts\secrets\bootstrap-gcp-rms-go.ps1 -ProjectId rms-go-prod -BillingAccountId <BILLING_ACCOUNT_ID> -All
```

The script generates a timestamped log file in `go-kusumc/scripts/secrets/`.

## 6) Secret management process

### Publish secrets from local env file to GSM
```powershell
cd C:\Project-Play\Unified-IoT-Portal-18-Jan\rms-go\go-kusumc
.\scripts\secrets\publish-gsm-secrets.ps1 -ProjectId rms-go-prod
```

### Render runtime env file from GSM
```powershell
.\scripts\secrets\render-env-from-gsm.ps1 -ProjectId rms-go-prod -IncludeOptional -OutputFile .env.runtime
```

### Run stack
```powershell
docker compose --env-file .env.runtime up -d --build
```

## 7) Hosting boundary: what stays on PC vs what goes to Git

## Keep on Git (tracked)
- source code, scripts, docs, templates.
- `.env.example` (without real secret values).
- deployment runbooks and checklists.

## Keep only on PC / secret systems (not tracked)
- `.env.local`, `.env.runtime`, any private key material.
- certificate private keys and fullchain files for live domain.
- local logs and temporary artifacts.

## Recommended policy
- Git contains instructions and automation, not secret content.
- GSM contains deployment secrets.
- GoDaddy controls DNS records and certificate issuance path.

## 8) Domain and cert decision flow (GoDaddy + current certs)

Because deploy wildcard files were not present at expected path on this machine, do not assume cert readiness.

Use this flow:
1. Run local cert check:
```powershell
cd C:\Project-Play\Unified-IoT-Portal-18-Jan\rms-go\go-kusumc
.\scripts\secrets\check-certificate-status.ps1 -WarnDays 45
```
2. If cert missing/expired/expiring soon, issue new wildcard cert for target domain.
3. Install cert files to secure local path (not in git).
4. Set compose env variables to that secure path.
5. Validate TLS endpoints before production traffic.

## 9) Routing rights and IAM recommendations

Owner account (you):
- `roles/owner` during bootstrap, later reduce if desired.

Ops user (optional):
- `roles/editor` or narrower custom roles.

Service account (`rms-go-deployer`):
- `roles/secretmanager.secretAccessor`
- `roles/compute.admin`
- `roles/iam.serviceAccountUser`
- `roles/dns.admin`

## 10) Immediate next execution plan

1. Install/enable `gcloud` on this machine.
2. Run `gcloud auth login`.
3. Execute `bootstrap-gcp-rms-go.ps1` in `-DryRun`, then apply.
4. Publish secrets to GSM.
5. Initialize git in `rms-go`, push to GitHub remote.
6. Validate cert status and decide reuse vs renewal.

## 11) Accountability log template

For every run, append:
- date/time
- operator
- command executed
- result (pass/fail)
- follow-up action

This keeps the process auditable and repeatable for future environments.

## 12) Executed run (2026-03-15, provided inputs)

Inputs used:
- `ProjectId`: `rms-go`
- `BillingAccountId`: `01867F-E32F7F-B132E3`
- Active account: `himanshuikaria@gmail.com`

Execution summary:
1. Dry-run executed successfully.
2. Real apply executed successfully.
3. Project created and active:
   - `projectId`: `rms-go`
   - `name`: `RMS GO`
   - `projectNumber`: `880473642851`
4. Billing linked successfully to provided billing account.
5. Default gcloud config set:
   - project: `rms-go`
   - region: `asia-south1`
   - zone: `asia-south1-a`
6. Service account created:
   - `rms-go-deployer@rms-go.iam.gserviceaccount.com`
7. IAM role bindings applied for service account:
   - `roles/secretmanager.secretAccessor`
   - `roles/compute.admin`
   - `roles/iam.serviceAccountUser`
   - `roles/dns.admin`
8. Required APIs enabled (including):
   - `secretmanager.googleapis.com`
   - `compute.googleapis.com`
   - `iam.googleapis.com`
   - `cloudresourcemanager.googleapis.com`
   - `dns.googleapis.com`
   - `certificatemanager.googleapis.com`
   - `artifactregistry.googleapis.com`
   - `cloudbuild.googleapis.com`

Trace log file:
- `go-kusumc/scripts/secrets/gcp-bootstrap-log-20260315-040357.txt`

Notable note observed during run:
- The previously active project (`rms-go-hkbase-20260315`) lacked Cloud Resource Manager API, which produced warning text during initial config capture. This did not block final provisioning of `rms-go`.
