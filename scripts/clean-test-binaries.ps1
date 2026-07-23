param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^[a-z0-9][a-z0-9-]*$')]
    [string]$Suite
)

$ErrorActionPreference = "Stop"

$repository = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$testBinaryRoot = [IO.Path]::GetFullPath((Join-Path $repository "tests/.artifacts/testbin")).TrimEnd(
    [IO.Path]::DirectorySeparatorChar,
    [IO.Path]::AltDirectorySeparatorChar
)
$target = [IO.Path]::GetFullPath((Join-Path $testBinaryRoot $Suite)).TrimEnd(
    [IO.Path]::DirectorySeparatorChar,
    [IO.Path]::AltDirectorySeparatorChar
)
$prefix = $testBinaryRoot + [IO.Path]::DirectorySeparatorChar

if (-not $target.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase)) {
    throw "refusing to clean test binaries outside the repository testbin directory"
}
if (Test-Path -LiteralPath $target) {
    Remove-Item -LiteralPath $target -Recurse -Force
    Write-Host "Removed temporary $Suite test binaries: $target"
}
