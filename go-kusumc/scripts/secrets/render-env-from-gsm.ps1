Param(
    [Parameter(Mandatory = $true)]
    [string]$ProjectId,
    [string]$SecretPrefix = "rms-go-",
    [string]$OutputFile = "$PSScriptRoot/../../.env.runtime",
    [switch]$IncludeOptional,
    [switch]$StdoutOnly
)

$ErrorActionPreference = 'Stop'

function Require-Command {
    param([string]$Name)
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "Required command '$Name' is not available in PATH."
    }
}

function Fetch-Secret {
    param(
        [string]$Project,
        [string]$SecretId
    )

    $value = & gcloud secrets versions access latest --secret $SecretId --project $Project 2>$null
    if ($LASTEXITCODE -ne 0) {
        return $null
    }

    return [string]$value
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

$keys = @($requiredKeys)
if ($IncludeOptional) {
    $keys += $optionalKeys
}

$lines = New-Object System.Collections.Generic.List[string]
$missingRequired = @()

foreach ($key in $keys) {
    $secretId = ($SecretPrefix + $key.ToLowerInvariant().Replace('_', '-'))
    $value = Fetch-Secret -Project $ProjectId -SecretId $secretId

    if ($null -eq $value) {
        if ($requiredKeys -contains $key) {
            $missingRequired += $key
            Write-Host "[warn] Missing required secret: $secretId" -ForegroundColor Yellow
        }
        else {
            Write-Host "[skip] Optional secret not found: $secretId" -ForegroundColor DarkGray
        }
        continue
    }

    $lines.Add("$key=$value")
    Write-Host "[ok] Loaded $key from $secretId" -ForegroundColor Green
}

if ($missingRequired.Count -gt 0) {
    throw "Missing required secrets in GSM: $($missingRequired -join ', ')"
}

$content = ($lines -join [Environment]::NewLine)

if ($StdoutOnly) {
    Write-Output $content
    Write-Host "[done] Rendered env content to stdout." -ForegroundColor Green
    exit 0
}

$dir = Split-Path -Parent $OutputFile
if (-not [string]::IsNullOrWhiteSpace($dir) -and -not (Test-Path -LiteralPath $dir)) {
    New-Item -ItemType Directory -Path $dir | Out-Null
}

[System.IO.File]::WriteAllText($OutputFile, $content)
Write-Host "[done] Wrote env file: $OutputFile" -ForegroundColor Green
Write-Host "[note] Keep this file local-only and never commit it." -ForegroundColor Yellow
