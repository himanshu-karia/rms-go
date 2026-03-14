Param(
    [Parameter(Mandatory = $true)]
    [string]$ProjectId,
    [Parameter(Mandatory = $true)]
    [string]$BillingAccountId,
    [string]$ProjectName = "RMS GO",
    [string]$Region = "asia-south1",
    [string]$Zone = "asia-south1-a",
    [string]$OwnerUserEmail = "himanshuikaria@gmail.com",
    [string]$OpsUserEmail,
    [switch]$CreateProject,
    [switch]$CreateServiceAccount,
    [switch]$EnableApis,
    [switch]$BindIam,
    [switch]$SetDefaults,
    [switch]$All,
    [switch]$DryRun,
    [string]$LogFile = "$PSScriptRoot/gcp-bootstrap-log-$(Get-Date -Format yyyyMMdd-HHmmss).txt"
)

$ErrorActionPreference = 'Stop'

if ($All) {
    $CreateProject = $true
    $CreateServiceAccount = $true
    $EnableApis = $true
    $BindIam = $true
    $SetDefaults = $true
}

function Require-Command {
    param([string]$Name)
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "Required command not found in PATH: $Name"
    }
}

function Invoke-Step {
    param(
        [string]$Title,
        [ScriptBlock]$Action
    )

    $stamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    "[$stamp] START $Title" | Tee-Object -FilePath $LogFile -Append | Out-Host
    try {
        & $Action
        $stamp2 = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
        "[$stamp2] DONE  $Title" | Tee-Object -FilePath $LogFile -Append | Out-Host
    }
    catch {
        $stamp3 = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
        "[$stamp3] FAIL  $Title :: $($_.Exception.Message)" | Tee-Object -FilePath $LogFile -Append | Out-Host
        throw
    }
}

function Exec {
    param([string]$Command)
    if ($DryRun) {
        "[dry-run] $Command" | Tee-Object -FilePath $LogFile -Append | Out-Host
        return
    }

    "[exec] $Command" | Tee-Object -FilePath $LogFile -Append | Out-Host
    Invoke-Expression $Command
    if ($LASTEXITCODE -ne 0) {
        throw "Command failed with exit code $LASTEXITCODE: $Command"
    }
}

Require-Command -Name 'gcloud'

$saName = "rms-go-deployer"
$saEmail = "$saName@$ProjectId.iam.gserviceaccount.com"

Invoke-Step -Title "Capture current gcloud auth/config" -Action {
    Exec "gcloud auth list"
    Exec "gcloud config list"
}

if ($CreateProject) {
    Invoke-Step -Title "Create project + link billing" -Action {
        Exec "gcloud projects create $ProjectId --name \"$ProjectName\""
        Exec "gcloud beta billing projects link $ProjectId --billing-account $BillingAccountId"
    }
}

if ($SetDefaults) {
    Invoke-Step -Title "Set default project/region/zone" -Action {
        Exec "gcloud config set project $ProjectId"
        Exec "gcloud config set compute/region $Region"
        Exec "gcloud config set compute/zone $Zone"
    }
}

if ($EnableApis) {
    Invoke-Step -Title "Enable required APIs" -Action {
        Exec "gcloud services enable secretmanager.googleapis.com --project $ProjectId"
        Exec "gcloud services enable compute.googleapis.com --project $ProjectId"
        Exec "gcloud services enable iam.googleapis.com --project $ProjectId"
        Exec "gcloud services enable cloudresourcemanager.googleapis.com --project $ProjectId"
        Exec "gcloud services enable dns.googleapis.com --project $ProjectId"
        Exec "gcloud services enable certificatemanager.googleapis.com --project $ProjectId"
        Exec "gcloud services enable artifactregistry.googleapis.com --project $ProjectId"
        Exec "gcloud services enable cloudbuild.googleapis.com --project $ProjectId"
    }
}

if ($CreateServiceAccount) {
    Invoke-Step -Title "Create deployment service account" -Action {
        Exec "gcloud iam service-accounts create $saName --display-name \"RMS GO Deployer\" --project $ProjectId"
    }
}

if ($BindIam) {
    Invoke-Step -Title "Assign IAM roles" -Action {
        Exec "gcloud projects add-iam-policy-binding $ProjectId --member=user:$OwnerUserEmail --role=roles/owner"
        if (-not [string]::IsNullOrWhiteSpace($OpsUserEmail)) {
            Exec "gcloud projects add-iam-policy-binding $ProjectId --member=user:$OpsUserEmail --role=roles/editor"
        }

        Exec "gcloud projects add-iam-policy-binding $ProjectId --member=serviceAccount:$saEmail --role=roles/secretmanager.secretAccessor"
        Exec "gcloud projects add-iam-policy-binding $ProjectId --member=serviceAccount:$saEmail --role=roles/compute.admin"
        Exec "gcloud projects add-iam-policy-binding $ProjectId --member=serviceAccount:$saEmail --role=roles/iam.serviceAccountUser"
        Exec "gcloud projects add-iam-policy-binding $ProjectId --member=serviceAccount:$saEmail --role=roles/dns.admin"
    }
}

Invoke-Step -Title "Summarize project state" -Action {
    Exec "gcloud projects describe $ProjectId"
    Exec "gcloud services list --enabled --project $ProjectId"
}

Write-Host "[done] GCP bootstrap workflow completed. Log: $LogFile" -ForegroundColor Green
