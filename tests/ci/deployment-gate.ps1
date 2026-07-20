# testkit:scenario SDE-001
# testkit:layer e2e
# testkit:domain deployment

param(
    [string]$ReportDir = "tests/.artifacts/deployment",
    [string]$Project = "ardents-ci",
    [string]$StateDir = "tests/.artifacts/runtime/ci",
    [string]$Image = "ardents/ardd-local:ci",
    [ValidateRange(10, 300)]
    [int]$TimeoutSeconds = 120,
    [switch]$Build
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location $root
$reportPath = [IO.Path]::GetFullPath($ReportDir)
$artifactRoot = [IO.Path]::GetFullPath((Join-Path $root "tests/.artifacts"))
if (-not $reportPath.StartsWith($artifactRoot + [IO.Path]::DirectorySeparatorChar)) {
    throw "deployment evidence path must stay under tests/.artifacts: $reportPath"
}
New-Item -ItemType Directory -Force -Path $reportPath | Out-Null

$common = @{
    Project = $Project
    StateDir = $StateDir
    Image = $Image
    TimeoutSeconds = $TimeoutSeconds
}
try {
    & ./tests/resource-snapshot.ps1 -Label before-ci-deployment `
        -OutputPath (Join-Path $reportPath "resources-before.json")
    $up = $common.Clone()
    if ($Build) { $up.Build = $true }
    & ./ardents.ps1 up @up

    $statusText = (& ./ardents.ps1 status @common) -join "`n"
    $statusText | Set-Content -Encoding utf8 (Join-Path $reportPath "status.json")
    $status = $statusText | ConvertFrom-Json
    if ($status.Count -ne 3) { throw "deployment status returned $($status.Count) nodes, want 3" }
    foreach ($node in $status) {
        if ($node.error) { throw "$($node.service) status failed: $($node.error)" }
        if (-not $node.ready -or $node.network_state -ne "ready") {
            throw "$($node.service) is not product ready"
        }
        if ($node.service -ne "seed" -and -not $node.joined) {
            throw "$($node.service) did not join the Waku network"
        }
        if (-not $node.workload_responsive -or -not $node.data_responsive) {
            throw "$($node.service) workload/data surface is not responsive"
        }
    }
    & ./tests/resource-snapshot.ps1 -Label after-ci-deployment `
        -OutputPath (Join-Path $reportPath "resources-after.json")
    Write-Host "deployment-gate=passed nodes=3"
}
finally {
    if (Test-Path -LiteralPath (Join-Path $StateDir "cluster.json")) {
        & ./ardents.ps1 down @common -RemoveVolumes
    }
}
