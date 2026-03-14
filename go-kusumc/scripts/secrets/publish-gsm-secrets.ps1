Param(
    [Parameter(Mandatory = $true)]
    [string]$ProjectId,
    [string]$EnvFile = "$PSScriptRoot/../../.env.local",
    [string]$SecretPrefix = "rms-go-",
    [string]$LabelEnvironment = "dev",
    [switch]$IncludeOptional,
    [switch]$DryRun
)

$ErrorActionPreference = 'Stop'

function Require-Command {
    param([string]$Name)
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "Required command '$Name' is not available in PATH."
    }
}

function Parse-EnvFile {
    param([string]$Path)

    if (-not (Test-Path -LiteralPath $Path)) {
        throw "Env file not found: $Path"
    }

    $result = @{}
    $lines = Get-Content -LiteralPath $Path
    foreach ($rawLine in $lines) {
        $line = $rawLine.Trim()
        if ([string]::IsNullOrWhiteSpace($line)) { continue }
        if ($line.StartsWith('#')) { continue }
        $idx = $line.IndexOf('=')
        if ($idx -lt 1) { continue }
        $key = $line.Substring(0, $idx).Trim()
        $value = $line.Substring($idx + 1)
        if ($value.StartsWith('"') -and $value.EndsWith('"') -and $value.Length -ge 2) {
            $value = $value.Substring(1, $value.Length - 2)
        }
        $result[$key] = $value
    }
    return $result
}

function Ensure-Secret {
    param(
        [string]$Project,
        [string]$SecretId,
        [string]$Environment,
        [switch]$Simulate
    )

    $exists = $false
    gcloud secrets describe $SecretId --project $Project *> $null
    if ($LASTEXITCODE -eq 0) {
        $exists = $true
    }

    if ($exists) {
        Write-Host "[skip] Secret exists: $SecretId" -ForegroundColor DarkGray
        return
    }

    if ($Simulate) {
        Write-Host "[dry-run] Create secret: $SecretId" -ForegroundColor Yellow
        return
    }

    gcloud secrets create $SecretId --project $Project --replication-policy automatic --labels "app=rms-go,env=$Environment"
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to create secret: $SecretId"
    }

    Write-Host "[ok] Created secret: $SecretId" -ForegroundColor Green
}

function Add-SecretVersion {
    param(
        [string]$Project,
        [string]$SecretId,
        [string]$Value,
        [switch]$Simulate
    )

    if ($Simulate) {
        Write-Host "[dry-run] Add version: $SecretId" -ForegroundColor Yellow
        return
    }

    $tmpFile = [System.IO.Path]::GetTempFileName()
    try {
        [System.IO.File]::WriteAllText($tmpFile, $Value)
        gcloud secrets versions add $SecretId --project $Project --data-file $tmpFile *> $null
        if ($LASTEXITCODE -ne 0) {
            throw "Failed to add secret version: $SecretId"
        }
        Write-Host "[ok] Updated secret version: $SecretId" -ForegroundColor Green
    }
    finally {
        Remove-Item -LiteralPath $tmpFile -ErrorAction SilentlyContinue
    }
}

Require-Command -Name 'gcloud'

$requiredKeys = @(
    'AUTH_SECRET',
    'GOVT_CREDS_KEY',
    'POSTGRES_PASSWORD',
    'SERVICE_MQTT_PASSWORD',
    'EMQX_DASHBOARD_PASSWORD',
    'EMQX_APP_SECRET'
)

$optionalKeys = @(
    'TIMESCALE_URI',
    'REDIS_URL',
    'EMQX_API_URL',
    'EMQX_APP_ID',
    'FRONTEND_ORIGINS'
)

$envMap = Parse-EnvFile -Path $EnvFile

Write-Host "[info] Using env file: $EnvFile" -ForegroundColor Cyan
Write-Host "[info] Target GCP project: $ProjectId" -ForegroundColor Cyan
Write-Host "[info] Secret prefix: $SecretPrefix" -ForegroundColor Cyan

$keysToPublish = @($requiredKeys)
if ($IncludeOptional) {
    $keysToPublish += $optionalKeys
}

$missing = @()
foreach ($k in $requiredKeys) {
    if (-not $envMap.ContainsKey($k) -or [string]::IsNullOrWhiteSpace([string]$envMap[$k])) {
        $missing += $k
    }
}
if ($missing.Count -gt 0) {
    throw "Missing required keys in env file: $($missing -join ', ')"
}

foreach ($key in $keysToPublish) {
    if (-not $envMap.ContainsKey($key)) {
        Write-Host "[skip] Key not present in env file: $key" -ForegroundColor DarkGray
        continue
    }

    $value = [string]$envMap[$key]
    if ([string]::IsNullOrWhiteSpace($value)) {
        Write-Host "[skip] Empty value for key: $key" -ForegroundColor DarkGray
        continue
    }

    $secretId = ($SecretPrefix + $key.ToLowerInvariant().Replace('_', '-'))

    Ensure-Secret -Project $ProjectId -SecretId $secretId -Environment $LabelEnvironment -Simulate:$DryRun
    Add-SecretVersion -Project $ProjectId -SecretId $secretId -Value $value -Simulate:$DryRun
}

Write-Host "[done] Secret publish flow completed." -ForegroundColor Green
