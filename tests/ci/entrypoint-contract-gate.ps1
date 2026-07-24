param(
    [string]$Root = (Split-Path -Parent (Split-Path -Parent $PSScriptRoot))
)

$ErrorActionPreference = "Stop"
$root = [IO.Path]::GetFullPath($Root)
if (-not (Test-Path -LiteralPath $root -PathType Container)) {
    throw "entrypoint contract root is unavailable: $root"
}

function Get-RequiredFile([string]$RelativePath, [string]$Failure) {
    $path = Join-Path $root $RelativePath
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        throw $Failure
    }
    return $path
}

function Assert-OnlyCanonicalTargets(
    [object[]]$Targets,
    [string[]]$Allowed,
    [string]$MissingFailure,
    [string]$UnsupportedFailure
) {
    if ($Targets.Count -eq 0) { throw $MissingFailure }
    foreach ($target in $Targets) {
        $normalized = ([string]$target).Trim('"', "'").Replace("\", "/")
        if ($normalized -cnotin $Allowed) {
            throw "$UnsupportedFailure`: $normalized"
        }
    }
}

$backupRelative = "scripts/deploy/data.ps1"
$catalogRelative = "tests/tooling/testcatalog/main.go"
$backupPath = Get-RequiredFile $backupRelative "canonical backup entrypoint is missing: $backupRelative"
$catalogPath = Get-RequiredFile $catalogRelative "canonical testcatalog entrypoint is missing: $catalogRelative"
$catalogImplementation = Get-RequiredFile "tests/tooling/testcatalog/catalog.go" "canonical testcatalog implementation is missing"
$distributionPath = Get-RequiredFile "scripts/release/distribution-files.txt" "distribution file mapping is missing"
$runnerPath = Get-RequiredFile "tests/run.ps1" "canonical test runner is missing"
$workflowPath = Get-RequiredFile ".github/workflows/ci.yml" "CI workflow is missing"
$orchestratorPath = Get-RequiredFile "ardents.ps1" "deployment orchestrator is missing"

$distributionMappings = [Collections.Generic.Dictionary[string, string]]::new([StringComparer]::Ordinal)
foreach ($line in Get-Content -LiteralPath $distributionPath) {
    if ([string]::IsNullOrWhiteSpace($line)) { continue }
    $parts = $line -split "\|", 2
    if ($parts.Count -ne 2 -or
        [IO.Path]::IsPathRooted($parts[0]) -or [IO.Path]::IsPathRooted($parts[1]) -or
        $parts[0] -match "(^|/)\.\.(/|$)" -or $parts[1] -match "(^|/)\.\.(/|$)") {
        throw "invalid distribution file mapping: $line"
    }
    if (-not (Test-Path -LiteralPath (Join-Path $root $parts[0]) -PathType Leaf)) {
        throw "distribution mapping references a missing source: $($parts[0])"
    }
    if (($parts[0].Contains("scripts/deploy/") -or $parts[1].Contains("scripts/deploy/")) -and
        ($parts[0].Contains("data.ps1") -or $parts[1].Contains("data.ps1")) -and
        ($parts[0] -cne $backupRelative -or $parts[1] -cne $backupRelative)) {
        throw "distribution bundle references an unsupported backup entrypoint: $line"
    }
    $distributionMappings[$parts[0]] = $parts[1]
}
if (-not $distributionMappings.ContainsKey($backupRelative) -or
    $distributionMappings[$backupRelative] -cne $backupRelative) {
    throw "distribution bundle does not preserve the canonical backup entrypoint"
}

$orchestrator = Get-Content -Raw -LiteralPath $orchestratorPath
$backupTargets = @(
    [regex]::Matches($orchestrator, '["''](?<target>[^"'']*scripts[\\/]deploy[\\/][^"'']*data\.ps1[^"'']*)["'']') |
        ForEach-Object { $_.Groups["target"].Value }
)
Assert-OnlyCanonicalTargets $backupTargets @(
    '$root/scripts/deploy/data.ps1',
    "./scripts/deploy/data.ps1",
    "scripts/deploy/data.ps1"
) "deployment orchestrator does not use the canonical backup entrypoint" "deployment orchestrator uses an unsupported backup entrypoint"

$runner = Get-Content -Raw -LiteralPath $runnerPath
$catalogCommand = "./tests/tooling/testcatalog"
$runnerCatalogTargets = @(
    [regex]::Matches($runner, '["''](?<target>\.?[\\/]*tests[\\/][^"'']*testcatalog[^"'']*)["'']') |
        ForEach-Object { $_.Groups["target"].Value }
)
Assert-OnlyCanonicalTargets $runnerCatalogTargets @($catalogCommand) `
    "test runner does not use the canonical testcatalog contract" `
    "test runner uses an unsupported testcatalog entrypoint"

$workflow = Get-Content -Raw -LiteralPath $workflowPath
$workflowCatalogTargets = @(
    [regex]::Matches($workflow, '(?im)^\s*go\s+run\s+(?<target>"[^"]+"|''[^'']+''|\S+)') |
        ForEach-Object { $_.Groups["target"].Value } |
        Where-Object { $_ -match "testcatalog" }
)
Assert-OnlyCanonicalTargets $workflowCatalogTargets @($catalogCommand) `
    "CI does not use the canonical testcatalog contract" `
    "CI uses an unsupported testcatalog entrypoint"
if ($workflow -notmatch '(?m)^\s*\./tests/ci/entrypoint-contract-gate-test\.ps1(?:\s|$)') {
    throw "CI does not enforce the canonical entrypoint contract"
}

$legacyBackup = "cluster" + "-data.ps1"
$legacyCatalogPattern = "tests[\\/]cmd[\\/]test" + "catalog"
$legacyModePattern = "(?i)(?<![A-Za-z0-9_])--?" + "mode(?:\s+|=)validate(?![A-Za-z0-9_])"
$legacyPatterns = @(
    [pscustomobject]@{ Name = $legacyBackup; Pattern = [regex]::Escape($legacyBackup) },
    [pscustomobject]@{ Name = "obsolete catalog path"; Pattern = $legacyCatalogPattern },
    [pscustomobject]@{ Name = "obsolete catalog mode"; Pattern = $legacyModePattern }
)
$textExtensions = [Collections.Generic.HashSet[string]]::new(
    [string[]]@(".go", ".json", ".md", ".ps1", ".sh", ".txt", ".yaml", ".yml"),
    [StringComparer]::OrdinalIgnoreCase
)
$auditRoot = [IO.Path]::GetFullPath((Join-Path $root "docs/audit"))
$artifactRoot = [IO.Path]::GetFullPath((Join-Path $root "tests/.artifacts"))
foreach ($file in Get-ChildItem -LiteralPath $root -Recurse -File) {
    if (-not $textExtensions.Contains($file.Extension)) { continue }
    $fullPath = [IO.Path]::GetFullPath($file.FullName)
    if ($fullPath.StartsWith($auditRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase) -or
        $fullPath.StartsWith($artifactRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase) -or
        $fullPath.Contains("$([IO.Path]::DirectorySeparatorChar).git$([IO.Path]::DirectorySeparatorChar)")) {
        continue
    }
    $content = Get-Content -Raw -LiteralPath $fullPath
    foreach ($legacy in $legacyPatterns) {
        if ($content -match $legacy.Pattern) {
            $rootPrefix = $root.TrimEnd([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar)
            $relative = $fullPath.Substring($rootPrefix.Length).TrimStart(
                [IO.Path]::DirectorySeparatorChar,
                [IO.Path]::AltDirectorySeparatorChar
            ).Replace("\", "/")
            throw "legacy entrypoint reference '$($legacy.Name)' remains in $relative"
        }
    }
}

if ((Get-Item -LiteralPath $backupPath).Length -eq 0 -or
    (Get-Item -LiteralPath $catalogPath).Length -eq 0 -or
    (Get-Item -LiteralPath $catalogImplementation).Length -eq 0) {
    throw "canonical entrypoint implementation is empty"
}

Write-Host "entrypoint-contract-gate=passed"
