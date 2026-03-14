param(
  [string]$Domain = "rms-iot.local",
  [switch]$UpdateHosts,
  [switch]$ReissueCert,
  [switch]$ImportCa,
  [switch]$Force
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$projectRoot = Resolve-Path (Join-Path $scriptDir "..")
$certDir = Join-Path $projectRoot "infra/nginx/certs/localhost-certs"
$serverCnf = Join-Path $certDir "server.cnf"
$caCrt = Join-Path $certDir "rms-local-ca.crt"
$caKey = Join-Path $certDir "rms-local-ca.key"
$serverCrt = Join-Path $certDir "rms-local.crt"
$serverKey = Join-Path $certDir "rms-local.key"
$serverCsr = Join-Path $certDir "rms-local.csr"
$hostsPath = "$env:WINDIR\System32\drivers\etc\hosts"

function Write-Step([string]$message) {
  Write-Host "[setup-local-domain] $message" -ForegroundColor Cyan
}

function Ensure-File([string]$path) {
  if (-not (Test-Path $path)) {
    throw "Required file not found: $path"
  }
}

function Update-ServerCnfDomain([string]$domain) {
  Ensure-File $serverCnf
  $raw = Get-Content $serverCnf -Raw

  if ($raw -notmatch "DNS\.3\s*=\s*$([regex]::Escape($domain))") {
    if ($raw -match "DNS\.3\s*=\s*.+") {
      $raw = [regex]::Replace($raw, "DNS\.3\s*=\s*.+", "DNS.3 = $domain")
    }
    else {
      $raw = $raw.TrimEnd() + "`r`nDNS.3 = $domain`r`n"
    }
    Set-Content -Path $serverCnf -Value $raw -NoNewline
    Write-Step "Updated server.cnf SAN DNS.3 -> $domain"
  }
  else {
    Write-Step "server.cnf already contains DNS.3 = $domain"
  }
}

function Backup-IfExists([string]$path) {
  if (Test-Path $path) {
    $stamp = Get-Date -Format "yyyyMMdd-HHmmss"
    $backup = "$path.bak-$stamp"
    Copy-Item $path $backup -Force
    Write-Step "Backup created: $backup"
  }
}

function Ensure-OpenSSL {
  $cmd = Get-Command openssl -ErrorAction SilentlyContinue
  if (-not $cmd) {
    throw "openssl not found in PATH. Install OpenSSL or run from Git Bash with openssl available."
  }
}

function Reissue-TlsCert {
  Ensure-OpenSSL
  Ensure-File $serverCnf
  Ensure-File $caCrt
  Ensure-File $caKey

  Update-ServerCnfDomain -domain $Domain
  Backup-IfExists $serverCrt
  Backup-IfExists $serverKey

  Push-Location $certDir
  try {
    & openssl req -new -nodes -newkey rsa:2048 -keyout "rms-local.key" -out "rms-local.csr" -config "server.cnf"
    if ($LASTEXITCODE -ne 0) { throw "openssl req failed" }

    & openssl x509 -req -in "rms-local.csr" -CA "rms-local-ca.crt" -CAkey "rms-local-ca.key" -CAcreateserial -out "rms-local.crt" -days 825 -sha256 -extensions v3_req -extfile "server.cnf"
    if ($LASTEXITCODE -ne 0) { throw "openssl x509 failed" }

    Write-Step "Reissued rms-local.crt / rms-local.key with SAN including $Domain"
  }
  finally {
    Pop-Location
  }
}

function Add-HostsEntry {
  $entry = "127.0.0.1 $Domain"
  $content = Get-Content $hostsPath -ErrorAction Stop
  if ($content -match "(^|\s)$([regex]::Escape($Domain))(\s|$)") {
    Write-Step "Hosts entry already present for $Domain"
    return
  }

  try {
    Add-Content -Path $hostsPath -Value "`r`n$entry"
    Write-Step "Added hosts entry: $entry"
  }
  catch {
    throw "Failed to update hosts file. Re-run PowerShell as Administrator. Error: $($_.Exception.Message)"
  }
}

function Import-CaCert {
  Ensure-File $caCrt
  try {
    $imported = Import-Certificate -FilePath $caCrt -CertStoreLocation "Cert:\CurrentUser\Root"
    if ($imported) {
      Write-Step "CA imported into CurrentUser Root store"
    }
  }
  catch {
    throw "Failed to import CA certificate: $($_.Exception.Message)"
  }
}

Write-Step "Domain: $Domain"
Write-Step "Cert directory: $certDir"

if (-not $UpdateHosts -and -not $ReissueCert -and -not $ImportCa) {
  Write-Step "No action flags provided. Running full setup (-UpdateHosts -ReissueCert -ImportCa)."
  $UpdateHosts = $true
  $ReissueCert = $true
  $ImportCa = $true
}

if ($ReissueCert) {
  Reissue-TlsCert
}

if ($ImportCa) {
  Import-CaCert
}

if ($UpdateHosts) {
  Add-HostsEntry
}

Write-Step "Done. Restart stack: docker compose down --remove-orphans ; docker compose up -d --build"
