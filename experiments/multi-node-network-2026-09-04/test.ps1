$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$root = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$driver = Join-Path $root 'experiments\multi-node-network-2026-09-04\cmd\test-driver'
$go = (Get-Command go -ErrorAction Stop).Source
$sources = @(
    (Join-Path $driver 'doc.go')
    (Join-Path $driver 'main.go')
    (Join-Path $driver 'prebake.go')
    (Join-Path $driver 'verify.go')
    (Join-Path $driver 'selftest.go')
    (Join-Path $driver 'convergence.go')
    (Join-Path $driver 'encoding.go')
    (Join-Path $driver 'epoch.go')
    (Join-Path $driver 'fixtures.go')
    (Join-Path $driver 'record.go')
    (Join-Path $driver 'sourceplan.go')
    (Join-Path $driver 'verify_adversary.go')
)

& $go test @sources (Join-Path $driver 'sourceplan_test.go')
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}
& $go run @sources self-test
exit $LASTEXITCODE
