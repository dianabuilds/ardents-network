$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$runID = [guid]::NewGuid().ToString("N")
$fixtureRoot = Join-Path ([IO.Path]::GetTempPath()) "ardents-release-source-$runID"
$fixtureScriptDir = Join-Path $fixtureRoot "scripts/release"
$fixtureProtocolDir = Join-Path $fixtureRoot "internal/ingressproxy"
$buildScript = Join-Path $fixtureScriptDir "build.ps1"
$global:ArdentsReleaseSourceGateDockerCalled = $false
$global:ArdentsReleaseSourceGateFixtureRoot = $fixtureRoot
$global:ArdentsReleaseSourceGateForbiddenFile = ""

function global:docker {
    $global:ArdentsReleaseSourceGateDockerCalled = $true
    foreach ($argument in $args) {
        if ($argument -notmatch '^type=bind,source=([^,]+),target=/workspace,readonly$') {
            continue
        }
        $sourcePath = [IO.Path]::GetFullPath($Matches[1])
        if ($sourcePath -eq [IO.Path]::GetFullPath($global:ArdentsReleaseSourceGateFixtureRoot)) {
            throw "release build mounted the mutable worktree"
        }
        if ($global:ArdentsReleaseSourceGateForbiddenFile -and
            (Test-Path -LiteralPath (Join-Path $sourcePath $global:ArdentsReleaseSourceGateForbiddenFile))) {
            throw "ignored source entered the release snapshot"
        }
    }
    throw "release source identity gate Docker sentinel"
}

function Invoke-Git([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments) {
    & git -C $fixtureRoot @Arguments | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "fixture git command failed: git $($Arguments -join ' ')"
    }
}

function Invoke-BuildAttempt([string]$Name, [string]$Commit) {
    $global:ArdentsReleaseSourceGateDockerCalled = $false
    $outputDir = Join-Path $fixtureRoot "out-$Name"
    $previousLocation = Get-Location
    $failure = ""
    try {
        & $buildScript -Version "v0.0.0-$Name" -Commit $Commit -OutputDir $outputDir
    } catch {
        $failure = $_.Exception.Message
    } finally {
        Set-Location $previousLocation
    }
    [pscustomobject]@{
        Failure = $failure
        DockerCalled = $global:ArdentsReleaseSourceGateDockerCalled
    }
}

function Invoke-ExpectedSourceFailure(
    [string]$Name,
    [string]$Commit,
    [string]$ExpectedMessage
) {
    $attempt = Invoke-BuildAttempt -Name $Name -Commit $Commit
    if ($attempt.Failure -notlike "*$ExpectedMessage*") {
        throw "$Name returned the wrong error: $($attempt.Failure)"
    }
    if ($attempt.DockerCalled) {
        throw "$Name reached Docker before source validation failed"
    }
}

try {
    [IO.Directory]::CreateDirectory($fixtureScriptDir) | Out-Null
    [IO.Directory]::CreateDirectory($fixtureProtocolDir) | Out-Null
    Copy-Item -LiteralPath (Join-Path $root "scripts/release/build.ps1") -Destination $buildScript
    [IO.File]::WriteAllText((Join-Path $fixtureProtocolDir "protocol_version.txt"), "1`n")
    [IO.File]::WriteAllText((Join-Path $fixtureRoot ".gitignore"), "ignored.go`n")
    Invoke-Git init
    Invoke-Git config user.name "Ardents Release Test"
    Invoke-Git config user.email "release-test@ardents.invalid"

    [IO.File]::WriteAllText((Join-Path $fixtureRoot "source.txt"), "source-a`n")
    Invoke-Git add .gitignore source.txt scripts/release/build.ps1 internal/ingressproxy/protocol_version.txt
    Invoke-Git commit -m "source A"
    $commitA = (& git -C $fixtureRoot rev-parse HEAD).Trim()

    [IO.File]::AppendAllText((Join-Path $fixtureRoot "source.txt"), "source-b`n")
    Invoke-Git add source.txt
    Invoke-Git commit -m "source B"
    $commitB = (& git -C $fixtureRoot rev-parse HEAD).Trim()

    Invoke-ExpectedSourceFailure -Name "mismatch" -Commit $commitA `
        -ExpectedMessage "release Commit must resolve to current HEAD"

    [IO.File]::AppendAllText((Join-Path $fixtureRoot "source.txt"), "dirty`n")
    Invoke-ExpectedSourceFailure -Name "tracked-dirty" -Commit $commitB `
        -ExpectedMessage "release source tree must be clean"

    Invoke-Git reset --hard HEAD
    [IO.File]::WriteAllText((Join-Path $fixtureRoot "untracked.txt"), "untracked`n")
    Invoke-ExpectedSourceFailure -Name "untracked-dirty" -Commit $commitB `
        -ExpectedMessage "release source tree must be clean"

    Invoke-Git clean -fd
    [IO.File]::WriteAllText((Join-Path $fixtureRoot "ignored.go"), "package ignored`n")
    $global:ArdentsReleaseSourceGateForbiddenFile = "ignored.go"
    $cleanAttempt = Invoke-BuildAttempt -Name "clean" -Commit $commitB
    if ($cleanAttempt.Failure -notlike "*Docker sentinel*") {
        throw "clean current HEAD failed before Docker: $($cleanAttempt.Failure)"
    }
    if (-not $cleanAttempt.DockerCalled) {
        throw "clean current HEAD did not reach Docker"
    }

    Write-Host "release-source-identity-gate=passed"
} finally {
    Remove-Item function:\docker -ErrorAction SilentlyContinue
    Remove-Variable ArdentsReleaseSourceGateDockerCalled -Scope Global -ErrorAction SilentlyContinue
    Remove-Variable ArdentsReleaseSourceGateFixtureRoot -Scope Global -ErrorAction SilentlyContinue
    Remove-Variable ArdentsReleaseSourceGateForbiddenFile -Scope Global -ErrorAction SilentlyContinue
    if (Test-Path -LiteralPath $fixtureRoot) {
        Remove-Item -LiteralPath $fixtureRoot -Recurse -Force
    }
}
