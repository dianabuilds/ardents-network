$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location $root
$fixtureRoot = Join-Path $root "tests/.artifacts/dr06-index-test-$([guid]::NewGuid().ToString('N'))"
$evidenceRoot = Join-Path $fixtureRoot "evidence"
$metadataRoot = Join-Path $fixtureRoot "metadata"
$outputPath = Join-Path $fixtureRoot "out/qualification-index.json"
$commit = "b0b0f951bd06e8ffae47f28669cd350681102839"
$contract = Get-Content -Raw -LiteralPath "tests/ci/dr06-gates.json" | ConvertFrom-Json
$jobNames = @($contract.gates | ForEach-Object workflow_job_name)
$artifactTemplates = @(
    @($contract.gates.required_artifacts | ForEach-Object { $_ }) +
    @($contract.gates.attempt_artifacts | ForEach-Object { $_ }) |
    Sort-Object -Unique
)
$requiredArtifactCount = $artifactTemplates.Count
$contractHashes = [ordered]@{
    go_mod_sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath "go.mod").Hash.ToLowerInvariant()
    workflow_sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath ".github/workflows/ci.yml").Hash.ToLowerInvariant()
    gate_contract_sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath "tests/ci/dr06-gates.json").Hash.ToLowerInvariant()
    release_materials_sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath "scripts/release/materials.json").Hash.ToLowerInvariant()
}
$materials = Get-Content -Raw -LiteralPath "scripts/release/materials.json" | ConvertFrom-Json

function Expand-Template([string]$Template, [int]$Attempt) {
    return $Template.
        Replace("{run_id}", "fixture-run").
        Replace("{attempt}", [string]$Attempt).
        Replace("{release_version}", "v0.0.0-dr06").
        Replace("*", "sample")
}

function Write-Artifact([string]$Relative, [object]$Value) {
    $path = Join-Path $evidenceRoot $Relative
    New-Item -ItemType Directory -Force (Split-Path -Parent $path) | Out-Null
    if ($Value -is [string]) {
        Set-Content -Encoding utf8 -LiteralPath $path -Value $Value
    } else {
        $Value | ConvertTo-Json -Depth 12 | Set-Content -Encoding utf8 -LiteralPath $path
    }
}

function Write-AttemptArtifacts([int]$Attempt) {
    foreach ($template in @($contract.gates.required_artifacts | ForEach-Object { $_ } | Sort-Object -Unique)) {
        Write-Artifact (Expand-Template ([string]$template) $Attempt) "evidence"
    }
    foreach ($gate in $contract.gates) {
        foreach ($template in @($gate.attempt_artifacts)) {
            $manifest = [ordered]@{
                schema_version = 1
                gate = [string]$gate.id
                environment_contract = [string]$gate.environment
                source_commit = $commit
                workflow_run = [ordered]@{ run_id = "fixture-run"; run_attempt = [string]$Attempt }
                toolchains = [ordered]@{
                    go = "go version fixture"
                    powershell = "7.5.0"
                    docker_client = "fixture"
                    docker_server = "fixture"
                }
                contracts = $contractHashes
                declared_material_names = @($gate.material_names)
                immutable_materials = @($materials.images | Where-Object {
                    @($gate.material_names) -contains [string]$_.name
                })
            }
            Write-Artifact (Expand-Template ([string]$template) $Attempt) $manifest
        }
    }
}

function Write-Attempt(
    [int]$Attempt,
    [string]$FailedJob = "",
    [string]$FailedOutcome = "failure"
) {
    $jobs = @($jobNames | ForEach-Object {
        $outcome = if ($_ -eq $FailedJob) { $FailedOutcome } else { "success" }
        [ordered]@{
            id = "$Attempt-$($_ -replace '[^A-Za-z0-9]', '-')"
            name = $_
            status = "completed"
            conclusion = $outcome
            started_at = "2026-07-27T00:00:00Z"
            completed_at = "2026-07-27T00:01:00Z"
            runner_name = "fixture-$Attempt"
            runner_group_name = "fixture"
            labels = @("ubuntu-24.04")
            steps = @()
        }
    })
    [ordered]@{ jobs = $jobs } | ConvertTo-Json -Depth 8 |
        Set-Content -Encoding utf8 -LiteralPath (Join-Path $metadataRoot "jobs-attempt-$Attempt.json")
}

function Invoke-Index([int]$Attempt) {
    & ./tests/ci/create-dr06-evidence-index.ps1 `
        -EvidenceRoot $evidenceRoot `
        -JobMetadataRoot $metadataRoot `
        -OutputPath $outputPath `
        -ReleaseVersion "v0.0.0-dr06" `
        -ExpectedCommit $commit `
        -RunID "fixture-run" `
        -RunAttempt $Attempt `
        -DurableEvidenceURI "urn:ardents:test:fixture" `
        -NativeInstallContract "privileged-systemd-container"
}

try {
    New-Item -ItemType Directory -Force $evidenceRoot,$metadataRoot | Out-Null
    Write-AttemptArtifacts -Attempt 1

    Write-Attempt -Attempt 1
    Invoke-Index -Attempt 1
    $index = Get-Content -Raw -LiteralPath $outputPath | ConvertFrom-Json
    if ($index.qualification_accepted -ne $false -or
        $index.status -ne "candidate_complete_pending_durable_handoff_and_release_owner_review" -or
        @($index.gates).Count -ne 17 -or
        @($index.artifacts).Count -ne $requiredArtifactCount -or
        @($index.capability_gate_intersections).Count -ne 15) {
        throw "single-attempt DR-06 index contract mismatch"
    }

    $missingTemplate = [string]$contract.gates[0].required_artifacts[0]
    $missingRelative = $missingTemplate.
        Replace("{run_id}", "fixture-run").
        Replace("{attempt}", "1").
        Replace("{release_version}", "v0.0.0-dr06").
        Replace("*", "sample")
    $missingPath = Join-Path $evidenceRoot $missingRelative
    Remove-Item -LiteralPath $missingPath -Force
    $missingRejected = $false
    try {
        Invoke-Index -Attempt 1
    } catch {
        $missingRejected = $_.Exception.Message -like "*evidence is missing*"
    }
    if (-not $missingRejected) {
        throw "DR-06 index accepted a packet with missing required gate evidence"
    }
    Set-Content -Encoding utf8 -LiteralPath $missingPath -Value "evidence"

    Write-Attempt -Attempt 1 -FailedJob "Reproducible release candidate" -FailedOutcome "cancelled"
    Write-Attempt -Attempt 2
    Write-AttemptArtifacts -Attempt 2
    $failedCandidate = $contract.gates | Where-Object id -ceq "release-candidate"
    $failedEnvironmentPath = Join-Path $evidenceRoot (
        Expand-Template ([string]$failedCandidate.attempt_artifacts[0]) 1
    )
    Remove-Item -LiteralPath $failedEnvironmentPath -Force
    $missingAttemptEvidenceRejected = $false
    try {
        Invoke-Index -Attempt 2
    } catch {
        $missingAttemptEvidenceRejected = $_.Exception.Message -like "*attempt evidence is missing*"
    }
    if (-not $missingAttemptEvidenceRejected) {
        throw "DR-06 index accepted a retry without failed-gate attempt evidence"
    }
    Write-AttemptArtifacts -Attempt 1

    $missingLogRejected = $false
    try {
        Invoke-Index -Attempt 2
    } catch {
        $missingLogRejected = $_.Exception.Message -like "*failed attempt log is missing*"
    }
    if (-not $missingLogRejected) {
        throw "DR-06 index accepted a retry without the earlier workflow log"
    }
    Write-Artifact "workflow-attempt-1.log" "failed attempt log"
    Invoke-Index -Attempt 2
    $retryIndex = Get-Content -Raw -LiteralPath $outputPath | ConvertFrom-Json
    if ($retryIndex.retry_policy.earlier_failed_attempts -ne 1 -or
        $retryIndex.retry_policy.automatic_acceptance_allowed -ne $false -or
        $retryIndex.status -ne "candidate_complete_pending_retry_disposition_durable_handoff_and_release_owner_review") {
        throw "DR-06 retry history was hidden by a later successful attempt"
    }

    Write-Host "dr06-evidence-index-test=passed"
} finally {
    if (Test-Path -LiteralPath $fixtureRoot) {
        Remove-Item -LiteralPath $fixtureRoot -Recurse -Force
    }
}
