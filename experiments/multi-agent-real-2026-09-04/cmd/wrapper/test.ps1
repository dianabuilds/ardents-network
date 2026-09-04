$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$wrapper = $PSScriptRoot
$sourceNames = Get-Content -LiteralPath (Join-Path $wrapper 'sources.txt') |
    Where-Object { $_ -and -not $_.StartsWith('#') }
$testNames = Get-Content -LiteralPath (Join-Path $wrapper 'test-sources.txt') |
    Where-Object { $_ -and -not $_.StartsWith('#') }
$sources = $sourceNames | ForEach-Object { Join-Path $wrapper $_ }
$tests = $testNames | ForEach-Object { Join-Path $wrapper $_ }
$binary = Join-Path ([IO.Path]::GetTempPath()) ("ardents-multi-agent-wrapper-{0}.exe" -f [guid]::NewGuid().ToString('N'))

try {
    & go test @sources @tests
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    & go vet @sources
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    & go build -o $binary @sources
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    & $binary self-test
    exit $LASTEXITCODE
}
finally {
    if (Test-Path -LiteralPath $binary -PathType Leaf) {
        Remove-Item -LiteralPath $binary -Force
    }
}
