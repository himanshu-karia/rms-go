param(
    [string]$RouteComparePath = "../route-compare.md",
    [string]$ParityMatrixPath = "01-parity-audit-matrix.md",
    [int]$GoExtractedCount = 382,
    [switch]$WriteBack
)

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$routeCompareFullPath = Join-Path $scriptDir $RouteComparePath
$parityMatrixFullPath = Join-Path $scriptDir $ParityMatrixPath

if (-not (Test-Path $routeCompareFullPath)) {
    throw "route-compare not found: $routeCompareFullPath"
}
if (-not (Test-Path $parityMatrixFullPath)) {
    throw "parity matrix not found: $parityMatrixFullPath"
}

$lines = Get-Content $routeCompareFullPath
$rows = @()
foreach ($line in $lines) {
    if ($line -notmatch '^\|') { continue }
    $parts = $line.Split('|')
    if ($parts.Length -lt 5) { continue }

    $legacy = $parts[1].Trim()
    $goMatch = $parts[3].Trim()
    $status = $parts[4].Trim()

    if ($legacy -eq '' -or $legacy -eq 'Legacy Route' -or $legacy -eq '---') { continue }

    $rows += [pscustomobject]@{
        Legacy = $legacy
        GoMatch = $goMatch
        Status = $status
    }
}

$legacyCount = $rows.Count
$exact = ($rows | Where-Object { $_.Status -eq 'exact' }).Count
$missing = ($rows | Where-Object { $_.Status -eq 'missing' }).Count
$aliasedProxy = ($rows | Where-Object { $_.GoMatch -match ',' -or $_.GoMatch -match '\balias\b' }).Count
$extra = $GoExtractedCount - $legacyCount

$metricsBlock = @(
    "<!-- PARITY_METRICS_TABLE_START -->",
    "| Metric | Count | Basis |",
    "|---|---:|---|",
    "| Legacy routes (extracted) | $legacyCount | ``docs/route-compare.md`` |",
    "| Exact matches | $exact | ``Status=exact`` |",
    "| Missing routes | $missing | ``Status=missing`` |",
    "| Aliased route entries | $aliasedProxy | Route-compare rows (route-level aliases tracked separately in REST contract notes) |",
    "| Extra Go routes vs legacy | $extra | ``Go extracted ($GoExtractedCount) - Legacy extracted ($legacyCount)`` |",
    "<!-- PARITY_METRICS_TABLE_END -->"
) -join [Environment]::NewLine

Write-Output "legacy_count=$legacyCount"
Write-Output "exact=$exact"
Write-Output "missing=$missing"
Write-Output "aliased_proxy=$aliasedProxy"
Write-Output "extra_vs_legacy=$extra"
Write-Output ""
Write-Output $metricsBlock

if ($WriteBack) {
    $content = Get-Content $parityMatrixFullPath -Raw
    $pattern = '<!-- PARITY_METRICS_TABLE_START -->[\s\S]*?<!-- PARITY_METRICS_TABLE_END -->'
    if ($content -notmatch $pattern) {
        throw "metrics markers not found in $parityMatrixFullPath"
    }

    $updated = [regex]::Replace($content, $pattern, [System.Text.RegularExpressions.MatchEvaluator]{ param($m) $metricsBlock })
    Set-Content -Path $parityMatrixFullPath -Value $updated -NoNewline
    Write-Output "updated=$parityMatrixFullPath"
}
