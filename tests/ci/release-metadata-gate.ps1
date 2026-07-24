param(
    [Parameter(Mandatory = $true)]
    [string]$ArtifactDir,
    [Parameter(Mandatory = $true)]
    [string]$ExpectedVersion,
    [Parameter(Mandatory = $true)]
    [string]$ExpectedCommit,
    [string]$ExpectedSourceDigest,
    [string]$ExpectedMaterialsPolicyPath
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$verifier = Join-Path $root "scripts/release/verify.ps1"
$artifactPath = [IO.Path]::GetFullPath($ArtifactDir)
function Invoke-Verifier([switch]$RequirePublishedImages, [string]$ExpectedImageNamespace) {
    $parameters = @{
        ArtifactDir = $artifactPath
        ExpectedVersion = $ExpectedVersion
        ExpectedCommit = $ExpectedCommit
    }
    if (-not [string]::IsNullOrWhiteSpace($ExpectedMaterialsPolicyPath)) {
        $parameters.ExpectedMaterialsPolicyPath = $ExpectedMaterialsPolicyPath
    }
    if (-not [string]::IsNullOrWhiteSpace($ExpectedSourceDigest)) {
        $parameters.ExpectedSourceDigest = $ExpectedSourceDigest
    }
    if ($RequirePublishedImages) {
        $parameters.RequirePublishedImages = $true
        $parameters.ExpectedImageNamespace = $ExpectedImageNamespace
    }
    & $verifier @parameters
}

function Assert-Rejected([string]$Case, [scriptblock]$Mutation, [scriptblock]$Restore) {
    try {
        & $Mutation
        $rejected = $false
        try {
            Invoke-Verifier
        } catch {
            $rejected = $true
        }
        if (-not $rejected) { throw "release verifier accepted invalid metadata: $Case" }
    } finally {
        & $Restore
    }
}

function Set-OuterChecksum([string]$Name) {
    $checksumPath = Join-Path $artifactPath "SHA256SUMS"
    $lines = [Collections.Generic.List[string]]::new()
    $lines.AddRange([string[]](Get-Content -LiteralPath $checksumPath))
    $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $artifactPath $Name)).Hash.ToLowerInvariant()
    for ($index = 0; $index -lt $lines.Count; $index++) {
        if ($lines[$index] -match "^[0-9a-f]{64}  $([regex]::Escape($Name))$") {
            $lines[$index] = "$hash  $Name"
            [IO.File]::WriteAllLines($checksumPath, $lines, [Text.UTF8Encoding]::new($false))
            return
        }
    }
    throw "SHA256SUMS has no entry for $Name"
}

function Assert-ManifestTamperRejected([string]$ManifestPath, [switch]$KeepOuterValid) {
    $originalBytes = [IO.File]::ReadAllBytes($ManifestPath)
    $checksumPath = Join-Path $artifactPath "SHA256SUMS"
    $originalChecksumBytes = [IO.File]::ReadAllBytes($checksumPath)
    try {
        $lines = [Collections.Generic.List[string]]::new()
        $lines.AddRange([string[]](Get-Content -LiteralPath $ManifestPath))
        if ($lines.Count -eq 0) { throw "cannot exercise tamper rejection on an empty manifest: $ManifestPath" }
        $replacement = if ($lines[0][0] -eq '0') { '1' } else { '0' }
        $lines[0] = $replacement + $lines[0].Substring(1)
        [IO.File]::WriteAllLines($ManifestPath, $lines, [Text.UTF8Encoding]::new($false))
        if ($KeepOuterValid) { Set-OuterChecksum (Split-Path -Leaf $ManifestPath) }
        Assert-Rejected "tampered $(Split-Path -Leaf $ManifestPath)" {} {}
    } finally {
        [IO.File]::WriteAllBytes($ManifestPath, $originalBytes)
        [IO.File]::WriteAllBytes($checksumPath, $originalChecksumBytes)
    }
}

Invoke-Verifier
Assert-ManifestTamperRejected (Join-Path $artifactPath "SHA256SUMS")
Assert-ManifestTamperRejected (Join-Path $artifactPath "ARTIFACTS.sha256") -KeepOuterValid

$provenancePath = Join-Path $artifactPath "provenance.unsigned.intoto.json"
$provenanceBytes = [IO.File]::ReadAllBytes($provenancePath)
$checksumBytes = [IO.File]::ReadAllBytes((Join-Path $artifactPath "SHA256SUMS"))
Assert-Rejected "provenance version mismatch" {
    $document = Get-Content -Raw -LiteralPath $provenancePath | ConvertFrom-Json
    $document.predicate.buildDefinition.externalParameters.version = "v-invalid"
    $document | ConvertTo-Json -Depth 20 | Set-Content -Encoding utf8 -LiteralPath $provenancePath
    Set-OuterChecksum "provenance.unsigned.intoto.json"
} {
    [IO.File]::WriteAllBytes($provenancePath, $provenanceBytes)
    [IO.File]::WriteAllBytes((Join-Path $artifactPath "SHA256SUMS"), $checksumBytes)
}

function Assert-ProvenanceMaterialSubstitutionRejected([string]$Name) {
    $provenanceBytes = [IO.File]::ReadAllBytes($provenancePath)
    $checksumBytes = [IO.File]::ReadAllBytes((Join-Path $artifactPath "SHA256SUMS"))
    Assert-Rejected "provenance material substitution: $Name" {
        $document = Get-Content -Raw -LiteralPath $provenancePath | ConvertFrom-Json
        $material = @($document.predicate.buildDefinition.resolvedDependencies | Where-Object name -ceq $Name)
        if ($material.Count -ne 1) { throw "test material is missing: $Name" }
        $material[0].digest.sha256 = "f" * 64
        $document | ConvertTo-Json -Depth 20 | Set-Content -Encoding utf8 -LiteralPath $provenancePath
        Set-OuterChecksum "provenance.unsigned.intoto.json"
    } {
        [IO.File]::WriteAllBytes($provenancePath, $provenanceBytes)
        [IO.File]::WriteAllBytes((Join-Path $artifactPath "SHA256SUMS"), $checksumBytes)
    }
}

Assert-ProvenanceMaterialSubstitutionRejected "source"
Assert-ProvenanceMaterialSubstitutionRejected "image:go"
Assert-ProvenanceMaterialSubstitutionRejected "image:debian"

if ([string]::IsNullOrWhiteSpace($ExpectedMaterialsPolicyPath)) {
    $policyPath = Join-Path $root "scripts/release/materials.json"
    $policyBytes = [IO.File]::ReadAllBytes($policyPath)
    $provenanceBytes = [IO.File]::ReadAllBytes($provenancePath)
    $checksumBytes = [IO.File]::ReadAllBytes((Join-Path $artifactPath "SHA256SUMS"))
    Assert-Rejected "worktree policy and provenance substitution" {
        $replacementDigest = "f" * 64
        $policy = Get-Content -Raw -LiteralPath $policyPath | ConvertFrom-Json
        $base = @($policy.images | Where-Object name -ceq "debian")
        if ($base.Count -ne 1) { throw "test base material is missing" }
        $base[0].reference = $base[0].reference -replace 'sha256:[0-9a-f]{64}$', "sha256:$replacementDigest"
        $base[0].digest.sha256 = $replacementDigest
        $policy | ConvertTo-Json -Depth 20 | Set-Content -Encoding utf8 -LiteralPath $policyPath

        $document = Get-Content -Raw -LiteralPath $provenancePath | ConvertFrom-Json
        $material = @($document.predicate.buildDefinition.resolvedDependencies | Where-Object name -ceq "image:debian")
        if ($material.Count -ne 1) { throw "test provenance base material is missing" }
        $material[0].uri = $base[0].reference
        $material[0].digest.sha256 = $replacementDigest
        $document | ConvertTo-Json -Depth 20 | Set-Content -Encoding utf8 -LiteralPath $provenancePath
        Set-OuterChecksum "provenance.unsigned.intoto.json"
    } {
        [IO.File]::WriteAllBytes($policyPath, $policyBytes)
        [IO.File]::WriteAllBytes($provenancePath, $provenanceBytes)
        [IO.File]::WriteAllBytes((Join-Path $artifactPath "SHA256SUMS"), $checksumBytes)
    }
}

$sbomPath = Join-Path $artifactPath "sbom.cdx.json"
$sbomBytes = [IO.File]::ReadAllBytes($sbomPath)
$checksumBytes = [IO.File]::ReadAllBytes((Join-Path $artifactPath "SHA256SUMS"))
Assert-Rejected "SBOM identity mismatch" {
    $document = Get-Content -Raw -LiteralPath $sbomPath | ConvertFrom-Json
    $document.metadata.component.name = "not-ardents"
    $document | ConvertTo-Json -Depth 20 | Set-Content -Encoding utf8 -LiteralPath $sbomPath
    Set-OuterChecksum "sbom.cdx.json"
} {
    [IO.File]::WriteAllBytes($sbomPath, $sbomBytes)
    [IO.File]::WriteAllBytes((Join-Path $artifactPath "SHA256SUMS"), $checksumBytes)
}

$releaseManifestPath = Join-Path $artifactPath "release-manifest.json"
$releaseManifestBytes = [IO.File]::ReadAllBytes($releaseManifestPath)
$checksumBytes = [IO.File]::ReadAllBytes((Join-Path $artifactPath "SHA256SUMS"))
Assert-Rejected "ingress proxy protocol mismatch" {
    $document = Get-Content -Raw -LiteralPath $releaseManifestPath | ConvertFrom-Json
    $document.components.ingress_proxy.protocol_version = "invalid"
    $document | ConvertTo-Json -Depth 20 | Set-Content -Encoding utf8 -LiteralPath $releaseManifestPath
    Set-OuterChecksum "release-manifest.json"
} {
    [IO.File]::WriteAllBytes($releaseManifestPath, $releaseManifestBytes)
    [IO.File]::WriteAllBytes((Join-Path $artifactPath "SHA256SUMS"), $checksumBytes)
}

$requiredArtifact = Join-Path $artifactPath "ardentsd-$ExpectedVersion-linux-amd64"
$hiddenArtifact = "$requiredArtifact.missing"
Assert-Rejected "missing canonical artifact" {
    Move-Item -LiteralPath $requiredArtifact -Destination $hiddenArtifact
} {
    if (Test-Path -LiteralPath $hiddenArtifact) { Move-Item -LiteralPath $hiddenArtifact -Destination $requiredArtifact }
}

$extraArtifact = Join-Path $artifactPath "unexpected.txt"
Assert-Rejected "extra payload" {
    Set-Content -LiteralPath $extraArtifact -Value "unexpected"
} {
    if (Test-Path -LiteralPath $extraArtifact) { Remove-Item -LiteralPath $extraArtifact -Force }
}

$publishedImagesPath = Join-Path $artifactPath "published-images.json"
$checksumPath = Join-Path $artifactPath "SHA256SUMS"
$checksumBytes = [IO.File]::ReadAllBytes($checksumPath)
$expectedIngressProtocol = (Get-Content -Raw -LiteralPath (Join-Path $artifactPath "release-manifest.json") |
    ConvertFrom-Json).components.ingress_proxy.protocol_version
try {
    $publishedImages = [ordered]@{
        schema_version = 1
        version = $ExpectedVersion
        commit = $ExpectedCommit
        images = [ordered]@{
            node = [ordered]@{ name = "ghcr.io/example/ardents-node"; digest = "sha256:$('a' * 64)" }
            ingress_proxy = [ordered]@{
                name = "ghcr.io/example/ardents-ingress-proxy"
                digest = "sha256:$('b' * 64)"
                protocol_version = $expectedIngressProtocol
                optional = $true
            }
        }
    }
    $publishedImages | ConvertTo-Json -Depth 8 | Set-Content -Encoding utf8 -LiteralPath $publishedImagesPath
    $lines = [Collections.Generic.List[string]]::new()
    $lines.AddRange([string[]](Get-Content -LiteralPath $checksumPath))
    $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $publishedImagesPath).Hash.ToLowerInvariant()
    $lines.Add("$hash  published-images.json")
    [IO.File]::WriteAllLines($checksumPath, @($lines | Sort-Object), [Text.UTF8Encoding]::new($false))
    Invoke-Verifier -RequirePublishedImages -ExpectedImageNamespace "ghcr.io/example"
    $wrongNamespaceRejected = $false
    try {
        Invoke-Verifier -RequirePublishedImages -ExpectedImageNamespace "ghcr.io/not-example"
    } catch {
        $wrongNamespaceRejected = $true
    }
    if (-not $wrongNamespaceRejected) { throw "published image verifier accepted the wrong namespace" }
    Assert-Rejected "published manifest accepted by candidate verifier" {} {}
} finally {
    if (Test-Path -LiteralPath $publishedImagesPath) { Remove-Item -LiteralPath $publishedImagesPath -Force }
    [IO.File]::WriteAllBytes($checksumPath, $checksumBytes)
}
Invoke-Verifier
Write-Host "release-metadata-gate=passed"
