param([switch]$Check)

# Compatibility entry point. The canonical generator now checks both public
# surfaces and the shared identity artifact outputs.
& (Join-Path $PSScriptRoot "generate-api.ps1") -Check:$Check
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
