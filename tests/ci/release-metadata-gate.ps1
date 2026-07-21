param(
    [Parameter(Mandatory = $true)]
    [string]$ArtifactDir,
    [Parameter(Mandatory = $true)]
    [string]$ExpectedVersion,
    [Parameter(Mandatory = $true)]
    [string]$ExpectedCommit
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$verifier = Join-Path $root "scripts/release/verify.ps1"
$artifactPath = [IO.Path]::GetFullPath($ArtifactDir)
function Assert-Rejected([string]$Case, [scriptblock]$Mutation, [scriptblock]$Restore) {
    try {
        & $Mutation
        $rejected = $false
        try {
            & $verifier -ArtifactDir $artifactPath -ExpectedVersion $ExpectedVersion -ExpectedCommit $ExpectedCommit
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

& $verifier -ArtifactDir $artifactPath -ExpectedVersion $ExpectedVersion -ExpectedCommit $ExpectedCommit
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
    & $verifier -ArtifactDir $artifactPath -ExpectedVersion $ExpectedVersion -ExpectedCommit $ExpectedCommit `
        -RequirePublishedImages -ExpectedImageNamespace "ghcr.io/example"
    $wrongNamespaceRejected = $false
    try {
        & $verifier -ArtifactDir $artifactPath -ExpectedVersion $ExpectedVersion -ExpectedCommit $ExpectedCommit `
            -RequirePublishedImages -ExpectedImageNamespace "ghcr.io/not-example"
    } catch {
        $wrongNamespaceRejected = $true
    }
    if (-not $wrongNamespaceRejected) { throw "published image verifier accepted the wrong namespace" }
    Assert-Rejected "published manifest accepted by candidate verifier" {} {}
} finally {
    if (Test-Path -LiteralPath $publishedImagesPath) { Remove-Item -LiteralPath $publishedImagesPath -Force }
    [IO.File]::WriteAllBytes($checksumPath, $checksumBytes)
}
& $verifier -ArtifactDir $artifactPath -ExpectedVersion $ExpectedVersion -ExpectedCommit $ExpectedCommit
Write-Host "release-metadata-gate=passed"
