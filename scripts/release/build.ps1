param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$')]
    [string]$Version,
    [string]$Commit,
    [string]$OutputDir,
    [long]$SourceDateEpoch = 0
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location $root

if ([string]::IsNullOrWhiteSpace($Commit)) {
    $Commit = (& git rev-parse HEAD).Trim()
}
if ($Commit -notmatch '^[0-9a-fA-F]{7,40}$') { throw "Commit must be a Git object ID" }
$headCommit = (& git rev-parse --verify HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $headCommit -notmatch '^[0-9a-f]{40}$') {
    throw "release source HEAD is unavailable"
}
$resolvedCommit = (& git rev-parse --verify "$Commit^{commit}" 2>$null).Trim()
if ($LASTEXITCODE -ne 0 -or $resolvedCommit -notmatch '^[0-9a-f]{40}$') {
    throw "Commit must name an existing Git commit"
}
if ($resolvedCommit -ne $headCommit) {
    throw "release Commit must resolve to current HEAD; historical trees are not built from the active worktree"
}
$Commit = $headCommit
$dirtyEntries = @(& git status --porcelain=v1 --untracked-files=all)
if ($LASTEXITCODE -ne 0) { throw "release source status is unavailable" }
if ($dirtyEntries.Count -ne 0) {
    throw "release source tree must be clean; tracked and untracked changes are not allowed"
}
if ($SourceDateEpoch -le 0) {
    $SourceDateEpoch = [long]((& git show -s --format=%ct $Commit).Trim())
}
if ([string]::IsNullOrWhiteSpace($OutputDir)) {
    $OutputDir = Join-Path $root "dist/$Version"
}
$outputPath = [IO.Path]::GetFullPath($OutputDir)
if (Test-Path -LiteralPath $outputPath) {
    throw "release output already exists; choose an empty path: $outputPath"
}

$runID = [guid]::NewGuid().ToString("N")
$snapshotPath = Join-Path ([IO.Path]::GetTempPath()) "ardents-release-source-$runID"
$archivePath = Join-Path ([IO.Path]::GetTempPath()) "ardents-release-source-$runID.tar"
[IO.Directory]::CreateDirectory($snapshotPath) | Out-Null

try {
    & git -c core.autocrlf=false archive --format=tar --output=$archivePath $Commit
    if ($LASTEXITCODE -ne 0) { throw "release source snapshot failed" }
    & tar -xf $archivePath -C $snapshotPath
    if ($LASTEXITCODE -ne 0) { throw "release source snapshot extraction failed" }

    [IO.Directory]::CreateDirectory($outputPath) | Out-Null
    $buildDate = [DateTimeOffset]::FromUnixTimeSeconds($SourceDateEpoch).UtcDateTime.ToString("yyyy-MM-ddTHH:mm:ssZ")
    $ingressProtocol = (Get-Content -Raw -LiteralPath (Join-Path $snapshotPath "internal/ingressproxy/protocol_version.txt")).Trim()
    if ($ingressProtocol -notmatch '^[1-9][0-9]*$') { throw "ingress proxy protocol version is invalid" }

    docker run --rm `
        -e "ARDENTS_VERSION=$Version" `
        -e "ARDENTS_COMMIT=$Commit" `
        -e "ARDENTS_BUILD_DATE=$buildDate" `
        -e "SOURCE_DATE_EPOCH=$SourceDateEpoch" `
        -e "ARDENTS_SOURCE_DIRTY=false" `
        --mount "type=bind,source=$snapshotPath,target=/workspace,readonly" `
        --mount "type=bind,source=$outputPath,target=/out" `
        --mount source=ardents-go-build-cache,target=/root/.cache/go-build `
        --mount source=ardents-go-mod-cache,target=/go/pkg/mod `
        -w /workspace golang:1.26-bookworm /bin/sh scripts/release/bundle.sh
    if ($LASTEXITCODE -ne 0) { throw "Docker release build failed" }

    $imageTag = "ardents/node:$Version"
    $proxyImageTag = "ardents/ingress-proxy:$Version"
    $previousGitInfo = $env:BUILDX_GIT_INFO
    try {
        $env:BUILDX_GIT_INFO = "false"
        docker build `
            --provenance=false `
            --build-arg "ARDENTS_VERSION=$Version" `
            --build-arg "ARDENTS_COMMIT=$Commit" `
            --build-arg "ARDENTS_BUILD_DATE=$buildDate" `
            --build-arg "GO_BUILD_PARALLELISM=2" `
            -t $imageTag -f (Join-Path $snapshotPath "deploy/docker/images/node.Dockerfile") $snapshotPath
        if ($LASTEXITCODE -ne 0) { throw "Docker release image build failed" }
        docker build `
            --provenance=false `
            --build-arg "ARDENTS_VERSION=$Version" `
            --build-arg "ARDENTS_COMMIT=$Commit" `
            --build-arg "ARDENTS_BUILD_DATE=$buildDate" `
            --build-arg "ARDENTS_INGRESS_PROTOCOL=$ingressProtocol" `
            --build-arg "GO_BUILD_PARALLELISM=2" `
            -t $proxyImageTag -f (Join-Path $snapshotPath "deploy/docker/images/workload-ingress-proxy.Dockerfile") $snapshotPath
        if ($LASTEXITCODE -ne 0) { throw "Docker ingress proxy release image build failed" }
    } finally {
        if ($null -eq $previousGitInfo) { Remove-Item Env:BUILDX_GIT_INFO -ErrorAction SilentlyContinue }
        else { $env:BUILDX_GIT_INFO = $previousGitInfo }
    }
    docker save -o (Join-Path $outputPath "ardents-$Version-linux-amd64.docker.tar") $imageTag
    if ($LASTEXITCODE -ne 0) { throw "Docker release image export failed" }
    docker save -o (Join-Path $outputPath "ardents-ingress-proxy-$Version-linux-amd64.docker.tar") $proxyImageTag
    if ($LASTEXITCODE -ne 0) { throw "Docker ingress proxy release image export failed" }

    docker run --rm `
        -e "ARDENTS_VERSION=$Version" `
        -e "ARDENTS_COMMIT=$Commit" `
        -e "ARDENTS_BUILD_DATE=$buildDate" `
        -e "SOURCE_DATE_EPOCH=$SourceDateEpoch" `
        -e "ARDENTS_SOURCE_DIRTY=false" `
        -e "ARDENTS_INGRESS_PROTOCOL=$ingressProtocol" `
        --mount "type=bind,source=$snapshotPath,target=/workspace,readonly" `
        --mount "type=bind,source=$outputPath,target=/out" `
        -w /workspace mcr.microsoft.com/powershell:7.5-debian-12 `
        pwsh -NoLogo -NoProfile -File scripts/release/metadata.ps1 -ArtifactDir /out
    if ($LASTEXITCODE -ne 0) { throw "release metadata generation failed" }

    & (Join-Path $snapshotPath "scripts/release/verify.ps1") `
        -ArtifactDir $outputPath -ExpectedVersion $Version -ExpectedCommit $Commit -RequireClean

    Write-Host "Release artifacts created: $outputPath"
} finally {
    if (Test-Path -LiteralPath $archivePath) { Remove-Item -LiteralPath $archivePath -Force }
    if (Test-Path -LiteralPath $snapshotPath) { Remove-Item -LiteralPath $snapshotPath -Recurse -Force }
}
