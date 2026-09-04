param(
    [Parameter(Mandatory = $true)]
    [string]$OutputPath
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$wrapper = $PSScriptRoot
$repository = (Resolve-Path (Join-Path $wrapper '..\..\..\..')).Path
$output = [IO.Path]::GetFullPath($OutputPath)
$repositoryPrefix = $repository.TrimEnd('\') + '\'
if ($output.StartsWith($repositoryPrefix, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'wrapper output must stay outside the repository'
}
$parent = Split-Path -Parent $output
if (-not (Test-Path -LiteralPath $parent -PathType Container)) {
    throw "wrapper output parent does not exist: $parent"
}
if (Test-Path -LiteralPath $output) {
    throw "wrapper output already exists: $output"
}

$sourceNames = Get-Content -LiteralPath (Join-Path $wrapper 'sources.txt') |
    Where-Object { $_ -and -not $_.StartsWith('#') }
$sources = $sourceNames | ForEach-Object { Join-Path $wrapper $_ }

& go build -o $output @sources
exit $LASTEXITCODE
