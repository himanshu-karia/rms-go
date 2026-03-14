param(
  [Parameter(Mandatory = $true)]
  [string]$Phone,
  [string]$ApiBase = "https://rms-iot.local:7443/api/mobile",
  [string]$AuthBase = "https://rms-iot.local:7443/api",
  [string]$AuthToken = "",
  [string]$AuthUsername = "",
  [string]$AuthPassword = "",
  [string]$PackageName = "com.autogridmobility.rmsmqtt1",
  [string]$ActivityName = "com.autogridmobility.rmsmqtt1.MainActivity",
  [int]$TimeoutSec = 120,
  [int]$PollIntervalSec = 2,
  [switch]$LaunchApp,
  [switch]$RequestOtpInApp,
  [switch]$RequestOtpFromServer,
  [switch]$PressEnterAfterInject,
  [switch]$ValidateOtpConsumed,
  [int]$ConsumeTimeoutSec = 25
)

$ErrorActionPreference = "Stop"
$appOtpUiReady = $false

Write-Host "[OTP-INJECT] Checking connected devices..."
$adbDevices = adb devices
if (-not ($adbDevices | Select-String "\tdevice")) {
  throw "No Android device/emulator connected. Run 'adb devices' and connect one device."
}

function Get-UiDumpXml {
  return (adb exec-out uiautomator dump /dev/tty 2>$null | Out-String)
}

function Get-AppUiDumpXml([int]$WaitSec = 10, [int]$PollMs = 500) {
  $deadline = (Get-Date).AddSeconds($WaitSec)
  while ((Get-Date) -lt $deadline) {
    $xml = Get-UiDumpXml
    if ($xml -match [Regex]::Escape("package=`"$PackageName`"")) {
      return $xml
    }
    Ensure-AppForeground
    Start-Sleep -Milliseconds $PollMs
  }
  return Get-UiDumpXml
}

function Find-BoundsByText([string]$Xml, [string[]]$Candidates) {
  foreach ($candidate in $Candidates) {
    if ([string]::IsNullOrWhiteSpace($candidate)) { continue }
    $pattern = 'text="' + [Regex]::Escape($candidate) + '"[^>]*bounds="\[(\d+),(\d+)\]\[(\d+),(\d+)\]"'
    $m = [Regex]::Match($Xml, $pattern)
    if ($m.Success) {
      return @([int]$m.Groups[1].Value, [int]$m.Groups[2].Value, [int]$m.Groups[3].Value, [int]$m.Groups[4].Value)
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

function Clear-FocusedInput([int]$MaxChars = 96) {
  adb shell input keyevent 123 | Out-Null
  Start-Sleep -Milliseconds 120
  for ($i = 1; $i -le $MaxChars; $i++) {
    adb shell input keyevent 67 | Out-Null
  }
}

function Ensure-DeviceUnlocked {
  adb shell input keyevent 224 | Out-Null
  Start-Sleep -Milliseconds 250
  adb shell input swipe 540 1900 540 400 | Out-Null
  Start-Sleep -Milliseconds 250
  adb shell input keyevent 82 | Out-Null
  Start-Sleep -Milliseconds 250
}

function Ensure-AppForeground {
  Ensure-DeviceUnlocked
  adb shell cmd statusbar collapse 2>$null | Out-Null
  adb shell input keyevent 4 | Out-Null
  Start-Sleep -Milliseconds 250
  adb shell am start -n "$PackageName/$ActivityName" | Out-Null
  Start-Sleep -Milliseconds 600
}

function Wait-ForOtpFieldBounds([int]$WaitSec = 18, [int]$PollMs = 500) {
  $deadline = (Get-Date).AddSeconds($WaitSec)
  while ((Get-Date) -lt $deadline) {
    $xml = Get-AppUiDumpXml -WaitSec 3 -PollMs 300
    $bounds = Find-BoundsByText -Xml $xml -Candidates @("OTP")
    if ($null -ne $bounds) {
      return $bounds
    }
    Start-Sleep -Milliseconds $PollMs
  }
  return $null
}

function Get-VisibleAuthError([string]$Xml) {
  $patterns = @(
    "Failed to request OTP",
    "Request failed with status",
    "Unable to resolve host",
    "SSL",
    "Cleartext HTTP is not allowed",
    "Network",
    "Missing Authorization Header"
  )
  foreach ($pattern in $patterns) {
    if ($Xml -match [Regex]::Escape($pattern)) {
      return $pattern
    }
  }
  return ""
}

if ($LaunchApp -or $RequestOtpInApp) {
  Write-Host "[OTP-INJECT] Launching app..."
  Ensure-AppForeground
  Start-Sleep -Seconds 1
}

$headers = @{ "X-Internal-Test" = "1" }
if (-not [string]::IsNullOrWhiteSpace($AuthToken)) {
  $headers["Authorization"] = "Bearer $AuthToken"
} elseif (-not [string]::IsNullOrWhiteSpace($AuthUsername) -and -not [string]::IsNullOrWhiteSpace($AuthPassword)) {
  Write-Host "[OTP-INJECT] Logging in to acquire auth token..."
  $loginBody = @{ username = $AuthUsername; password = $AuthPassword } | ConvertTo-Json
  $login = Invoke-RestMethod -Method Post -Uri ("{0}/auth/login" -f $AuthBase.TrimEnd('/')) -SkipCertificateCheck -ContentType "application/json" -Body $loginBody
  if (-not $login.token) {
    throw "Login succeeded but token missing in response."
  }
  $headers["Authorization"] = "Bearer $($login.token)"
}

if ($RequestOtpInApp) {
  Write-Host "[OTP-INJECT] Requesting OTP from app UI..."
  Ensure-AppForeground
  $xmlBefore = Get-AppUiDumpXml -WaitSec 6 -PollMs 300
  $phoneBounds = Find-BoundsByText -Xml $xmlBefore -Candidates @("Phone")
  if ($null -ne $phoneBounds) {
    Tap-Bounds $phoneBounds | Out-Null
    Start-Sleep -Milliseconds 300
  }
  Clear-FocusedInput
  adb shell input text $Phone | Out-Null
  Start-Sleep -Milliseconds 300

  $xmlAfterPhone = Get-AppUiDumpXml -WaitSec 6 -PollMs 300
  $requestBounds = Find-BoundsByText -Xml $xmlAfterPhone -Candidates @("Request OTP", "Resend OTP")
  if ($null -eq $requestBounds) {
    Write-Warning "Could not find Request OTP button in current UI. Falling back to server OTP request."
    $RequestOtpFromServer = $true
  } else {
    Tap-Bounds $requestBounds | Out-Null
    Start-Sleep -Milliseconds 500
    adb shell input keyevent 66 | Out-Null
    Start-Sleep -Milliseconds 400
    Tap-Bounds $requestBounds | Out-Null
    Start-Sleep -Milliseconds 600

    $otpBoundsAfterRequest = Wait-ForOtpFieldBounds -WaitSec 16
    if ($null -ne $otpBoundsAfterRequest) {
      Write-Host "[OTP-INJECT] OTP field became visible after Request OTP."
      $appOtpUiReady = $true
    } else {
      $xmlAfterReq = Get-AppUiDumpXml -WaitSec 4 -PollMs 300
      $authErr = Get-VisibleAuthError -Xml $xmlAfterReq
      if (-not [string]::IsNullOrWhiteSpace($authErr)) {
        Write-Warning "OTP field not visible after Request OTP click. App error marker: $authErr"
      } else {
        Write-Warning "OTP field not visible yet after Request OTP click."
      }
    }
  }
}

if ($RequestOtpFromServer -and -not $appOtpUiReady) {
  Write-Host "[OTP-INJECT] Requesting OTP from server..."
  $requestBody = @{ phone = $Phone; device_fingerprint = "adb-internal-test"; device_name = "adb"; app_version = "1.0.0" } | ConvertTo-Json
  Invoke-RestMethod -Method Post -Uri ("{0}/auth/request-otp" -f $ApiBase.TrimEnd('/')) -SkipCertificateCheck -ContentType "application/json" -Body $requestBody -Headers $headers | Out-Null
}

$otpUrl = "{0}/auth/dev-otp/latest?phone={1}" -f $ApiBase.TrimEnd('/'), [uri]::EscapeDataString($Phone)
$deadline = (Get-Date).AddSeconds($TimeoutSec)
$otp = $null
$otpRef = $null

Write-Host "[OTP-INJECT] Polling server-generated OTP for phone $Phone ..."
while ((Get-Date) -lt $deadline) {
  try {
    $resp = Invoke-RestMethod -Method Get -Uri $otpUrl -SkipCertificateCheck -Headers $headers -TimeoutSec 8
    if ($resp -and $resp.otp) {
      $otp = [string]$resp.otp
      $otpRef = [string]$resp.otp_ref
      break
    }
  } catch {
    Start-Sleep -Seconds $PollIntervalSec
    continue
  }
  Start-Sleep -Seconds $PollIntervalSec
}

if ([string]::IsNullOrWhiteSpace($otp)) {
  throw "No active OTP found for phone $Phone within timeout. Request OTP in app first, then rerun this script."
}

Write-Host "[OTP-INJECT] Found OTP ref: $otpRef"
Write-Host "[OTP-INJECT] Injecting OTP via ADB input text..."

$otpBounds = Wait-ForOtpFieldBounds -WaitSec 20
if ($null -eq $otpBounds) {
  throw "[OTP-INJECT] OTP field is not visible. Current screen is likely still phone entry (Request OTP). OTP will not be injected into phone field."
}

Tap-Bounds $otpBounds | Out-Null
Start-Sleep -Milliseconds 300

Clear-FocusedInput
adb shell input text $otp | Out-Null

if ($PressEnterAfterInject) {
  adb shell input keyevent 4 | Out-Null
  Start-Sleep -Milliseconds 300

  $verifyTapped = $false
  for ($i = 1; $i -le 3; $i++) {
    $xmlBeforeVerify = Get-UiDumpXml
    $verifyBounds = Find-BoundsByText -Xml $xmlBeforeVerify -Candidates @("Verify and Continue", "Verify")
    if ($null -ne $verifyBounds) {
      Tap-Bounds $verifyBounds | Out-Null
      $verifyTapped = $true
      break
    }
    Start-Sleep -Milliseconds 300
  }

  if (-not $verifyTapped) {
    adb shell input keyevent 66 | Out-Null
  }
}

if ($ValidateOtpConsumed) {
  Write-Host "[OTP-INJECT] Validating OTP consumption after verify action..."
  $consumeDeadline = (Get-Date).AddSeconds($ConsumeTimeoutSec)
  $consumed = $false
  while ((Get-Date) -lt $consumeDeadline) {
    try {
      $latest = Invoke-RestMethod -Method Get -Uri $otpUrl -SkipCertificateCheck -Headers $headers -TimeoutSec 6
      if (-not $latest -or -not $latest.otp_ref -or [string]$latest.otp_ref -ne $otpRef) {
        $consumed = $true
        break
      }
    } catch {
      $message = $_.Exception.Message
      $details = if ($_.ErrorDetails) { $_.ErrorDetails.Message } else { "" }
      if ($message -match "404" -or $details -match "mobile_otp_not_found") {
        $consumed = $true
        break
      }
    }
    Start-Sleep -Seconds 1
  }

  if (-not $consumed) {
    throw "[OTP-INJECT] OTP was not consumed after verify action. App may still be on login/OTP step."
  }
  Write-Host "[OTP-INJECT] OTP consumption validated."
}

Write-Host "[OTP-INJECT] OTP injected."
Write-Host "[OTP-INJECT] If verify button is visible, tap it now (or run with -PressEnterAfterInject)."
