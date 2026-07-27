param(
    [Parameter(Mandatory = $true)]
    [string]$EvidenceRoot,
    [Parameter(Mandatory = $true)]
    [string]$JobMetadataRoot,
    [Parameter(Mandatory = $true)]
    [string]$OutputPath,
    [Parameter(Mandatory = $true)]
    [string]$ReleaseVersion,
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^[0-9a-f]{40}$')]
    [string]$ExpectedCommit,
    [Parameter(Mandatory = $true)]
    [string]$RunID,
    [Parameter(Mandatory = $true)]
    [ValidateRange(1, 1000)]
    [int]$RunAttempt,
    [string]$DurableEvidenceURI,
    [string]$NativeInstallContract,
    [Parameter(Mandatory = $true)]
    [string]$ValidationErrorPath
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
$validationPath = Resolve-RepositoryPath $ValidationErrorPath "ValidationErrorPath"
$safeDurableEvidenceURI = $DurableEvidenceURI
if ([string]::IsNullOrWhiteSpace($safeDurableEvidenceURI)) {
    $safeDurableEvidenceURI = "unavailable"
} elseif ($safeDurableEvidenceURI -match '[?#]' -or
    $safeDurableEvidenceURI -match '^[a-z][a-z0-9+.-]*://[^/]*@') {
    $safeDurableEvidenceURI = "invalid-redacted"
}
$safeNativeInstallContract = if ($NativeInstallContract -eq "privileged-systemd-container") {
    $NativeInstallContract
} else {
    "unresolved"
}

$contractPath = Join-Path $root "tests/ci/dr06-gates.json"
$contract = Get-Content -Raw -LiteralPath $contractPath | ConvertFrom-Json
$documents = @()
if (Test-Path -LiteralPath $metadataPath -PathType Container) {
    foreach ($file in Get-ChildItem -LiteralPath $metadataPath -Filter "jobs-attempt-*.json" -File | Sort-Object Name) {
        if ($file.BaseName -notmatch '^jobs-attempt-([0-9]+)$') { continue }
        $attempt = [int]$Matches[1]
        try {
            $document = Get-Content -Raw -LiteralPath $file.FullName | ConvertFrom-Json
            foreach ($job in @($document.jobs)) {
                $documents += [pscustomobject][ordered]@{
                    attempt = $attempt
                    job_id = [string]$job.id
                    job_name = [string]$job.name
                    status = [string]$job.status
                    outcome = [string]$job.conclusion
                    started_at = [string]$job.started_at
                    completed_at = [string]$job.completed_at
                    runner_name = [string]$job.runner_name
                    runner_group_name = [string]$job.runner_group_name
                    runner_labels = @($job.labels)
                }
            }
        } catch {
            $documents += [pscustomobject][ordered]@{
                attempt = $attempt
                job_id = ""
                job_name = "metadata-parse-error:$($file.Name)"
                status = "unavailable"
                outcome = "unavailable"
                started_at = ""
                completed_at = ""
                runner_name = ""
                runner_group_name = ""
                runner_labels = @()
            }
        }
    }
}

$gates = @()
foreach ($gate in @($contract.gates)) {
    $matches = @($documents | Where-Object job_name -ceq $gate.workflow_job_name)
    if ($matches.Count -eq 0) {
        $gates += [pscustomobject][ordered]@{
            gate = [string]$gate.id
            command = [string]$gate.command
            environment = [string]$gate.environment
            capability_claims = @($gate.capability_claims)
            attempt = $RunAttempt
            job_id = ""
            job_name = [string]$gate.workflow_job_name
            status = "unavailable"
            outcome = "unavailable"
            evidence_status = "unavailable"
        }
        continue
    }
    foreach ($match in $matches) {
        $gates += [pscustomobject][ordered]@{
            gate = [string]$gate.id
            command = [string]$gate.command
            environment = [string]$gate.environment
            capability_claims = @($gate.capability_claims)
            attempt = [int]$match.attempt
            job_id = [string]$match.job_id
            job_name = [string]$match.job_name
            status = [string]$match.status
            outcome = if ([string]::IsNullOrWhiteSpace([string]$match.outcome)) {
                "unavailable"
            } else {
                [string]$match.outcome
            }
            evidence_status = "not_validated"
            started_at = [string]$match.started_at
            completed_at = [string]$match.completed_at
            runner_name = [string]$match.runner_name
            runner_group_name = [string]$match.runner_group_name
            runner_labels = @($match.runner_labels)
        }
    }
}

$artifacts = @()
if (Test-Path -LiteralPath $evidencePath -PathType Container) {
    $prefix = $evidencePath.TrimEnd(
        [IO.Path]::DirectorySeparatorChar,
        [IO.Path]::AltDirectorySeparatorChar
    ) + [IO.Path]::DirectorySeparatorChar
    foreach ($file in Get-ChildItem -LiteralPath $evidencePath -Recurse -File | Sort-Object FullName) {
        $fullName = [IO.Path]::GetFullPath($file.FullName)
        if (-not $fullName.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase)) { continue }
        $artifacts += [pscustomobject][ordered]@{
            path = $fullName.Substring($prefix.Length).Replace("\", "/")
            size = [long]$file.Length
            sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $fullName).Hash.ToLowerInvariant()
        }
    }
}

$validationError = "qualification index validation did not complete"
if (Test-Path -LiteralPath $validationPath -PathType Leaf) {
    $validationError = (Get-Content -Raw -LiteralPath $validationPath).Trim()
}
$index = [ordered]@{
    schema_version = 1
    qualification_accepted = $false
    status = "rejected"
    rejection = [ordered]@{
        reason = "gate_or_evidence_validation_failed"
        validation_error = $validationError
        follow_up_required = $true
    }
    source = [ordered]@{
        commit = $ExpectedCommit
        workflow = ".github/workflows/ci.yml"
        gate_contract = "tests/ci/dr06-gates.json"
        gate_contract_sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $contractPath).Hash.ToLowerInvariant()
    }
    release_version = $ReleaseVersion
    workflow_run = [ordered]@{
        run_id = $RunID
        run_attempt = $RunAttempt
    }
    environment_decisions = [ordered]@{
        native_install = $safeNativeInstallContract
    }
    retention = [ordered]@{
        staging_days = 90
        durable_evidence_uri = $safeDurableEvidenceURI
        handoff_status = "required_for_rejected_packet"
    }
    gates = $gates
    artifacts = $artifacts
    generated_at = (Get-Date).ToUniversalTime().ToString("o")
}

$parent = Split-Path -Parent $indexPath
[IO.Directory]::CreateDirectory($parent) | Out-Null
$index | ConvertTo-Json -Depth 20 | Set-Content -Encoding utf8 -LiteralPath $indexPath
Write-Host "dr06-rejection-index=created gates=$($gates.Count) artifacts=$($artifacts.Count)"
