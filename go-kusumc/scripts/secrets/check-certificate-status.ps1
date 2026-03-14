Param(
    [string[]]$CertPaths = @(
        "$PSScriptRoot/../../infra/nginx/certs/hkbase.in.crt",
        "$PSScriptRoot/../../infra/nginx/certs/hkbase.in-certs/hkbase.in.crt",
        "$PSScriptRoot/../../infra/nginx/certs/localhost-certs/rms-local.crt"
    ),
    [int]$WarnDays = 30,
    [switch]$FailOnMissing
)

$ErrorActionPreference = 'Stop'

function Test-Cert {
    param([string]$Path)

    if (-not (Test-Path -LiteralPath $Path)) {
        if ($FailOnMissing) {
            throw "Missing certificate file: $Path"
        }
        Write-Host "[missing] $Path" -ForegroundColor Yellow
        return
    }

    try {
        $cert = [System.Security.Cryptography.X509Certificates.X509Certificate2]::new($Path)
    }
    catch {
        Write-Host "[error] Could not parse cert: $Path :: $($_.Exception.Message)" -ForegroundColor Red
        return
    }

    $now = (Get-Date)
    $daysLeft = [math]::Floor(($cert.NotAfter - $now).TotalDays)

    $status = "OK"
    $color = "Green"
    if ($daysLeft -lt 0) {
        $status = "EXPIRED"
        $color = "Red"
    }
    elseif ($daysLeft -le $WarnDays) {
        $status = "EXPIRING_SOON"
        $color = "Yellow"
    }

    Write-Host "[$status] $Path" -ForegroundColor $color
    Write-Host "        Subject : $($cert.Subject)"
    Write-Host "        Issuer  : $($cert.Issuer)"
    Write-Host "        NotAfter: $($cert.NotAfter.ToString('u'))"
    Write-Host "        DaysLeft: $daysLeft"
}

Write-Host "[info] Checking certificate status (WarnDays=$WarnDays)" -ForegroundColor Cyan
foreach ($path in $CertPaths) {
    Test-Cert -Path $path
}
Write-Host "[done] Certificate check complete." -ForegroundColor Green
