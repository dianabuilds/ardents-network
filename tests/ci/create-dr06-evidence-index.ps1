param(
    [Parameter(Mandatory = $true)]
    [string]$EvidenceRoot,
    [Parameter(Mandatory = $true)]
    [string]$JobMetadataRoot,
    [Parameter(Mandatory = $true)]
    [string]$OutputPath,
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$')]
    [string]$ReleaseVersion,
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^[0-9a-f]{40}$')]
    [string]$ExpectedCommit,
    [Parameter(Mandatory = $true)]
    [string]$RunID,
    [Parameter(Mandatory = $true)]
    [ValidateRange(1, 1000)]
    [int]$RunAttempt,
    [Parameter(Mandatory = $true)]
    [string]$DurableEvidenceURI,
    [Parameter(Mandatory = $true)]
    [ValidateSet("privileged-systemd-container")]
    [string]$NativeInstallContract
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location $root

function Resolve-RepositoryPath([string]$Path, [string]$Name) {
    $resolved = [IO.Path]::GetFullPath($Path)
    $rootPath = [IO.Path]::GetFullPath($root).TrimEnd(
        [IO.Path]::DirectorySeparatorChar,
        [IO.Path]::AltDirectorySeparatorChar
    )
    if (-not $resolved.StartsWith(
        $rootPath + [IO.Path]::DirectorySeparatorChar,
        [StringComparison]::OrdinalIgnoreCase
    )) {
        throw "$Name must stay inside the repository: $Path"
    }
    return $resolved
}

$evidencePath = Resolve-RepositoryPath $EvidenceRoot "EvidenceRoot"
$metadataPath = Resolve-RepositoryPath $JobMetadataRoot "JobMetadataRoot"
$indexPath = Resolve-RepositoryPath $OutputPath "OutputPath"
if (-not (Test-Path -LiteralPath $evidencePath -PathType Container)) {
    throw "downloaded DR-06 evidence is unavailable: $evidencePath"
}
if (-not (Test-Path -LiteralPath $metadataPath -PathType Container)) {
    throw "DR-06 job metadata is unavailable: $metadataPath"
}
if ([string]::IsNullOrWhiteSpace($DurableEvidenceURI)) {
    throw "durable evidence authority/destination is required"
}
if ($DurableEvidenceURI -match '[?#]' -or $DurableEvidenceURI -match '^[a-z][a-z0-9+.-]*://[^/]*@') {
    throw "durable evidence reference must not contain credentials, fragments, or query parameters"
}

$gateContractPath = Join-Path $root "tests/ci/dr06-gates.json"
$gateContract = Get-Content -Raw -LiteralPath $gateContractPath | ConvertFrom-Json
if ($gateContract.schema_version -ne 1 -or @($gateContract.gates).Count -eq 0) {
    throw "DR-06 gate contract is missing or unsupported"
}
$gateContracts = @($gateContract.gates)
$gateIDs = @($gateContracts | ForEach-Object id)
if (@($gateIDs | Sort-Object -Unique).Count -ne $gateIDs.Count) {
    throw "DR-06 gate contract contains duplicate gate IDs"
}

$attemptDocuments = @()
foreach ($file in Get-ChildItem -LiteralPath $metadataPath -Filter "jobs-attempt-*.json" -File | Sort-Object Name) {
    $document = Get-Content -Raw -LiteralPath $file.FullName | ConvertFrom-Json
    if ($null -eq $document.jobs) { throw "job metadata has no jobs array: $($file.Name)" }
    $attempt = 0
    if ($file.BaseName -notmatch '^jobs-attempt-([0-9]+)$') {
        throw "job metadata filename does not identify an attempt: $($file.Name)"
    }
    $attempt = [int]$Matches[1]
    foreach ($job in @($document.jobs)) {
        $attemptDocuments += [pscustomobject][ordered]@{
            attempt = $attempt
            id = [string]$job.id
            name = [string]$job.name
            status = [string]$job.status
            conclusion = [string]$job.conclusion
            started_at = [string]$job.started_at
            completed_at = [string]$job.completed_at
            runner_name = [string]$job.runner_name
            runner_group_name = [string]$job.runner_group_name
            labels = @($job.labels)
            steps = @($job.steps | ForEach-Object {
                [ordered]@{
                    name = [string]$_.name
                    status = [string]$_.status
                    conclusion = [string]$_.conclusion
                    number = [int]$_.number
                    started_at = [string]$_.started_at
                    completed_at = [string]$_.completed_at
                }
            })
        }
    }
}
if ($attemptDocuments.Count -eq 0) { throw "no DR-06 job attempt metadata was captured" }

$gateAttempts = @()
foreach ($contract in $gateContracts) {
    $matches = @($attemptDocuments | Where-Object { $_.name -ceq $contract.workflow_job_name })
    if ($matches.Count -eq 0) {
        throw "DR-06 gate has no job metadata: $($contract.id) / $($contract.workflow_job_name)"
    }
    foreach ($match in $matches) {
        $gateAttempts += [pscustomobject][ordered]@{
            gate = $contract.id
            command = $contract.command
            environment = $contract.environment
            environment_output_path = $contract.environment_output_path
            material_names = @($contract.material_names)
            dependencies = @($contract.dependencies)
            capability_claims = @($contract.capability_claims)
            attempt_artifacts = @($contract.attempt_artifacts)
            required_artifacts = @($contract.required_artifacts)
            attempt = $match.attempt
            job_id = $match.id
            job_name = $match.name
            status = $match.status
            outcome = $match.conclusion
            started_at = $match.started_at
            completed_at = $match.completed_at
            runner_name = $match.runner_name
            runner_group_name = $match.runner_group_name
            runner_labels = @($match.labels)
            steps = @($match.steps)
        }
    }
    $latestAttempt = [int](($matches | Measure-Object -Property attempt -Maximum).Maximum)
    $latest = @($matches | Where-Object { [int]$_.attempt -eq $latestAttempt })
    if ($latest.Count -ne 1 -or $latest[0].conclusion -ne "success") {
        throw "latest DR-06 gate attempt did not pass: $($contract.id)"
    }
}

$artifactFiles = @()
$artifactPathByRelative = @{}
$evidencePrefix = $evidencePath.TrimEnd(
    [IO.Path]::DirectorySeparatorChar,
    [IO.Path]::AltDirectorySeparatorChar
) + [IO.Path]::DirectorySeparatorChar
foreach ($file in Get-ChildItem -LiteralPath $evidencePath -Recurse -File | Sort-Object FullName) {
    $fullName = [IO.Path]::GetFullPath($file.FullName)
    if (-not $fullName.StartsWith($evidencePrefix, [StringComparison]::OrdinalIgnoreCase)) {
        throw "downloaded artifact escaped the evidence root: $fullName"
    }
    $relative = $fullName.Substring($evidencePrefix.Length).Replace("\", "/")
    $artifactPathByRelative[$relative] = $fullName
    $artifactFiles += [pscustomobject][ordered]@{
        path = $relative
        size = [long]$file.Length
        sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $file.FullName).Hash.ToLowerInvariant()
    }
}
if ($artifactFiles.Count -eq 0) { throw "downloaded DR-06 evidence contains no files" }

$goModPath = Join-Path $root "go.mod"
$materialsPath = Join-Path $root "scripts/release/materials.json"
$workflowPath = Join-Path $root ".github/workflows/ci.yml"
$contractPath = Join-Path $root "tests/ci/dr06-gates.json"
$expectedContractHashes = [ordered]@{
    go_mod_sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $goModPath).Hash.ToLowerInvariant()
    workflow_sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $workflowPath).Hash.ToLowerInvariant()
    gate_contract_sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $contractPath).Hash.ToLowerInvariant()
    release_materials_sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $materialsPath).Hash.ToLowerInvariant()
}

function Expand-ArtifactTemplate([string]$Template, [int]$Attempt) {
    return $Template.
        Replace("{run_id}", $RunID).
        Replace("{attempt}", [string]$Attempt).
        Replace("{release_version}", $ReleaseVersion)
}

$validatedAttemptEvidence = @()
foreach ($contract in $gateContracts) {
    foreach ($gateAttempt in @($gateAttempts | Where-Object gate -ceq $contract.id)) {
        $executed = -not [string]::IsNullOrWhiteSpace([string]$gateAttempt.started_at) -and
            $gateAttempt.outcome -ne "skipped"
        if (-not $executed) {
            $validatedAttemptEvidence += [pscustomobject][ordered]@{
                gate = $contract.id
                attempt = [int]$gateAttempt.attempt
                outcome = [string]$gateAttempt.outcome
                evidence_status = "unavailable_not_executed"
                artifacts = @()
            }
            continue
        }
        $matchedPaths = [Collections.Generic.List[string]]::new()
        foreach ($template in @($contract.attempt_artifacts)) {
            $pattern = Expand-ArtifactTemplate ([string]$template) ([int]$gateAttempt.attempt)
            $matches = @($artifactFiles | Where-Object { $_.path -like $pattern })
            if ($matches.Count -eq 0) {
                throw "DR-06 attempt evidence is missing: $($contract.id) attempt $($gateAttempt.attempt) requires $pattern"
            }
            foreach ($match in $matches) {
                if (-not $matchedPaths.Contains([string]$match.path)) {
                    $matchedPaths.Add([string]$match.path)
                }
            }
        }
        foreach ($path in $matchedPaths) {
            if (-not $path.EndsWith("/environment.json", [StringComparison]::OrdinalIgnoreCase) -and
                -not $path.EndsWith("environment.json", [StringComparison]::OrdinalIgnoreCase)) {
                continue
            }
            $manifest = Get-Content -Raw -LiteralPath $artifactPathByRelative[$path] | ConvertFrom-Json
            if ($manifest.schema_version -ne 1 -or
                [string]$manifest.gate -cne [string]$contract.id -or
                [string]$manifest.environment_contract -cne [string]$contract.environment -or
                [string]$manifest.source_commit -cne $ExpectedCommit) {
                throw "DR-06 environment manifest identity mismatch: $($contract.id) attempt $($gateAttempt.attempt)"
            }
            foreach ($hashName in $expectedContractHashes.Keys) {
                if ([string]$manifest.contracts.$hashName -cne [string]$expectedContractHashes[$hashName]) {
                    throw "DR-06 environment manifest contract hash mismatch: $($contract.id) / $hashName"
                }
            }
            if ([string]::IsNullOrWhiteSpace([string]$manifest.toolchains.go) -or
                [string]::IsNullOrWhiteSpace([string]$manifest.toolchains.powershell)) {
                throw "DR-06 environment manifest lacks toolchain/material identity: $($contract.id)"
            }
            $expectedMaterialNames = @($contract.material_names | Sort-Object)
            $declaredMaterialNames = @($manifest.declared_material_names | Sort-Object)
            $actualMaterialNames = @($manifest.immutable_materials | ForEach-Object name | Sort-Object)
            if (@(Compare-Object $expectedMaterialNames $declaredMaterialNames).Count -ne 0 -or
                @(Compare-Object $expectedMaterialNames $actualMaterialNames).Count -ne 0) {
                throw "DR-06 environment manifest material mapping mismatch: $($contract.id)"
            }
            foreach ($material in @($manifest.immutable_materials)) {
                if ([string]$material.reference -notmatch '@sha256:[0-9a-f]{64}$') {
                    throw "DR-06 environment manifest contains a mutable material: $($contract.id) / $($material.name)"
                }
            }
            if ($contract.environment -match '(Docker|container|fixture|topology|release)' -and
                [string]::IsNullOrWhiteSpace([string]$manifest.toolchains.docker_server)) {
                throw "DR-06 environment manifest lacks Docker engine identity: $($contract.id)"
            }
        }
        $validatedAttemptEvidence += [pscustomobject][ordered]@{
            gate = $contract.id
            attempt = [int]$gateAttempt.attempt
            outcome = [string]$gateAttempt.outcome
            evidence_status = "validated"
            artifacts = @($matchedPaths)
        }
    }
}

$validatedEvidence = @()
foreach ($contract in $gateContracts) {
    $gateMatches = @($gateAttempts | Where-Object gate -ceq $contract.id)
    $latestAttempt = [int](($gateMatches | Measure-Object -Property attempt -Maximum).Maximum)
    $matchedPaths = [Collections.Generic.List[string]]::new()
    foreach ($template in @($contract.required_artifacts)) {
        $pattern = Expand-ArtifactTemplate ([string]$template) $latestAttempt
        $matches = @($artifactFiles | Where-Object { $_.path -like $pattern })
        if ($matches.Count -eq 0) {
            throw "DR-06 gate evidence is missing: $($contract.id) requires $pattern"
        }
        foreach ($match in $matches) {
            if (-not $matchedPaths.Contains([string]$match.path)) {
                $matchedPaths.Add([string]$match.path)
            }
        }
    }
    $validatedEvidence += [pscustomobject][ordered]@{
        gate = $contract.id
        attempt = $latestAttempt
        capability_claims = @($contract.capability_claims)
        artifacts = @($matchedPaths)
    }
}

$capabilityIntersections = @()
$capabilityIDs = @($gateContracts.capability_claims | ForEach-Object { $_ } | Sort-Object -Unique)
foreach ($capabilityID in $capabilityIDs) {
    $capabilityIntersections += [pscustomobject][ordered]@{
        capability = $capabilityID
        gates = @($gateContracts | Where-Object {
            @($_.capability_claims) -contains $capabilityID
        } | ForEach-Object id)
        disposition = "pending_release_owner_review"
    }
}

$materials = Get-Content -Raw -LiteralPath $materialsPath | ConvertFrom-Json
$dockerServer = ""
try { $dockerServer = (docker version --format '{{.Server.Version}}' 2>$null) } catch { $dockerServer = "" }
$workflowText = Get-Content -Raw -LiteralPath $workflowPath
$protocVersion = [regex]::Match($workflowText, 'protoc_version=([0-9.]+)').Groups[1].Value

$qualificationIndexFailures = @($attemptDocuments | Where-Object {
    $_.name -ceq "Commit-bound DR-06 evidence index" -and
    [int]$_.attempt -lt $RunAttempt -and
    $_.conclusion -and $_.conclusion -ne "success"
})
$failedAttemptNumbers = @(
    @($gateAttempts | Where-Object {
        $_.outcome -and $_.outcome -ne "success"
    } | ForEach-Object attempt) +
    @($qualificationIndexFailures | ForEach-Object attempt) |
    Sort-Object -Unique
)
$validatedAttemptLogs = @()
foreach ($attempt in $failedAttemptNumbers) {
    $logPath = "workflow-attempt-$attempt.log"
    $match = @($artifactFiles | Where-Object path -ceq $logPath)
    if ($match.Count -ne 1 -or [long]$match[0].size -eq 0) {
        throw "DR-06 failed attempt log is missing: attempt $attempt requires $logPath"
    }
    $validatedAttemptLogs += [pscustomobject][ordered]@{
        attempt = [int]$attempt
        path = $logPath
        sha256 = [string]$match[0].sha256
    }
}
$index = [ordered]@{
    schema_version = 1
    qualification_accepted = $false
    status = if ($failedAttemptNumbers.Count -eq 0) {
        "candidate_complete_pending_durable_handoff_and_release_owner_review"
    } else {
        "candidate_complete_pending_retry_disposition_durable_handoff_and_release_owner_review"
    }
    source = [ordered]@{
        commit = $ExpectedCommit
        clean_required = $true
        workflow = ".github/workflows/ci.yml"
        workflow_sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $workflowPath).Hash.ToLowerInvariant()
        gate_contract = "tests/ci/dr06-gates.json"
        gate_contract_sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $contractPath).Hash.ToLowerInvariant()
    }
    release_version = $ReleaseVersion
    workflow_run = [ordered]@{
        run_id = $RunID
        run_attempt = $RunAttempt
    }
    environment_decisions = [ordered]@{
        fast = "github-ubuntu-24.04-hosted-linux-container"
        native_install = $NativeInstallContract
    }
    retention = [ordered]@{
        staging_days = 90
        durable_evidence_uri = $DurableEvidenceURI
        handoff_status = "required_before_release_owner_acceptance"
        supported_lifetime_required = $true
        confirmation_command = "tests/ci/confirm-dr06-retention.ps1"
    }
    retry_policy = [ordered]@{
        earlier_failed_attempts = $failedAttemptNumbers.Count
        failed_attempts = @($failedAttemptNumbers)
        automatic_acceptance_allowed = ($failedAttemptNumbers.Count -eq 0)
        disposition_required = ($failedAttemptNumbers.Count -gt 0)
        validated_logs = $validatedAttemptLogs
    }
    toolchains_and_materials = [ordered]@{
        go_mod_sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $goModPath).Hash.ToLowerInvariant()
        index_go = (go version)
        index_powershell = $PSVersionTable.PSVersion.ToString()
        index_docker_server = $dockerServer
        protoc = $protocVersion
        release_materials_sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $materialsPath).Hash.ToLowerInvariant()
        release_materials = @($materials.images)
    }
    gates = $gateAttempts
    validated_attempt_evidence = $validatedAttemptEvidence
    validated_gate_evidence = $validatedEvidence
    capability_gate_intersections = $capabilityIntersections
    artifacts = $artifactFiles
    generated_at = (Get-Date).ToUniversalTime().ToString("o")
}

$parent = Split-Path -Parent $indexPath
[IO.Directory]::CreateDirectory($parent) | Out-Null
$index | ConvertTo-Json -Depth 20 | Set-Content -Encoding utf8 -LiteralPath $indexPath
Write-Host "dr06-evidence-index=created gates=$($gateContracts.Count) attempts=$($gateAttempts.Count) artifacts=$($artifactFiles.Count)"
