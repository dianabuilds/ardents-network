param(
    [double]$MaxGiB = 5,
    [switch]$Force,
    [switch]$StatusOnly
)

$ErrorActionPreference = "Stop"

$repository = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path.TrimEnd('\', '/')
$cache = (& go env GOCACHE).Trim()
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($cache)) {
    throw "unable to resolve external Go build cache"
}

$cachePath = [IO.Path]::GetFullPath($cache).TrimEnd('\', '/')
if ($cachePath.Equals($repository, [StringComparison]::OrdinalIgnoreCase) -or
    $cachePath.StartsWith($repository + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
    throw "refusing to clean a Go cache inside the repository: $cachePath"
}

$bytes = 0L
$files = 0L
if (Test-Path -LiteralPath $cachePath -PathType Container) {
    Get-ChildItem -LiteralPath $cachePath -File -Recurse -Force | ForEach-Object {
        $bytes += $_.Length
        $files++
    }
}
$gib = $bytes / 1GB
Write-Host ("External Go cache: {0:N2} GiB, {1:N0} files ({2})" -f $gib, $files, $cachePath)

if ($StatusOnly) {
    exit 0
}
if (-not $Force -and $gib -le $MaxGiB) {
    Write-Host ("Cache retained: size does not exceed the {0:N2} GiB cleanup threshold. Use -Force only after a release gate, for low disk space, or stale-cache diagnosis." -f $MaxGiB)
    exit 0
}

& go clean -cache -testcache
if ($LASTEXITCODE -ne 0) {
    throw "Go cache cleanup failed"
}

Write-Host "Cleaned external Go build and test caches: $cachePath"
