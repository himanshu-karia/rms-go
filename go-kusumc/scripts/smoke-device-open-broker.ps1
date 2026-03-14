param(
    [string]$BaseUrl = "http://localhost:8081",
    [string]$ApiKey = "",
    [string]$DeviceUuid = "",
    [string]$Imei = ""
)

$ErrorActionPreference = "Stop"

function Invoke-Json {
    param(
        [string]$Method,
        [string]$Url,
        [object]$Body = $null,
        [hashtable]$Headers = $null
    )

    $params = @{
        Method  = $Method
        Uri     = $Url
        Headers = $Headers
    }

    if ($Body -ne $null) {
        $params["Body"] = ($Body | ConvertTo-Json -Depth 10)
        $params["ContentType"] = "application/json"
    }

    return Invoke-RestMethod @params
}

if ([string]::IsNullOrWhiteSpace($DeviceUuid) -and [string]::IsNullOrWhiteSpace($Imei)) {
    Write-Host "Provide -DeviceUuid or -Imei to run device-open checks." -ForegroundColor Yellow
    exit 1
}

$deviceRef = if ($DeviceUuid) { $DeviceUuid } else { $Imei }
$headers = @{}
if (-not [string]::IsNullOrWhiteSpace($ApiKey)) {
    $headers["x-api-key"] = $ApiKey
}

Write-Host "[1/4] GET /api/device-open/credentials/local" -ForegroundColor Cyan
$localCreds = Invoke-Json -Method "GET" -Url "$BaseUrl/api/device-open/credentials/local?imei=$Imei" -Headers $headers
$localCreds | ConvertTo-Json -Depth 6

Write-Host "[2/4] GET /api/device-open/credentials/government" -ForegroundColor Cyan
$govtCreds = Invoke-Json -Method "GET" -Url "$BaseUrl/api/device-open/credentials/government?imei=$Imei" -Headers $headers
$govtCreds | ConvertTo-Json -Depth 6

Write-Host "[3/4] GET /api/device-open/vfd" -ForegroundColor Cyan
$vfd = Invoke-Json -Method "GET" -Url "$BaseUrl/api/device-open/vfd?deviceUuid=$deviceRef" -Headers $headers
$vfd | ConvertTo-Json -Depth 6

Write-Host "[4/4] POST /api/broker/sync" -ForegroundColor Cyan
$resync = Invoke-Json -Method "POST" -Url "$BaseUrl/api/broker/sync" -Body @{ deviceUuid = $deviceRef; reason = "smoke-check" } -Headers $headers
$resync | ConvertTo-Json -Depth 6

Write-Host "Smoke checks completed." -ForegroundColor Green
