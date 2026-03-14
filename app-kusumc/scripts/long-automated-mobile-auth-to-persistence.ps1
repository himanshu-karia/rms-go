param(
  [string]$ServerBaseUrl = "https://rms-iot.local:7443",
  [string]$TimescaleUri = "postgres://postgres:password@localhost:5433/telemetry?sslmode=disable",
  [string]$AndroidProjectPath = "./RMSMQTT1 - L",
  [switch]$InstallDebug,
  [switch]$RunAdbSmoke,
  [switch]$RunAdbLoginOtpFlow,
  [switch]$RunAdbUiAudit,
  [string]$MobileLoginPhone = "9999999999",
  [string]$MobileLoginUser = "Him",
  [string]$MobileLoginPass = "0554",
  [string]$MobileApiBase = "https://rms-iot.local:7443/api/mobile",
  [string]$AuthApiBase = "https://rms-iot.local:7443/api",
  [string]$AuditProjectId = "pm-kusum-solar-pump-msedcl",
  [string]$AuditDeviceId = "869630050762180"
)

$ErrorActionPreference = "Stop"

function Run-Step($Name, [scriptblock]$Action) {
  Write-Host "`n=== $Name ===" -ForegroundColor Cyan
  & $Action
}

function Assert-LastExitCode($Context) {
  if ($LASTEXITCODE -ne 0) {
    throw "$Context failed with exit code $LASTEXITCODE"
  }
}

function Invoke-CommandWithRetry($Context, [scriptblock]$Action, [int]$Retries = 3, [int]$DelaySec = 5) {
  for ($attempt = 1; $attempt -le $Retries; $attempt++) {
    & $Action
    if ($LASTEXITCODE -eq 0) {
      return
    }
    if ($attempt -lt $Retries) {
      Write-Host "$Context failed (attempt $attempt/$Retries). Retrying in $DelaySec sec..." -ForegroundColor Yellow
      Start-Sleep -Seconds $DelaySec
    }
  }
  throw "$Context failed after $Retries attempts (last exit code $LASTEXITCODE)"
}

function Run-GoTestWithRetry($TestPattern, [int]$Retries = 2, [switch]$WarnOnly) {
  for ($attempt = 1; $attempt -le $Retries; $attempt++) {
    go test -tags=integration ./tests/e2e -run $TestPattern -count=1 -v
    if ($LASTEXITCODE -eq 0) {
      return $true
    }
    if ($attempt -lt $Retries) {
      Write-Host "Retrying $TestPattern (attempt $($attempt + 1)/$Retries)..." -ForegroundColor Yellow
      Start-Sleep -Seconds 3
    }
  }
  if ($WarnOnly) {
    Write-Warning "Test pattern $TestPattern failed after $Retries attempts (warn-only mode)."
    return $false
  }
  throw "Test pattern $TestPattern failed after $Retries attempts."
}

function Wait-BackendReady([int]$MaxAttempts = 30, [int]$SleepSec = 5) {
  Write-Host "Waiting for backend TLS endpoint on localhost:443 ..."
  for ($i = 1; $i -le $MaxAttempts; $i++) {
    try {
      $probe = Test-NetConnection -ComputerName "localhost" -Port 443 -WarningAction SilentlyContinue
      if ($probe.TcpTestSucceeded) {
        Write-Host "Backend port reachable (attempt $i)."
        Start-Sleep -Seconds 5
        return
      }
    } catch {
      # ignore until timeout
    }
    Start-Sleep -Seconds $SleepSec
  }
  throw "Backend TLS port did not become ready in time."
}

function Wait-DockerReady([int]$MaxAttempts = 24, [int]$SleepSec = 5) {
  Write-Host "Checking Docker daemon readiness ..."
  for ($i = 1; $i -le $MaxAttempts; $i++) {
    docker info *> $null
    if ($LASTEXITCODE -eq 0) {
      Write-Host "Docker daemon ready (attempt $i)."
      return
    }
    Start-Sleep -Seconds $SleepSec
  }
  throw "Docker daemon is not ready. Start Docker Desktop and ensure 'docker info' succeeds before rerunning this script."
}

Run-Step "Build and refresh backend stack" {
  Push-Location "../go-kusumc"
  Wait-DockerReady
  Remove-Item Env:TIMESCALE_URI -ErrorAction SilentlyContinue
  docker compose down --volumes
  Assert-LastExitCode "docker compose down"
  Invoke-CommandWithRetry -Context "docker compose up" -Retries 3 -DelaySec 8 -Action {
    docker compose up --build -d
  }
  Pop-Location
  Wait-BackendReady
}

Run-Step "Run backend user+device lifecycle E2E" {
  Push-Location "../go-kusumc"
  $env:BASE_URL = $ServerBaseUrl
  $env:TIMESCALE_URI = $TimescaleUri
  $env:HTTP_TLS_INSECURE = "true"
  $env:PROJECT_ID = "pm-kusum-solar-pump-msedcl"
  Run-GoTestWithRetry -TestPattern 'TestRMSMegaFlow|TestKusumFullCycle' -Retries 2
  Run-GoTestWithRetry -TestPattern 'TestDeviceCommandLifecycle' -Retries 2
  Pop-Location
}

Run-Step "Run backend mobile bridge E2E" {
  Push-Location "../go-kusumc"
  $env:BASE_URL = $ServerBaseUrl
  $env:TIMESCALE_URI = $TimescaleUri
  $env:HTTP_TLS_INSECURE = "true"
  $env:PROJECT_ID = "pm-kusum-solar-pump-msedcl"
  go test -tags=integration ./tests/e2e -run 'TestMobileIngest_IdempotencyReplay|TestMobileCommandStatus_Mapping' -count=1 -v
  Assert-LastExitCode "backend mobile bridge e2e"
  Pop-Location
}

Run-Step "Compile Android app and tests" {
  Push-Location $AndroidProjectPath
  .\gradlew.bat :app:assembleDebug :app:compileDebugAndroidTestKotlin :app:testDebugUnitTest --tests com.autogridmobility.rmsmqtt1.viewmodel.MobileAuthViewModelTest --console=plain --no-daemon
  Assert-LastExitCode "android compile/test"
  Pop-Location
}

if ($InstallDebug -or $RunAdbSmoke) {
  Run-Step "Install debug APK" {
    Push-Location $AndroidProjectPath
    $apk = Resolve-Path ".\app\build\outputs\apk\debug\app-debug.apk"
    adb install -r $apk.Path
    Assert-LastExitCode "adb install"
    Pop-Location
  }
}

if ($RunAdbSmoke) {
  Run-Step "ADB app smoke launch" {
    & "$PSScriptRoot\mobile-adb-smoke.ps1"
    Assert-LastExitCode "adb smoke"
  }
}

if ($RunAdbLoginOtpFlow) {
  Run-Step "ADB OTP login inject flow" {
    & "$PSScriptRoot\mobile-adb-inject-server-otp.ps1" `
      -Phone $MobileLoginPhone `
      -ApiBase $MobileApiBase `
      -AuthBase $AuthApiBase `
      -AuthUsername $MobileLoginUser `
      -AuthPassword $MobileLoginPass `
      -RequestOtpInApp `
      -RequestOtpFromServer `
      -LaunchApp `
      -PressEnterAfterInject
    Assert-LastExitCode "adb otp login inject"
  }
}

if ($RunAdbUiAudit) {
  Run-Step "ADB authenticated UI auditor flow" {
    & "$PSScriptRoot\mobile-ui-auditor.ps1" `
      -LaunchApp `
      -NavigateDrawer `
      -RunOnDemandCommand `
      -RunSimulationToggle `
      -AutoLoginWithBypass `
      -AutoLoginWithOtp `
      -OtpPhone $MobileLoginPhone `
      -MobileApiBase $MobileApiBase `
      -VerifyBackendCommandApi `
      -AuthBase $AuthApiBase `
      -AuthUsername $MobileLoginUser `
      -AuthPassword $MobileLoginPass `
      -CommandQueryBase $AuthApiBase `
      -ProjectId $AuditProjectId `
      -DeviceId $AuditDeviceId
    Assert-LastExitCode "adb ui auditor"
  }
}

Run-Step "Long automated chain complete" {
  Write-Host "Validated user-role APIs, device bootstrap/command/rotation lifecycle, mobile ingest path, and Android build/test readiness."
  Write-Host "For on-device UI automation extension, integrate UiAutomator or Maestro and invoke from this script."
}
