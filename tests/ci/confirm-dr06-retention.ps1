param(
    [Parameter(Mandatory = $true)]
    [string]$IndexPath,
    [Parameter(Mandatory = $true)]
    [string]$EvidenceRoot,
    [Parameter(Mandatory = $true)]
    [string]$DurableEvidenceURI,
    [Parameter(Mandatory = $true)]
    [string]$ReceiptPath
)

$ErrorActionPreference = "Stop"
if ([string]::IsNullOrWhiteSpace($DurableEvidenceURI) -or
    $DurableEvidenceURI -match '[?#]' -or
    $DurableEvidenceURI -match '^[a-z][a-z0-9+.-]*://[^/]*@') {
    throw "durable evidence reference is missing or contains private URL material"
}
$indexFullPath = [IO.Path]::GetFullPath($IndexPath)
$evidenceFullPath = [IO.Path]::GetFullPath($EvidenceRoot)
$receiptFullPath = [IO.Path]::GetFullPath($ReceiptPath)
if (-not (Test-Path -LiteralPath $indexFullPath -PathType Leaf)) {
    throw "DR-06 qualification index is unavailable"
}
if (-not (Test-Path -LiteralPath $evidenceFullPath -PathType Container)) {
    throw "durable DR-06 evidence mirror is unavailable"
}

$index = Get-Content -Raw -LiteralPath $indexFullPath | ConvertFrom-Json
if ($index.schema_version -ne 1 -or [bool]$index.qualification_accepted) {
    throw "DR-06 qualification index contract mismatch"
}
if ($index.retention.durable_evidence_uri -cne $DurableEvidenceURI) {
    throw "durable evidence authority differs from the qualification index"
}
if (@($index.artifacts).Count -eq 0) {
    throw "DR-06 qualification index contains no artifacts"
}

$rootPrefix = $evidenceFullPath.TrimEnd(
    [IO.Path]::DirectorySeparatorChar,
    [IO.Path]::AltDirectorySeparatorChar
) + [IO.Path]::DirectorySeparatorChar
foreach ($artifact in @($index.artifacts)) {
    $candidate = [IO.Path]::GetFullPath((Join-Path $evidenceFullPath ([string]$artifact.path)))
    if (-not $candidate.StartsWith($rootPrefix, [StringComparison]::OrdinalIgnoreCase)) {
        throw "indexed artifact escapes the durable evidence root: $($artifact.path)"
    }
    if (-not (Test-Path -LiteralPath $candidate -PathType Leaf)) {
        throw "durable evidence artifact is missing: $($artifact.path)"
    }
    $item = Get-Item -LiteralPath $candidate
    $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $candidate).Hash.ToLowerInvariant()
    if ([long]$item.Length -ne [long]$artifact.size -or $hash -cne [string]$artifact.sha256) {
        throw "durable evidence artifact differs from the qualification index: $($artifact.path)"
    }
}

$receipt = [ordered]@{
    schema_version = 1
    qualification_accepted = $false
    release_owner_review_required = $true
    source_commit = [string]$index.source.commit
    release_version = [string]$index.release_version
    run_id = [string]$index.workflow_run.run_id
    run_attempt = [int]$index.workflow_run.run_attempt
    durable_evidence_uri = $DurableEvidenceURI
    artifact_count = @($index.artifacts).Count
    index_sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $indexFullPath).Hash.ToLowerInvariant()
    verified_at = (Get-Date).ToUniversalTime().ToString("o")
}
$parent = Split-Path -Parent $receiptFullPath
if ($parent) { [IO.Directory]::CreateDirectory($parent) | Out-Null }
$receipt | ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 -LiteralPath $receiptFullPath
Write-Host "dr06-retention=verified artifacts=$($receipt.artifact_count)"
