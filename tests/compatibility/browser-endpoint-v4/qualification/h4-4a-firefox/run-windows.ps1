param(
    [string]$Firefox = $env:ARDENTS_REFERENCE_C2_FIREFOX
)

$ErrorActionPreference = 'Stop'
if ([System.Environment]::OSVersion.Platform -ne [System.PlatformID]::Win32NT) {
    throw 'H4-4A Firefox qualification requires an interactive Windows desktop host.'
}
if ([string]::IsNullOrWhiteSpace($Firefox) -or -not (Test-Path -LiteralPath $Firefox -PathType Leaf)) {
    throw 'ARDENTS_REFERENCE_C2_FIREFOX must name an existing Firefox executable.'
}
$env:ARDENTS_REFERENCE_C2_FIREFOX = (Resolve-Path -LiteralPath $Firefox).Path

go test -tags=h4_4_firefox ./internal/endpoint -run '^TestFirefoxBrowser.*Qualification$' -count=1 -timeout=1m
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}
go test -tags=h4_4_firefox ./tests/e2e/service -run '^TestReferenceC2CarriesDynamicPublisherApplicationThroughFirefoxBrowserEntryQualification$' -count=1 -timeout=1m
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}
Remove-Item Env:ARDENTS_REFERENCE_C2_FIREFOX -ErrorAction SilentlyContinue
go test ./tests/e2e/service -run '^TestReferenceC2RunsEveryRoleInSeparateProcesses$' -count=1 -timeout=1m
exit $LASTEXITCODE
