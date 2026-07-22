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

& go clean -cache -testcache
if ($LASTEXITCODE -ne 0) {
    throw "Go cache cleanup failed"
}

Write-Host "Cleaned external Go build and test caches: $cachePath"
