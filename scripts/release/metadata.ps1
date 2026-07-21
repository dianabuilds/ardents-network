param(
    [Parameter(Mandatory = $true)]
    [string]$ArtifactDir
)

$ErrorActionPreference = "Stop"
$artifactPath = [IO.Path]::GetFullPath($ArtifactDir)
$version = $env:ARDENTS_VERSION
$commit = $env:ARDENTS_COMMIT
$buildDate = $env:ARDENTS_BUILD_DATE
$epoch = [long]$env:SOURCE_DATE_EPOCH
$sourceDirty = [bool]::Parse($env:ARDENTS_SOURCE_DIRTY)
$ingressProtocol = $env:ARDENTS_INGRESS_PROTOCOL
if ([string]::IsNullOrWhiteSpace($version) -or [string]::IsNullOrWhiteSpace($commit) -or $epoch -le 0 -or
    $ingressProtocol -notmatch '^[1-9][0-9]*$') {
    throw "release identity environment is incomplete"
}

$modules = foreach ($line in Get-Content -LiteralPath (Join-Path $artifactPath "modules.tsv")) {
    $parts = $line -split "`t", 3
    if ($parts.Count -lt 1 -or [string]::IsNullOrWhiteSpace($parts[0])) { continue }
    $moduleVersion = if ($parts.Count -gt 1 -and $parts[1]) { $parts[1] } else { "unknown" }
    [ordered]@{
        type = "library"
        name = $parts[0]
        version = $moduleVersion
        purl = "pkg:golang/$($parts[0])@$moduleVersion"
    }
}

$sbom = [ordered]@{
    bomFormat = "CycloneDX"
    specVersion = "1.5"
    version = 1
    metadata = [ordered]@{
        timestamp = $buildDate
        component = [ordered]@{ type = "application"; name = "ardents"; version = $version }
        tools = @([ordered]@{ vendor = "Ardents"; name = "release-metadata.ps1"; version = "1" })
    }
    components = @($modules)
}
$sbom | ConvertTo-Json -Depth 10 | Set-Content -Encoding utf8 -LiteralPath (Join-Path $artifactPath "sbom.cdx.json")

$releaseManifest = [ordered]@{
    schema_version = 1
    version = $version
    commit = $commit
    platform = "linux/amd64"
    components = [ordered]@{
        node = [ordered]@{
            artifact = "ardents-$version-linux-amd64.docker.tar"
            local_image = "ardents/node:$version"
        }
        ingress_proxy = [ordered]@{
            artifact = "ardents-ingress-proxy-$version-linux-amd64.docker.tar"
            local_image = "ardents/ingress-proxy:$version"
            protocol_version = $ingressProtocol
            optional = $true
        }
    }
}
$releaseManifest | ConvertTo-Json -Depth 8 | Set-Content -Encoding utf8 -LiteralPath (Join-Path $artifactPath "release-manifest.json")

$subjects = foreach ($file in Get-ChildItem -LiteralPath $artifactPath -File | Where-Object {
    $_.Name -match '^ardents(?:ctl|d)-.+-linux-' -or $_.Name -match '^ardents-.+\.(tar\.gz|docker\.tar)$'
} | Sort-Object Name) {
    [ordered]@{
        name = $file.Name
        digest = [ordered]@{ sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $file.FullName).Hash.ToLowerInvariant() }
    }
}

$provenance = [ordered]@{
    _type = "https://in-toto.io/Statement/v1"
    subject = @($subjects)
    predicateType = "https://slsa.dev/provenance/v1"
    predicate = [ordered]@{
        buildDefinition = [ordered]@{
            buildType = "https://ardents.net/build/docker-go/v1"
            externalParameters = [ordered]@{
                version = $version
                commit = $commit
                source_date_epoch = $epoch
                source_dirty = $sourceDirty
                targets = @("linux/amd64")
            }
            resolvedDependencies = @($modules | ForEach-Object {
                [ordered]@{ uri = $_.purl; digest = [ordered]@{} }
            })
        }
        runDetails = [ordered]@{
            builder = [ordered]@{ id = "pkg:docker/golang@1.26-bookworm" }
            metadata = [ordered]@{ invocationId = "$commit-$epoch"; startedOn = $buildDate; finishedOn = $buildDate }
        }
    }
}
$provenance | ConvertTo-Json -Depth 12 | Set-Content -Encoding utf8 -LiteralPath (Join-Path $artifactPath "provenance.unsigned.intoto.json")

$subjectChecksumLines = foreach ($subject in $subjects) {
    "$($subject.digest.sha256)  $($subject.name)"
}
[IO.File]::WriteAllLines((Join-Path $artifactPath "ARTIFACTS.sha256"), $subjectChecksumLines, [Text.UTF8Encoding]::new($false))

Remove-Item -LiteralPath (Join-Path $artifactPath "modules.tsv") -Force

$checksumFiles = Get-ChildItem -LiteralPath $artifactPath -File |
    Where-Object { $_.Name -ne "SHA256SUMS" } |
    Sort-Object Name
$checksumLines = foreach ($file in $checksumFiles) {
    $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $file.FullName).Hash.ToLowerInvariant()
    "$hash  $($file.Name)"
}
[IO.File]::WriteAllLines((Join-Path $artifactPath "SHA256SUMS"), $checksumLines, [Text.UTF8Encoding]::new($false))
