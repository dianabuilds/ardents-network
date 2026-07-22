$ErrorActionPreference = "Stop"

$forbiddenCaches = @(
    ".cache\go-build",
    ".gocache"
)
$forbiddenCaches += @(Get-ChildItem -Path . -Directory -Filter ".tmp-go-cache*" | ForEach-Object { $_.FullName })
$presentCaches = @($forbiddenCaches | Where-Object { Test-Path -LiteralPath $_ })
if ($presentCaches.Count -gt 0) {
    throw "Go cache must live outside the repository: $($presentCaches -join ', ')"
}

$unformatted = @(& gofmt -l api cmd internal scripts sdk tests)
if ($LASTEXITCODE -ne 0) {
    throw "gofmt inspection failed"
}
if ($unformatted.Count -gt 0) {
    $unformatted | ForEach-Object { Write-Error "unformatted Go file: $_" }
    throw "Go formatting gate failed"
}

Write-Host "Go formatting gate passed"
