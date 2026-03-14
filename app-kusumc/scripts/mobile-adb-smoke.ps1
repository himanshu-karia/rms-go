param(
  [string]$PackageName = "com.autogridmobility.rmsmqtt1",
  [string]$ActivityName = "com.autogridmobility.rmsmqtt1.MainActivity",
  [int]$LaunchWaitSec = 8
)

$ErrorActionPreference = "Stop"

Write-Host "[ADB] Checking connected devices..."
$adbDevices = adb devices
if (-not ($adbDevices | Select-String "\tdevice")) {
  throw "No Android device/emulator connected. Run 'adb devices' and connect one device."
}

Write-Host "[ADB] Force-stopping app (if running)..."
adb shell am force-stop $PackageName | Out-Null

Write-Host "[ADB] Clearing previous logs..."
adb logcat -c | Out-Null

Write-Host "[ADB] Launching app..."
adb shell am start -n "$PackageName/$ActivityName" | Out-Null
Start-Sleep -Seconds $LaunchWaitSec

Write-Host "[ADB] Capturing top activity and basic logs..."
$activityLines = adb shell dumpsys activity activities | Select-String $PackageName | Select-Object -First 10
$activityLines

if (-not $activityLines) {
  throw "[ADB] App activity not visible after launch: $PackageName"
}

$logLines = adb logcat -d | Select-String "RMSMQTT1|Mobile|Auth|Sync|MQTT|AndroidRuntime|FATAL EXCEPTION|NoSuchMethodException|Cannot create an instance of class|$PackageName" | Select-Object -Last 120
$logLines

$fatalCrash = $logLines | Where-Object {
  $_.Line -match $PackageName -and $_.Line -match "AndroidRuntime|FATAL EXCEPTION|NoSuchMethodException|Cannot create an instance of class"
}

if ($fatalCrash) {
  throw "[ADB] Fatal crash detected for $PackageName during smoke launch."
}

Write-Host "[ADB] Smoke launch complete."
