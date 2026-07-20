param([string]$ReportDir = "tests/.artifacts/security")

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location $root
$reportPath = [IO.Path]::GetFullPath($ReportDir)
$artifactRoot = [IO.Path]::GetFullPath((Join-Path $root "tests/.artifacts"))
if (-not $reportPath.StartsWith($artifactRoot + [IO.Path]::DirectorySeparatorChar)) {
    throw "security evidence path must stay under tests/.artifacts: $reportPath"
}
New-Item -ItemType Directory -Force -Path $reportPath | Out-Null

$scanner = "golang.org/x/vuln/cmd/govulncheck@v1.6.0"
$jsonPath = Join-Path $reportPath "govulncheck.jsonl"
$verbosePath = Join-Path $reportPath "govulncheck.verbose.txt"
$mounts = @(
    "run", "--rm", "--init",
    "--mount", "type=bind,source=$root,target=/workspace,readonly",
    "--mount", "type=bind,source=$reportPath,target=/out",
    "--mount", "type=volume,source=ardents-go-mod-cache,target=/go/pkg/mod",
    "--mount", "type=volume,source=ardents-go-build-cache,target=/root/.cache/go-build",
    "-w", "/workspace", "golang:1.26-bookworm", "/bin/sh", "-c"
)

& docker @mounts "go run $scanner -format json ./... > /out/govulncheck.jsonl"
if ($LASTEXITCODE -ne 0) { throw "govulncheck JSON scan failed" }
& docker @mounts "go run $scanner -show verbose ./... > /out/govulncheck.verbose.txt 2>&1"
$verboseExit = $LASTEXITCODE

$findings = @{}
$verbose = Get-Content -Raw -LiteralPath $verbosePath
$sectionPattern = '(?ms)^=== (Symbol|Package|Module) Results ===\s*(.*?)(?=^=== |\z)'
foreach ($section in [regex]::Matches($verbose, $sectionPattern)) {
    $level = $section.Groups[1].Value.ToLowerInvariant()
    foreach ($finding in [regex]::Matches($section.Groups[2].Value, '(?m)^Vulnerability #\d+: (GO-\d{4}-\d+)\s*$')) {
        $id = $finding.Groups[1].Value
        if ($findings.ContainsKey($id)) { throw "duplicate security classification for $id" }
        $findings[$id] = $level
    }
}

$expected = [ordered]@{ "GO-2026-4479" = "symbol"; "GO-2026-5932" = "module" }
$exceptions = Get-Content -Raw -LiteralPath "docs/security-exceptions.md"
foreach ($id in $expected.Keys) {
    if (-not $exceptions.Contains($id)) { throw "accepted finding $id is absent from docs/security-exceptions.md" }
}
$actualIds = @($findings.Keys | Sort-Object)
$expectedIds = @($expected.Keys | Sort-Object)
if (@(Compare-Object $expectedIds $actualIds).Count -ne 0) {
    throw "security finding set changed: expected [$($expectedIds -join ', ')], actual [$($actualIds -join ', ')]"
}
foreach ($id in $expectedIds) {
    if ($findings[$id] -ne $expected[$id]) {
        throw "security reachability changed for ${id}: expected $($expected[$id]), actual $($findings[$id])"
    }
}
if ($actualIds.Count -gt 0 -and $verboseExit -eq 0) {
    throw "verbose scanner unexpectedly returned success while accepted findings remain"
}

[ordered]@{
    scanner = $scanner
    status = "accepted_findings_reconciled"
    verbose_exit_code = $verboseExit
    findings = @($expectedIds | ForEach-Object { [ordered]@{ id = $_; precision = $findings[$_] } })
} | ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 (Join-Path $reportPath "reconciliation.json")
Write-Host "security-gate=passed findings=$($actualIds.Count)"
