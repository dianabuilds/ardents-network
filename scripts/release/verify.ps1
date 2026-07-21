param(
    [Parameter(Mandatory = $true)]
    [string]$ArtifactDir,
    [Parameter(Mandatory = $true)]
    [string]$ExpectedVersion,
    [Parameter(Mandatory = $true)]
    [string]$ExpectedCommit,
    [switch]$RequireClean
)

$ErrorActionPreference = "Stop"
$artifactPath = [IO.Path]::GetFullPath($ArtifactDir)
$checksumPath = Join-Path $artifactPath "SHA256SUMS"
$subjectChecksumPath = Join-Path $artifactPath "ARTIFACTS.sha256"
$provenancePath = Join-Path $artifactPath "provenance.unsigned.intoto.json"
$sbomPath = Join-Path $artifactPath "sbom.cdx.json"

foreach ($required in $checksumPath, $subjectChecksumPath, $provenancePath, $sbomPath) {
    if (-not (Test-Path -LiteralPath $required -PathType Leaf)) {
        throw "release metadata is incomplete: missing $required"
    }
}
$unexpectedDirectories = @(Get-ChildItem -LiteralPath $artifactPath -Directory)
if ($unexpectedDirectories.Count -ne 0) {
    throw "release output contains internal directories: $($unexpectedDirectories.Name -join ', ')"
}

$checksums = [ordered]@{}
foreach ($line in Get-Content -LiteralPath $checksumPath) {
    if ($line -notmatch '^([0-9a-f]{64})  ([^/\\]+)$') {
        throw "invalid SHA256SUMS entry: $line"
    }
    $name = $Matches[2]
    if ($checksums.Contains($name)) { throw "duplicate SHA256SUMS entry: $name" }
    $checksums[$name] = $Matches[1]
}

$expectedSubjectNames = @(
    "ardents-$ExpectedVersion-linux-amd64.docker.tar"
    "ardents-$ExpectedVersion-linux-amd64.tar.gz"
    "ardentsctl-$ExpectedVersion-linux-amd64"
    "ardentsd-$ExpectedVersion-linux-amd64"
) | Sort-Object
$expectedPayloadNames = @(
    $expectedSubjectNames
    "ARTIFACTS.sha256"
    "provenance.unsigned.intoto.json"
    "sbom.cdx.json"
) | Sort-Object
$payloadNames = @(Get-ChildItem -LiteralPath $artifactPath -File |
    Where-Object Name -ne "SHA256SUMS" | Sort-Object Name | ForEach-Object Name)
$payloadDifference = @(Compare-Object $expectedPayloadNames $payloadNames)
if ($payloadDifference.Count -ne 0) {
    throw "release payload is not the canonical linux/amd64 set for ${ExpectedVersion}: $($payloadDifference | Out-String)"
}
$checksumNames = @($checksums.Keys | Sort-Object)
$difference = @(Compare-Object $payloadNames $checksumNames)
if ($difference.Count -ne 0) {
    throw "SHA256SUMS does not cover the exact release payload: $($difference | Out-String)"
}
foreach ($name in $checksumNames) {
    $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $artifactPath $name)).Hash.ToLowerInvariant()
    if ($actual -ne $checksums[$name]) { throw "checksum mismatch: $name" }
}

$provenance = Get-Content -Raw -LiteralPath $provenancePath | ConvertFrom-Json
if ($provenance._type -ne "https://in-toto.io/Statement/v1") { throw "unsupported provenance statement type" }
if ($provenance.predicateType -ne "https://slsa.dev/provenance/v1") { throw "unsupported provenance predicate type" }
$build = $provenance.predicate.buildDefinition.externalParameters
if ($build.version -ne $ExpectedVersion) { throw "provenance version mismatch" }
if ($build.commit -ne $ExpectedCommit) { throw "provenance commit mismatch" }
if ($RequireClean -and [bool]$build.source_dirty) { throw "release provenance reports a dirty source tree" }

$subjectChecksums = [ordered]@{}
foreach ($line in Get-Content -LiteralPath $subjectChecksumPath) {
    if ($line -notmatch '^([0-9a-f]{64})  ([^/\\]+)$') { throw "invalid ARTIFACTS.sha256 entry: $line" }
    if ($subjectChecksums.Contains($Matches[2])) { throw "duplicate ARTIFACTS.sha256 entry: $($Matches[2])" }
    $subjectChecksums[$Matches[2]] = $Matches[1]
}
$subjectManifestNames = @($subjectChecksums.Keys | Sort-Object)
$manifestDifference = @(Compare-Object $expectedSubjectNames $subjectManifestNames)
if ($manifestDifference.Count -ne 0) {
    throw "ARTIFACTS.sha256 does not match distributable artifacts: $($manifestDifference | Out-String)"
}
$subjects = @($provenance.subject)
$subjectNames = @($subjects | ForEach-Object name | Sort-Object)
$subjectDifference = @(Compare-Object $expectedSubjectNames $subjectNames)
if ($subjectDifference.Count -ne 0) {
    throw "provenance subjects do not match distributable artifacts: $($subjectDifference | Out-String)"
}
foreach ($subject in $subjects) {
    if (-not $checksums.Contains($subject.name)) { throw "provenance subject is absent from SHA256SUMS: $($subject.name)" }
    if ($subject.digest.sha256 -ne $checksums[$subject.name]) { throw "provenance digest mismatch: $($subject.name)" }
    if ($subject.digest.sha256 -ne $subjectChecksums[$subject.name]) { throw "ARTIFACTS.sha256 digest mismatch: $($subject.name)" }
}

$sbom = Get-Content -Raw -LiteralPath $sbomPath | ConvertFrom-Json
if ($sbom.bomFormat -ne "CycloneDX" -or $sbom.specVersion -ne "1.5") { throw "unsupported SBOM format" }
if ($sbom.metadata.component.name -ne "ardents") { throw "SBOM component identity mismatch" }
if ($sbom.metadata.component.version -ne $ExpectedVersion) { throw "SBOM version mismatch" }
if ($sbom.metadata.component.version -ne $build.version) { throw "SBOM and provenance versions differ" }

Write-Host "Release metadata verified: $artifactPath"
