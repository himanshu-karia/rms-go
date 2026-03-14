Param(
    [switch]$SkipCompose,
    [switch]$SkipDown,
    [string]$ProjectRoot = "${PSScriptRoot}\.."
)

# Purpose: Spin up infra, apply schema + seeds, and leave services running for manual command publish/ingest checks.

$ErrorActionPreference = 'Stop'
Push-Location $ProjectRoot

function Ensure-ComposeUp {
    if ($SkipCompose) { return }
    Write-Host "[info] docker compose up --build" -ForegroundColor Cyan
    docker compose up -d --build --remove-orphans | Out-Null
}

function Wait-Timescale {
    Write-Host "[info] waiting for timescaledb..." -ForegroundColor Cyan
    for ($i=0; $i -lt 30; $i++) {
        $ready = docker compose exec timescaledb pg_isready -U postgres -d telemetry 2>$null
        if ($LASTEXITCODE -eq 0) { Write-Host "[info] timescaledb ready"; return }
        Start-Sleep -Seconds 2
    }
    throw "timescaledb not ready"
}

function Apply-Sql([string]$Path) {
    Write-Host "[info] applying $Path" -ForegroundColor Cyan
    Get-Content -Raw $Path | docker compose exec -T timescaledb psql -U postgres -d telemetry > $null
}

function Main {
    Ensure-ComposeUp
    Wait-Timescale
    Apply-Sql "$ProjectRoot/schemas/v1_init.sql"
    if (Test-Path "$ProjectRoot/test-fixtures/seed_commands.sql") {
        Apply-Sql "$ProjectRoot/test-fixtures/seed_commands.sql"
    }
    Write-Host "[done] infra ready. Start server: `n`t$env:GO_PORT=8081; go run .\cmd\server" -ForegroundColor Green
    Write-Host "[tip] use mqtt-cli or mosquitto_sub to watch: channels/<project>/commands/<imei>/resp" -ForegroundColor Gray
}

try { Main } finally { if (-not $SkipDown) { Write-Host "[note] keeping stack running; pass -SkipDown to leave up" } }

Pop-Location
