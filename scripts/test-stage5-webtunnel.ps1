param(
    [Parameter(Mandatory = $true)][string]$ClientBinary,
    [Parameter(Mandatory = $true)][string]$ServerBinary
)

$ErrorActionPreference = 'Stop'
$repository = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$env:ARDENTS_WEBTUNNEL_CLIENT = (Resolve-Path -LiteralPath $ClientBinary).Path
$env:ARDENTS_WEBTUNNEL_SERVER = (Resolve-Path -LiteralPath $ServerBinary).Path

Push-Location $repository
try {
    & go test -tags=live ./tests/live/network `
        -run '^TestBlockedEntryCommandsAcrossNamespaces$' `
        -count=1 -v -timeout=30m
    if ($LASTEXITCODE -ne 0) {
        throw 'Stage 5 blocked-entry live test failed'
    }
} finally {
    Pop-Location
}
