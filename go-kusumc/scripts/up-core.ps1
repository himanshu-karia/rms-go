Param(
    [string]$ProjectRoot = "$PSScriptRoot/..",
    [string]$ComposeFile = "docker-compose.yml",
    [switch]$Build,
    [switch]$Clean,
    [int]$WaitSeconds = 180
)

$ErrorActionPreference = 'Stop'

function Invoke-Compose {
    param([string[]]$ComposeArgs)
    Write-Host "[info] docker compose $($ComposeArgs -join ' ')" -ForegroundColor Cyan
    docker compose @ComposeArgs
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose failed: $($ComposeArgs -join ' ')"
    }
}

function Wait-ServiceHealthy {
    param(
        [string]$Service,
        [int]$TimeoutSec = 180
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    while ((Get-Date) -lt $deadline) {
        $id = (docker compose -f $ComposeFile ps -q $Service 2>$null | Select-Object -First 1)
        if (-not [string]::IsNullOrWhiteSpace($id)) {
            $status = (docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' $id 2>$null).Trim()
            if ($status -eq 'healthy' -or $status -eq 'running') {
                Write-Host "[ok] $Service is $status" -ForegroundColor Green
                return
            }
        }
        Start-Sleep -Seconds 2
    }
    throw "$Service did not become healthy/running within $TimeoutSec seconds"
}

function Run-EmqxBootstrapper {
    Write-Host "[stage] Starting emqx-bootstrapper one-shot service" -ForegroundColor Yellow
    docker compose -f $ComposeFile up -d --remove-orphans emqx-bootstrapper
    if ($LASTEXITCODE -ne 0) {
        throw "failed to start emqx-bootstrapper"
    }

    $deadline = (Get-Date).AddSeconds($WaitSeconds)
    while ((Get-Date) -lt $deadline) {
        $id = (docker compose -f $ComposeFile ps -a -q emqx-bootstrapper 2>$null | Select-Object -First 1)
        if (-not [string]::IsNullOrWhiteSpace($id)) {
            $state = (docker inspect -f '{{.State.Status}}' $id 2>$null).Trim()
            if ($state -eq 'exited') {
                $code = (docker inspect -f '{{.State.ExitCode}}' $id 2>$null).Trim()
                if ($code -eq '0') {
                    Write-Host "[ok] emqx-bootstrapper exited(0): expected for one-shot provisioning" -ForegroundColor Green
                    return
                }
                Write-Host "[error] emqx-bootstrapper failed with exit code $code. Showing recent logs:" -ForegroundColor Red
                docker compose -f $ComposeFile logs --tail 120 emqx-bootstrapper
                throw "emqx-bootstrapper failed"
            }
        }
        Start-Sleep -Seconds 2
    }

    Write-Host "[error] emqx-bootstrapper did not finish within $WaitSeconds seconds. Showing recent logs:" -ForegroundColor Red
    docker compose -f $ComposeFile logs --tail 120 emqx-bootstrapper
    throw "emqx-bootstrapper timeout"
}

Push-Location $ProjectRoot
try {
    if ($Clean) {
        Invoke-Compose @('-f', $ComposeFile, 'down', '--volumes', '--remove-orphans')
    }

    if ($Build) {
        Invoke-Compose @('-f', $ComposeFile, 'build')
    }

    # Stage 1: Core infra first
    Invoke-Compose @('-f', $ComposeFile, 'up', '-d', '--remove-orphans', 'redis', 'timescaledb', 'emqx')
    Wait-ServiceHealthy -Service 'redis' -TimeoutSec $WaitSeconds
    Wait-ServiceHealthy -Service 'timescaledb' -TimeoutSec $WaitSeconds
    Wait-ServiceHealthy -Service 'emqx' -TimeoutSec $WaitSeconds

    # Stage 2: Migrations + EMQX bootstrap one-shot
    Invoke-Compose @('-f', $ComposeFile, 'up', '-d', '--remove-orphans', 'db-migrations')
    Run-EmqxBootstrapper

    # Stage 3: API + edge services
    Invoke-Compose @('-f', $ComposeFile, 'up', '-d', '--remove-orphans', 'ingestion-go', 'nginx', 'prometheus')
    Wait-ServiceHealthy -Service 'ingestion-go' -TimeoutSec $WaitSeconds
    Wait-ServiceHealthy -Service 'nginx' -TimeoutSec $WaitSeconds

    Write-Host "[done] Core stack is up with staged sequencing." -ForegroundColor Green
}
finally {
    Pop-Location
}
