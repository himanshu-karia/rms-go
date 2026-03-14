Param(
    [string]$ProjectRoot = "$PSScriptRoot/..",
    [string]$ComposeFile = "docker-compose.integration.yml",
    [switch]$SkipCompose,
    [switch]$KeepUp
)

$ErrorActionPreference = 'Stop'

$OrderedTests = @(
    'TestBootstrapConnectPersist',
    'TestLiveBootstrapTLS',
    'TestDeviceOpenAliasCoverage',
    'TestDeviceConfigurationApply',
    'TestDeviceLifecycle',
    'TestDeviceCommandLifecycle',
    'TestMQTTCredRotation',
    'TestMQTTRotationForcesDisconnect',
    'TestKusumFullCycle',
    'TestSolarRMSFullCycle',
    'TestStory_FullCycle',
    'TestUIAndDeviceOpenFullCycle',
    'TestRMSMegaFlow'
)

function Start-Compose {
    if ($SkipCompose) { return }
    Write-Host "[ordered-e2e] docker compose -f $ComposeFile up -d --build --remove-orphans redis timescaledb emqx emqx-bootstrapper server nginx" -ForegroundColor Cyan
    docker compose -f $ComposeFile up -d --build --remove-orphans redis timescaledb emqx emqx-bootstrapper server nginx | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose up failed"
    }
}

function Wait-EmqxBootstrapper {
    Write-Host "[ordered-e2e] waiting for emqx-bootstrapper one-shot completion..." -ForegroundColor Cyan
    for ($i = 0; $i -lt 90; $i++) {
        $id = (docker compose -f $ComposeFile ps -a -q emqx-bootstrapper 2>$null | Select-Object -First 1)
        if (-not [string]::IsNullOrWhiteSpace($id)) {
            $status = (docker inspect -f '{{.State.Status}}' $id 2>$null).Trim()
            if ($status -eq 'exited') {
                $code = (docker inspect -f '{{.State.ExitCode}}' $id 2>$null).Trim()
                if ($code -eq '0') {
                    Write-Host "[ordered-e2e] emqx-bootstrapper exited(0): expected for one-shot provisioning" -ForegroundColor Green
                    return
                }
                Write-Host "[ordered-e2e] emqx-bootstrapper failed with exit code $code" -ForegroundColor Red
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
    Write-Host "[ordered-e2e] docker compose -f $ComposeFile down" -ForegroundColor Cyan
    docker compose -f $ComposeFile down | Out-Null
}

function Wait-Timescale {
    Write-Host "[ordered-e2e] waiting for timescaledb..." -ForegroundColor Cyan
    for ($i = 0; $i -lt 40; $i++) {
        docker compose -f $ComposeFile exec -T timescaledb pg_isready -U postgres -d telemetry 2>$null | Out-Null
        if ($LASTEXITCODE -eq 0) {
            Write-Host "[ordered-e2e] timescaledb ready" -ForegroundColor Green
            return
        }
        Start-Sleep -Seconds 2
    }
    throw "timescaledb not ready"
}

function Set-HostEnv {
    if (-not $env:BASE_URL) { $env:BASE_URL = "http://localhost:8081" }
    if (-not $env:BOOTSTRAP_URL) { $env:BOOTSTRAP_URL = "$($env:BASE_URL.TrimEnd('/'))/api/bootstrap" }
    if (-not $env:BOOTSTRAP_IMEI) { $env:BOOTSTRAP_IMEI = "999$([string](Get-Random -Minimum 10000000000 -Maximum 99999999999))" }
    if (-not $env:HTTP_TLS_INSECURE) { $env:HTTP_TLS_INSECURE = "true" }
    if (-not $env:MQTT_TLS_INSECURE) { $env:MQTT_TLS_INSECURE = "true" }
    if (-not $env:MQTT_BROKER) { $env:MQTT_BROKER = "mqtts://rms-iot.local:18883" }
    if (-not $env:TIMESCALE_URI) { $env:TIMESCALE_URI = "postgres://postgres:password@localhost:5433/telemetry?sslmode=disable" }
    if (-not $env:PROJECT_ID) { $env:PROJECT_ID = "test-project" }
}

function Wait-ApiReady {
    $probeUrl = "$($env:BASE_URL.TrimEnd('/'))/api/auth/login"
    Write-Host "[ordered-e2e] waiting for API readiness at $probeUrl" -ForegroundColor Cyan
    for ($i = 0; $i -lt 60; $i++) {
        try {
            $resp = Invoke-WebRequest -Uri $probeUrl -Method POST -ContentType 'application/json' -Body '{}' -TimeoutSec 3 -SkipHttpErrorCheck -ErrorAction Stop
            if ($null -ne $resp -and $resp.StatusCode -ge 200 -and $resp.StatusCode -lt 500) {
                Write-Host "[ordered-e2e] API ready" -ForegroundColor Green
                return
            }
        }
        catch {
        }
        Start-Sleep -Seconds 2
    }
    throw "API not ready"
}

function Initialize-IntegrationFixtures {
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
        throw "failed to seed integration fixtures"
    }
}

function Invoke-OrderedTests {
    $services = @(docker compose -f $ComposeFile config --services 2>$null)
    $useTestRunner = $services -contains "test-runner"

    $idx = 0
    foreach ($testName in $OrderedTests) {
        $idx++
        Write-Host "[ordered-e2e] [$idx/$($OrderedTests.Count)] running $testName" -ForegroundColor Cyan
        if ($useTestRunner) {
            docker compose -f $ComposeFile run --rm --no-deps --entrypoint go test-runner test -tags=integration ./tests/e2e -run "^$testName$" -count=1 -v
        }
        else {
            go test -tags=integration ./tests/e2e -run "^$testName$" -count=1 -v
        }
        if ($LASTEXITCODE -ne 0) {
            throw "ordered test failed: $testName"
        }
    }
    Write-Host "[ordered-e2e] all ordered tests passed" -ForegroundColor Green
}

Push-Location $ProjectRoot
try {
    Start-Compose
    Wait-Timescale
    Wait-EmqxBootstrapper
    Set-HostEnv
    Wait-ApiReady
    Initialize-IntegrationFixtures
    Invoke-OrderedTests
}
finally {
    Stop-Compose
    Pop-Location
}
