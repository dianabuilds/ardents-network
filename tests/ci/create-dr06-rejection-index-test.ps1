$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location $root
$fixtureRoot = Join-Path $root "tests/.artifacts/dr06-rejection-test-$([guid]::NewGuid().ToString('N'))"
$evidenceRoot = Join-Path $fixtureRoot "evidence"
$metadataRoot = Join-Path $fixtureRoot "metadata"
$outputPath = Join-Path $fixtureRoot "out/qualification-index.json"
$errorPath = Join-Path $fixtureRoot "out/index-validation-error.txt"
$contract = Get-Content -Raw -LiteralPath "tests/ci/dr06-gates.json" | ConvertFrom-Json

try {
    New-Item -ItemType Directory -Force $evidenceRoot,$metadataRoot,(Split-Path -Parent $outputPath) | Out-Null
    Set-Content -Encoding utf8 -LiteralPath (Join-Path $evidenceRoot "partial-failure.log") -Value "failed"
    Set-Content -Encoding utf8 -LiteralPath $errorPath -Value "latest DR-06 gate attempt did not pass"
    $jobs = @($contract.gates | ForEach-Object {
        [ordered]@{
            id = "fixture-$($_.id)"
            name = $_.workflow_job_name
            status = "completed"
            conclusion = if ($_.id -eq "application-process") { "failure" } else { "skipped" }
            started_at = "2026-07-27T00:00:00Z"
            completed_at = "2026-07-27T00:01:00Z"
            runner_name = "fixture"
            runner_group_name = "fixture"
            labels = @("ubuntu-24.04")
        }
    })
    [ordered]@{ jobs = $jobs } | ConvertTo-Json -Depth 8 |
        Set-Content -Encoding utf8 -LiteralPath (Join-Path $metadataRoot "jobs-attempt-1.json")

    ./tests/ci/create-dr06-rejection-index.ps1 `
        -EvidenceRoot $evidenceRoot `
        -JobMetadataRoot $metadataRoot `
        -OutputPath $outputPath `
        -ReleaseVersion "v0.0.0-dr06" `
        -ExpectedCommit "b0b0f951bd06e8ffae47f28669cd350681102839" `
        -RunID "fixture-run" `
        -RunAttempt 1 `
        -DurableEvidenceURI "urn:ardents:test:fixture" `
        -NativeInstallContract "privileged-systemd-container" `
        -ValidationErrorPath $errorPath

    $index = Get-Content -Raw -LiteralPath $outputPath | ConvertFrom-Json
    $failed = @($index.gates | Where-Object gate -ceq "application-process")
    if ($index.qualification_accepted -ne $false -or
        $index.status -ne "rejected" -or
        $failed.Count -ne 1 -or
        $failed[0].outcome -ne "failure" -or
        @($index.artifacts).Count -ne 1) {
        throw "DR-06 rejected aggregate contract mismatch"
    }

    ./tests/ci/create-dr06-rejection-index.ps1 `
        -EvidenceRoot $evidenceRoot `
        -JobMetadataRoot $metadataRoot `
        -OutputPath $outputPath `
        -ReleaseVersion "invalid-candidate" `
        -ExpectedCommit "b0b0f951bd06e8ffae47f28669cd350681102839" `
        -RunID "fixture-run" `
        -RunAttempt 1 `
        -ValidationErrorPath $errorPath
    $invalidInputIndex = Get-Content -Raw -LiteralPath $outputPath | ConvertFrom-Json
    if ($invalidInputIndex.retention.durable_evidence_uri -ne "unavailable" -or
        $invalidInputIndex.environment_decisions.native_install -ne "unresolved") {
        throw "DR-06 rejected aggregate did not preserve unresolved human inputs safely"
    }
    Write-Host "dr06-rejection-index-test=passed"
} finally {
    if (Test-Path -LiteralPath $fixtureRoot) {
        Remove-Item -LiteralPath $fixtureRoot -Recurse -Force
    }
}
