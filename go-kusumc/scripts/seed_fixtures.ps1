Param()

Set-StrictMode -Version Latest
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$rootDir = Join-Path $scriptDir '..'
$sqlFile = Join-Path $rootDir 'test-fixtures\seed_commands.sql'

if (-not (Test-Path $sqlFile)) {
    Write-Error "Seed SQL not found: $sqlFile"
    exit 1
}

Write-Host "Applying seed fixtures to timescaledb (database: telemetry) using docker compose"
Push-Location $rootDir
try {
    docker compose exec -T timescaledb psql -U postgres -d telemetry < $sqlFile
} finally {
    Pop-Location
}

Write-Host "Seeding completed."
