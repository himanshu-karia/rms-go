param(
  [string]$BaseUrl = "https://localhost:7443/api",
  [string]$Username = "Him",
  [string]$Password = "0554"
)

$ErrorActionPreference = "Stop"

function Assert-True {
  param(
    [bool]$Condition,
    [string]$Message
  )

  if (-not $Condition) {
    throw "ASSERT FAILED: $Message"
  }
}

function Get-AuthToken {
  $body = @{ username = $Username; password = $Password } | ConvertTo-Json
  $res = Invoke-RestMethod -Uri "$BaseUrl/auth/login" -Method Post -ContentType "application/json" -Body $body -SkipCertificateCheck
  Assert-True ([bool]$res.token) "Login token missing"
  return [string]$res.token
}

function Get-StatusCode {
  param(
    [string]$Url,
    [hashtable]$Headers,
    [string]$Method = "GET",
    [string]$ContentType,
    [string]$Body
  )

  try {
    $invokeParams = @{
      Uri = $Url
      Headers = $Headers
      Method = $Method
      SkipCertificateCheck = $true
      UseBasicParsing = $true
    }
    if ($ContentType) {
      $invokeParams["ContentType"] = $ContentType
    }
    if ($null -ne $Body) {
      $invokeParams["Body"] = $Body
    }

    $resp = Invoke-WebRequest @invokeParams
    return [int]$resp.StatusCode
  }
  catch {
    if ($_.Exception.Response -and $_.Exception.Response.StatusCode) {
      return [int]$_.Exception.Response.StatusCode
    }
    throw
  }
}

function Add-OptionalEndpointCheck {
  param(
    [System.Collections.Generic.List[object]]$Results,
    [string]$Name,
    [string]$Url,
    [hashtable]$Headers,
    [string]$Method = "GET",
    [string]$ContentType,
    [string]$Body,
    [int[]]$AllowedStatuses = @()
  )

  $status = Get-StatusCode -Url $Url -Headers $Headers -Method $Method -ContentType $ContentType -Body $Body
  if ($status -eq 200) {
    $Results.Add([pscustomobject]@{ check = $Name; status = "PASS"; detail = "200 OK" })
    return
  }

  if ($AllowedStatuses -contains $status) {
    $Results.Add([pscustomobject]@{ check = $Name; status = "PASS"; detail = "allowed status $status" })
    return
  }

  if ($status -eq 401 -or $status -eq 403) {
    $Results.Add([pscustomobject]@{ check = $Name; status = "WARN"; detail = "permission-gated ($status)" })
    return
  }

  $Results.Add([pscustomobject]@{ check = $Name; status = "FAIL"; detail = "unexpected status $status" })
  throw "ASSERT FAILED: $Name returned unexpected status $status"
}

function Get-AnyDeviceUuid {
  param([hashtable]$Headers)

  $res = Invoke-RestMethod -Uri "$BaseUrl/devices?limit=1" -Headers $Headers -SkipCertificateCheck
  if ($res.devices -and $res.devices.Count -gt 0) {
    $candidate = $res.devices[0].uuid
    if (-not $candidate) {
      $candidate = $res.devices[0].id
    }
    if ($candidate) {
      return [string]$candidate
    }
  }

  return "00000000-0000-0000-0000-000000000000"
}

function Get-AnyStateId {
  param([hashtable]$Headers)
  $states = Invoke-RestMethod -Uri "$BaseUrl/admin/states" -Headers $Headers -SkipCertificateCheck
  if ($states -is [System.Array]) {
    Assert-True ($states.Count -gt 0) "No states found"
    return [string]$states[0].id
  }
  if ($states.states) {
    Assert-True ($states.states.Count -gt 0) "No states in states envelope"
    return [string]$states.states[0].id
  }
  throw "Unknown states response shape"
}

function Get-AnyAuthorityId {
  param(
    [hashtable]$Headers,
    [string]$StateId
  )

  $res = Invoke-RestMethod -Uri "$BaseUrl/admin/state-authorities?state_id=$StateId" -Headers $Headers -SkipCertificateCheck
  if ($res -is [System.Array]) {
    Assert-True ($res.Count -gt 0) "No authorities found"
    return [string]$res[0].id
  }
  if ($res.state_authorities) {
    Assert-True ($res.state_authorities.Count -gt 0) "No authorities in state_authorities envelope"
    return [string]$res.state_authorities[0].id
  }
  if ($res.stateAuthorities) {
    Assert-True ($res.stateAuthorities.Count -gt 0) "No authorities in stateAuthorities envelope"
    return [string]$res.stateAuthorities[0].id
  }
  throw "Unknown authorities response shape"
}

function Get-AnyProjectId {
  param(
    [hashtable]$Headers,
    [string]$AuthorityId
  )

  $res = Invoke-RestMethod -Uri "$BaseUrl/admin/projects?state_authority_id=$AuthorityId" -Headers $Headers -SkipCertificateCheck
  if ($res -is [System.Array]) {
    Assert-True ($res.Count -gt 0) "No projects found"
    return [string]$res[0].id
  }
  if ($res.projects) {
    Assert-True ($res.projects.Count -gt 0) "No projects in projects envelope"
    return [string]$res.projects[0].id
  }
  throw "Unknown projects response shape"
}

$results = [System.Collections.Generic.List[object]]::new()

try {
  $token = Get-AuthToken
  $headers = @{ Authorization = "Bearer $token" }
  $results.Add([pscustomobject]@{ check = "auth.login"; status = "PASS"; detail = "token issued" })

  $stateId = Get-AnyStateId -Headers $headers
  $results.Add([pscustomobject]@{ check = "admin.states.list"; status = "PASS"; detail = "state_id=$stateId" })

  $authorityName = "SmokeAuthority-$(Get-Date -Format 'HHmmss')"
  $camelBody = @{ stateId = $stateId; name = $authorityName; metadata = @{ source = "smoke"; mode = "camel" } } | ConvertTo-Json -Depth 5
  $createdCamel = Invoke-RestMethod -Uri "$BaseUrl/admin/state-authorities" -Method Post -Headers $headers -ContentType "application/json" -Body $camelBody -SkipCertificateCheck
  Assert-True ([bool]$createdCamel.authority) "authority key missing in camel create response"
  Assert-True ([bool]$createdCamel.state_authority) "state_authority key missing in camel create response"
  $results.Add([pscustomobject]@{ check = "admin.state-authorities.create.camel"; status = "PASS"; detail = "authority + state_authority present" })

  $authorityName2 = "SmokeAuthoritySnake-$(Get-Date -Format 'HHmmss')"
  $snakeBody = @{ state_id = $stateId; name = $authorityName2; contact_info = @{ source = "smoke"; mode = "snake" } } | ConvertTo-Json -Depth 5
  $createdSnake = Invoke-RestMethod -Uri "$BaseUrl/admin/state-authorities" -Method Post -Headers $headers -ContentType "application/json" -Body $snakeBody -SkipCertificateCheck
  Assert-True ([bool]$createdSnake.authority) "authority key missing in snake create response"
  $results.Add([pscustomobject]@{ check = "admin.state-authorities.create.snake"; status = "PASS"; detail = "snake payload accepted" })

  $authorityId = Get-AnyAuthorityId -Headers $headers -StateId $stateId
  $projectId = Get-AnyProjectId -Headers $headers -AuthorityId $authorityId
  $protocols = Invoke-RestMethod -Uri "$BaseUrl/admin/protocol-versions?state_id=$stateId&state_authority_id=$authorityId&project_id=$projectId" -Headers $headers -SkipCertificateCheck
  Assert-True ($null -ne $protocols.protocol_versions) "protocol_versions envelope missing"
  Assert-True ($null -ne $protocols.protocolVersions) "protocolVersions envelope missing"
  $results.Add([pscustomobject]@{ check = "admin.protocol-versions.list.envelopes"; status = "PASS"; detail = "snake + camel list keys present" })

  $uuid = "00000000-0000-0000-0000-000000000000"
  $legacy = Invoke-WebRequest -Uri "$BaseUrl/devices/$uuid/govt-creds" -Headers $headers -SkipCertificateCheck -UseBasicParsing
  Assert-True ($legacy.Headers["Deprecation"] -contains "true") "legacy govt-creds deprecation header missing"
  $results.Add([pscustomobject]@{ check = "devices.govt-creds.deprecation"; status = "PASS"; detail = "legacy deprecation headers present" })

  $canonical = Invoke-WebRequest -Uri "$BaseUrl/devices/$uuid/government-credentials/history" -Headers $headers -SkipCertificateCheck -UseBasicParsing
  Assert-True ($null -eq $canonical.Headers["Deprecation"]) "canonical endpoint should not be deprecated"
  $results.Add([pscustomobject]@{ check = "devices.government-credentials.canonical"; status = "PASS"; detail = "canonical path not deprecated" })

  # Optional route checks mapped to UI pages (PASS on 200, WARN on 401/403).
  Add-OptionalEndpointCheck -Results $results -Name "ui.rules-alerts.rules" -Url "$BaseUrl/rules" -Headers $headers -AllowedStatuses @(400)
  Add-OptionalEndpointCheck -Results $results -Name "ui.rules-alerts.alerts" -Url "$BaseUrl/alerts" -Headers $headers
  Add-OptionalEndpointCheck -Results $results -Name "ui.command-catalog" -Url "$BaseUrl/commands/catalog-admin" -Headers $headers -AllowedStatuses @(400)
  Add-OptionalEndpointCheck -Results $results -Name "ui.scheduler" -Url "$BaseUrl/scheduler/schedules" -Headers $headers
  Add-OptionalEndpointCheck -Results $results -Name "ui.device-inventory" -Url "$BaseUrl/devices?limit=1" -Headers $headers

  # M3 expansion: telemetry, installations, and user-group pages.
  $deviceUuid = Get-AnyDeviceUuid -Headers $headers
  Add-OptionalEndpointCheck -Results $results -Name "ui.telemetry.history" -Url "$BaseUrl/telemetry/devices/$deviceUuid/history?limit=1" -Headers $headers -AllowedStatuses @(400,404)
  Add-OptionalEndpointCheck -Results $results -Name "ui.telemetry.live-token" -Url "$BaseUrl/telemetry/devices/$deviceUuid/live-token" -Headers $headers -Method "POST" -ContentType "application/json" -Body "{}" -AllowedStatuses @(400,404)
  Add-OptionalEndpointCheck -Results $results -Name "ui.installations.list" -Url "$BaseUrl/installations?limit=1" -Headers $headers
  Add-OptionalEndpointCheck -Results $results -Name "ui.user-groups.list" -Url "$BaseUrl/user-groups" -Headers $headers

  $results | Format-Table -AutoSize | Out-String | Write-Output
  Write-Output "ALL CHECKS PASSED"
}
catch {
  $results.Add([pscustomobject]@{ check = "runtime"; status = "FAIL"; detail = $_.Exception.Message })
  $results | Format-Table -AutoSize | Out-String | Write-Output
  throw
}
