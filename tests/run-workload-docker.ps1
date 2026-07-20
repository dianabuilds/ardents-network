param(
    [string]$ReportDir = "tests/.artifacts/reports/stb-402-docker",
    [switch]$Keep
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root
$project = "ardents-workload-$PID"
$composeFile = Join-Path $root "docker/docker-compose.workload-test.yml"
$reportPath = [IO.Path]::GetFullPath($ReportDir)
New-Item -ItemType Directory -Force -Path $reportPath | Out-Null
$prefix = @("compose", "-p", $project, "-f", $composeFile)

try {
    & (Join-Path $PSScriptRoot "resource-snapshot.ps1") -Label "before-workload-docker"
    & docker volume create ardents-go-mod-cache | Out-Null
    & docker volume create ardents-go-build-cache | Out-Null
    & docker @prefix up --build --abort-on-container-exit --exit-code-from tests tests
    if ($LASTEXITCODE -ne 0) { throw "isolated workload integration failed with exit code $LASTEXITCODE" }
    & docker @prefix logs --no-color *> (Join-Path $reportPath "compose.log")
} finally {
    & (Join-Path $PSScriptRoot "resource-snapshot.ps1") -Label "after-workload-docker"
    if (-not $Keep) {
        & docker @prefix down --volumes --remove-orphans | Out-Null
    }
}
