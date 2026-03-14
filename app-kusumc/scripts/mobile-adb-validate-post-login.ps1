param(
  [string]$PackageName = "com.autogridmobility.rmsmqtt1",
  [int]$TimeoutSec = 45,
  [int]$PollIntervalSec = 3,
  [string[]]$SuccessTexts = @("Home", "Logout", "PMKUSUM IoT Monitor"),
  [string[]]$FailureHints = @("Phone", "OTP")
)

$ErrorActionPreference = "Stop"

Write-Host "[ADB-POST-LOGIN] Checking connected devices..."
$adbDevices = adb devices
if (-not ($adbDevices | Select-String "\tdevice")) {
  throw "No Android device/emulator connected. Run 'adb devices' and connect one device."
}

$deadline = (Get-Date).AddSeconds($TimeoutSec)
$matchedText = $null

Write-Host "[ADB-POST-LOGIN] Waiting for authenticated UI markers..."
while ((Get-Date) -lt $deadline) {
  $xml = adb exec-out uiautomator dump /dev/tty 2>$null | Out-String

  foreach ($text in $SuccessTexts) {
    if (-not [string]::IsNullOrWhiteSpace($text) -and $xml -match [Regex]::Escape($text)) {
      $matchedText = $text
      break
    }
  }

  if ($matchedText) {
    break
  }

  Start-Sleep -Seconds $PollIntervalSec
}

if (-not $matchedText) {
  $xmlFinal = adb exec-out uiautomator dump /dev/tty 2>$null | Out-String
  $failureObserved = @()
  foreach ($hint in $FailureHints) {
    if (-not [string]::IsNullOrWhiteSpace($hint) -and $xmlFinal -match [Regex]::Escape($hint)) {
      $failureObserved += $hint
    }
  }

  if ($failureObserved.Count -gt 0) {
    throw "[ADB-POST-LOGIN] Timed out waiting for authenticated UI. Login hints still visible: $($failureObserved -join ', ')."
  }

  throw "[ADB-POST-LOGIN] Timed out waiting for authenticated UI markers: $($SuccessTexts -join ', ')."
}

Write-Host "[ADB-POST-LOGIN] Authenticated UI detected via marker: $matchedText"
