param(
    [Parameter(Mandatory = $true)]
    [string]$OutputDir,
    [string]$Commit = ""
)

$ErrorActionPreference = "Stop"
$resolvedOutput = [System.IO.Path]::GetFullPath($OutputDir)
$repositoryRoot = (& git rev-parse --show-toplevel).Trim()
$headCommit = (& git rev-parse HEAD).Trim()
if ($LASTEXITCODE -ne 0) {
    throw "resolve evidence repository failed"
}
$repositoryRoot = [System.IO.Path]::GetFullPath($repositoryRoot)
if ($resolvedOutput.StartsWith(
    $repositoryRoot + [System.IO.Path]::DirectorySeparatorChar,
    [System.StringComparison]::OrdinalIgnoreCase
)) {
    throw "CGA-06 evidence output must be outside the source worktree"
}
$dirty = @(& git status --porcelain=v1 --untracked-files=all)
if ($LASTEXITCODE -ne 0 -or $dirty.Count -ne 0) {
    throw "CGA-06 evidence requires a clean Git worktree"
}
if ([string]::IsNullOrWhiteSpace($Commit)) {
    $Commit = $headCommit
}
if ($Commit -notmatch '^[0-9a-f]{40}$' -or $Commit -ne $headCommit) {
    throw "CGA-06 evidence commit must equal the full checked-out HEAD"
}
& git cat-file -e ($Commit + "^{commit}")
if ($LASTEXITCODE -ne 0) {
    throw "CGA-06 evidence commit does not resolve to a commit object"
}
if (Test-Path -LiteralPath $resolvedOutput) {
    if ((Get-ChildItem -LiteralPath $resolvedOutput -Force | Measure-Object).Count -ne 0) {
        throw "CGA-06 evidence output directory must be empty"
    }
} else {
    New-Item -ItemType Directory -Path $resolvedOutput | Out-Null
}

$gates = @(
    @{
        Name = "restore"
        Package = "./internal/authority"
        Run = 'TestRecoveryOnly|TestVerifyRestoredAuthority'
    },
    @{
        Name = "migration"
        Package = "./internal/authority|./internal/provision"
        Packages = @("./internal/authority", "./internal/provision")
        Run = 'TestMigrateLocalV2|TestLocalV2MigrationRequiresFreshRotationCAS|TestBuildLocalV2MigrationEvidenceReads'
    },
    @{
        Name = "downgrade"
        Package = "./internal/authority|./internal/provision"
        Packages = @("./internal/authority", "./internal/provision")
        Run = 'TestMigrateLocalV2RejectsUnknownDowngrade|TestBuildLocalV2MigrationEvidenceRejects|TestLocalV2DowngradeRestoresCompleteStoppedBackup'
    }
)

$results = @()
foreach ($gate in $gates) {
    $packages = @($gate.Package)
    if ($gate.ContainsKey("Packages")) {
        $packages = $gate.Packages
    }
    $outputPath = Join-Path $resolvedOutput ($gate.Name + ".jsonl")
    $testOutput = & go test -count=1 -json @packages -run $gate.Run 2>&1
    $exitCode = $LASTEXITCODE
    $testOutput | Set-Content -LiteralPath $outputPath -Encoding utf8
    if ($exitCode -ne 0) {
        throw "CGA-06 $($gate.Name) drill failed; evidence retained at $outputPath"
    }
    $results += [ordered]@{
        name = $gate.Name
        command = "go test -count=1 -json $($packages -join ' ') -run '$($gate.Run)'"
        file = [System.IO.Path]::GetFileName($outputPath)
        sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $outputPath).Hash.ToLowerInvariant()
    }
}

$manifest = [ordered]@{
    schema = "ardents.cga06.recovery-migration-drill/v1"
    commit = $Commit
    generated_at = [DateTime]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ")
    qualification_changed = $false
    qualification = "realm.channel-grant-authority"
    gates = $results
}
$manifestPath = Join-Path $resolvedOutput "manifest.json"
$manifest | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $manifestPath -Encoding utf8
Write-Output $manifestPath
