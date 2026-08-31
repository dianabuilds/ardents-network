[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateScript({ Test-Path -LiteralPath $_ -PathType Leaf })]
    [string]$Firefox,

    [Parameter(Mandatory = $true)]
    [ValidateScript({ Test-Path -LiteralPath $_ -PathType Leaf })]
    [string]$SignedXPI
)

$ErrorActionPreference = 'Stop'
if ([System.Environment]::OSVersion.Platform -ne [System.PlatformID]::Win32NT) {
    throw 'signed Firefox Browser Entry qualification requires an interactive Windows desktop host'
}

$env:ARDENTS_REFERENCE_C2_FIREFOX = (Resolve-Path -LiteralPath $Firefox).Path
$env:ARDENTS_H4_4_SIGNED_XPI = (Resolve-Path -LiteralPath $SignedXPI).Path
$env:ARDENTS_H4_4_SIGNED_FIREFOX_XPI = $env:ARDENTS_H4_4_SIGNED_XPI
try {
    & go test -tags=h4_4_signed_xpi ./internal/browserentry/installer -run '^TestMozillaSignedAlphaBrowserEntryXPI$' -count=1
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    Write-Host ''
    Write-Host 'Firefox will open a fresh temporary profile and Explorer will select the signed XPI.'
    Write-Host 'Drag that file into Firefox, approve the add-on, then open http://reference.ard/.'
    Write-Host 'Do not use your ordinary Firefox profile. This run closes and removes only its temporary profile and native-host registration.'
    Write-Host ''
    & go test -tags=h4_4_signed_firefox ./tests/e2e/service -run '^TestReferenceC2CarriesDynamicPublisherApplicationThroughSignedFirefoxBrowserEntry$' -count=1 -timeout=5m
    exit $LASTEXITCODE
}
finally {
    Remove-Item Env:ARDENTS_H4_4_SIGNED_FIREFOX_XPI -ErrorAction SilentlyContinue
    Remove-Item Env:ARDENTS_H4_4_SIGNED_XPI -ErrorAction SilentlyContinue
    Remove-Item Env:ARDENTS_REFERENCE_C2_FIREFOX -ErrorAction SilentlyContinue
}
