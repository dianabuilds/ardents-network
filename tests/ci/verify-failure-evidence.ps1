param(
    [string]$ReportDir = "tests/.artifacts/reports/ci-deliberate-failure"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location $root
$reportPath = [IO.Path]::GetFullPath($ReportDir)
$testsRoot = Split-Path -Parent $PSScriptRoot
$artifactRoot = [IO.Path]::GetFullPath((Join-Path $testsRoot ".artifacts"))
if (-not $reportPath.StartsWith($artifactRoot + [IO.Path]::DirectorySeparatorChar)) {
    throw "failure evidence path must stay under tests/.artifacts: $reportPath"
}
if (Test-Path -LiteralPath $reportPath) {
    Remove-Item -LiteralPath $reportPath -Recurse -Force
}

$blocked = $false
$failureMessage = ""
try {
    & (Join-Path $testsRoot "run.ps1") integration `
        -Scenario "CI-DELIBERATE-FAILURE" `
        -ReportDir $ReportDir
} catch {
    $blocked = $true
    $failureMessage = $_.Exception.Message
}
if (-not $blocked) { throw "deliberately failing canonical run did not block" }

$summaryPath = Join-Path $reportPath "summary.json"
$junitPath = Join-Path $reportPath "junit.xml"
$rawPath = Join-Path $reportPath "raw"
if (-not (Test-Path -LiteralPath $summaryPath)) {
    throw "failure summary was not retained; canonical runner error: $failureMessage"
}
if (-not (Test-Path -LiteralPath $junitPath)) {
    throw "failure JUnit was not retained; canonical runner error: $failureMessage"
}
if (-not (Test-Path -LiteralPath $rawPath)) {
    throw "raw failure directory was not retained; canonical runner error: $failureMessage"
}
$rawFiles = @(Get-ChildItem -LiteralPath $rawPath -Filter "*.json" -File)
if ($rawFiles.Count -eq 0) {
    throw "raw failure evidence was not retained; canonical runner error: $failureMessage"
}

$summary = Get-Content -Raw -LiteralPath $summaryPath | ConvertFrom-Json
if ($summary.summary.failed -lt 1) { throw "canonical summary did not record a failure" }
$junit = [xml](Get-Content -Raw -LiteralPath $junitPath)
if ([int]$junit.testsuite.failures -lt 1) { throw "JUnit did not record a failure" }

$verification = [ordered]@{
    blocked = $true
    failure_message = $failureMessage
    summary_failed = [int]$summary.summary.failed
    raw_evidence_count = $rawFiles.Count
    summary = "summary.json"
    junit = "junit.xml"
}
$verification | ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 (Join-Path $reportPath "verification.json")
Write-Host "deliberate-failure-gate=passed"
