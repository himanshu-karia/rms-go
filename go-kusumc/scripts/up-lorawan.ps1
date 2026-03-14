Param(
    [string]$ProjectRoot = "$PSScriptRoot/..",
    [string]$ComposeFile = "docker-compose.yml",
    [switch]$Build
)

$ErrorActionPreference = 'Stop'

Push-Location $ProjectRoot
try {
    # Bring up core stack using staged sequencing first.
    $coreScript = Join-Path $PSScriptRoot 'up-core.ps1'
    if ($Build) {
        & $coreScript -ProjectRoot $ProjectRoot -ComposeFile $ComposeFile -Build
    } else {
        & $coreScript -ProjectRoot $ProjectRoot -ComposeFile $ComposeFile
    }
    if ($LASTEXITCODE -ne 0) {
        throw "up-core staged startup failed"
    }

    $args = @('-f', $ComposeFile, '--profile', 'lorawan', 'up', '-d', '--remove-orphans', 'chirpstack-postgres', 'chirpstack-redis', 'chirpstack', 'chirpstack-gateway-bridge')
    Write-Host "[info] docker compose $($args -join ' ')" -ForegroundColor Cyan
    docker compose @args
    if ($LASTEXITCODE -ne 0) {
        throw "lorawan compose up failed"
    }
}
finally {
    Pop-Location
}
