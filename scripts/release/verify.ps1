param(
    [Parameter(Mandatory = $true)]
    [string]$ArtifactDir,
    [Parameter(Mandatory = $true)]
    [string]$ExpectedVersion,
    [Parameter(Mandatory = $true)]
    [string]$ExpectedCommit,
    [switch]$RequireClean,
    [switch]$RequirePublishedImages,
    [string]$ExpectedImageNamespace
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$expectedIngressProtocol = (Get-Content -Raw -LiteralPath (Join-Path $root "internal/ingressproxy/protocol_version.txt")).Trim()
$artifactPath = [IO.Path]::GetFullPath($ArtifactDir)
$checksumPath = Join-Path $artifactPath "SHA256SUMS"
$subjectChecksumPath = Join-Path $artifactPath "ARTIFACTS.sha256"
$provenancePath = Join-Path $artifactPath "provenance.unsigned.intoto.json"
$sbomPath = Join-Path $artifactPath "sbom.cdx.json"
$releaseManifestPath = Join-Path $artifactPath "release-manifest.json"
$publishedImagesPath = Join-Path $artifactPath "published-images.json"

foreach ($required in $checksumPath, $subjectChecksumPath, $provenancePath, $sbomPath, $releaseManifestPath) {
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
    "ardents-ingress-proxy-$ExpectedVersion-linux-amd64.docker.tar"
    "ardents-$ExpectedVersion-linux-amd64.tar.gz"
    "ardentsctl-$ExpectedVersion-linux-amd64"
    "ardentsd-$ExpectedVersion-linux-amd64"
) | Sort-Object
$expectedPayloadNames = @(
    $expectedSubjectNames
    "ARTIFACTS.sha256"
    "provenance.unsigned.intoto.json"
    "release-manifest.json"
    "sbom.cdx.json"
)
if ($RequirePublishedImages) {
    if ([string]::IsNullOrWhiteSpace($ExpectedImageNamespace)) {
        throw "ExpectedImageNamespace is required for published image verification"
    }
    if (-not (Test-Path -LiteralPath $publishedImagesPath -PathType Leaf)) {
        throw "published release metadata is incomplete: missing $publishedImagesPath"
    }
    $expectedPayloadNames += "published-images.json"
}
$expectedPayloadNames = @($expectedPayloadNames | Sort-Object)
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

$releaseManifest = Get-Content -Raw -LiteralPath $releaseManifestPath | ConvertFrom-Json
if ($releaseManifest.schema_version -ne 1 -or $releaseManifest.platform -ne "linux/amd64") {
    throw "release manifest contract mismatch"
}
if ($releaseManifest.version -ne $ExpectedVersion -or $releaseManifest.commit -ne $ExpectedCommit) {
    throw "release manifest identity mismatch"
}
if ($releaseManifest.components.node.artifact -ne "ardents-$ExpectedVersion-linux-amd64.docker.tar" -or
    $releaseManifest.components.node.local_image -ne "ardents/node:$ExpectedVersion") {
    throw "release manifest node component mismatch"
}
if ($releaseManifest.components.ingress_proxy.artifact -ne "ardents-ingress-proxy-$ExpectedVersion-linux-amd64.docker.tar" -or
    $releaseManifest.components.ingress_proxy.local_image -ne "ardents/ingress-proxy:$ExpectedVersion" -or
    $releaseManifest.components.ingress_proxy.protocol_version -ne $expectedIngressProtocol -or
    -not [bool]$releaseManifest.components.ingress_proxy.optional) {
    throw "release manifest ingress proxy component mismatch"
}

if ($RequirePublishedImages) {
    $imageNamespace = $ExpectedImageNamespace.Trim().TrimEnd('/').ToLowerInvariant()
    $expectedNodeImage = "$imageNamespace/ardents-node"
    $expectedProxyImage = "$imageNamespace/ardents-ingress-proxy"
    $publishedImages = Get-Content -Raw -LiteralPath $publishedImagesPath | ConvertFrom-Json
    if ($publishedImages.schema_version -ne 1 -or $publishedImages.version -ne $ExpectedVersion -or
        $publishedImages.commit -ne $ExpectedCommit) {
        throw "published image manifest identity mismatch"
    }
    if ($publishedImages.images.node.name -cne $expectedNodeImage -or
        $publishedImages.images.ingress_proxy.name -cne $expectedProxyImage -or
        $publishedImages.images.node.name -cne $publishedImages.images.node.name.ToLowerInvariant() -or
        $publishedImages.images.ingress_proxy.name -cne $publishedImages.images.ingress_proxy.name.ToLowerInvariant() -or
        $publishedImages.images.node.digest -notmatch '^sha256:[0-9a-f]{64}$' -or
        $publishedImages.images.ingress_proxy.digest -notmatch '^sha256:[0-9a-f]{64}$' -or
        $publishedImages.images.ingress_proxy.protocol_version -ne $expectedIngressProtocol -or
        -not [bool]$publishedImages.images.ingress_proxy.optional) {
        throw "published image manifest component mismatch"
    }
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
