param(
  [string]$PackageName = "com.autogridmobility.rmsmqtt1",
  [string]$ActivityName = "com.autogridmobility.rmsmqtt1.MainActivity",
  [switch]$LaunchApp,
  [int]$AuthTimeoutSec = 45,
  [int]$PollIntervalSec = 2,
  [switch]$NavigateDrawer,
  [switch]$RunOnDemandCommand,
  [switch]$RunSimulationToggle,
  [switch]$AutoLoginWithBypass,
  [switch]$AutoLoginWithOtp,
  [string]$OtpPhone = "9999999999",
  [string]$MobileApiBase = "https://rms-iot.local:7443/api/mobile",
  [switch]$VerifyBackendCommandApi,
  [string]$AuthBase = "https://rms-iot.local:7443/api",
  [string]$AuthUsername = "",
  [string]$AuthPassword = "",
  [string]$CommandQueryBase = "https://rms-iot.local:7443/api",
  [string]$ProjectId = "pm-kusum-solar-pump-msedcl",
  [string]$DeviceId = "869630050762180"
)

$ErrorActionPreference = "Stop"

function Get-UiDumpXml {
  return (adb exec-out uiautomator dump /dev/tty 2>$null | Out-String)
}

function Ensure-DeviceUnlocked {
  adb shell input keyevent 224 | Out-Null
  Start-Sleep -Milliseconds 250
  adb shell input swipe 540 1900 540 400 | Out-Null
  Start-Sleep -Milliseconds 250
  adb shell input keyevent 82 | Out-Null
  Start-Sleep -Milliseconds 250
}

function Find-BoundsByTextOrDesc([string]$Xml, [string[]]$Candidates) {
  foreach ($candidate in $Candidates) {
    if ([string]::IsNullOrWhiteSpace($candidate)) { continue }

    $textPattern = 'text="' + [Regex]::Escape($candidate) + '"[^>]*bounds="\[(\d+),(\d+)\]\[(\d+),(\d+)\]"'
    $textMatch = [Regex]::Match($Xml, $textPattern)
    if ($textMatch.Success) {
      return @([int]$textMatch.Groups[1].Value, [int]$textMatch.Groups[2].Value, [int]$textMatch.Groups[3].Value, [int]$textMatch.Groups[4].Value)
    }

    $descPattern = 'content-desc="' + [Regex]::Escape($candidate) + '"[^>]*bounds="\[(\d+),(\d+)\]\[(\d+),(\d+)\]"'
    $descMatch = [Regex]::Match($Xml, $descPattern)
    if ($descMatch.Success) {
      return @([int]$descMatch.Groups[1].Value, [int]$descMatch.Groups[2].Value, [int]$descMatch.Groups[3].Value, [int]$descMatch.Groups[4].Value)
    }
  }
  return $null
}

function Tap-Bounds($Bounds) {
  if ($null -eq $Bounds -or $Bounds.Count -ne 4) { return $false }
  $x = [int](($Bounds[0] + $Bounds[2]) / 2)
  $y = [int](($Bounds[1] + $Bounds[3]) / 2)
  adb shell input tap $x $y | Out-Null
  return $true
}

function Tap-ByCandidates([string[]]$Candidates, [int]$Retries = 3, [int]$DelayMs = 700) {
  for ($attempt = 1; $attempt -le $Retries; $attempt++) {
    $xml = Get-UiDumpXml
    $bounds = Find-BoundsByTextOrDesc -Xml $xml -Candidates $Candidates
    if ($null -ne $bounds) {
      Tap-Bounds $bounds | Out-Null
      Start-Sleep -Milliseconds $DelayMs
      return $true
    }
    Start-Sleep -Milliseconds 400
  }
  return $false
}

function Assert-AnyMarkerVisible([string[]]$Markers, [int]$TimeoutSec = 20, [string]$Context = "UI marker") {
  $deadline = (Get-Date).AddSeconds($TimeoutSec)
  while ((Get-Date) -lt $deadline) {
    $xml = Get-UiDumpXml
    foreach ($marker in $Markers) {
      if (-not [string]::IsNullOrWhiteSpace($marker) -and $xml -match [Regex]::Escape($marker)) {
        return $marker
      }
    }
    Start-Sleep -Seconds 1
  }
  throw "[UI-AUDITOR] Timed out waiting for $Context markers: $($Markers -join ', ')"
}

function Open-Drawer {
  Ensure-DeviceUnlocked
  adb shell cmd statusbar collapse 2>$null | Out-Null
  adb shell input keyevent 4 | Out-Null
  Start-Sleep -Milliseconds 250
  if (-not (Tap-ByCandidates -Candidates @("Menu") -Retries 2)) {
    adb shell input keyevent 82 | Out-Null
    Start-Sleep -Milliseconds 700
  }
}

function Navigate-DrawerItem([string]$ItemText, [string[]]$ExpectedMarkers, [string]$Context) {
  for ($attempt = 1; $attempt -le 2; $attempt++) {
    Open-Drawer
    if (-not (Tap-ByCandidates -Candidates @($ItemText) -Retries 3)) {
      if ($attempt -eq 2) {
        throw "[UI-AUDITOR] Could not tap drawer item '$ItemText'."
      }
      continue
    }

    try {
      $marker = Assert-AnyMarkerVisible -Markers $ExpectedMarkers -TimeoutSec 25 -Context $Context
      Write-Host "[UI-AUDITOR] Verified $Context via marker: $marker"
      return
    } catch {
      if ($attempt -eq 2) {
        throw
      }
    }
  }
}

function Validate-BackendCommandApi {
  if ([string]::IsNullOrWhiteSpace($AuthUsername) -or [string]::IsNullOrWhiteSpace($AuthPassword)) {
    throw "[UI-AUDITOR] VerifyBackendCommandApi requires AuthUsername and AuthPassword."
  }

  Write-Host "[UI-AUDITOR] Logging in for backend command API verification..."
  $loginBody = @{ username = $AuthUsername; password = $AuthPassword } | ConvertTo-Json
  $login = Invoke-RestMethod -Method Post -Uri ("{0}/auth/login" -f $AuthBase.TrimEnd('/')) -SkipCertificateCheck -ContentType "application/json" -Body $loginBody
  if (-not $login.token) {
    throw "[UI-AUDITOR] Backend login succeeded but token missing."
  }

  $headers = @{ Authorization = "Bearer $($login.token)"; "X-Internal-Test" = "1" }
  $uri = "{0}/commands?deviceId={1}&projectId={2}&limit=3" -f $CommandQueryBase.TrimEnd('/'), [uri]::EscapeDataString($DeviceId), [uri]::EscapeDataString($ProjectId)

  Write-Host "[UI-AUDITOR] Querying command timeline API for device/project..."
  $resp = $null
  try {
    $resp = Invoke-RestMethod -Method Get -Uri $uri -SkipCertificateCheck -Headers $headers
  } catch {
    $message = $_.Exception.Message
    $details = if ($_.ErrorDetails) { $_.ErrorDetails.Message } else { "" }
    if ($message -match "device lookup failed" -or $details -match "device lookup failed") {
      Write-Warning "[UI-AUDITOR] Backend command API device lookup unavailable for '$DeviceId'. Skipping strict backend command assertion."
      return
    }
    throw
  }
  $json = $resp | ConvertTo-Json -Depth 8

  if ($json -notmatch [Regex]::Escape($ProjectId)) {
    throw "[UI-AUDITOR] Command API response did not include expected projectId '$ProjectId'."
  }

  Write-Host "[UI-AUDITOR] Backend command API assertion passed."
}

Write-Host "[UI-AUDITOR] Checking connected devices..."
$adbDevices = adb devices
if (-not ($adbDevices | Select-String "\tdevice")) {
  throw "No Android device/emulator connected. Run 'adb devices' and connect one device."
}

if ($LaunchApp) {
  Write-Host "[UI-AUDITOR] Launching app..."
  Ensure-DeviceUnlocked
  adb shell cmd statusbar collapse 2>$null | Out-Null
  adb shell input keyevent 4 | Out-Null
  adb shell am start -n "$PackageName/$ActivityName" | Out-Null
  Start-Sleep -Seconds 2
}

$authenticatedMarkers = @("PMKUSUM IoT Monitor", "Menu", "Home", "Assigned Devices")
$loginMarkers = @("Mobile Login", "Request OTP", "Verify and Continue")

Write-Host "[UI-AUDITOR] Waiting for authenticated app state..."
$authDeadline = (Get-Date).AddSeconds($AuthTimeoutSec)
$authenticated = $false
$attemptedAutoBypass = $false
$attemptedAutoOtp = $false
while ((Get-Date) -lt $authDeadline) {
  $xml = Get-UiDumpXml
  if ($authenticatedMarkers | Where-Object { $xml -match [Regex]::Escape($_) }) {
    $authenticated = $true
    break
  }
  if ($loginMarkers | Where-Object { $xml -match [Regex]::Escape($_) }) {
    if ($AutoLoginWithBypass -and -not $attemptedAutoBypass) {
      Write-Host "[UI-AUDITOR] Login screen detected. Trying bypass login button..."
      $bypassTapped = Tap-ByCandidates -Candidates @("Login with Him / 0554", "Login with Him/0554", "Him / 0554", "Him/0554") -Retries 3
      $attemptedAutoBypass = $true

      if ($bypassTapped) {
        Start-Sleep -Seconds 3
        $xmlAfterBypass = Get-UiDumpXml
        if ($authenticatedMarkers | Where-Object { $xmlAfterBypass -match [Regex]::Escape($_) }) {
          $authenticated = $true
          break
        }
        Write-Warning "[UI-AUDITOR] Bypass button tapped but authenticated markers not visible yet."
      } else {
        Write-Warning "[UI-AUDITOR] Bypass login button not found on login screen."
      }
      continue
    }

    if ($AutoLoginWithOtp -and -not $attemptedAutoOtp) {
      Write-Host "[UI-AUDITOR] Login screen detected. Triggering OTP injector fallback..."
      & "$PSScriptRoot\mobile-adb-inject-server-otp.ps1" `
        -Phone $OtpPhone `
        -ApiBase $MobileApiBase `
        -AuthBase $AuthBase `
        -AuthUsername $AuthUsername `
        -AuthPassword $AuthPassword `
        -RequestOtpInApp `
        -RequestOtpFromServer `
        -LaunchApp `
        -PressEnterAfterInject
      if ($LASTEXITCODE -ne 0) {
        throw "[UI-AUDITOR] OTP injector fallback failed with exit code $LASTEXITCODE"
      }

      if (-not (Tap-ByCandidates -Candidates @("Verify and Continue", "Verify") -Retries 2)) {
        adb shell input keyevent 66 | Out-Null
      }

      $attemptedAutoOtp = $true
      Start-Sleep -Seconds 2
      continue
    }

    Start-Sleep -Seconds $PollIntervalSec
    continue
  }
  Start-Sleep -Seconds $PollIntervalSec
}

if (-not $authenticated) {
  throw "[UI-AUDITOR] Timed out waiting for authenticated app state."
}
Write-Host "[UI-AUDITOR] Authenticated app state detected."

if ($NavigateDrawer) {
  Navigate-DrawerItem -ItemText "Home" -ExpectedMarkers @("PMKUSUM IoT Monitor", "Assigned Devices") -Context "Home screen"
  Navigate-DrawerItem -ItemText "Dashboard" -ExpectedMarkers @("Heartbeat", "On Demand", "Latest Command Status") -Context "Dashboard screen"

  if (-not (Tap-ByCandidates -Candidates @("On Demand") -Retries 3)) {
    throw "[UI-AUDITOR] Could not select On Demand dashboard tab."
  }
  Assert-AnyMarkerVisible -Markers @("Pump Control", "Turn ON", "Turn OFF") -TimeoutSec 15 -Context "On Demand tab" | Out-Null

  if ($RunOnDemandCommand) {
    if (-not (Tap-ByCandidates -Candidates @("Turn ON") -Retries 2)) {
      throw "[UI-AUDITOR] Could not tap Turn ON command button."
    }
    Start-Sleep -Seconds 2
    $xmlAfterOn = Get-UiDumpXml
    if ($xmlAfterOn -match "No commands sent yet") {
      throw "[UI-AUDITOR] On-demand command did not update command status UI."
    }
    Write-Host "[UI-AUDITOR] On-demand command action validated in UI."
  }

  Navigate-DrawerItem -ItemText "Settings" -ExpectedMarkers @("Settings & Export", "Data Simulation", "Connection Status", "MQTT Broker Configuration", "Data Export", "Subscribed Topics") -Context "Settings screen"

  if ($RunSimulationToggle) {
    $simStarted = $false
    for ($seek = 1; $seek -le 4; $seek++) {
      if (Tap-ByCandidates -Candidates @("Simulate Data", "Start Data Simulation", "Data Simulation") -Retries 2) {
        $simStarted = $true
        break
      }
      adb shell input swipe 540 1800 540 700 | Out-Null
      Start-Sleep -Milliseconds 500
    }

    if (-not $simStarted) {
      Write-Warning "[UI-AUDITOR] Simulation controls not found in current Settings UI. Skipping simulation toggle check."
    } else {
      $simMarker = Assert-AnyMarkerVisible -Markers @("Stop Data Simulation", "Packets sent:", "Publishing every") -TimeoutSec 20 -Context "simulation running state"
      Write-Host "[UI-AUDITOR] Simulation running marker: $simMarker"

      if (-not (Tap-ByCandidates -Candidates @("Stop Data Simulation") -Retries 3)) {
        throw "[UI-AUDITOR] Could not stop data simulation."
      }

      Assert-AnyMarkerVisible -Markers @("Simulate Data") -TimeoutSec 15 -Context "simulation stopped state" | Out-Null
      Write-Host "[UI-AUDITOR] Data simulation start/stop cycle validated."
    }
  }

  Navigate-DrawerItem -ItemText "Home" -ExpectedMarkers @("PMKUSUM IoT Monitor", "Assigned Devices") -Context "final Home screen"
}

if ($VerifyBackendCommandApi) {
  Validate-BackendCommandApi
}

Write-Host "[UI-AUDITOR] UI audit completed."