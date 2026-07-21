param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$')]
    [string]$Version,
    [string]$Commit,
    [string]$OutputDir,
    [long]$SourceDateEpoch = 0,
    [switch]$AllowDirty
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location $root

if ([string]::IsNullOrWhiteSpace($Commit)) {
    $Commit = (& git rev-parse HEAD).Trim()
}
if ($Commit -notmatch '^[0-9a-fA-F]{7,40}$') { throw "Commit must be a Git object ID" }
$dirty = -not [string]::IsNullOrWhiteSpace(((& git status --porcelain) -join "`n"))
if ($dirty -and -not $AllowDirty) {
    throw "release source tree is dirty; commit the release candidate or use -AllowDirty only for local verification"
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
[IO.Directory]::CreateDirectory($outputPath) | Out-Null

$buildDate = [DateTimeOffset]::FromUnixTimeSeconds($SourceDateEpoch).UtcDateTime.ToString("yyyy-MM-ddTHH:mm:ssZ")

docker run --rm `
    -e "ARDENTS_VERSION=$Version" `
    -e "ARDENTS_COMMIT=$Commit" `
    -e "ARDENTS_BUILD_DATE=$buildDate" `
    -e "SOURCE_DATE_EPOCH=$SourceDateEpoch" `
    -e "ARDENTS_SOURCE_DIRTY=$($dirty.ToString().ToLowerInvariant())" `
    --mount "type=bind,source=$root,target=/workspace,readonly" `
    --mount "type=bind,source=$outputPath,target=/out" `
    --mount source=ardents-go-build-cache,target=/root/.cache/go-build `
    --mount source=ardents-go-mod-cache,target=/go/pkg/mod `
    -w /workspace golang:1.26-bookworm /bin/sh scripts/release/bundle.sh
if ($LASTEXITCODE -ne 0) { throw "Docker release build failed" }

$imageTag = "ardents/ardd:$Version"
$previousGitInfo = $env:BUILDX_GIT_INFO
try {
    $env:BUILDX_GIT_INFO = "false"
    docker build `
        --provenance=false `
        --build-arg "ARDENTS_VERSION=$Version" `
        --build-arg "ARDENTS_COMMIT=$Commit" `
        --build-arg "ARDENTS_BUILD_DATE=$buildDate" `
        --build-arg "GO_BUILD_PARALLELISM=2" `
        -t $imageTag -f deploy/docker/images/ardd.Dockerfile .
    if ($LASTEXITCODE -ne 0) { throw "Docker release image build failed" }
} finally {
    if ($null -eq $previousGitInfo) { Remove-Item Env:BUILDX_GIT_INFO -ErrorAction SilentlyContinue }
    else { $env:BUILDX_GIT_INFO = $previousGitInfo }
}
docker save -o (Join-Path $outputPath "ardents-$Version-linux-amd64.docker.tar") $imageTag
if ($LASTEXITCODE -ne 0) { throw "Docker release image export failed" }

docker run --rm `
    -e "ARDENTS_VERSION=$Version" `
    -e "ARDENTS_COMMIT=$Commit" `
    -e "ARDENTS_BUILD_DATE=$buildDate" `
    -e "SOURCE_DATE_EPOCH=$SourceDateEpoch" `
    -e "ARDENTS_SOURCE_DIRTY=$($dirty.ToString().ToLowerInvariant())" `
    --mount "type=bind,source=$root,target=/workspace,readonly" `
    --mount "type=bind,source=$outputPath,target=/out" `
    -w /workspace mcr.microsoft.com/powershell:7.5-debian-12 `
    pwsh -NoLogo -NoProfile -File scripts/release/metadata.ps1 -ArtifactDir /out
if ($LASTEXITCODE -ne 0) { throw "release metadata generation failed" }

Write-Host "Release artifacts created: $outputPath"
