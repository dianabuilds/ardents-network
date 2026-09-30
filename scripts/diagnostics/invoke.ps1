[CmdletBinding(PositionalBinding=$false)]
param(
    [Parameter(Mandatory=$true)][string]$EvidenceRoot,
    [ValidateSet('runner','online','debugger','network','dashboard','monitor')][string]$Service = 'runner',
    [string]$RunName = 'latest',
    [switch]$Build,
    [Parameter(Position=0,ValueFromRemainingArguments=$true)][string[]]$Command
)
$ErrorActionPreference = 'Stop'
$sourceRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '../..')).Path
if (-not [IO.Path]::IsPathRooted($EvidenceRoot)) { throw 'EvidenceRoot must be absolute' }
if (-not (Test-Path -LiteralPath $EvidenceRoot -PathType Container)) { throw 'Create a private evidence directory outside Git first' }
$evidencePath = (Resolve-Path -LiteralPath $EvidenceRoot).Path
$sourcePrefix = $sourceRoot.TrimEnd('\','/') + [IO.Path]::DirectorySeparatorChar
if ($evidencePath -eq $sourceRoot -or $evidencePath.StartsWith($sourcePrefix,[StringComparison]::OrdinalIgnoreCase)) { throw 'Evidence must be outside source' }
if ($RunName -notmatch '^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$') { throw 'Invalid RunName' }
$env:ARDENTS_DIAGNOSTIC_SOURCE = $sourceRoot
$env:ARDENTS_DIAGNOSTIC_EVIDENCE = $evidencePath
$env:ARDENTS_DIAGNOSTIC_RUN = $RunName
$revision = & git -C $sourceRoot rev-parse HEAD
if ($LASTEXITCODE -ne 0) { throw 'Cannot record source revision' }
$env:ARDENTS_DIAGNOSTIC_SOURCE_SHA = $revision.Trim()
if ($Build) {
    & docker build -f (Join-Path $PSScriptRoot 'Dockerfile') -t ardents-diagnostics:local $sourceRoot
    if ($LASTEXITCODE -ne 0) { throw 'Diagnostic image build failed' }
}
$imageIdentity = & docker image inspect ardents-diagnostics:local --format '{{.Id}}'
if ($LASTEXITCODE -ne 0) { throw 'Build diagnostic image explicitly with -Build first' }
$env:ARDENTS_DIAGNOSTIC_IMAGE = $imageIdentity.Trim()
$composeFile = Join-Path $PSScriptRoot 'compose.yaml'
if ($Service -eq 'dashboard') {
    & docker compose -f $composeFile --profile view up -d dashboard
} elseif ($Service -eq 'monitor') {
    if (-not $Command -or $Command.Count -eq 0) { throw 'Supply monitor flags and an explicit -- source command' }
    & docker compose -f $composeFile --profile monitor run --rm --service-ports monitor @Command
} else {
    if (-not $Command -or $Command.Count -eq 0) { $Command = @('doctor') }
    & docker compose -f $composeFile --profile online --profile debug --profile network run --rm $Service @Command
}
if ($LASTEXITCODE -ne 0) { throw "Diagnostic operation failed: exit $LASTEXITCODE" }
