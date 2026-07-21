param(
    [Parameter(Mandatory = $true)]
    [string]$ArtifactDir,
    [Parameter(Mandatory = $true)]
    [string]$ExpectedVersion,
    [Parameter(Mandatory = $true)]
    [string]$ExpectedCommit
)

$ErrorActionPreference = "Stop"
$artifactPath = [IO.Path]::GetFullPath($ArtifactDir)
$releaseManifest = Get-Content -Raw -LiteralPath (Join-Path $artifactPath "release-manifest.json") | ConvertFrom-Json
$expectedIngressProtocol = $releaseManifest.components.ingress_proxy.protocol_version
$images = @(
    [ordered]@{
        archive = "ardents-$ExpectedVersion-linux-amd64.docker.tar"
        reference = "ardents/node:$ExpectedVersion"
        title = "Ardents Network"
    },
    [ordered]@{
        archive = "ardents-ingress-proxy-$ExpectedVersion-linux-amd64.docker.tar"
        reference = "ardents/ingress-proxy:$ExpectedVersion"
        title = "Ardents Workload Ingress Proxy"
    }
)

foreach ($image in $images) {
    & docker load --input (Join-Path $artifactPath $image.archive) | Out-Host
    if ($LASTEXITCODE -ne 0) { throw "release image load failed: $($image.archive)" }
    $inspection = & docker image inspect $image.reference | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0 -or $inspection.Count -ne 1) { throw "release image inspect failed: $($image.reference)" }
    $labels = $inspection[0].Config.Labels
    if ($labels.'org.opencontainers.image.title' -ne $image.title -or
        $labels.'org.opencontainers.image.version' -ne $ExpectedVersion -or
        $labels.'org.opencontainers.image.revision' -ne $ExpectedCommit) {
        throw "release image identity mismatch: $($image.reference)"
    }
}

$proxy = (& docker image inspect "ardents/ingress-proxy:$ExpectedVersion" | ConvertFrom-Json)[0]
if ($proxy.Config.User -ne "65534:65534" -or
    $proxy.Config.Entrypoint.Count -ne 1 -or
    $proxy.Config.Entrypoint[0] -ne "/ardents-ingress-proxy" -or
    $proxy.Config.Labels.'io.ardents.ingress.protocol' -ne $expectedIngressProtocol) {
    throw "release ingress proxy runtime contract mismatch"
}

Write-Host "release-images-gate=passed"
