param(
    [Parameter(Mandatory = $true)]
    [ValidateSet("Prepare", "Promote")]
    [string]$Mode,
    [string]$LocalImage,
    [Parameter(Mandatory = $true)]
    [string]$RemoteImage,
    [Parameter(Mandatory = $true)]
    [string]$Version,
    [string]$CandidateIdentity,
    [string]$ExpectedDigest
)

$ErrorActionPreference = "Stop"
$remoteName = $RemoteImage.ToLowerInvariant()
$versionTag = "$remoteName`:$Version"

if ($Mode -eq "Prepare") {
    if ([string]::IsNullOrWhiteSpace($LocalImage) -or [string]::IsNullOrWhiteSpace($CandidateIdentity)) {
        throw "prepare requires LocalImage and CandidateIdentity"
    }
    $candidateTag = "$remoteName`:candidate-$Version-$CandidateIdentity"
    & docker image inspect $LocalImage *> $null
    if ($LASTEXITCODE -ne 0) { throw "local release image is unavailable: $LocalImage" }

    & docker tag $LocalImage $candidateTag
    if ($LASTEXITCODE -ne 0) { throw "tag release candidate image failed: $LocalImage" }
    & docker push $candidateTag | Out-Host
    if ($LASTEXITCODE -ne 0) { throw "push release candidate image failed: $candidateTag" }

    $candidateDigest = (& docker buildx imagetools inspect $candidateTag --format '{{.Manifest.Digest}}').Trim()
    if ($LASTEXITCODE -ne 0 -or $candidateDigest -notmatch '^sha256:[0-9a-f]{64}$') {
        throw "cannot resolve release candidate image digest: $candidateTag"
    }

    $publishedDigest = (& docker buildx imagetools inspect $versionTag --format '{{.Manifest.Digest}}' 2>$null)
    if ($LASTEXITCODE -eq 0) {
        $publishedDigest = $publishedDigest.Trim()
        if ($publishedDigest -ne $candidateDigest) {
            throw "refusing to overwrite $versionTag ($publishedDigest) with $candidateDigest"
        }
        $needsPromotion = $false
    } else {
        $publishedDigest = $null
        $needsPromotion = $true
    }
    [ordered]@{
        name = $remoteName
        digest = $candidateDigest
        candidate_tag = $candidateTag
        version_tag = $versionTag
        needs_promotion = $needsPromotion
    } | ConvertTo-Json -Compress
    exit 0
}

$candidateDigest = $ExpectedDigest
if ($candidateDigest -notmatch '^sha256:[0-9a-f]{64}$') { throw "promote requires a valid ExpectedDigest" }
$publishedDigest = (& docker buildx imagetools inspect $versionTag --format '{{.Manifest.Digest}}' 2>$null)
if ($LASTEXITCODE -eq 0) {
    if ($publishedDigest.Trim() -ne $candidateDigest) {
        throw "version tag changed after preflight: $versionTag"
    }
} else {
    & docker buildx imagetools create --prefer-index=false --tag $versionTag "$remoteName@$candidateDigest" | Out-Host
    if ($LASTEXITCODE -ne 0) { throw "publish versioned image failed: $versionTag" }
}
$verifiedDigest = (& docker buildx imagetools inspect $versionTag --format '{{.Manifest.Digest}}').Trim()
if ($LASTEXITCODE -ne 0 -or $verifiedDigest -ne $candidateDigest) {
    throw "published image digest verification failed: $versionTag"
}

[ordered]@{ name = $remoteName; digest = $candidateDigest; version_tag = $versionTag } | ConvertTo-Json -Compress
