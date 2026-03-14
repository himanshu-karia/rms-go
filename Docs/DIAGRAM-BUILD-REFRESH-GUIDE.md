# RMS-Go Diagram Build and Refresh Guide

Last updated: 2026-03-01

Purpose: keep architecture and flow visuals reproducible from Mermaid sources.

## Diagram source locations
- Firmware flow/sequence diagrams:
  - `Docs/firmware-integration-kusumc/diagrams/`
  - `Docs/firmware-integration-kusumc-legacy-only/diagrams/`
- DB schema diagrams:
  - `go-kusumc/new-db-schema/diagrams/`

Each diagram should have:
- Source: `*.mmd`
- Export: `*.svg`

## Tooling
Use Mermaid CLI (`mmdc`):

```bash
npm install -g @mermaid-js/mermaid-cli
```

Verify installation:

```bash
mmdc -h
```

## Export a single diagram

```bash
mmdc -i input.mmd -o output.svg
```

## Batch export (Linux/macOS)

```bash
find Docs/firmware-integration-kusumc/diagrams Docs/firmware-integration-kusumc-legacy-only/diagrams go-kusumc/new-db-schema/diagrams -name "*.mmd" -print0 \
  | xargs -0 -I{} sh -c 'mmdc -i "$1" -o "${1%.mmd}.svg"' _ {}
```

## Batch export (PowerShell)

```powershell
$paths = @(
  "Docs/firmware-integration-kusumc/diagrams",
  "Docs/firmware-integration-kusumc-legacy-only/diagrams",
  "go-kusumc/new-db-schema/diagrams"
)
Get-ChildItem -Path $paths -Filter *.mmd -Recurse | ForEach-Object {
  $out = [System.IO.Path]::ChangeExtension($_.FullName, ".svg")
  mmdc -i $_.FullName -o $out
}
```

## Refresh policy
Refresh diagram exports when any of these change:
- MQTT topic/payload contract
- API flow affecting bootstrap/auth/telemetry/command sequence
- Compose topology or service dependency architecture
- DB schema evolution affecting entity relationships

## PR checklist for diagram updates
- [ ] Updated `.mmd` source committed
- [ ] Re-generated `.svg` committed
- [ ] Related canonical doc references updated
- [ ] Reviewer verified source and export are in sync
