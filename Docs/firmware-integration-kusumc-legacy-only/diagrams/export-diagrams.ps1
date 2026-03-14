param(
  [string]$InputDir = (Join-Path $PSScriptRoot '.'),
  [string]$OutputDir = (Join-Path $PSScriptRoot '.')
)

$ErrorActionPreference = 'Stop'

# Uses mermaid-cli via pnpm dlx so we don't have to permanently add deps.
# Generates one SVG per .mmd file.

$mmdFiles = Get-ChildItem -Path $InputDir -Filter '*.mmd' -File | Sort-Object Name
if (-not $mmdFiles) {
  Write-Host "No .mmd files found in $InputDir" -ForegroundColor Yellow
  exit 0
}

foreach ($file in $mmdFiles) {
  $outSvg = Join-Path $OutputDir ($file.BaseName + '.svg')
  Write-Host "[mermaid] $($file.Name) -> $([IO.Path]::GetFileName($outSvg))"

  # mermaid-cli expects a plain mermaid diagram file.
  # Our .mmd files contain only Mermaid source (no markdown fences).
  pnpm dlx @mermaid-js/mermaid-cli@10.9.1 -i $file.FullName -o $outSvg -b transparent
}
