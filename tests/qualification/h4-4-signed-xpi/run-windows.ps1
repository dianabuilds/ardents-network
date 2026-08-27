[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$SignedXPI
)

$ErrorActionPreference = 'Stop'

if ([string]::IsNullOrWhiteSpace($SignedXPI) -or -not [System.IO.Path]::IsPathRooted($SignedXPI)) {
    throw 'SignedXPI must be one absolute Mozilla-signed XPI path'
}
$resolved = (Resolve-Path -LiteralPath $SignedXPI -ErrorAction Stop).Path
if (-not (Test-Path -LiteralPath $resolved -PathType Leaf)) {
    throw "Mozilla-signed XPI is unavailable: $resolved"
}
$env:ARDENTS_H4_4_SIGNED_XPI = $resolved
& go test -tags=h4_4_signed_xpi ./internal/browserentry/installer -run '^TestMozillaSignedAlphaBrowserEntryXPI$' -count=1
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}
