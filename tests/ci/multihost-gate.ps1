# testkit:scenario NFM-001
# testkit:layer e2e
# testkit:domain network-foundation

param(
    [string]$ReportDir = "tests/.artifacts/reports/multihost",
    [ValidateSet("Never", "IfMissing", "Always")]
    [string]$BuildMode = "IfMissing",
    [ValidateRange(1, 4)]
    [int]$BuildParallelism = 1,
    [switch]$Keep
)

$ErrorActionPreference = "Stop"
& "$PSScriptRoot/../run-multihost.ps1" @PSBoundParameters

