$ErrorActionPreference = "Stop"

$unformatted = @(& gofmt -l boundary cmd internal tests)
if ($LASTEXITCODE -ne 0) {
    throw "gofmt inspection failed"
}
if ($unformatted.Count -gt 0) {
    $unformatted | ForEach-Object { Write-Error "unformatted Go file: $_" }
    throw "Go formatting gate failed"
}

Write-Host "Go formatting gate passed"
