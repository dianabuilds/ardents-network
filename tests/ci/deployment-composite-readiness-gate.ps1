# testkit:scenario SDE-004
# testkit:layer integration
# testkit:domain deployment

param(
    [string]$ReportDir = "tests/.artifacts/deployment/composite-readiness"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
. (Join-Path $root "scripts/deploy/composite-readiness.ps1")

$requestedReportPath = if ([IO.Path]::IsPathRooted($ReportDir)) { $ReportDir } else { Join-Path $root $ReportDir }
$reportPath = [IO.Path]::GetFullPath($requestedReportPath)
$artifactRoot = [IO.Path]::GetFullPath((Join-Path $root "tests/.artifacts"))
if (-not $reportPath.StartsWith($artifactRoot + [IO.Path]::DirectorySeparatorChar)) {
    throw "composite readiness evidence path must stay under tests/.artifacts: $reportPath"
}
New-Item -ItemType Directory -Force -Path $reportPath | Out-Null

function Invoke-ReadinessCase([string]$Component, [scriptblock]$Probe, [string]$ExpectedReason) {
    $script:clock = [DateTime]::Parse("2026-07-24T00:00:00Z").ToUniversalTime()
    $failure = $null
    try {
        Wait-ArdentsCompositeReadiness `
            -Service "peer2" `
            -TimeoutSeconds 2 `
            -Probe $Probe `
            -UtcNow {
                $current = $script:clock
                $script:clock = $script:clock.AddSeconds(1)
                return $current
            } `
            -Pause {}
    } catch {
        $failure = $_.Exception.Message
    }
    if ([string]::IsNullOrWhiteSpace($failure)) {
        throw "$Component degradation was accepted"
    }
    if ($failure -notmatch [regex]::Escape($ExpectedReason)) {
        throw "$Component failure did not retain reason '$ExpectedReason': $failure"
    }
    return [ordered]@{ component = $Component; accepted = $false; failure = $failure }
}

$cases = @(
    Invoke-ReadinessCase "protected_api" { throw "protected API unavailable" } "protected API unavailable"
    Invoke-ReadinessCase "access_grant" { throw "Access Grant admission denied" } "Access Grant admission denied"
    Invoke-ReadinessCase "network" {
        [ordered]@{ runtime = [ordered]@{ readiness = [ordered]@{
            ready = $false; reason = "network: relay unavailable"
            checks = @([ordered]@{ name = "network"; ready = $false; reason = "relay unavailable" })
        } } }
    } "network: relay unavailable"
    Invoke-ReadinessCase "diagnostics" {
        [ordered]@{ runtime = [ordered]@{ readiness = [ordered]@{
            ready = $false; reason = "diagnostics: persistence degraded"
            checks = @([ordered]@{ name = "diagnostics"; ready = $false; reason = "persistence degraded" })
        } } }
    } "diagnostics: persistence degraded"
    Invoke-ReadinessCase "identity" {
        [ordered]@{ runtime = [ordered]@{ readiness = [ordered]@{
            ready = $false; reason = "identity: retained Principal is missing"
            checks = @([ordered]@{ name = "identity"; ready = $false; reason = "retained Principal is missing" })
        } } }
    } "identity: retained Principal is missing"
    Invoke-ReadinessCase "network_only_legacy" {
        [ordered]@{ network = [ordered]@{ state = "ready"; joined = $true } }
    } "node runtime response has no composite readiness"
)

$blockingStartedAt = [DateTime]::UtcNow
$blockingFailure = $null
try {
    Invoke-ArdentsBoundedProcess `
        -FilePath ([Diagnostics.Process]::GetCurrentProcess().MainModule.FileName) `
        -ArgumentList @("-NoProfile", "-NonInteractive", "-Command", "Start-Sleep -Seconds 30") `
        -Deadline ([DateTime]::UtcNow.AddMilliseconds(250)) | Out-Null
} catch {
    $blockingFailure = $_.Exception.Message
}
$blockingElapsed = ([DateTime]::UtcNow - $blockingStartedAt).TotalSeconds
if ($blockingFailure -notmatch "deadline exceeded") {
    throw "blocking probe was not terminated by its deadline: $blockingFailure"
}
if ($blockingElapsed -ge 5) {
    throw "blocking probe exceeded the bounded termination window: $blockingElapsed seconds"
}

$script:clock = [DateTime]::Parse("2026-07-24T00:00:00Z").ToUniversalTime()
Wait-ArdentsCompositeReadiness `
    -Service "peer2" `
    -TimeoutSeconds 2 `
    -Probe {
        [ordered]@{ runtime = [ordered]@{ readiness = [ordered]@{
            ready = $true; reason = ""
            checks = @(
                [ordered]@{ name = "protected_api"; ready = $true }
                [ordered]@{ name = "access_grant"; ready = $true }
                [ordered]@{ name = "network"; ready = $true }
                [ordered]@{ name = "diagnostics"; ready = $true }
                [ordered]@{ name = "identity"; ready = $true }
            )
        } } }
    } `
    -UtcNow {
        $current = $script:clock
        $script:clock = $script:clock.AddSeconds(1)
        return $current
    } `
    -Pause {}

$evidence = [ordered]@{
    scenario = "SDE-004"
    result = "passed"
    bounded_timeout_seconds = 2
    blocking_probe_elapsed_seconds = $blockingElapsed
    blocking_probe_failure = $blockingFailure
    degradation_matrix = $cases
    healthy_composite_accepted = $true
}
$evidence | ConvertTo-Json -Depth 7 | Set-Content -Encoding utf8 -LiteralPath (Join-Path $reportPath "composite-readiness.json")
Write-Host "deployment-composite-readiness-gate=passed matrix_cases=$($cases.Count)"
