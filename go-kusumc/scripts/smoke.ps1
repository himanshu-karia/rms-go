[CmdletBinding()]
param(
  [string]$BaseUrl = "https://rms-iot.local:7443",
  [string]$ProjectId = "smoke_proj",
  [string]$Email = "",
  [string]$Password = "SmokePassword123!",
  [string]$Role = "manager",
  [int]$WaitSeconds = 120,
  [switch]$SkipComposeUp,
  [switch]$SkipComposeDown
)

$ErrorActionPreference = "Stop"

function Invoke-Api {
  param(
    [Parameter(Mandatory=$true)][ValidateSet('GET','POST','PUT','DELETE')][string]$Method,
    [Parameter(Mandatory=$true)][string]$Path,
    [object]$Body,
    [string]$Token,
    [string]$OutFile
  )

  $uri = ($BaseUrl.TrimEnd('/') + $Path)
  $headers = @{}
  if ($Token) { $headers['Authorization'] = "Bearer $Token" }

  $common = @{
    Uri = $uri
    Method = $Method
    Headers = $headers
    SkipCertificateCheck = $true
    SkipHttpErrorCheck = $true
  }

  if ($OutFile) {
    return Invoke-WebRequest @common -OutFile $OutFile -PassThru
  }

  if ($null -ne $Body) {
    $json = $Body | ConvertTo-Json -Depth 20
    return Invoke-WebRequest @common -ContentType 'application/json' -Body $json
  }

  return Invoke-WebRequest @common
}

function Require-Status {
  param(
    [Parameter(Mandatory=$true)]$Resp,
    [Parameter(Mandatory=$true)][int[]]$Expected
  )
  $code = [int]$Resp.StatusCode
  if ($Expected -notcontains $code) {
    $body = ""
    try { $body = $Resp.Content } catch {}
    throw "Unexpected HTTP $code (expected: $($Expected -join ', ')). Body: $body"
  }
}

function Get-HeaderValue {
  param(
    [Parameter(Mandatory=$true)]$Resp,
    [Parameter(Mandatory=$true)][string]$Name
  )
  try {
    $v = $Resp.Headers[$Name]
    if ($v) { return [string]$v }
  } catch {}
  try {
    # Some PS versions expose .Headers as a dictionary with .GetValues
    $v2 = $Resp.Headers.GetValues($Name)
    if ($v2) { return ($v2 -join ',') }
  } catch {}
  return ""
}

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
Push-Location $repoRoot

try {
  if (-not $Email) {
    $Email = "smoke_manager_{0}@test.com" -f ([guid]::NewGuid().ToString("N").Substring(0, 8))
  }
  if ($ProjectId -eq "smoke_proj") {
    $ProjectId = "smoke_proj_{0}" -f ([guid]::NewGuid().ToString("N").Substring(0, 8))
  }

  if (-not $SkipComposeUp) {
    Write-Host "[smoke] docker compose up (nginx + ingestion-go + deps)"
    docker compose up --build -d nginx ingestion-go timescaledb redis emqx db-migrations | Out-Host
  }

  Write-Host "[smoke] waiting for API readiness at $BaseUrl"
  $deadline = (Get-Date).AddSeconds($WaitSeconds)
  $ready = $false
  while ((Get-Date) -lt $deadline) {
    try {
      $resp = Invoke-Api -Method POST -Path "/api/auth/login" -Body @{} -Token ""
      # login endpoint is POST-only; missing body should be 400 when ready
      if ([int]$resp.StatusCode -in @(200,400,401)) {
        $ready = $true
        break
      }
    } catch {
      # ignore and retry
    }
    Start-Sleep -Seconds 2
  }
  if (-not $ready) {
    throw "API not ready after $WaitSeconds seconds at $BaseUrl"
  }

  Write-Host "[smoke] register/login ($Role)"
  $rg = Invoke-Api -Method POST -Path "/api/auth/register" -Body @{ username=$Email; password=$Password; role=$Role } -Token ""
  # ok: 200/201; already exists: 409
  # some older builds may incorrectly return 500 on unique constraint; treat as already-exists
  if ([int]$rg.StatusCode -eq 500 -and $rg.Content -match "users_username_key") {
    Write-Host "[smoke] register returned duplicate-user 500; continuing" -ForegroundColor Yellow
  } else {
    Require-Status $rg @(200,201,409)
  }

  $lg = Invoke-Api -Method POST -Path "/api/auth/login" -Body @{ username=$Email; password=$Password } -Token ""
  Require-Status $lg @(200)
  $token = ($lg.Content | ConvertFrom-Json).token
  if (-not $token) { throw "Login succeeded but no token returned." }

  Write-Host "[smoke] create project $ProjectId"
  $pr = Invoke-Api -Method POST -Path "/api/projects" -Body @{ id=$ProjectId; name="Smoke Project"; config=@{ generated=$true } } -Token $token
  Require-Status $pr @(200,201)

  $imei = "SMOKE_{0}" -f ([int][double]::Parse((Get-Date -UFormat %s)))
  Write-Host "[smoke] create device IMEI=$imei"
  $dv = Invoke-Api -Method POST -Path "/api/devices" -Body @{ name="Smoke Device"; imei=$imei; projectId=$ProjectId; attributes=@{} } -Token $token
  Require-Status $dv @(200)
  $dvJson = $dv.Content | ConvertFrom-Json
  $deviceId = $dvJson.device_id
  if (-not $deviceId) { throw "Device create succeeded but device_id missing." }

  Write-Host "[smoke] ingest telemetry"
  $sentinelValue = 99.9
  $ig = Invoke-Api -Method POST -Path "/api/ingest" -Body @{
    imei       = $imei
    device_id  = $deviceId
    project_id = $ProjectId
    packet_type = "telemetry"
    msgid      = [guid]::NewGuid().ToString("N")
    ts         = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
    temp       = $sentinelValue
    smoke      = $true
  } -Token ""
  Require-Status $ig @(200)

  Write-Host "[smoke] verify telemetry history"
  $found = $false
  $telemetryDeadline = (Get-Date).AddSeconds(30)
  while ((Get-Date) -lt $telemetryDeadline) {
    $th = Invoke-Api -Method GET -Path ("/api/telemetry/history?imei={0}" -f $imei) -Token $token
    Require-Status $th @(200)
    if ($th.Content -match [Regex]::Escape("$sentinelValue")) {
      $found = $true
      break
    }
    Start-Sleep -Seconds 2
  }
  if (-not $found) {
    throw "Telemetry history did not contain expected value $sentinelValue."
  }

  Write-Host "[smoke] verify telemetry project-history"
  $fromIso = (Get-Date).AddMinutes(-10).ToString("o")
  $toIso = (Get-Date).AddMinutes(10).ToString("o")
  $ph = Invoke-Api -Method GET -Path ("/api/telemetry/history/project?projectId={0}&from={1}&to={2}&page=1&limit=50&packet_type=telemetry" -f $ProjectId, [Uri]::EscapeDataString($fromIso), [Uri]::EscapeDataString($toIso)) -Token $token
  Require-Status $ph @(200)
  if ($ph.Content -notmatch [Regex]::Escape("$sentinelValue")) {
    throw "Telemetry project-history did not contain expected value $sentinelValue."
  }

  Write-Host "[smoke] master-data CRUD"
  $mdType = "smoke_type"
  $mdCreate = Invoke-Api -Method POST -Path ("/api/master-data/{0}" -f $mdType) -Token $token -Body @{ name="Smoke Item"; code="SMK"; value="v1"; projectId=$ProjectId }
  Require-Status $mdCreate @(200,201)
  $mdId = ($mdCreate.Content | ConvertFrom-Json)._id
  if (-not $mdId) { throw "Master-data create returned no _id." }

  $mdList = Invoke-Api -Method GET -Path ("/api/master-data/{0}?projectId={1}" -f $mdType, $ProjectId) -Token $token
  Require-Status $mdList @(200)
  if ($mdList.Content -notmatch [Regex]::Escape($mdId)) {
    throw "Master-data list did not include created id $mdId."
  }

  $mdUpdate = Invoke-Api -Method PUT -Path ("/api/master-data/{0}/{1}" -f $mdType, $mdId) -Token $token -Body @{ name="Smoke Item Updated"; value="v2" }
  Require-Status $mdUpdate @(200)

  $mdDelete = Invoke-Api -Method DELETE -Path ("/api/master-data/{0}/{1}" -f $mdType, $mdId) -Token $token
  Require-Status $mdDelete @(200,204)

  Write-Host "[smoke] maintenance create + resolve (notes alias)"
  $ticketId = "SMK-{0}" -f ([guid]::NewGuid().ToString("N").Substring(0,8))
  $woCreate = Invoke-Api -Method POST -Path "/api/maintenance/work-orders" -Token $token -Body @{ ticket_id=$ticketId; title="Smoke WO"; device_id=$deviceId; priority="HIGH" }
  Require-Status $woCreate @(201)

  $woList = Invoke-Api -Method GET -Path "/api/maintenance/work-orders" -Token $token
  Require-Status $woList @(200)
  $wo = ($woList.Content | ConvertFrom-Json | Where-Object { $_.ticket_id -eq $ticketId } | Select-Object -First 1)
  if (-not $wo) { throw "Work order not found after create (ticket_id=$ticketId)." }

  $woResolve = Invoke-Api -Method PUT -Path ("/api/maintenance/work-orders/{0}/resolve" -f $wo.id) -Token $token -Body @{ notes="resolved via smoke" }
  Require-Status $woResolve @(200)

  Write-Host "[smoke] telemetry CSV export (explicit format + filters)"
  $csv = Invoke-Api -Method GET -Path ("/api/telemetry/export?imei={0}&format=csv&from={1}&to={2}&packet_type=telemetry" -f $imei, [Uri]::EscapeDataString($fromIso), [Uri]::EscapeDataString($toIso)) -Token $token
  Require-Status $csv @(200)
  $csvCt = Get-HeaderValue -Resp $csv -Name "Content-Type"
  $csvCd = Get-HeaderValue -Resp $csv -Name "Content-Disposition"
  if ($csvCt -and $csvCt -notmatch "text/csv") {
    throw "Telemetry CSV export unexpected Content-Type: $csvCt"
  }
  if ($csvCd -and $csvCd -notmatch "attachment") {
    throw "Telemetry CSV export missing attachment Content-Disposition: $csvCd"
  }
  if ($csv.Content -notmatch "^time,device_id,data" ) {
    throw "Telemetry export did not look like expected CSV (missing header)."
  }

  Write-Host "[smoke] telemetry XLSX export"
  $xlsxPath = Join-Path $env:TEMP ("telemetry_{0}.xlsx" -f $imei)
  $xlsx = Invoke-Api -Method GET -Path ("/api/telemetry/export?imei={0}&format=xlsx&from={1}&to={2}&packet_type=telemetry" -f $imei, [Uri]::EscapeDataString($fromIso), [Uri]::EscapeDataString($toIso)) -Token $token -OutFile $xlsxPath
  Require-Status $xlsx @(200)
  if (-not (Test-Path $xlsxPath)) { throw "Telemetry XLSX export did not produce file: $xlsxPath" }
  if ((Get-Item $xlsxPath).Length -lt 100) { throw "Telemetry XLSX export file too small: $xlsxPath" }

  Write-Host "[smoke] telemetry PDF export"
  $pdfPath = Join-Path $env:TEMP ("telemetry_{0}.pdf" -f $imei)
  $pdf = Invoke-Api -Method GET -Path ("/api/telemetry/export?imei={0}&format=pdf&from={1}&to={2}&packet_type=telemetry" -f $imei, [Uri]::EscapeDataString($fromIso), [Uri]::EscapeDataString($toIso)) -Token $token -OutFile $pdfPath
  Require-Status $pdf @(200)
  if (-not (Test-Path $pdfPath)) { throw "Telemetry PDF export did not produce file: $pdfPath" }
  if ((Get-Item $pdfPath).Length -lt 100) { throw "Telemetry PDF export file too small: $pdfPath" }

  Write-Host "[smoke] compliance report download"
  $outPath = Join-Path $env:TEMP ("compliance_{0}.pdf" -f $ProjectId)
  $cp = Invoke-Api -Method GET -Path ("/api/reports/{0}/compliance" -f $ProjectId) -Token $token -OutFile $outPath
  Require-Status $cp @(200)
  if (-not (Test-Path $outPath)) {
    throw "Compliance report download did not produce file: $outPath"
  }

  Write-Host "[smoke] ✅ PASSED" -ForegroundColor Green
  Write-Host "[smoke] compliance file: $outPath"
}
finally {
  Pop-Location

  if (-not $SkipComposeDown) {
    try {
      Push-Location $repoRoot
      Write-Host "[smoke] docker compose down"
      docker compose down | Out-Host
    } catch {
      Write-Warning "docker compose down failed: $($_.Exception.Message)"
    } finally {
      Pop-Location
    }
  }
}
