Param(
    [string]$ProjectRoot = "$PSScriptRoot/..",
    [string]$ComposeFile = "docker-compose.yml",
    [string]$TestName = "TestDeviceLifecycle",
    [switch]$SkipCompose,
    [switch]$KeepUp
)

$ErrorActionPreference = 'Stop'

function Start-Compose {
    if ($SkipCompose) { return }
    Write-Host "[info] docker compose -f $ComposeFile up -d --build --remove-orphans redis timescaledb emqx emqx-bootstrapper server nginx" -ForegroundColor Cyan
    docker compose -f $ComposeFile up -d --build --remove-orphans redis timescaledb emqx emqx-bootstrapper server nginx | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose up failed"
    }
}

function Wait-EmqxBootstrapper {
    Write-Host "[info] waiting for emqx-bootstrapper one-shot completion..." -ForegroundColor Cyan
    for ($i = 0; $i -lt 90; $i++) {
        $id = (docker compose -f $ComposeFile ps -a -q emqx-bootstrapper 2>$null | Select-Object -First 1)
        if (-not [string]::IsNullOrWhiteSpace($id)) {
            $status = (docker inspect -f '{{.State.Status}}' $id 2>$null).Trim()
            if ($status -eq 'exited') {
                $code = (docker inspect -f '{{.State.ExitCode}}' $id 2>$null).Trim()
                if ($code -eq '0') {
                    Write-Host "[info] emqx-bootstrapper exited(0): expected for one-shot provisioning" -ForegroundColor Green
                    return
                }
                Write-Host "[error] emqx-bootstrapper failed with exit code $code" -ForegroundColor Red
                docker compose -f $ComposeFile logs --tail 120 emqx-bootstrapper
                throw "emqx-bootstrapper failed"
            }
        }
        Start-Sleep -Seconds 2
    }
    throw "emqx-bootstrapper did not finish in time"
}

function Stop-Compose {
    if ($KeepUp -or $SkipCompose) { return }
    Write-Host "[info] docker compose -f $ComposeFile down" -ForegroundColor Cyan
    docker compose -f $ComposeFile down | Out-Null
    if ($LASTEXITCODE -ne 0) {
        Write-Warning "docker compose down returned non-zero exit code: $LASTEXITCODE"
    }
}

function Wait-Timescale {
    Write-Host "[info] waiting for timescaledb..." -ForegroundColor Cyan
    for ($i = 0; $i -lt 30; $i++) {
        docker compose -f $ComposeFile exec -T timescaledb pg_isready -U postgres -d telemetry 2>$null | Out-Null
        if ($LASTEXITCODE -eq 0) {
            Write-Host "[info] timescaledb ready" -ForegroundColor Green
            return
        }
        Start-Sleep -Seconds 2
    }
    throw "timescaledb not ready"
}

function Main {
    Start-Compose
    Wait-Timescale
    Wait-EmqxBootstrapper
    $services = @(docker compose -f $ComposeFile config --services 2>$null)
    $composeName = [System.IO.Path]::GetFileName($ComposeFile).ToLowerInvariant()

    if ($services -contains "test-runner") {
        Write-Host "[info] running integration tests via test-runner service (test: $TestName)" -ForegroundColor Cyan
        docker compose -f $ComposeFile run --rm test-runner go test -tags=integration ./tests/e2e -run $TestName -count=1
        if ($LASTEXITCODE -ne 0) {
            throw "integration test command failed via test-runner"
        }
        return
    }

    docker compose -f $ComposeFile exec -T ingestion-go sh -lc "command -v go >/dev/null 2>&1"
    if ($LASTEXITCODE -eq 0) {
        Write-Host "[info] running integration tests inside ingestion-go (test: $TestName)" -ForegroundColor Cyan
        docker compose -f $ComposeFile exec -T ingestion-go go test -tags=integration ./tests/e2e -run $TestName -count=1
        if ($LASTEXITCODE -ne 0) {
            throw "integration test command failed in ingestion-go"
        }
        return
    }

    Write-Host "[info] go toolchain not available in ingestion-go; running integration tests on host (test: $TestName)" -ForegroundColor Yellow

    if (-not $env:BASE_URL) {
        if ($composeName -eq "docker-compose.integration.yml") {
            $env:BASE_URL = "http://localhost:8081"
        }
        else {
            $env:BASE_URL = "https://rms-iot.local:7443"
        }
    }
    if (-not $env:BOOTSTRAP_URL) { $env:BOOTSTRAP_URL = "$($env:BASE_URL.TrimEnd('/'))/api/bootstrap" }
    if (-not $env:HTTP_TLS_INSECURE) { $env:HTTP_TLS_INSECURE = "true" }
    if (-not $env:MQTT_TLS_INSECURE) { $env:MQTT_TLS_INSECURE = "true" }
    if (-not $env:MQTT_BROKER) { $env:MQTT_BROKER = "mqtts://rms-iot.local:18883" }
    if (-not $env:TIMESCALE_URI) { $env:TIMESCALE_URI = "postgres://postgres:password@localhost:5433/telemetry?sslmode=disable" }

    $seedSql = @"
insert into projects (id, name, config)
values ('test-project','Test Project','{}')
on conflict (id) do nothing;
insert into command_catalog (name, scope, project_id, payload_schema, transport)
select 'E2E_Set','project','test-project','{}'::jsonb,'mqtt'
where not exists (
  select 1 from command_catalog where project_id='test-project' and name='E2E_Set'
);
"@
    $seedSql | docker compose -f $ComposeFile exec -T timescaledb psql -U postgres -d telemetry | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "failed to seed host integration fixtures"
    }

    go test -tags=integration ./tests/e2e -run $TestName -count=1
    if ($LASTEXITCODE -ne 0) {
        throw "integration test command failed on host"
    }
}

Push-Location $ProjectRoot
try {
    Main
}
finally {
    Stop-Compose
    Pop-Location
}
