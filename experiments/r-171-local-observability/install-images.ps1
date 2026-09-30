param([Parameter(Mandatory = $true)][string]$EvidenceRoot)
$ErrorActionPreference = 'Stop'
$taskRoot = [IO.Path]::GetFullPath($EvidenceRoot)
$taskRepo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../..'))
if ($taskRoot.Equals($taskRepo, [StringComparison]::OrdinalIgnoreCase) -or
    $taskRoot.StartsWith($taskRepo + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'EvidenceRoot must be outside the repository.'
}
$taskAncestor = [IO.DirectoryInfo]::new([IO.Path]::GetDirectoryName($taskRoot))
while ($null -ne $taskAncestor) {
    if ($taskAncestor.Exists -and ($taskAncestor.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
        throw 'EvidenceRoot ancestors must not redirect through reparse points.'
    }
    $taskAncestor = $taskAncestor.Parent
}
if (Test-Path -LiteralPath $taskRoot) { throw 'EvidenceRoot must be a new directory; preserve prior attempts.' }
New-Item -ItemType Directory -Path $taskRoot | Out-Null
$taskLockPath = Join-Path $PSScriptRoot 'images.json'
$taskLock = Get-Content -LiteralPath $taskLockPath -Raw | ConvertFrom-Json
$taskHasher = [Security.Cryptography.SHA256]::Create()
try { $taskLockHash = [BitConverter]::ToString($taskHasher.ComputeHash([IO.File]::ReadAllBytes($taskLockPath))).Replace('-','') } finally { $taskHasher.Dispose() }
$taskReceipt = [ordered]@{
    started_utc = (Get-Date).ToUniversalTime().ToString('o')
    lock_sha256 = $taskLockHash
    platform = $taskLock.platform
    scope = 'public image download and metadata inspection only; no containers started'
    complete = $false
    images = @()
}
try {
    foreach ($taskImage in $taskLock.images) {
        if ($taskImage.name -notmatch '^[a-z]+$' -or $taskImage.image -notmatch '^[a-z]+/[a-z]+@sha256:[0-9a-f]{64}$') {
            throw 'Invalid pinned image input.'
        }
        $taskPull = & docker pull --platform $taskLock.platform $taskImage.image 2>&1
        $taskPullExit = $LASTEXITCODE
        $taskPull | Out-File -LiteralPath (Join-Path $taskRoot ($taskImage.name + '-pull.log')) -Encoding utf8
        if ($taskPullExit -ne 0) { throw "Image download failed: $($taskImage.name) exit $taskPullExit" }
        $taskInspect = & docker image inspect $taskImage.image
        if ($LASTEXITCODE -ne 0) { throw "Image inspection failed: $($taskImage.name)" }
        $taskInspect | Out-File -LiteralPath (Join-Path $taskRoot ($taskImage.name + '-image.json')) -Encoding utf8
        $taskMetadata = @($taskInspect | ConvertFrom-Json)[0]
        if ($taskMetadata.Os -ne 'linux' -or $taskMetadata.Architecture -ne 'amd64' -or
            $taskMetadata.RepoDigests -notcontains $taskImage.image) {
            throw "Downloaded image identity/platform mismatch: $($taskImage.name)"
        }
        $taskReceipt.images += [ordered]@{
            name = $taskImage.name
            source = $taskImage.image
            config_digest = $taskMetadata.Id
            size_bytes = $taskMetadata.Size
            default_user = $taskMetadata.Config.User
            entrypoint = $taskMetadata.Config.Entrypoint
        }
    }
    $taskReceipt.complete = $true
} finally {
    $taskReceipt.ended_utc = (Get-Date).ToUniversalTime().ToString('o')
    $taskReceipt | ConvertTo-Json -Depth 8 | Out-File -LiteralPath (Join-Path $taskRoot 'receipt.json') -Encoding utf8
}