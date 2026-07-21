# testkit:scenario SOAK-001
# testkit:layer e2e
# testkit:domain release-process

param(
    [ValidateRange(1, 10080)]
    [int]$DurationMinutes = 2880,
    [ValidateRange(15, 240)]
    [int]$CheckpointMinutes = 60,
    [ValidateRange(1, 24)]
    [int]$FullCycleEvery = 6,
    [ValidateRange(180, 3600)]
    [int]$StabilitySeconds = 3000,
    [string]$CandidateImage = "ardents/ardd-testnet:dev",
    [string]$ReportDir = "tests/.artifacts/reports/stb-705-soak",
    [switch]$PlanOnly
)

$ErrorActionPreference = "Stop"
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot "../.."))
Set-Location $root
$reportPath = [IO.Path]::GetFullPath($ReportDir)
$statePath = Join-Path $reportPath "state.json"
$eventsPath = Join-Path $reportPath "events.jsonl"
$resourceScript = Join-Path $root "tests/resource-snapshot.ps1"
$powershell = (Get-Process -Id $PID).Path
$checkpointCount = [math]::Ceiling($DurationMinutes / $CheckpointMinutes)

function Get-CandidateIdentity {
    $commit = (& git rev-parse HEAD).Trim()
    if ($LASTEXITCODE -ne 0) { throw "cannot resolve candidate Git commit" }
    $imageID = (& docker image inspect $CandidateImage --format "{{.Id}}").Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($imageID)) {
        throw "candidate image is unavailable: $CandidateImage"
    }
    $labels = (& docker image inspect $CandidateImage --format "{{json .Config.Labels}}") | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0 -or $labels.'org.opencontainers.image.revision' -ne $commit) {
        throw "candidate image revision does not match Git commit $commit"
    }
    if ([string]::IsNullOrWhiteSpace($labels.'org.opencontainers.image.created') -or
        $labels.'org.opencontainers.image.created' -eq "unknown") {
        throw "candidate image build date is unavailable"
    }
    return [ordered]@{
        commit = $commit
        image = $CandidateImage
        image_id = $imageID
        version = $labels.'org.opencontainers.image.version'
        build_date = $labels.'org.opencontainers.image.created'
    }
}

function Write-Event([string]$Kind, [hashtable]$Data = @{}) {
    $event = [ordered]@{ at = [DateTime]::UtcNow.ToString("o"); kind = $Kind }
    foreach ($entry in $Data.GetEnumerator()) { $event[$entry.Key] = $entry.Value }
    Add-Content -Encoding utf8 -Path $eventsPath -Value ($event | ConvertTo-Json -Compress -Depth 12)
}

function Save-State {
    $temporary = "$statePath.tmp"
    $script:state | ConvertTo-Json -Depth 20 | Set-Content -Encoding utf8 $temporary
    Move-Item -Force $temporary $statePath
}

function Remove-TimedOutResources([int]$ChildPID) {
    $testContainer = "ardents-tests-$ChildPID"
    & docker rm -f $testContainer 2>$null | Out-Null
    foreach ($cleanup in @(
        @{ Project = "ardents-stb307-$ChildPID"; File = "deploy/docker/compose/docker-compose.testnet.yml" },
        @{ Project = "ardents-workload-$ChildPID"; File = "deploy/docker/compose/docker-compose.workload-test.yml" }
    )) {
        & docker compose -p $cleanup.Project -f $cleanup.File down -v --remove-orphans 2>$null | Out-Null
    }
}

function Assert-NoChildResources([int]$ChildPID) {
    $patterns = @("ardents-tests-$ChildPID", "ardents-stb307-$ChildPID", "ardents-workload-$ChildPID")
    foreach ($pattern in $patterns) {
        $remaining = @(& docker ps -a --filter "name=$pattern" --format "{{.Names}}")
        if ($LASTEXITCODE -ne 0) { throw "cannot inspect child Docker resources for $pattern" }
        if ($remaining.Count -gt 0) { throw "child Docker resources remain after teardown: $($remaining -join ',')" }
    }
}

function ConvertTo-ProcessArgument([string]$Value) {
    if ($Value.Contains('"')) { throw "process argument contains an unsupported quote" }
    if ($Value -notmatch '\s') { return $Value }
    return '"' + $Value + '"'
}

function Invoke-BoundedScript {
    param(
        [string]$Name,
        [string]$Script,
        [string[]]$Arguments,
        [int]$TimeoutSeconds,
        [string]$LogDir
    )
    New-Item -ItemType Directory -Force -Path $LogDir | Out-Null
    $stdout = Join-Path $LogDir "$Name.stdout.log"
    $stderr = Join-Path $LogDir "$Name.stderr.log"
    $argumentList = @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $Script) + $Arguments
    $started = [DateTime]::UtcNow
    Write-Event "command_started" @{ name = $Name; timeout_seconds = $TimeoutSeconds; log_dir = $LogDir }
    $startInfo = [Diagnostics.ProcessStartInfo]::new()
    $startInfo.FileName = $powershell
    $startInfo.Arguments = (($argumentList | ForEach-Object { ConvertTo-ProcessArgument $_ }) -join " ")
    $startInfo.UseShellExecute = $false
    $startInfo.CreateNoWindow = $true
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = $startInfo
    if (-not $process.Start()) { throw "cannot start bounded child process: $Name" }
    $stdoutTask = $process.StandardOutput.ReadToEndAsync()
    $stderrTask = $process.StandardError.ReadToEndAsync()
    $finished = $process.WaitForExit($TimeoutSeconds * 1000)
    if (-not $finished) {
        $process.Kill()
        $process.WaitForExit()
        [IO.File]::WriteAllText($stdout, $stdoutTask.Result, [Text.UTF8Encoding]::new($false))
        [IO.File]::WriteAllText($stderr, $stderrTask.Result, [Text.UTF8Encoding]::new($false))
        Remove-TimedOutResources $process.Id
        Write-Event "command_timeout" @{ name = $Name; pid = $process.Id; elapsed_seconds = [math]::Round(([DateTime]::UtcNow - $started).TotalSeconds, 1) }
        throw "$Name exceeded its $TimeoutSeconds second deadline"
    }
    $process.WaitForExit()
    [IO.File]::WriteAllText($stdout, $stdoutTask.Result, [Text.UTF8Encoding]::new($false))
    [IO.File]::WriteAllText($stderr, $stderrTask.Result, [Text.UTF8Encoding]::new($false))
    $exitCode = $process.ExitCode
    Assert-NoChildResources $process.Id
    $result = [ordered]@{
        name = $Name
        started_at = $started.ToString("o")
        ended_at = [DateTime]::UtcNow.ToString("o")
        exit_code = $exitCode
        stdout = $stdout
        stderr = $stderr
    }
    Write-Event "command_finished" @{ name = $Name; exit_code = $exitCode }
    if ($exitCode -ne 0) { throw "$Name failed with exit code $exitCode; see $stderr" }
    return $result
}

function Capture-Resources([string]$Label, [string]$CheckpointDir) {
    $path = Join-Path $CheckpointDir "$Label-resources.json"
    $log = Join-Path $CheckpointDir "$Label-resources.log"
    & $resourceScript -Label $Label -OutputPath $path -SampleSeconds 2 *> $log
    if ($LASTEXITCODE -ne 0) { throw "resource snapshot failed: $Label" }
    $snapshot = Get-Content -Raw $path | ConvertFrom-Json
    if ($null -eq $snapshot.memory.available_mb -or $snapshot.memory.available_mb -lt 4096) {
        throw "host available memory is below 4 GiB or unavailable"
    }
    $diskTotal = [double]$snapshot.disk.free_gb + [double]$snapshot.disk.used_gb
    if ($diskTotal -le 0 -or ([double]$snapshot.disk.used_gb / $diskTotal) -ge 0.90) {
        throw "host disk usage reached 90% or is unavailable"
    }
    return $snapshot
}

function Assert-Candidate([Collections.IDictionary]$Expected) {
    $actual = Get-CandidateIdentity
    if ($actual.commit -ne $Expected.commit -or $actual.image_id -ne $Expected.image_id) {
        throw "release candidate changed during soak"
    }
}

function Invoke-MultiHost {
    param([int]$Index, [string]$CheckpointDir, [bool]$Full, [int]$SampleSeconds)
    $args = @("-ReportDir", (Join-Path $CheckpointDir "multihost"), "-BuildMode", "Never")
    $name = "multihost-full"
    $timeout = 1200
    if (-not $Full) {
        $name = "multihost-stability"
        $args += @("-StabilityOnly", "-StabilitySeconds", "$SampleSeconds")
        $timeout = $SampleSeconds + 600
    }
    return Invoke-BoundedScript -Name $name -Script (Join-Path $root "tests/ci/multihost-gate.ps1") `
        -Arguments $args -TimeoutSeconds $timeout -LogDir (Join-Path $CheckpointDir "commands")
}

function Invoke-SelectedScenario {
    param([string]$Suite, [string]$Scenario, [string]$CheckpointDir)
    $scenarioReport = Join-Path $CheckpointDir ("{0}-{1}" -f $Suite, $Scenario.ToLower())
    return Invoke-BoundedScript -Name ("{0}-{1}" -f $Suite, $Scenario.ToLower()) `
        -Script (Join-Path $root "tests/run.ps1") `
        -Arguments @("-Suite", $Suite, "-Scenario", $Scenario, "-ReportDir", $scenarioReport) `
        -TimeoutSeconds 600 -LogDir (Join-Path $CheckpointDir "commands")
}

function Invoke-FullCheckpoint([int]$Index, [string]$CheckpointDir) {
    $commands = [Collections.Generic.List[object]]::new()
    $commands.Add((Invoke-MultiHost -Index $Index -CheckpointDir $CheckpointDir -Full $true -SampleSeconds 0))
    foreach ($selection in @(
        @{ Suite = "integration"; Scenario = "DAI-003" },
        @{ Suite = "integration"; Scenario = "DAI-004" },
        @{ Suite = "e2e"; Scenario = "WKE-001" },
        @{ Suite = "e2e"; Scenario = "E2E-NPI-001" }
    )) {
        $commands.Add((Invoke-SelectedScenario -Suite $selection.Suite -Scenario $selection.Scenario -CheckpointDir $CheckpointDir))
    }
    $commands.Add((Invoke-BoundedScript -Name "workload-docker" `
        -Script (Join-Path $root "tests/run-workload-docker.ps1") `
        -Arguments @("-ReportDir", (Join-Path $CheckpointDir "workload-docker")) `
        -TimeoutSeconds 1200 -LogDir (Join-Path $CheckpointDir "commands")))
    return $commands
}

function Wait-Until([DateTime]$Target) {
    while ([DateTime]::UtcNow -lt $Target) {
        $remaining = [math]::Ceiling(($Target - [DateTime]::UtcNow).TotalSeconds)
        Start-Sleep -Seconds ([math]::Min(30, $remaining))
    }
}

New-Item -ItemType Directory -Force -Path $reportPath | Out-Null
$candidate = Get-CandidateIdentity
$plan = [ordered]@{
    scenario_id = "SOAK-001"
    duration_minutes = $DurationMinutes
    checkpoint_minutes = $CheckpointMinutes
    checkpoints = $checkpointCount
    full_cycle_every = $FullCycleEvery
    stability_seconds = $StabilitySeconds
    candidate = $candidate
}
$plan | ConvertTo-Json -Depth 10 | Set-Content -Encoding utf8 (Join-Path $reportPath "plan.json")
if ($PlanOnly) {
    Write-Host ("SOAK-001 plan valid: {0} checkpoints, candidate {1}, image {2}" -f $checkpointCount, $candidate.commit, $candidate.image_id)
    exit 0
}

if (Test-Path $statePath) { throw "soak report directory already contains state: $statePath" }
if (@(& git status --porcelain).Count -gt 0) { throw "soak requires a clean candidate worktree" }
$startedAt = [DateTime]::UtcNow
$deadline = $startedAt.AddMinutes($DurationMinutes)
$script:state = [ordered]@{
    scenario_id = "SOAK-001"
    status = "running"
    started_at = $startedAt.ToString("o")
    deadline = $deadline.ToString("o")
    candidate = $candidate
    checkpoints = [Collections.Generic.List[object]]::new()
}
Save-State
Write-Event "soak_started" @{ deadline = $deadline.ToString("o"); commit = $candidate.commit; image_id = $candidate.image_id }

try {
    for ($index = 0; $index -lt $checkpointCount; $index++) {
        $scheduled = $startedAt.AddMinutes($index * $CheckpointMinutes)
        Wait-Until $scheduled
        Assert-Candidate $candidate
        $checkpointDir = Join-Path $reportPath ("checkpoint-{0:D3}" -f $index)
        New-Item -ItemType Directory -Force -Path $checkpointDir | Out-Null
        $checkpoint = [ordered]@{
            index = $index
            scheduled_at = $scheduled.ToString("o")
            started_at = [DateTime]::UtcNow.ToString("o")
            full = ($index % $FullCycleEvery) -eq 0
            status = "running"
            commands = [Collections.Generic.List[object]]::new()
        }
        $script:state.active_checkpoint = $checkpoint
        Save-State
        Write-Event "checkpoint_started" @{ index = $index; full = $checkpoint.full }
        $checkpoint.before_resources = Capture-Resources "checkpoint-$index-before" $checkpointDir
        if ($checkpoint.full) {
            foreach ($command in @(Invoke-FullCheckpoint $index $checkpointDir)) { $checkpoint.commands.Add($command) }
        }
        $nextScheduled = $startedAt.AddMinutes(($index + 1) * $CheckpointMinutes)
        $availableSeconds = [math]::Floor(($nextScheduled - [DateTime]::UtcNow).TotalSeconds) - 120
        $sampleSeconds = [math]::Min($StabilitySeconds, $availableSeconds)
        if ($sampleSeconds -ge 180) {
            $checkpoint.commands.Add((Invoke-MultiHost -Index $index -CheckpointDir (Join-Path $checkpointDir "stability") -Full $false -SampleSeconds $sampleSeconds))
        }
        Assert-Candidate $candidate
        $checkpoint.after_resources = Capture-Resources "checkpoint-$index-after" $checkpointDir
        $checkpoint.status = "passed"
        $checkpoint.ended_at = [DateTime]::UtcNow.ToString("o")
        $script:state.checkpoints.Add($checkpoint)
        $script:state.Remove("active_checkpoint")
        Save-State
        Write-Event "checkpoint_passed" @{ index = $index; commands = $checkpoint.commands.Count }
    }
    Wait-Until $deadline
    Assert-Candidate $candidate
    $script:state.status = "passed"
    $script:state.ended_at = [DateTime]::UtcNow.ToString("o")
    Save-State
    Write-Event "soak_passed" @{ checkpoints = $script:state.checkpoints.Count }
} catch {
    $script:state.status = "failed"
    $script:state.error = $_.Exception.Message
    $script:state.ended_at = [DateTime]::UtcNow.ToString("o")
    Save-State
    Write-Event "soak_failed" @{ error = $_.Exception.Message }
    throw
}

Write-Host "SOAK-001 passed; report: $reportPath"
