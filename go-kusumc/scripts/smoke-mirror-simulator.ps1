param(
    [string]$BaseUrl = "http://localhost:8081",
    [string]$Imei = "359760000000001",
    [string]$Topic = "heartbeat",
    [string]$MirrorUser = "",
    [string]$MirrorPass = "",
    [string]$MirrorClientId = "",
    [string]$SimulatorDeviceUuid = "",
    [string]$BearerToken = ""
)

$ErrorActionPreference = "Stop"

Write-Host "== Mirror Ingest ==" -ForegroundColor Cyan
if ([string]::IsNullOrWhiteSpace($MirrorUser) -or [string]::IsNullOrWhiteSpace($MirrorPass) -or [string]::IsNullOrWhiteSpace($MirrorClientId)) {
    Write-Host "Mirror credentials missing. Provide -MirrorUser, -MirrorPass, -MirrorClientId" -ForegroundColor Yellow
} else {
    $basic = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes("$MirrorUser`:$MirrorPass"))
    $headers = @{
        Authorization = "Basic $basic"
        "X-RMS-IMEI" = $Imei
        "X-RMS-ClientId" = $MirrorClientId
        "X-RMS-MsgId" = "smoke-$(Get-Date -Format 'yyyyMMddHHmmss')"
    }
    $payload = @{ packet_type = $Topic; imei = $Imei; timestamp = [int][double]::Parse((Get-Date -UFormat %s)); data = @{ ping = "ok" } }
    $mirrorUrl = "$BaseUrl/api/telemetry/mirror/$Topic"
    try {
        $resp = Invoke-RestMethod -Method Post -Uri $mirrorUrl -Headers $headers -Body ($payload | ConvertTo-Json -Depth 5) -ContentType "application/json"
        Write-Host "Mirror response: $($resp | ConvertTo-Json -Compress)" -ForegroundColor Green
    } catch {
        Write-Host "Mirror call failed: $($_.Exception.Message)" -ForegroundColor Red
    }
}

Write-Host "== Simulator Sessions ==" -ForegroundColor Cyan
if ([string]::IsNullOrWhiteSpace($SimulatorDeviceUuid) -or [string]::IsNullOrWhiteSpace($BearerToken)) {
    Write-Host "Simulator inputs missing. Provide -SimulatorDeviceUuid and -BearerToken" -ForegroundColor Yellow
    exit 0
}

$simHeaders = @{ Authorization = "Bearer $BearerToken" }
$sessionUrl = "$BaseUrl/api/simulator/sessions"

try {
    $createPayload = @{ deviceUuid = $SimulatorDeviceUuid; expiresInMinutes = 60 }
    $createResp = Invoke-RestMethod -Method Post -Uri $sessionUrl -Headers $simHeaders -Body ($createPayload | ConvertTo-Json) -ContentType "application/json"
    Write-Host "Session created: $($createResp.session.id)" -ForegroundColor Green

    $listResp = Invoke-RestMethod -Method Get -Uri $sessionUrl -Headers $simHeaders
    Write-Host "Sessions count: $($listResp.count)" -ForegroundColor Green

    $revokeId = $createResp.session.id
    $revokeResp = Invoke-RestMethod -Method Delete -Uri "$sessionUrl/$revokeId" -Headers $simHeaders
    Write-Host "Session revoked: $($revokeResp.id)" -ForegroundColor Green
} catch {
    Write-Host "Simulator call failed: $($_.Exception.Message)" -ForegroundColor Red
}
