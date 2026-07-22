param([switch]$Check)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root
$env:GOCACHE = Join-Path $root ".gocache"
$arguments = @("run", "./scripts/generate-identity-artifact-vectors")
if ($Check) { $arguments += "-check" }
& go @arguments
if ($LASTEXITCODE -ne 0) { throw "identity artifact vector generation failed" }
