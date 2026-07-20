param(
    [ValidateSet("fast", "integration", "e2e", "all")]
    [string]$Suite = "fast",
    [string]$Domain = "",
    [string]$Scenario = "",
    [string]$Tag = "",
    [string]$Profile = "",
    [string]$ReportDir = "",
    [string]$JsonReport = "",
    [string]$JUnitReport = "",
    [string]$CoverageProfile = "",
    [string]$ContainerImage = "ardents/test-runner:dev",
    [switch]$RebuildContainer
)

$ErrorActionPreference = "Stop"

function Convert-ToContainerPath {
    param([string]$Path, [string]$Root)
    if (-not $Path) { return "" }
    if (-not [IO.Path]::IsPathRooted($Path)) { return $Path.Replace("\", "/") }
    $full = [IO.Path]::GetFullPath($Path)
    $relative = [IO.Path]::GetRelativePath($Root, $full)
    if ($relative.StartsWith("..")) { throw "container report path must remain inside the repository: $Path" }
    return $relative.Replace("\", "/")
}

function Invoke-ContainerRunner {
    $root = Split-Path -Parent $PSScriptRoot
    $dockerfile = Join-Path $root "docker/test-runner.Dockerfile"
    $resourceDir = Join-Path $PSScriptRoot ".artifacts/resources"
    $runID = [DateTime]::UtcNow.ToString("yyyyMMdd-HHmmss") + "-$PID"

    & (Join-Path $PSScriptRoot "resource-snapshot.ps1") -Label "before-$Suite" -OutputPath (Join-Path $resourceDir "$runID-before.json")

    & docker image inspect $ContainerImage *> $null
    $imageExists = $LASTEXITCODE -eq 0
    if ($RebuildContainer -or -not $imageExists) {
        Write-Host "==> build test container $ContainerImage"
        $previousGitInfo = $env:BUILDX_GIT_INFO
        try {
            $env:BUILDX_GIT_INFO = "false"
            & docker build -t $ContainerImage -f $dockerfile $root
            if ($LASTEXITCODE -ne 0) { throw "test container build failed" }
        } finally {
            if ($null -eq $previousGitInfo) { Remove-Item Env:BUILDX_GIT_INFO -ErrorAction SilentlyContinue }
            else { $env:BUILDX_GIT_INFO = $previousGitInfo }
        }
    }

    $dockerArgs = @(
        "run", "--rm", "--init",
        "--name", "ardents-tests-$PID",
        "--mount", "type=bind,source=$root,target=/workspace",
        "--mount", "type=volume,source=ardents-go-mod-cache,target=/go/pkg/mod",
        "--mount", "type=volume,source=ardents-go-build-cache,target=/root/.cache/go-build",
        "-e", "ARDENTS_TEST_RUNTIME=container"
    )
    foreach ($entry in Get-ChildItem Env: | Where-Object { $_.Name -like "ARDENTS_*" -and $_.Name -ne "ARDENTS_TEST_RUNTIME" }) {
        $dockerArgs += @("-e", $entry.Name)
    }
    $dockerArgs += @($ContainerImage, "-Suite", $Suite)
    foreach ($option in @(
        @{ Name = "Domain"; Value = $Domain },
        @{ Name = "Scenario"; Value = $Scenario },
        @{ Name = "Tag"; Value = $Tag },
        @{ Name = "Profile"; Value = $Profile },
        @{ Name = "ReportDir"; Value = (Convert-ToContainerPath $ReportDir $root) },
        @{ Name = "JsonReport"; Value = (Convert-ToContainerPath $JsonReport $root) },
        @{ Name = "JUnitReport"; Value = (Convert-ToContainerPath $JUnitReport $root) }
        @{ Name = "CoverageProfile"; Value = (Convert-ToContainerPath $CoverageProfile $root) }
    )) {
        if ($option.Value) { $dockerArgs += @("-$($option.Name)", $option.Value) }
    }

    Write-Host "==> run $Suite suite in Linux container"
    $runError = $null
    try {
        & docker @dockerArgs
        if ($LASTEXITCODE -ne 0) { throw "container test run failed with exit code $LASTEXITCODE" }
    } catch {
        $runError = $_
    } finally {
        & (Join-Path $PSScriptRoot "resource-snapshot.ps1") -Label "after-$Suite" -OutputPath (Join-Path $resourceDir "$runID-after.json")
    }
    if ($runError) { throw $runError }
}

if ($env:ARDENTS_TEST_RUNTIME -ne "container") {
    Invoke-ContainerRunner
    exit 0
}

$windowsTag = ""
if ($IsWindows -or $env:OS -eq "Windows_NT") {
    $windowsTag = "gowaku_no_rln"
}

function Get-EffectiveTags {
    param(
        [string[]]$Tags
    )

    $effectiveTags = @()
    if ($windowsTag) {
        $effectiveTags += $windowsTag
    }
    if ($Tags) {
        $effectiveTags += $Tags
    }
    return $effectiveTags
}

function Invoke-GoTest {
    param(
        [string[]]$Tags,
        [string[]]$Packages
    )

    $args = @("test")
    $effectiveTags = Get-EffectiveTags -Tags $Tags
    if ($effectiveTags.Count -gt 0) {
        $args += "-tags"
        $args += ($effectiveTags -join " ")
    }
    $args += $Packages
    if ($CoverageProfile) {
        $coverageParent = Split-Path -Parent $CoverageProfile
        if ($coverageParent) {
            New-Item -ItemType Directory -Force -Path $coverageParent | Out-Null
        }
        $args += "-coverprofile=$CoverageProfile"
    }

    & go @args
    if ($LASTEXITCODE -ne 0) {
        throw "go test failed"
    }
}

function Invoke-RepoGuards {
    & go run ./tests/cmd/importguard
    if ($LASTEXITCODE -ne 0) {
        throw "repository guard checks failed"
    }
}

function Get-GoTestPackages {
    param(
        [string[]]$Tags,
        [string[]]$Patterns
    )

    $args = @("list")
    $effectiveTags = Get-EffectiveTags -Tags $Tags
    if ($effectiveTags.Count -gt 0) {
        $args += "-tags"
        $args += ($effectiveTags -join " ")
    }
    $args += "-f"
    $args += "{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}"
    $args += $Patterns

    $packages = & go @args
    if ($LASTEXITCODE -ne 0) {
        throw "go list failed"
    }
    return @($packages | Where-Object { $_ -and $_.Trim() -ne "" })
}

function Write-RunnerFailureReport {
    param(
        [string]$SuiteName,
        [string]$Layer,
        [string]$RawReportDir,
        [string]$Package,
        [string]$Phase,
        [string]$Detail
    )

    if (-not $RawReportDir) {
        return
    }
    New-Item -ItemType Directory -Force -Path $RawReportDir | Out-Null
    $safePhase = $Phase -replace "[^A-Za-z0-9_.-]", "-"
    $path = Join-Path $RawReportDir ("runner-{0}-{1}.json" -f $safePhase, [guid]::NewGuid().ToString("N"))
    $now = (Get-Date).ToUniversalTime().ToString("o")
    $report = [ordered]@{
        package    = $Package
        spec       = [ordered]@{
            layer       = $Layer
            domain      = "qa-runner"
            scenario_id = "runner-$safePhase"
            suite       = $SuiteName
            tags        = @($SuiteName, "runner-failure")
            speed       = "default"
            environment = "local"
        }
        test_name   = "[$Phase] $Package"
        status      = "failed"
        started_at  = $now
        ended_at    = $now
        duration    = 0
        steps       = @([ordered]@{
            kind   = $Phase
            name   = $Package
            status = "failed"
            detail = $Detail
        })
    }
    $report | ConvertTo-Json -Depth 10 | Set-Content -Path $path -Encoding UTF8
    Write-Host "==> raw failure $path"
}

function Get-StableBinaryPath {
    param(
        [string]$SuiteName,
        [string]$ImportPath
    )

    $name = $ImportPath -replace "[/:\\\.]", "_"
    $dir = Join-Path (Join-Path (Join-Path $PSScriptRoot ".artifacts") "testbin") $SuiteName
    $ext = ""
    if ($IsWindows -or $env:OS -eq "Windows_NT") {
        $ext = ".exe"
    }
    return Join-Path $dir ($name + $ext)
}

function Test-IsAdministrator {
    if (-not ($IsWindows -or $env:OS -eq "Windows_NT")) {
        return $false
    }

    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Get-StableTestBuildPlan {
    param(
        [string]$SuiteName,
        [string[]]$Packages
    )

    $plan = @()
    foreach ($pkg in $Packages) {
        $plan += [pscustomobject]@{
            Package    = $pkg
            BinaryPath = Get-StableBinaryPath -SuiteName $SuiteName -ImportPath $pkg
        }
    }
    return $plan
}

function Build-StableGoTestBinaries {
    param(
        [string]$SuiteName,
        [string]$Layer,
        [string[]]$Tags,
        [string[]]$Packages,
        [string]$RawReportDir
    )

    $effectiveTags = Get-EffectiveTags -Tags $Tags
    $plan = Get-StableTestBuildPlan -SuiteName $SuiteName -Packages $Packages

    foreach ($item in $plan) {
        $buildArgs = @("test", "-c")
        if ($effectiveTags.Count -gt 0) {
            $buildArgs += "-tags"
            $buildArgs += ($effectiveTags -join " ")
        }
        $buildArgs += "-o"
        $buildArgs += $item.BinaryPath
        $buildArgs += $item.Package

        Write-Host "==> build $($item.Package)"
        & go @buildArgs
        if ($LASTEXITCODE -ne 0) {
            Write-RunnerFailureReport -SuiteName $SuiteName -Layer $Layer -RawReportDir $RawReportDir -Package $item.Package -Phase "build" -Detail "go test -c failed"
            throw "build failed for $($item.Package)"
        }
    }

    return $plan
}

function Ensure-WindowsFirewallRules {
    param(
        [string]$SuiteName,
        [object[]]$BuildPlan
    )

    if (-not ($IsWindows -or $env:OS -eq "Windows_NT")) {
        return
    }
    if (-not $BuildPlan -or $BuildPlan.Count -eq 0) {
        return
    }
    if (-not (Get-Command New-NetFirewallRule -ErrorAction SilentlyContinue)) {
        Write-Warning "Windows firewall bootstrap skipped: New-NetFirewallRule is unavailable"
        return
    }
    if (-not (Test-IsAdministrator)) {
        Write-Warning "Windows firewall bootstrap skipped: rerun the tagged suite from an elevated shell to pre-approve all suite binaries once"
        return
    }

    $groupName = "Ardents Test Suites"

    foreach ($item in $BuildPlan) {
        $ruleName = "Ardents Tests [$SuiteName] $($item.Package)"
        $existing = @(Get-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue)
        $matchesProgram = $false
        foreach ($rule in $existing) {
            $appFilter = Get-NetFirewallApplicationFilter -AssociatedNetFirewallRule $rule -ErrorAction SilentlyContinue
            if ($appFilter -and $appFilter.Program -eq $item.BinaryPath) {
                $matchesProgram = $true
                break
            }
        }
        if ($matchesProgram) {
            continue
        }

        Write-Host "==> allow $($item.Package)"
        New-NetFirewallRule `
            -DisplayName $ruleName `
            -DisplayGroup $groupName `
            -Direction Inbound `
            -Action Allow `
            -Enabled True `
            -Profile Any `
            -Program $item.BinaryPath | Out-Null
    }
}

function Get-TestCatalog {
    param(
        [string[]]$Tags,
        [string[]]$Patterns,
        [string]$Layer = "",
        [string]$Domain = "",
        [string]$Scenario = "",
        [string]$Tag = "",
        [string]$Profile = ""
    )

    $args = @("run", "./tests/cmd/testcatalog")
    $effectiveTags = Get-EffectiveTags -Tags $Tags
    if ($effectiveTags.Count -gt 0) {
        $args += "-tags"
        $args += ($effectiveTags -join " ")
    }
    if ($Layer) {
        $args += "-layer"
        $args += $Layer
    }
    if ($Domain) {
        $args += "-domain"
        $args += $Domain
    }
    if ($Scenario) {
        $args += "-scenario"
        $args += $Scenario
    }
    if ($Tag) {
        $args += "-tag"
        $args += $Tag
    }
    if ($Profile) {
        $args += "-suite"
        $args += $Profile
    }
    $args += $Patterns

    $json = & go @args
    if ($LASTEXITCODE -ne 0) {
        throw "catalog generation failed"
    }
    if (-not $json) {
        return @()
    }

    $entries = $json | ConvertFrom-Json
    return @($entries)
}

function Group-CatalogEntries {
    param(
        [object[]]$Entries
    )

    $grouped = @{}
    foreach ($entry in $Entries) {
        if (-not $grouped.ContainsKey($entry.package)) {
            $grouped[$entry.package] = @()
        }
        $grouped[$entry.package] += $entry.test_name
    }
    return $grouped
}

function Get-TestRunPattern {
    param(
        [string[]]$Tests
    )

    if (-not $Tests -or $Tests.Count -eq 0) {
        return ""
    }

    $escaped = $Tests | ForEach-Object { [regex]::Escape($_) }
    return "^(" + ($escaped -join "|") + ")$"
}

function Invoke-StableGoTest {
    param(
        [string]$SuiteName,
        [string]$Layer,
        [string[]]$Tags,
        [string[]]$Packages,
        [hashtable]$Selections,
        [string]$RawReportDir
    )

    $testPackages = Get-GoTestPackages -Tags $Tags -Patterns $Packages
    if ($testPackages.Count -eq 0) {
        return
    }

    if ($Selections) {
        $selectedPackages = @($Selections.Keys)
        $testPackages = @($testPackages | Where-Object { $Selections.ContainsKey($_) })
        if ($testPackages.Count -eq 0) {
            throw "selection produced no runnable packages"
        }
    }

    $binDir = Join-Path (Join-Path (Join-Path $PSScriptRoot ".artifacts") "testbin") $SuiteName
    New-Item -ItemType Directory -Force -Path $binDir | Out-Null

    $previousReportDir = $env:ARDENTS_TESTKIT_REPORT_DIR
    try {
        if ($RawReportDir) {
            New-Item -ItemType Directory -Force -Path $RawReportDir | Out-Null
            $env:ARDENTS_TESTKIT_REPORT_DIR = $RawReportDir
        } else {
            Remove-Item Env:ARDENTS_TESTKIT_REPORT_DIR -ErrorAction SilentlyContinue
        }

        $buildPlan = Build-StableGoTestBinaries -SuiteName $SuiteName -Layer $Layer -Tags $Tags -Packages $testPackages -RawReportDir $RawReportDir
        Ensure-WindowsFirewallRules -SuiteName $SuiteName -BuildPlan $buildPlan

        foreach ($item in $buildPlan) {
            $pkg = $item.Package
            $binPath = $item.BinaryPath
            $runArgs = @("-test.v")
            if ($Selections -and $Selections.ContainsKey($pkg)) {
                $pattern = Get-TestRunPattern -Tests $Selections[$pkg]
                if ($pattern) {
                    $runArgs += "-test.run"
                    $runArgs += $pattern
                }
            }

            Write-Host "==> run   $pkg"
            $reportCountBefore = @(Get-ReportFiles -RawReportDir $RawReportDir).Count
            & $binPath @runArgs
            if ($LASTEXITCODE -ne 0) {
                $reportCountAfter = @(Get-ReportFiles -RawReportDir $RawReportDir).Count
                if ($reportCountAfter -le $reportCountBefore) {
                    Write-RunnerFailureReport -SuiteName $SuiteName -Layer $Layer -RawReportDir $RawReportDir -Package $pkg -Phase "test" -Detail "test binary returned a non-zero exit code without a testkit failure report"
                }
                throw "tests failed for $pkg"
            }
        }
    }
    finally {
        if ($null -ne $previousReportDir) {
            $env:ARDENTS_TESTKIT_REPORT_DIR = $previousReportDir
        } else {
            Remove-Item Env:ARDENTS_TESTKIT_REPORT_DIR -ErrorAction SilentlyContinue
        }
    }
}

function Get-DefaultReportBaseDir {
    param(
        [string]$SuiteName
    )

    return Join-Path (Join-Path (Join-Path $PSScriptRoot ".artifacts") "reports") $SuiteName
}

function Get-ReportFiles {
    param(
        [string]$RawReportDir
    )

    if (-not $RawReportDir -or -not (Test-Path $RawReportDir)) {
        return @()
    }
    return @(Get-ChildItem -Path $RawReportDir -Filter *.json -File | Sort-Object FullName)
}

function Write-JUnitReport {
    param(
        [object]$ReportObject,
        [string]$Path
    )

    $tests = @($ReportObject.tests)
    $failureCount = @($tests | Where-Object { $_.status -ne "passed" }).Count
    $durationSeconds = [math]::Round(([double]$ReportObject.summary.duration_ms) / 1000.0, 3)

    $lines = @()
    $lines += '<?xml version="1.0" encoding="UTF-8"?>'
    $lines += "<testsuite name=`"$($ReportObject.suite)`" tests=`"$($tests.Count)`" failures=`"$failureCount`" time=`"$durationSeconds`">"

    foreach ($test in $tests) {
        $testDurationSeconds = [math]::Round(([double]$test.duration_ms) / 1000.0, 3)
        $className = [System.Security.SecurityElement]::Escape($test.package)
        $testName = [System.Security.SecurityElement]::Escape($test.test_name)
        $lines += "  <testcase classname=`"$className`" name=`"$testName`" time=`"$testDurationSeconds`">"
        if ($test.status -ne "passed") {
            $failedStep = ""
            foreach ($step in @($test.steps)) {
                if ($null -ne $step -and $step.status -ne "passed") {
                    $failedStep = "$($step.kind): $($step.name)"
                    break
                }
            }
            if (-not $failedStep) {
                $failedStep = "test failed"
            }
            $message = [System.Security.SecurityElement]::Escape($failedStep)
            $lines += "    <failure message=`"$message`">$message</failure>"
        }
        $lines += "  </testcase>"
    }

    $lines += "</testsuite>"
    Set-Content -Path $Path -Value $lines -Encoding UTF8
}

function Write-CanonicalReports {
    param(
        [string]$SuiteName,
        [string]$RawReportDir,
        [string]$JsonPath,
        [string]$JUnitPath,
        [hashtable]$Filters
    )

    $files = Get-ReportFiles -RawReportDir $RawReportDir
    if ($files.Count -eq 0) {
        Write-Warning "No testkit reports were captured under $RawReportDir"
        return
    }

    $reports = @()
    foreach ($file in $files) {
        $reports += (Get-Content $file.FullName -Raw | ConvertFrom-Json)
    }

    $passed = @($reports | Where-Object { $_.status -eq "passed" }).Count
    $failed = @($reports | Where-Object { $_.status -ne "passed" }).Count
    $durationMs = 0
    $tests = @()

    foreach ($report in $reports) {
        $testDurationMs = [math]::Round(([double]$report.duration) / 1000000.0, 3)
        $durationMs += $testDurationMs
        $tests += [ordered]@{
            package      = $report.package
            test_name    = $report.test_name
            status       = $report.status
            layer        = $report.spec.layer
            domain       = $report.spec.domain
            scenario_id  = $report.spec.scenario_id
            suite        = $report.spec.suite
            tags         = @($report.spec.tags)
            speed        = $report.spec.speed
            environment  = $report.spec.environment
            duration_ms  = $testDurationMs
            steps        = @($report.steps)
            raw_report   = $report
        }
    }

    $reportObject = [ordered]@{
        suite        = $SuiteName
        generated_at = (Get-Date).ToUniversalTime().ToString("o")
        raw_report_dir = $RawReportDir
        filters      = $Filters
        summary      = [ordered]@{
            total       = $reports.Count
            passed      = $passed
            failed      = $failed
            duration_ms = $durationMs
        }
        tests        = $tests
    }

    if ($JsonPath) {
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $JsonPath) | Out-Null
        $reportObject | ConvertTo-Json -Depth 20 | Set-Content -Path $JsonPath -Encoding UTF8
        Write-Host "==> json  $JsonPath"
    }

    if ($JUnitPath) {
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $JUnitPath) | Out-Null
        Write-JUnitReport -ReportObject $reportObject -Path $JUnitPath
        Write-Host "==> junit $JUnitPath"
    }
}

function Get-SuiteConfig {
    param(
        [string]$Suite
    )

    switch ($Suite) {
        "integration" {
            return @{
                SuiteName = "integration"
                Tags      = @("integration")
                Packages  = @("./...")
                Layer     = "integration"
            }
        }
        "e2e" {
            return @{
                SuiteName = "e2e"
                Tags      = @("integration", "e2e")
                Packages  = @("./tests/e2e/...")
                Layer     = "e2e"
            }
        }
        "all" {
            return @{
                SuiteName = "all"
                Tags      = @("integration", "e2e")
                Packages  = @("./...")
                Layer     = ""
            }
        }
        default {
            throw "unsupported suite $Suite"
        }
    }
}

function Resolve-ReportPaths {
    param(
        [string]$SuiteName,
        [bool]$NeedsArtifacts
    )

    if (-not $NeedsArtifacts) {
        return @{
            BaseDir = ""
            RawDir  = ""
            Json    = ""
            JUnit   = ""
        }
    }

    $baseDir = $ReportDir
    if (-not $baseDir) {
        $baseDir = Get-DefaultReportBaseDir -SuiteName $SuiteName
    }
    $jsonPath = $JsonReport
    if (-not $jsonPath) {
        $jsonPath = Join-Path $baseDir "summary.json"
    }
    $junitPath = $JUnitReport
    if (-not $junitPath) {
        $junitPath = Join-Path $baseDir "junit.xml"
    }

    return @{
        BaseDir = $baseDir
        RawDir  = Join-Path $baseDir "raw"
        Json    = $jsonPath
        JUnit   = $junitPath
    }
}

function Reset-ReportArtifacts {
    param(
        [hashtable]$Paths
    )

    if (-not $Paths -or -not $Paths.RawDir) {
        return
    }

    if (Test-Path $Paths.RawDir) {
        Remove-Item -Path $Paths.RawDir -Recurse -Force
    }
    New-Item -ItemType Directory -Force -Path $Paths.RawDir | Out-Null

    foreach ($path in @($Paths.Json, $Paths.JUnit)) {
        if ($path -and (Test-Path $path)) {
            Remove-Item -Path $path -Force
        }
    }
}

switch ($Suite) {
    "fast" {
        if ($Domain -or $Scenario -or $Tag -or $Profile -or $ReportDir -or $JsonReport -or $JUnitReport) {
            throw "selection and reporting flags are only supported for tagged suites"
        }
        Invoke-RepoGuards
        Invoke-GoTest -Packages @("./...")
    }
    default {
        if ($CoverageProfile) {
            throw "coverage profile is supported only by the fast suite"
        }
        $config = Get-SuiteConfig -Suite $Suite
        Invoke-RepoGuards
        $needsSelection = [bool]($Domain -or $Scenario -or $Tag -or $Profile)
        $needsArtifacts = $needsSelection -or [bool]($ReportDir -or $JsonReport -or $JUnitReport)
        $reportPaths = Resolve-ReportPaths -SuiteName $config.SuiteName -NeedsArtifacts $needsArtifacts
        if ($needsArtifacts) {
            Reset-ReportArtifacts -Paths $reportPaths
        }

        $filters = [ordered]@{
            domain   = $Domain
            scenario = $Scenario
            tag      = $Tag
            profile  = $Profile
            layer    = $config.Layer
        }
        $entries = @()
        $selections = $null
        if ($needsSelection) {
            $entries = Get-TestCatalog -Tags $config.Tags -Patterns $config.Packages -Layer $config.Layer -Domain $Domain -Scenario $Scenario -Tag $Tag -Profile $Profile
            if ($entries.Count -eq 0) {
                Write-RunnerFailureReport -SuiteName $config.SuiteName -Layer $config.Layer -RawReportDir $reportPaths.RawDir -Package "catalog-selection" -Phase "selection" -Detail "no catalog entries matched the requested selection"
                Write-CanonicalReports -SuiteName $config.SuiteName -RawReportDir $reportPaths.RawDir -JsonPath $reportPaths.Json -JUnitPath $reportPaths.JUnit -Filters $filters
                throw "no catalog entries matched the requested selection"
            }
            $selections = Group-CatalogEntries -Entries $entries
        }

        $runError = $null
        try {
            Invoke-StableGoTest -SuiteName $config.SuiteName -Layer $config.Layer -Tags $config.Tags -Packages $config.Packages -Selections $selections -RawReportDir $reportPaths.RawDir
        }
        catch {
            $runError = $_
            if ($needsArtifacts -and @(Get-ReportFiles -RawReportDir $reportPaths.RawDir).Count -eq 0) {
                Write-RunnerFailureReport -SuiteName $config.SuiteName -Layer $config.Layer -RawReportDir $reportPaths.RawDir -Package "runner" -Phase "orchestration" -Detail $_.Exception.Message
            }
        }
        finally {
            if ($needsArtifacts) {
                Write-CanonicalReports -SuiteName $config.SuiteName -RawReportDir $reportPaths.RawDir -JsonPath $reportPaths.Json -JUnitPath $reportPaths.JUnit -Filters $filters
            }
        }
        if ($null -ne $runError) {
            throw $runError
        }
    }
}
