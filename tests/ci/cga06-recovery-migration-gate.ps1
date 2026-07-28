param(
    [Parameter(Mandatory = $true)]
    [string]$OutputDir,
    [string]$Commit = ""
)

$ErrorActionPreference = "Stop"
$resolvedOutput = [System.IO.Path]::GetFullPath($OutputDir)
if (Test-Path -LiteralPath $resolvedOutput) {
    if ((Get-ChildItem -LiteralPath $resolvedOutput -Force | Measure-Object).Count -ne 0) {
        throw "CGA-06 evidence output directory must be empty"
    }
} else {
    New-Item -ItemType Directory -Path $resolvedOutput | Out-Null
}
if ([string]::IsNullOrWhiteSpace($Commit)) {
    $Commit = (& git rev-parse HEAD).Trim()
    if ($LASTEXITCODE -ne 0) {
        throw "resolve evidence commit failed"
    }
}
if ($Commit -notmatch '^[0-9a-f]{40}$') {
    throw "CGA-06 evidence commit must be a full Git object ID"
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
        Run = 'TestMigrateLocalV2RejectsUnknownDowngrade|TestBuildLocalV2MigrationEvidenceRejects'
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
