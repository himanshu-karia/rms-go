Param(
    [string]$ProjectRoot = "$PSScriptRoot/..",
    [string]$ComposeFile = "docker-compose.yml"
)

$ErrorActionPreference = 'Stop'

Push-Location $ProjectRoot
try {
    $args = @('-f', $ComposeFile, '--profile', 'lorawan', 'down', '--remove-orphans')

    Write-Host "[info] docker compose $($args -join ' ')" -ForegroundColor Cyan
    docker compose @args
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose down failed"
    }
}
finally {
    Pop-Location
}
