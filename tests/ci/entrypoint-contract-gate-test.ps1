# testkit:scenario CIE-001
# testkit:layer integration
# testkit:domain ci

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$gate = Join-Path $PSScriptRoot "entrypoint-contract-gate.ps1"
$fixtureRoot = Join-Path ([IO.Path]::GetTempPath()) ("ardents-entrypoint-contract-{0}" -f [guid]::NewGuid().ToString("N"))

function Invoke-Gate([string]$Fixture, [string]$ExpectedFailure = "") {
    $failure = ""
    try {
        & $gate -Root $Fixture
    } catch {
        $failure = $_.Exception.Message
    }
    if ([string]::IsNullOrWhiteSpace($ExpectedFailure)) {
        if (-not [string]::IsNullOrWhiteSpace($failure)) {
            throw "valid entrypoint fixture was rejected: $failure"
        }
        return
    }
    if ($failure -notlike "*$ExpectedFailure*") {
        throw "entrypoint fixture returned the wrong failure; want '$ExpectedFailure', got '$failure'"
    }
}

function Write-ValidFixture {
    $paths = @(
        ".github/workflows",
        "scripts/deploy",
        "scripts/release",
        "tests/ci",
        "tests/tooling/testcatalog"
    )
    foreach ($path in $paths) {
        [IO.Directory]::CreateDirectory((Join-Path $fixtureRoot $path)) | Out-Null
    }
    [IO.File]::WriteAllText((Join-Path $fixtureRoot "ardents.ps1"), '& "./scripts/deploy/data.ps1" -Action backup')
    [IO.File]::WriteAllText((Join-Path $fixtureRoot "scripts/deploy/data.ps1"), 'param([string]$Action)')
    [IO.File]::WriteAllText(
        (Join-Path $fixtureRoot "scripts/release/distribution-files.txt"),
        "scripts/deploy/data.ps1|scripts/deploy/data.ps1`n"
    )
    [IO.File]::WriteAllText((Join-Path $fixtureRoot "tests/tooling/testcatalog/main.go"), "package main`nfunc main() {}`n")
    [IO.File]::WriteAllText((Join-Path $fixtureRoot "tests/tooling/testcatalog/catalog.go"), "package main`n")
    [IO.File]::WriteAllText((Join-Path $fixtureRoot "tests/run.ps1"), '$args = @("run", "./tests/tooling/testcatalog")')
    [IO.File]::WriteAllText(
        (Join-Path $fixtureRoot ".github/workflows/ci.yml"),
        "run: |`n  go run ./tests/tooling/testcatalog ./tests/...`n  ./tests/ci/entrypoint-contract-gate-test.ps1`n"
    )
}

try {
    Write-ValidFixture
    Invoke-Gate $fixtureRoot

    Remove-Item -LiteralPath (Join-Path $fixtureRoot "scripts/deploy/data.ps1")
    Invoke-Gate $fixtureRoot "canonical backup entrypoint is missing"
    Write-ValidFixture

    Remove-Item -LiteralPath (Join-Path $fixtureRoot "tests/tooling/testcatalog/main.go")
    Invoke-Gate $fixtureRoot "canonical testcatalog entrypoint is missing"
    Write-ValidFixture

    [IO.File]::WriteAllText((Join-Path $fixtureRoot "ardents.ps1"), '& "./scripts/deploy/data.ps1-missing" -Action backup')
    Invoke-Gate $fixtureRoot "deployment orchestrator uses an unsupported backup entrypoint"
    Write-ValidFixture

    [IO.File]::AppendAllText((Join-Path $fixtureRoot "ardents.ps1"), "`n& `"./scripts/deploy/data.ps1-missing`" -Action backup")
    Invoke-Gate $fixtureRoot "deployment orchestrator uses an unsupported backup entrypoint"
    Write-ValidFixture

    [IO.File]::WriteAllText((Join-Path $fixtureRoot "tests/run.ps1"), '$args = @("run", "./tests/tooling/testcatalog-missing")')
    Invoke-Gate $fixtureRoot "test runner uses an unsupported testcatalog entrypoint"
    Write-ValidFixture

    [IO.File]::AppendAllText((Join-Path $fixtureRoot "tests/run.ps1"), "`n`$extra = `"./tests/tooling/testcatalog-missing`"")
    Invoke-Gate $fixtureRoot "test runner uses an unsupported testcatalog entrypoint"
    Write-ValidFixture

    [IO.File]::WriteAllText(
        (Join-Path $fixtureRoot ".github/workflows/ci.yml"),
        "run: |`n  go run ./tests/tooling/testcatalog-missing ./tests/...`n  ./tests/ci/entrypoint-contract-gate-test.ps1`n"
    )
    Invoke-Gate $fixtureRoot "CI uses an unsupported testcatalog entrypoint"
    Write-ValidFixture

    [IO.File]::AppendAllText(
        (Join-Path $fixtureRoot ".github/workflows/ci.yml"),
        "  go run ./tests/tooling/testcatalog-missing ./tests/...`n"
    )
    Invoke-Gate $fixtureRoot "CI uses an unsupported testcatalog entrypoint"
    Write-ValidFixture

    $legacyBackup = "cluster" + "-data.ps1"
    [IO.File]::AppendAllText((Join-Path $fixtureRoot "ardents.ps1"), "`n& ./scripts/deploy/$legacyBackup")
    Invoke-Gate $fixtureRoot "legacy entrypoint reference"
    Write-ValidFixture

    $legacyCatalog = "tests/cmd/" + "testcatalog"
    [IO.File]::AppendAllText((Join-Path $fixtureRoot "tests/run.ps1"), "`n& go run ./$legacyCatalog")
    Invoke-Gate $fixtureRoot "legacy entrypoint reference"
    Write-ValidFixture

    foreach ($legacyMode in @(
        ("-" + "mode validate"),
        ("-" + "mode=validate"),
        ("--" + "mode validate"),
        ("--" + "mode=validate"),
        ('"' + "--" + "mode=validate" + '"'),
        ("'" + "-" + "mode validate" + "'")
    )) {
        [IO.File]::AppendAllText((Join-Path $fixtureRoot "tests/run.ps1"), "`n& go run ./tests/tooling/testcatalog $legacyMode")
        Invoke-Gate $fixtureRoot "legacy entrypoint reference"
        Write-ValidFixture
    }

    Invoke-Gate $root
    Write-Host "entrypoint-contract-gate-test=passed"
} finally {
    if (Test-Path -LiteralPath $fixtureRoot) {
        Remove-Item -LiteralPath $fixtureRoot -Recurse -Force
    }
}
