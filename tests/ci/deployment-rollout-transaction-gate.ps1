# testkit:scenario SDE-003
# testkit:layer integration
# testkit:domain deployment

param(
    [string]$ReportDir = "tests/.artifacts/deployment/rollout-transaction"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$requestedReportPath = if ([IO.Path]::IsPathRooted($ReportDir)) { $ReportDir } else { Join-Path $root $ReportDir }
$reportPath = [IO.Path]::GetFullPath($requestedReportPath)
$artifactRoot = [IO.Path]::GetFullPath((Join-Path $root "tests/.artifacts"))
if (-not $reportPath.StartsWith($artifactRoot + [IO.Path]::DirectorySeparatorChar)) {
    throw "rollout transaction evidence path must stay under tests/.artifacts: $reportPath"
}
New-Item -ItemType Directory -Force -Path $reportPath | Out-Null

$testRoot = Join-Path ([IO.Path]::GetTempPath()) ("ardents-rollout-transaction-{0}" -f [guid]::NewGuid().ToString("N"))
$oldImage = "registry.example/ardents@sha256:$("1" * 64)"
$newImage = "registry.example/ardents@sha256:$("2" * 64)"
$global:FakeDockerEvents = [Collections.Generic.List[string]]::new()
$global:FakeNodeImages = [ordered]@{}
$global:JournalObservations = [Collections.Generic.List[object]]::new()
$global:CurrentStatePath = ""
$global:InjectedBoundary = ""
$global:InjectedNode = ""
$global:InjectedCompensationNode = ""
$global:CompensationFailuresRemaining = 0

function Write-ClusterFixture([string]$StatePath) {
    New-Item -ItemType Directory -Force -Path $StatePath | Out-Null
    [ordered]@{
        schema = "ardents.deployment/v1"
        project = "ardents-rollout-test"
        image = $oldImage
        previous_image = ""
        bootstrap_endpoint = "/ip4/127.0.0.1/tcp/60000/p2p/test"
        operator_principal = "p1_operator"
        node_principals = [ordered]@{
            seed = "p1_seed"
            peer2 = "p1_peer2"
            peer3 = "p1_peer3"
        }
        services = @("seed", "peer2", "peer3")
    } | ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 -LiteralPath (Join-Path $StatePath "cluster.json")
    foreach ($service in @("seed", "peer2", "peer3")) {
        $global:FakeNodeImages[$service] = $oldImage
    }
}

function global:docker {
    $arguments = @($args | ForEach-Object { [string]$_ })
    $event = $arguments -join " "
    $global:FakeDockerEvents.Add($event)
    $global:LASTEXITCODE = 0
    Set-Variable -Name LASTEXITCODE -Value 0 -Scope 1

    if ($arguments[0] -eq "inspect") {
        $node = $arguments[-1] -replace "-container$", ""
        return @([ordered]@{
            Mounts = @([ordered]@{
                Destination = "/var/lib/ardents"
                Name = "ardents-rollout-test_${node}_data"
            })
        }) | ConvertTo-Json -Depth 4
    }

    if ($arguments[0] -eq "run") {
        $backupMount = $arguments | Where-Object { $_ -match "^type=bind,source=.+,target=/backup(?:,readonly)?$" } | Select-Object -First 1
        if ($backupMount) {
            $archiveDir = $backupMount -replace "^type=bind,source=", "" -replace ",target=/backup(?:,readonly)?$", ""
            $createIndex = [Array]::IndexOf($arguments, "-czf")
            if ($createIndex -ge 0) {
                $archiveName = [IO.Path]::GetFileName($arguments[$createIndex + 1])
                $payloadPath = Join-Path $archiveDir "state.txt"
                Set-Content -Encoding utf8 -LiteralPath $payloadPath -Value "rollout backup"
                & tar -czf (Join-Path $archiveDir $archiveName) -C $archiveDir "state.txt"
                $tarExitCode = $LASTEXITCODE
                Remove-Item -LiteralPath $payloadPath -Force
                $global:LASTEXITCODE = $tarExitCode
                Set-Variable -Name LASTEXITCODE -Value $tarExitCode -Scope 1
                return
            }
            $verifyIndex = [Array]::IndexOf($arguments, "-tzf")
            if ($verifyIndex -ge 0) {
                $archiveName = [IO.Path]::GetFileName($arguments[$verifyIndex + 1])
                & tar -tzf (Join-Path $archiveDir $archiveName) | Out-Null
                $tarExitCode = $LASTEXITCODE
                $global:LASTEXITCODE = $tarExitCode
                Set-Variable -Name LASTEXITCODE -Value $tarExitCode -Scope 1
                return
            }
        }
        return
    }

    $service = @("seed", "peer2", "peer3") |
        Where-Object {
            $candidate = $_
            $arguments | Where-Object { $_ -eq $candidate -or $_ -eq "$candidate-operator" }
        } |
        Select-Object -Last 1
    if ($arguments -contains "ps" -and $arguments -contains "-aq") {
        return "$service-container"
    }
    if ($arguments -contains "create" -and $arguments -contains "--force-recreate") {
        $journalPath = Join-Path $global:CurrentStatePath "rollout-transaction.json"
        $journalNodeStatus = ""
        if (Test-Path -LiteralPath $journalPath) {
            $journal = Get-Content -Raw -LiteralPath $journalPath | ConvertFrom-Json
            $journalNodeStatus = [string](
                $journal.nodes |
                    Where-Object { $_.service -eq $service } |
                    Select-Object -ExpandProperty status -First 1
            )
        }
        $global:JournalObservations.Add([ordered]@{
            service = $service
            image = [string]$env:ARDENTS_DOCKER_IMAGE
            journal_exists = Test-Path -LiteralPath $journalPath
            journal_node_status = $journalNodeStatus
        })
        if (
            $env:ARDENTS_DOCKER_IMAGE -eq $oldImage -and
            $service -eq $global:InjectedCompensationNode -and
            $global:CompensationFailuresRemaining -gt 0
        ) {
            $global:CompensationFailuresRemaining--
            $global:LASTEXITCODE = 1
            Set-Variable -Name LASTEXITCODE -Value 1 -Scope 1
            return
        }
        $global:FakeNodeImages[$service] = [string]$env:ARDENTS_DOCKER_IMAGE
        if (
            $env:ARDENTS_DOCKER_IMAGE -eq $newImage -and
            $service -eq $global:InjectedNode -and
            $global:InjectedBoundary -eq "recreate"
        ) {
            $global:LASTEXITCODE = 1
            Set-Variable -Name LASTEXITCODE -Value 1 -Scope 1
        }
        return
    }
    if (
        $arguments -contains "start" -and
        $env:ARDENTS_DOCKER_IMAGE -eq $newImage -and
        $service -eq $global:InjectedNode -and
        $global:InjectedBoundary -eq "start"
    ) {
        $global:LASTEXITCODE = 1
        Set-Variable -Name LASTEXITCODE -Value 1 -Scope 1
        return
    }
    if ($arguments -contains "records" -and $arguments -contains "list") {
        return [ordered]@{
            records = @([ordered]@{
                sourceV1 = "local"
                nodeFacts = [ordered]@{
                    principal = "p1_fixture"
                    endpoints = @("/ip4/127.0.0.1/tcp/60000/p2p/fixture-peer")
                }
            })
        } | ConvertTo-Json -Depth 6
    }
    if ($arguments -contains "node" -and $arguments -contains "runtime") {
        $ready = -not (
            $service -eq $global:InjectedNode -and
            $global:InjectedBoundary -eq "readiness" -and
            $global:FakeNodeImages[$service] -eq $newImage
        )
        return [ordered]@{
            runtime = [ordered]@{
                readiness = [ordered]@{
                    ready = $ready
                    reason = if ($ready) { "" } else { "network: injected rollout degradation" }
                    checks = @(
                        [ordered]@{ name = "protected_api"; ready = $true; reason = "" }
                        [ordered]@{ name = "access_grant"; ready = $true; reason = "" }
                        [ordered]@{ name = "network"; ready = $ready; reason = if ($ready) { "" } else { "injected rollout degradation" } }
                        [ordered]@{ name = "diagnostics"; ready = $true; reason = "" }
                        [ordered]@{ name = "identity"; ready = $true; reason = "" }
                    )
                }
            }
        } | ConvertTo-Json -Depth 6
    }
    if ($arguments -contains "network" -and $arguments -contains "status") {
        return [ordered]@{ network = [ordered]@{ state = "ready"; joined = $true } } | ConvertTo-Json -Depth 3
    }
}

function Invoke-MutationFailureScenario([string]$Boundary, [string]$Node) {
    $statePath = Join-Path $testRoot "$Boundary-$Node"
    Write-ClusterFixture $statePath
    $global:CurrentStatePath = $statePath
    $global:FakeDockerEvents.Clear()
    $global:JournalObservations.Clear()
    $global:InjectedBoundary = $Boundary
    $global:InjectedNode = $Node
    $global:InjectedCompensationNode = ""
    $global:CompensationFailuresRemaining = 0

    $failure = $null
    try {
        & (Join-Path $root "scripts/deploy/rollout.ps1") upgrade `
            -StateDir $statePath `
            -Project "ardents-rollout-test" `
            -TimeoutSeconds 10 `
            -NewImage $newImage
    } catch {
        $failure = $_.Exception.Message
    }

    if ([string]::IsNullOrWhiteSpace($failure) -or $failure -notmatch "image change failed") {
        throw "$Boundary/$Node did not fail through rollout compensation: $failure"
    }
    foreach ($service in @("seed", "peer2", "peer3")) {
        if ($global:FakeNodeImages[$service] -ne $oldImage) {
            throw "$Boundary/$Node left $service on $($global:FakeNodeImages[$service]); want fallback $oldImage"
        }
    }
    if (Test-Path -LiteralPath (Join-Path $statePath "rollout-transaction.json")) {
        throw "$Boundary/$Node retained a journal after successful compensation"
    }
    $manifest = Get-Content -Raw -LiteralPath (Join-Path $statePath "cluster.json") | ConvertFrom-Json
    if ($manifest.image -ne $oldImage) {
        throw "$Boundary/$Node committed a failed target image to the cluster manifest"
    }
    $durableObservation = $global:JournalObservations |
        Where-Object {
            $_.service -eq $Node -and
            $_.image -eq $newImage -and
            $_.journal_exists -and
            $_.journal_node_status -eq "mutation_pending"
        } |
        Select-Object -First 1
    if ($null -eq $durableObservation) {
        throw "$Boundary/$Node reached recreation without a durable mutation_pending journal entry"
    }

    return [ordered]@{
        boundary = $Boundary
        node = $Node
        failure = $failure
        node_images = [ordered]@{
            seed = $global:FakeNodeImages.seed
            peer2 = $global:FakeNodeImages.peer2
            peer3 = $global:FakeNodeImages.peer3
        }
        journal_before_recreate = $durableObservation
        events = @($global:FakeDockerEvents)
    }
}

function Invoke-InterruptedRecoveryScenario(
    [string]$Boundary,
    [string]$Phase,
    [string]$NodeStatus,
    [string]$NodeImage
) {
    $statePath = Join-Path $testRoot "interrupted-$Boundary"
    Write-ClusterFixture $statePath
    $global:CurrentStatePath = $statePath
    $global:FakeDockerEvents.Clear()
    $global:JournalObservations.Clear()
    $global:InjectedBoundary = ""
    $global:InjectedNode = ""
    $global:InjectedCompensationNode = ""
    $global:CompensationFailuresRemaining = 0
    $global:FakeNodeImages.seed = $NodeImage
    $journalPath = Join-Path $statePath "rollout-transaction.json"
    [ordered]@{
        schema = "ardents.rollout-transaction/v1"
        project = "ardents-rollout-test"
        action = "upgrade"
        phase = $Phase
        target_image = $newImage
        fallback_image = $oldImage
        started_at = "2026-07-24T00:00:00Z"
        failure = ""
        nodes = @([ordered]@{
            service = "seed"
            status = $NodeStatus
            last_error = ""
            journaled_at = "2026-07-24T00:00:00Z"
            applied_at = ""
            restored_at = ""
        })
    } | ConvertTo-Json -Depth 8 | Set-Content -Encoding utf8 -LiteralPath $journalPath

    & (Join-Path $root "scripts/deploy/rollout.ps1") upgrade `
        -StateDir $statePath `
        -Project "ardents-rollout-test" `
        -TimeoutSeconds 10 `
        -NewImage $newImage

    if (Test-Path -LiteralPath $journalPath) {
        throw "interruption after $Boundary did not clear its completed compensation journal"
    }
    foreach ($service in @("seed", "peer2", "peer3")) {
        if ($global:FakeNodeImages[$service] -ne $oldImage) {
            throw "interruption after $Boundary left $service on $($global:FakeNodeImages[$service]); want $oldImage"
        }
    }
    $manifest = Get-Content -Raw -LiteralPath (Join-Path $statePath "cluster.json") | ConvertFrom-Json
    if ($manifest.image -ne $oldImage) {
        throw "interruption after $Boundary changed the cluster manifest"
    }

    return [ordered]@{
        boundary = $Boundary
        recovered_from_status = $NodeStatus
        initial_node_image = $NodeImage
        final_node_images = [ordered]@{
            seed = $global:FakeNodeImages.seed
            peer2 = $global:FakeNodeImages.peer2
            peer3 = $global:FakeNodeImages.peer3
        }
        events = @($global:FakeDockerEvents)
    }
}

try {
    $matrix = [Collections.Generic.List[object]]::new()
    foreach ($boundary in @("recreate", "start", "readiness")) {
        foreach ($node in @("seed", "peer2", "peer3")) {
            $matrix.Add((Invoke-MutationFailureScenario $boundary $node))
        }
    }

    $resumeStatePath = Join-Path $testRoot "resume-compensation"
    Write-ClusterFixture $resumeStatePath
    $global:CurrentStatePath = $resumeStatePath
    $global:FakeDockerEvents.Clear()
    $global:JournalObservations.Clear()
    $global:InjectedBoundary = "readiness"
    $global:InjectedNode = "peer3"
    $global:InjectedCompensationNode = "peer2"
    $global:CompensationFailuresRemaining = 1
    $firstFailure = $null
    try {
        & (Join-Path $root "scripts/deploy/rollout.ps1") upgrade `
            -StateDir $resumeStatePath `
            -Project "ardents-rollout-test" `
            -TimeoutSeconds 10 `
            -NewImage $newImage
    } catch {
        $firstFailure = $_.Exception.Message
    }
    $resumeJournalPath = Join-Path $resumeStatePath "rollout-transaction.json"
    if ([string]::IsNullOrWhiteSpace($firstFailure) -or -not (Test-Path -LiteralPath $resumeJournalPath)) {
        throw "injected compensation interruption did not retain a resumable journal"
    }

    $global:InjectedBoundary = ""
    $global:InjectedNode = ""
    $global:InjectedCompensationNode = ""
    & (Join-Path $root "scripts/deploy/rollout.ps1") upgrade `
        -StateDir $resumeStatePath `
        -Project "ardents-rollout-test" `
        -TimeoutSeconds 10 `
        -NewImage $newImage
    if (Test-Path -LiteralPath $resumeJournalPath) {
        throw "resumed compensation did not clear its completed journal"
    }
    foreach ($service in @("seed", "peer2", "peer3")) {
        if ($global:FakeNodeImages[$service] -ne $oldImage) {
            throw "resumed compensation left $service on $($global:FakeNodeImages[$service]); want $oldImage"
        }
    }
    $resumeManifest = Get-Content -Raw -LiteralPath (Join-Path $resumeStatePath "cluster.json") | ConvertFrom-Json
    if ($resumeManifest.image -ne $oldImage) {
        throw "resumed compensation changed the cluster manifest"
    }

    $interruptionMatrix = [Collections.Generic.List[object]]::new()
    $interruptionMatrix.Add((Invoke-InterruptedRecoveryScenario "recreate" "applying" "mutation_pending" $newImage))
    $interruptionMatrix.Add((Invoke-InterruptedRecoveryScenario "start" "applying" "recreated" $newImage))
    $interruptionMatrix.Add((Invoke-InterruptedRecoveryScenario "readiness" "applying" "started" $newImage))
    $interruptionMatrix.Add((Invoke-InterruptedRecoveryScenario "fallback-mutation" "compensating" "compensating" $oldImage))

    $matrix | ConvertTo-Json -Depth 8 |
        Set-Content -Encoding utf8 -LiteralPath (Join-Path $reportPath "mutation-matrix.json")
    $interruptionMatrix | ConvertTo-Json -Depth 8 |
        Set-Content -Encoding utf8 -LiteralPath (Join-Path $reportPath "interruption-matrix.json")
    [ordered]@{
        first_failure = $firstFailure
        node_images = $global:FakeNodeImages
        events = $global:FakeDockerEvents
    } | ConvertTo-Json -Depth 8 |
        Set-Content -Encoding utf8 -LiteralPath (Join-Path $reportPath "resume-compensation.json")
    @(
        "deployment-rollout-transaction-gate=passed"
        "matrix_cases=$($matrix.Count)"
        "interruption_cases=$($interruptionMatrix.Count)"
        "resumed_compensation=true"
    ) |
        Set-Content -Encoding utf8 -LiteralPath (Join-Path $reportPath "passed.txt")
    Write-Host "deployment-rollout-transaction-gate=passed matrix_cases=$($matrix.Count) interruption_cases=$($interruptionMatrix.Count) resumed_compensation=true"
}
finally {
    Remove-Item Function:\global:docker -ErrorAction SilentlyContinue
    Set-Location $root
    $resolvedTestRoot = [IO.Path]::GetFullPath($testRoot)
    $resolvedTempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
    if ($resolvedTestRoot.StartsWith($resolvedTempRoot, [StringComparison]::OrdinalIgnoreCase) -and
        (Test-Path -LiteralPath $resolvedTestRoot)) {
        Remove-Item -LiteralPath $resolvedTestRoot -Recurse -Force
    }
}
