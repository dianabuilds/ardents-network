[CmdletBinding(DefaultParameterSetName = 'Campaign')]
param(
    [Parameter(Mandatory = $true, ParameterSetName = 'Campaign')][string]$SourceRevision,
    [Parameter(Mandatory = $true, ParameterSetName = 'Campaign')][string]$CandidateRepository,
    [Parameter(Mandatory = $true, ParameterSetName = 'Campaign')][string]$ReleaseTag,
    [Parameter(Mandatory = $true, ParameterSetName = 'Campaign')][string]$CandidateArchive,
    [Parameter(Mandatory = $true, ParameterSetName = 'Campaign')][string]$ArchiveSHA256,
    [Parameter(Mandatory = $true, ParameterSetName = 'Campaign')][string]$ManifestPin,
    [Parameter(Mandatory = $true, ParameterSetName = 'Campaign')][string]$EndpointSHA256,
    [Parameter(Mandatory = $true, ParameterSetName = 'Campaign')][string]$ControlSHA256,
    [Parameter(Mandatory = $true, ParameterSetName = 'Campaign')][string]$Cohort,
    [Parameter(Mandatory = $true, ParameterSetName = 'Campaign')][string]$At,
    [Parameter(Mandatory = $true, ParameterSetName = 'Campaign')][string]$VPS,
    [Parameter(Mandatory = $true, ParameterSetName = 'Campaign')][string]$SSHKey,
    [Parameter(Mandatory = $true, ParameterSetName = 'Campaign')][string]$User,
    [Parameter(Mandatory = $true, ParameterSetName = 'Campaign')][ValidateRange(1024, 65456)][int]$BasePort,
    [Parameter(Mandatory = $true, ParameterSetName = 'Campaign')][string]$RemoteImageID,
    [Parameter(Mandatory = $true, ParameterSetName = 'Campaign')][string]$EvidenceOutput,
    [Parameter(Mandatory = $true, ParameterSetName = 'Campaign')][ValidateSet('-timeout=125m')][string]$CampaignTimeout,
    [Parameter(Mandatory = $true, ParameterSetName = 'SelfTest')][switch]$SelfTest
)

$ErrorActionPreference = 'Stop'
trap {
    [Console]::Error.WriteLine($_.Exception.Message)
    exit 1
}
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
$harnessRepository = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..\..\..')).Path
$repository = $harnessRepository
$candidateRepository = $harnessRepository
if ($PSCmdlet.ParameterSetName -eq 'Campaign') {
    if (-not [IO.Path]::IsPathRooted($CandidateRepository) -or -not (Test-Path -LiteralPath $CandidateRepository -PathType Container)) {
        throw 'CandidateRepository must be one existing absolute committed worktree.'
    }
    $candidateRepository = (Resolve-Path -LiteralPath $CandidateRepository).Path
}
$script:candidateRepository = $candidateRepository
$remoteImage = 'golang:1.26.6'
$remoteMemoryLimit = 1073741824L
$remotePIDLimit = 128L
$remoteNanoCPUs = 1000000000L
$campaignCleanupReserveMilliseconds = 30000L
$processStopBudgetMilliseconds = 10000L
$ubuntuHeader = @(
    'epoch_ms', 'container_name', 'container_id', 'image_id', 'running', 'restarting', 'oom_killed', 'restart_count',
    'nano_cpus', 'memory_limit', 'pids_limit', 'network_mode', 'restart_policy', 'cgroup_path',
    'cpu_usage_usec', 'cpu_nr_throttled', 'cpu_throttled_usec', 'memory_current', 'memory_max',
    'memory_events_oom', 'memory_events_oom_kill', 'memory_events_max', 'pids_current', 'pids_max',
    'pids_peak', 'pids_events_max', 'process_count', 'fd_count', 'host_network_rx_bytes',
    'host_network_tx_bytes'
)

function Write-Utf8([string]$Path, [string]$Value) {
    [System.IO.File]::WriteAllText($Path, $Value, $script:utf8NoBom)
}

function Write-Json([string]$Path, [object]$Value) {
    Write-Utf8 $Path (($Value | ConvertTo-Json -Depth 12) + "`n")
}

function Add-JsonLine([string]$Path, [object]$Value) {
    [System.IO.File]::AppendAllText($Path, (($Value | ConvertTo-Json -Depth 6 -Compress) + "`n"), $script:utf8NoBom)
}

function Assert-LowerSHA256([string]$Name, [string]$Value) {
    if ($Value -cnotmatch '^[0-9a-f]{64}$') {
        throw "$Name must be exactly 64 lowercase hexadecimal characters."
    }
}

function Test-PathWithin([string]$Child, [string]$Parent) {
    $parentPrefix = $Parent.TrimEnd([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar
    return $Child.Equals($Parent, [StringComparison]::OrdinalIgnoreCase) -or
        $Child.StartsWith($parentPrefix, [StringComparison]::OrdinalIgnoreCase)
}

function ConvertTo-NativeArgument([string]$Value) {
    if ($Value.IndexOf([char]0) -ge 0 -or $Value.Contains("`r") -or $Value.Contains("`n")) {
        throw 'native argument contains an unsupported control character.'
    }
    if ($Value.Length -ne 0 -and $Value -cnotmatch '[\s"]') { return $Value }

    # Quote one argv value according to the CommandLineToArgvW/MS C runtime
    # rules used by the Windows programs in this runner. Backslashes are only
    # special before a quote and immediately before the closing quote.
    $builder = New-Object System.Text.StringBuilder
    $quote = [char]34
    $backslash = [char]92
    [void]$builder.Append($quote)
    $pendingBackslashes = 0
    foreach ($character in $Value.ToCharArray()) {
        if ($character -eq $backslash) {
            $pendingBackslashes++
            continue
        }
        if ($character -eq $quote) {
            [void]$builder.Append($backslash, (2 * $pendingBackslashes) + 1)
            [void]$builder.Append($quote)
        }
        else {
            if ($pendingBackslashes -ne 0) { [void]$builder.Append($backslash, $pendingBackslashes) }
            [void]$builder.Append($character)
        }
        $pendingBackslashes = 0
    }
    if ($pendingBackslashes -ne 0) { [void]$builder.Append($backslash, 2 * $pendingBackslashes) }
    [void]$builder.Append($quote)
    return $builder.ToString()
}

function Join-NativeArguments([string[]]$Arguments) {
    return (($Arguments | ForEach-Object { ConvertTo-NativeArgument $_ }) -join ' ')
}

function ConvertTo-ShellLiteral([string]$Value) {
    if ($Value.Contains([char]0) -or $Value.Contains("`r") -or $Value.Contains("`n")) {
        throw 'remote shell literal contains a control character.'
    }
    $apostrophe = [string][char]39
    $doubleQuote = [string][char]34
    return $apostrophe + $Value.Replace($apostrophe, ($apostrophe + $doubleQuote + $apostrophe + $doubleQuote + $apostrophe)) + $apostrophe
}

function Find-OpenSSH {
    foreach ($root in @(
        (Join-Path $env:WINDIR 'System32\OpenSSH'),
        (Join-Path $env:WINDIR 'Sysnative\OpenSSH')
    )) {
        $ssh = Join-Path $root 'ssh.exe'
        if ([IO.File]::Exists($ssh)) { return $ssh }
    }
    $command = Get-Command ssh.exe -CommandType Application -ErrorAction SilentlyContinue
    if ($null -ne $command) { return $command.Source }
    throw 'Windows OpenSSH ssh.exe is required.'
}

function Invoke-NativeCaptured(
    [string]$Executable,
    [string[]]$Arguments,
    [string]$StdoutPath,
    [string]$StderrPath,
    [string]$WorkingDirectory = $script:repository,
    [string]$StdinPath = ''
) {
    # Start-Process connects native file handles directly. This avoids Windows
    # PowerShell 5.1 rewriting ordinary native stderr as NativeCommandError.
    $start = @{
        FilePath = $Executable
        ArgumentList = Join-NativeArguments $Arguments
        WorkingDirectory = $WorkingDirectory
        RedirectStandardOutput = $StdoutPath
        RedirectStandardError = $StderrPath
        WindowStyle = 'Hidden'
        Wait = $true
        PassThru = $true
    }
    if (-not [string]::IsNullOrEmpty($StdinPath)) { $start.RedirectStandardInput = $StdinPath }
    $process = Start-Process @start
    $code = $process.ExitCode
    foreach ($path in @($StdoutPath, $StderrPath)) {
        # Preserve the native byte stream exactly; create only a missing empty
        # stream so every invocation still has both evidence files.
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { Write-Utf8 $path '' }
    }
    return $code
}

function Read-ReleaseDescriptor([string]$Path) {
    $fields = @{}
    foreach ($line in [IO.File]::ReadAllLines($Path)) {
        if ($line -notmatch '^([a-z_]+)=(.+)$') { throw 'RELEASE contains a malformed or empty descriptor line.' }
        if ($fields.ContainsKey($Matches[1])) { throw "RELEASE duplicates field $($Matches[1])." }
        $fields[$Matches[1]] = $Matches[2]
    }
    return $fields
}

function Get-A11Attempts([int]$FirstPort) {
    return @(
        [pscustomobject]@{ Cell = 'normal-soak'; Attempt = 'primary'; Test = 'TestH48A11MultiHostPacedPublisherToUserSoak'; Remote = $true; Port = $FirstPort; GoTimeout = '40m'; DeadlineSeconds = 2700 },
        [pscustomobject]@{ Cell = 'application-loss'; Attempt = 'primary'; Test = 'TestH48A11MultiHostPublisherApplicationLossAfterWarmup'; Remote = $true; Port = $FirstPort + 8; GoTimeout = '8m'; DeadlineSeconds = 600 },
        [pscustomobject]@{ Cell = 'application-loss'; Attempt = 'canary'; Test = 'TestH48A11MultiHostTenCycleCanary'; Remote = $true; Port = $FirstPort + 16; GoTimeout = '5m'; DeadlineSeconds = 360 },
        [pscustomobject]@{ Cell = 'endpoint-loss'; Attempt = 'primary'; Test = 'TestH48A11MultiHostPublisherEndpointLossAfterWarmup'; Remote = $true; Port = $FirstPort + 24; GoTimeout = '8m'; DeadlineSeconds = 600 },
        [pscustomobject]@{ Cell = 'endpoint-loss'; Attempt = 'canary'; Test = 'TestH48A11MultiHostTenCycleCanary'; Remote = $true; Port = $FirstPort + 32; GoTimeout = '5m'; DeadlineSeconds = 360 },
        [pscustomobject]@{ Cell = 'carrier-loss'; Attempt = 'primary'; Test = 'TestH48A11MultiHostCarrierLossAfterWarmup'; Remote = $true; Port = $FirstPort + 40; GoTimeout = '8m'; DeadlineSeconds = 600 },
        [pscustomobject]@{ Cell = 'carrier-loss'; Attempt = 'canary'; Test = 'TestH48A11MultiHostTenCycleCanary'; Remote = $true; Port = $FirstPort + 48; GoTimeout = '5m'; DeadlineSeconds = 360 },
        [pscustomobject]@{ Cell = 'product-node-loss'; Attempt = 'primary'; Test = 'TestH48A11MultiHostProductNodeLossAfterWarmup'; Remote = $true; Port = $FirstPort + 56; GoTimeout = '8m'; DeadlineSeconds = 600 },
        [pscustomobject]@{ Cell = 'product-node-loss'; Attempt = 'canary'; Test = 'TestH48A11MultiHostTenCycleCanary'; Remote = $true; Port = $FirstPort + 64; GoTimeout = '5m'; DeadlineSeconds = 360 },
        [pscustomobject]@{ Cell = 'expiry-boundaries'; Attempt = 'primary'; Test = 'TestH48A11DeterministicExpiryBoundaries'; Remote = $false; Port = $null; GoTimeout = '5m'; DeadlineSeconds = 360 }
    )
}

function Get-BoundedAttemptSeconds(
    [int]$RequestedSeconds,
    [long]$CampaignElapsedMilliseconds,
    [long]$CampaignLimitMilliseconds,
    [long]$CleanupReserveMilliseconds
) {
    $remainingMilliseconds = $CampaignLimitMilliseconds - $CampaignElapsedMilliseconds - $CleanupReserveMilliseconds
    if ($remainingMilliseconds -le 0) { return 0 }
    $remainingWholeSeconds = [long][Math]::Floor($remainingMilliseconds / 1000.0)
    if ($remainingWholeSeconds -lt 1) { return 0 }
    return [int][Math]::Min([long]$RequestedSeconds, $remainingWholeSeconds)
}

function Get-BoundedCleanupDeadlineMilliseconds(
    [long]$CampaignElapsedMilliseconds,
    [long]$CampaignLimitMilliseconds,
    [long]$RequestedBudgetMilliseconds
) {
    if ($CampaignElapsedMilliseconds -ge $CampaignLimitMilliseconds) { return $CampaignLimitMilliseconds }
    return [Math]::Min($CampaignLimitMilliseconds, $CampaignElapsedMilliseconds + $RequestedBudgetMilliseconds)
}

function Test-ProcessStopReceipt([object]$Receipt) {
    $failures = New-Object System.Collections.Generic.List[string]
    try {
        $rootPID = [int]$Receipt.requested_root_pid
        $ownedPIDs = @($Receipt.owned_pids | ForEach-Object { [int]$_ })
        $stoppedPIDs = @($Receipt.stopped_pids | ForEach-Object { [int]$_ })
        $survivorPIDs = @($Receipt.survivor_pids | ForEach-Object { [int]$_ })
        $stopErrors = @($Receipt.stop_errors | ForEach-Object { [string]$_ })
        if ([string]$Receipt.schema -cne 'ardents-h4-8-a11-process-stop-v1' -or $rootPID -le 1) {
            $failures.Add('process-stop receipt schema/root PID is invalid.')
        }
        if ($ownedPIDs.Count -eq 0 -or $ownedPIDs -notcontains $rootPID -or
            @($ownedPIDs | Sort-Object -Unique).Count -ne $ownedPIDs.Count) {
            $failures.Add('process-stop receipt owned PID inventory is incomplete or ambiguous.')
        }
        if (@($stoppedPIDs | Sort-Object -Unique).Count -ne $stoppedPIDs.Count -or
            @($stoppedPIDs | Where-Object { $ownedPIDs -notcontains $_ }).Count -ne 0 -or
            @($ownedPIDs | Where-Object { $stoppedPIDs -notcontains $_ }).Count -ne 0) {
            $failures.Add('process-stop receipt stopped PID inventory is incomplete or ambiguous.')
        }
        if ($survivorPIDs.Count -ne 0) { $failures.Add('process-stop receipt retains surviving owned PIDs.') }
        if ($stopErrors.Count -ne 0) { $failures.Add('process-stop receipt contains stop errors.') }
        if ([bool]$Receipt.deadline_met -ne $true) { $failures.Add('process-stop receipt exceeded its bounded deadline.') }
        if ([bool]$Receipt.passed -ne $true) { $failures.Add('process-stop receipt disposition is not passing.') }
    }
    catch { $failures.Add("process-stop receipt is malformed: $($_.Exception.Message)") }
    return [pscustomobject]@{ Passed = $failures.Count -eq 0; Failures = @($failures) }
}

function Test-A11BoundedProcessSource([string]$Source) {
    $failures = New-Object System.Collections.Generic.List[string]
    foreach ($required in @(
        '$goStopResult = Stop-OwnedProcessTree $goProcess.Id',
        '$goCompletion = Wait-ProcessBounded $goProcess $CampaignClock $goCompletionDeadline',
        '$observerCompletion = Wait-ProcessBounded $observer.Process $CampaignClock $observerNaturalDeadline',
        '$observerStopResult = Stop-OwnedProcessTree $observer.Process.Id'
    )) {
        if ($Source.IndexOf($required, [StringComparison]::Ordinal) -lt 0) {
            $failures.Add("A11 runner source lacks bounded process seam: $required")
        }
    }
    if ($Source -match '\.WaitForExit\s*\(\s*\)') {
        $failures.Add('A11 runner source contains an unbounded process wait.')
    }
    if ($Source.IndexOf('Stop-Process -Id $observer.Process.Id -Force -ErrorAction SilentlyContinue', [StringComparison]::Ordinal) -ge 0) {
        $failures.Add('A11 runner source contains an error-silenced observer stop.')
    }
    return [pscustomobject]@{ Passed = $failures.Count -eq 0; Failures = @($failures) }
}

function Test-A11EntrypointCaptureSource([string]$Source) {
    $failures = New-Object System.Collections.Generic.List[string]
    foreach ($required in @(
        "'entrypoint.stdout.log'",
        "'entrypoint.stderr.log'",
        "'entrypoint.exitcode'",
        "'entrypoint-status.json'",
        'accepted_campaign_receipt',
        'Assert-CandidateSource $CandidateRepository $SourceRevision $ReleaseTag',
        'CandidateRepository HEAD does not equal SourceRevision.',
        'source_revision = $SourceRevision',
        'Write-EvidenceInventory $evidencePath',
        '$child.WaitForExit(7530000)',
        'exit 1'
    )) {
        if ($Source.IndexOf($required, [StringComparison]::Ordinal) -lt 0) {
            $failures.Add("A11 entrypoint capture source lacks exact seam: $required")
        }
    }
    return [pscustomobject]@{ Passed = $failures.Count -eq 0; Failures = @($failures) }
}

function ConvertTo-Int64([string]$Value, [string]$Name, [System.Collections.Generic.List[string]]$Failures) {
    $parsed = 0L
    if (-not [long]::TryParse($Value, [Globalization.NumberStyles]::None, [Globalization.CultureInfo]::InvariantCulture, [ref]$parsed)) {
        $Failures.Add("$Name is not a non-negative decimal integer.")
        return 0L
    }
    return $parsed
}

function Test-UbuntuResourceSeries([string]$Path, [string]$Container, [string]$ImageID) {
    $failures = New-Object System.Collections.Generic.List[string]
    $samples = 0
    $maximumMemory = 0L
    $maximumPIDs = 0L
    $lastTimestamp = $null
    $firstContainerID = $null
    $firstCgroup = $null
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        $failures.Add('Ubuntu resource series is absent.')
        return [pscustomobject]@{ Passed = $false; Samples = 0; MaximumMemory = 0; MaximumPIDs = 0; Failures = @($failures) }
    }
    $lines = @([IO.File]::ReadAllLines($Path))
    if ($lines.Count -lt 2 -or $lines[0] -cne ($script:ubuntuHeader -join "`t")) {
        $failures.Add('Ubuntu resource series header or sample set is incomplete.')
        return [pscustomobject]@{ Passed = $false; Samples = 0; MaximumMemory = 0; MaximumPIDs = 0; Failures = @($failures) }
    }
    for ($index = 1; $index -lt $lines.Count; $index++) {
        $values = $lines[$index].Split("`t")
        if ($values.Count -ne $script:ubuntuHeader.Count) {
            $failures.Add("Ubuntu resource row $index has an ambiguous field count.")
            continue
        }
        $row = @{}
        for ($field = 0; $field -lt $values.Count; $field++) { $row[$script:ubuntuHeader[$field]] = $values[$field] }
        $samples++
        $timestamp = ConvertTo-Int64 $row.epoch_ms "Ubuntu row $index epoch_ms" $failures
        if ($null -ne $lastTimestamp) {
            $gap = $timestamp - $lastTimestamp
            if ($gap -le 0 -or $gap -gt 2000) { $failures.Add("Ubuntu observer gap is $gap ms after row $($index - 1).") }
        }
        $lastTimestamp = $timestamp
        if ($row.container_name -cne $Container -or $row.container_id -cnotmatch '^[0-9a-f]{64}$' -or $row.image_id -cne $ImageID -or
            $row.running -cne 'true' -or $row.restarting -cne 'false' -or $row.oom_killed -cne 'false' -or
            $row.network_mode -cne 'host' -or $row.restart_policy -cne 'no') {
            $failures.Add("Ubuntu resource row $index has a Docker identity/state mismatch.")
        }
        if ($null -eq $firstContainerID) { $firstContainerID = $row.container_id } elseif ($row.container_id -cne $firstContainerID) {
            $failures.Add("Ubuntu resource row $index changed container identity.")
        }
        if ($row.cgroup_path -cnotmatch '^/[A-Za-z0-9_./:-]+$') { $failures.Add("Ubuntu resource row $index has an invalid cgroup path.") }
        if ($null -eq $firstCgroup) { $firstCgroup = $row.cgroup_path } elseif ($row.cgroup_path -cne $firstCgroup) {
            $failures.Add("Ubuntu resource row $index changed cgroup identity.")
        }
        foreach ($constant in @(
            @{ Name = 'restart_count'; Expected = 0L },
            @{ Name = 'nano_cpus'; Expected = $script:remoteNanoCPUs },
            @{ Name = 'memory_limit'; Expected = $script:remoteMemoryLimit },
            @{ Name = 'pids_limit'; Expected = $script:remotePIDLimit },
            @{ Name = 'memory_max'; Expected = $script:remoteMemoryLimit },
            @{ Name = 'pids_max'; Expected = $script:remotePIDLimit },
            @{ Name = 'memory_events_oom'; Expected = 0L },
            @{ Name = 'memory_events_oom_kill'; Expected = 0L },
            @{ Name = 'memory_events_max'; Expected = 0L },
            @{ Name = 'pids_events_max'; Expected = 0L }
        )) {
            $actual = ConvertTo-Int64 $row[$constant.Name] "Ubuntu row $index $($constant.Name)" $failures
            if ($actual -ne $constant.Expected) { $failures.Add("Ubuntu row $index $($constant.Name) is $actual, expected $($constant.Expected).") }
        }
        $memory = ConvertTo-Int64 $row.memory_current "Ubuntu row $index memory_current" $failures
        $pids = ConvertTo-Int64 $row.pids_current "Ubuntu row $index pids_current" $failures
        $processes = ConvertTo-Int64 $row.process_count "Ubuntu row $index process_count" $failures
        [void](ConvertTo-Int64 $row.fd_count "Ubuntu row $index fd_count" $failures)
        foreach ($metric in @('cpu_usage_usec', 'cpu_nr_throttled', 'cpu_throttled_usec', 'host_network_rx_bytes', 'host_network_tx_bytes')) {
            [void](ConvertTo-Int64 $row[$metric] "Ubuntu row $index $metric" $failures)
        }
        if ($memory -gt $script:remoteMemoryLimit) { $failures.Add("Ubuntu row $index exceeds the 1 GiB memory ceiling.") }
        if ($pids -gt $script:remotePIDLimit -or $processes -gt $script:remotePIDLimit) { $failures.Add("Ubuntu row $index exceeds the 128 PID ceiling.") }
        if ($pids -lt 1 -or $processes -lt 1) { $failures.Add("Ubuntu row $index observed no live container process.") }
        if ($row.pids_peak -cne 'unavailable') {
            $peak = ConvertTo-Int64 $row.pids_peak "Ubuntu row $index pids_peak" $failures
            if ($peak -gt $script:remotePIDLimit) { $failures.Add("Ubuntu row $index records a PID peak above 128.") }
        }
        $maximumMemory = [Math]::Max($maximumMemory, $memory)
        $maximumPIDs = [Math]::Max($maximumPIDs, $pids)
    }
    if ($samples -lt 2) { $failures.Add('Ubuntu observer retained fewer than two live samples.') }
    return [pscustomobject]@{ Passed = $failures.Count -eq 0; Samples = $samples; MaximumMemory = $maximumMemory; MaximumPIDs = $maximumPIDs; Failures = @($failures) }
}

function Test-WindowsResourceSeries([string]$Path, [int]$ExpectedRootPID, [bool]$RequireTwoSamples) {
    $failures = New-Object System.Collections.Generic.List[string]
    $samples = 0
    $lastMonotonic = $null
    $observedPIDs = New-Object 'System.Collections.Generic.HashSet[int]'
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        $failures.Add('Windows resource series is absent.')
        return [pscustomobject]@{ Passed = $false; Samples = 0; ObservedPIDs = @(); Failures = @($failures) }
    }
    foreach ($line in [IO.File]::ReadAllLines($Path)) {
        if ([string]::IsNullOrWhiteSpace($line)) { continue }
        try { $row = $line | ConvertFrom-Json }
        catch { $failures.Add('Windows resource series contains invalid JSON.'); continue }
        $samples++
        if ($null -ne $row.error) { $failures.Add("Windows sampling error: $($row.error)") }
        $monotonic = [long]$row.monotonic_ms
        if ($null -ne $lastMonotonic) {
            $gap = $monotonic - $lastMonotonic
            if ($gap -le 0 -or $gap -gt 2000) { $failures.Add("Windows observer gap is $gap ms.") }
        }
        $lastMonotonic = $monotonic
        $hasRoot = $false
        foreach ($process in @($row.processes)) {
            [void]$observedPIDs.Add([int]$process.pid)
            if ([int]$process.pid -eq $ExpectedRootPID) { $hasRoot = $true }
        }
        if (-not $hasRoot) { $failures.Add("Windows sample $samples omitted the qualification root PID.") }
    }
    $minimum = $(if ($RequireTwoSamples) { 2 } else { 1 })
    if ($samples -lt $minimum) { $failures.Add("Windows observer retained fewer than $minimum samples.") }
    return [pscustomobject]@{ Passed = $failures.Count -eq 0; Samples = $samples; ObservedPIDs = @($observedPIDs); Failures = @($failures) }
}

function Get-ExpiryMarkers {
    return @(
        'A11_EXPIRY_BOUNDARY owner=network-state before=2030-01-02T03:04:05Z at=2030-01-02T03:04:06Z',
        'A11_EXPIRY_BOUNDARY owner=alpha-control before=2030-01-02T03:04:05Z at=2030-01-02T03:04:06Z',
        'A11_EXPIRY_BOUNDARY owner=service-credential before=2030-01-02T03:04:05Z at=2030-01-02T03:04:06Z',
        'A11_EXPIRY_RESULT schema=ardents-h4-8-a11-expiry-v1 owners=3 before=2030-01-02T03:04:05Z at=2030-01-02T03:04:06Z status=accepted'
    )
}

function Test-ExpiryOutput([string]$Output) {
    $failures = New-Object System.Collections.Generic.List[string]
    $previousMarker = -1
    foreach ($marker in @(Get-ExpiryMarkers)) {
        $occurrences = [regex]::Matches($Output, [regex]::Escape($marker)).Count
        $position = $Output.IndexOf($marker, [StringComparison]::Ordinal)
        if ($occurrences -ne 1 -or $position -le $previousMarker) {
            $failures.Add("deterministic expiry stdout lacks one ordered exact marker: $marker")
        }
        $previousMarker = $position
    }
    return [pscustomobject]@{ Passed = $failures.Count -eq 0; Markers = @(Get-ExpiryMarkers); Failures = @($failures) }
}

function ConvertFrom-ExactUTCSecond([string]$Value) {
    if ($Value -cnotmatch '^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$') { return $null }
    $parsed = [DateTimeOffset]::MinValue
    if (-not [DateTimeOffset]::TryParseExact($Value, 'yyyy-MM-ddTHH:mm:ssZ', [Globalization.CultureInfo]::InvariantCulture,
            [Globalization.DateTimeStyles]::AssumeUniversal -bor [Globalization.DateTimeStyles]::AdjustToUniversal, [ref]$parsed)) {
        return $null
    }
    return $parsed
}

function Test-CandidateExpiryReport(
    [string]$Output,
    [string]$BundleRoot,
    [string]$ReferenceAt,
    [string]$ExpectedCohort,
    [string]$ExpectedRelease,
    [string]$ExpectedEndpointSHA256,
    [string]$ExpectedControlSHA256
) {
    $failures = New-Object System.Collections.Generic.List[string]
    $prefix = 'A11_EXPIRY_CANDIDATE_REPORT '
    $occurrences = [regex]::Matches($Output, [regex]::Escape($prefix)).Count
    $matching = @($Output -split "`r?`n" | Where-Object { $_.IndexOf($prefix, [StringComparison]::Ordinal) -ge 0 })
    if ($occurrences -ne 1 -or $matching.Count -ne 1) {
        $failures.Add('expiry stdout must contain exactly one candidate report marker.')
        return [pscustomobject]@{ Passed = $false; Marker = $null; Report = $null; Failures = @($failures) }
    }
    $markerOffset = $matching[0].IndexOf($prefix, [StringComparison]::Ordinal)
    $marker = $matching[0].Substring($markerOffset)
    $json = $marker.Substring($prefix.Length)
    if ($json -cnotmatch '^\{.*\}$') {
        $failures.Add('expiry candidate report marker must end with only one compact JSON object.')
        return [pscustomobject]@{ Passed = $false; Marker = $marker; Report = $null; Failures = @($failures) }
    }
    try { $report = $json | ConvertFrom-Json }
    catch {
        $failures.Add('expiry candidate report marker does not contain valid compact JSON.')
        return [pscustomobject]@{ Passed = $false; Marker = $marker; Report = $null; Failures = @($failures) }
    }
    try {
        $requireProperties = {
            param([object]$Value, [string[]]$Expected, [string]$Owner)
            if ($null -eq $Value) {
                $failures.Add("expiry candidate $Owner is absent.")
                return
            }
            $actual = @($Value.PSObject.Properties | ForEach-Object { $_.Name } | Sort-Object)
            $wanted = @($Expected | Sort-Object)
            if (($actual -join ',') -cne ($wanted -join ',')) {
                $failures.Add("expiry candidate $Owner properties differ from the exact v2 schema.")
            }
        }
        $requireBoolean = {
            param([object]$Value, [bool]$Expected, [string]$Owner)
            if ($Value -isnot [bool] -or [bool]$Value -ne $Expected) {
                $failures.Add("expiry candidate $Owner is not exact boolean $Expected.")
            }
        }
        $requirePositiveInteger = {
            param([object]$Value, [string]$Owner)
            if (($Value -isnot [int]) -and ($Value -isnot [long])) {
                $failures.Add("expiry candidate $Owner is not an integer.")
                return 0L
            }
            $parsed = ConvertTo-Int64 ([string]$Value) "expiry candidate $Owner" $failures
            if ($parsed -lt 1) { $failures.Add("expiry candidate $Owner must be positive.") }
            return $parsed
        }
        $requireClassOutcomes = {
            param([object]$Value, [string[]]$ExpectedOutcomes, [string]$Owner)
            $items = @($Value)
            $classes = @('release', 'network', 'compatibility')
            if ($items.Count -ne 3) {
                $failures.Add("expiry candidate $Owner must contain exactly three ordered component outcomes.")
                return
            }
            for ($index = 0; $index -lt 3; $index++) {
                & $requireProperties $items[$index] @('class', 'outcome') "$Owner component $index"
                if ([string]$items[$index].class -cne $classes[$index] -or
                    [string]$items[$index].outcome -cne $ExpectedOutcomes[$index]) {
                    $failures.Add("expiry candidate $Owner component $index differs from $($classes[$index])/$($ExpectedOutcomes[$index]).")
                }
            }
        }
        $requireInspection = {
            param([object]$Value, [string]$Owner, [string]$ReleaseOutcome, [bool]$Authorized, [string[]]$ComponentOutcomes)
            & $requireProperties $Value @('catalog_outcome', 'component_outcomes', 'release_outcome', 'release_authorized') $Owner
            if ([string]$Value.catalog_outcome -cne 'accepted' -or [string]$Value.release_outcome -cne $ReleaseOutcome) {
                $failures.Add("expiry candidate $Owner catalog/Release outcomes differ from accepted/$ReleaseOutcome.")
            }
            & $requireBoolean $Value.release_authorized $Authorized "$Owner release_authorized"
            & $requireClassOutcomes $Value.component_outcomes $ComponentOutcomes "$Owner component_outcomes"
        }

        & $requireProperties $report @('schema', 'endpoint_sha256', 'control_sha256', 'identity', 'bounds', 'points', 'status') 'report'
        & $requireProperties $report.identity @('catalog', 'components', 'release', 'network') 'identity'
        & $requireProperties $report.identity.catalog @('cohort', 'generation', 'digest', 'not_before', 'not_after') 'identity.catalog'
        & $requireProperties $report.identity.release @('identity', 'build_identity', 'artifact_digest', 'protocol') 'identity.release'
        & $requireProperties $report.identity.network @('id', 'epoch', 'digest', 'profile') 'identity.network'
        & $requireProperties $report.bounds @('reference_at', 'no_new_work_after', 'terminal', 'network_valid_until', 'tuf_expires') 'bounds'
        & $requireProperties $report.bounds.tuf_expires @('timestamp', 'snapshot', 'targets') 'bounds.tuf_expires'
        & $requireProperties $report.points @('reference', 'no_new_minus_one', 'no_new_exact', 'terminal_minus_one', 'terminal_exact', 'terminal_plus_one') 'points'

        if ([string]$report.schema -cne 'ardents-h4-8-a11-expiry-candidate-v2' -or
            [string]$report.endpoint_sha256 -cne $ExpectedEndpointSHA256 -or
            [string]$report.control_sha256 -cne $ExpectedControlSHA256 -or [string]$report.status -cne 'accepted') {
            $failures.Add('expiry candidate report differs from the exact input identity or accepted schema/status.')
        }
        foreach ($digestFact in @(
            @{ Name = 'endpoint_sha256'; Value = [string]$report.endpoint_sha256 },
            @{ Name = 'control_sha256'; Value = [string]$report.control_sha256 },
            @{ Name = 'identity.catalog.digest'; Value = [string]$report.identity.catalog.digest },
            @{ Name = 'identity.release.artifact_digest'; Value = [string]$report.identity.release.artifact_digest },
            @{ Name = 'identity.network.id'; Value = [string]$report.identity.network.id },
            @{ Name = 'identity.network.digest'; Value = [string]$report.identity.network.digest }
        )) {
            if ($digestFact.Value -cnotmatch '^[0-9a-f]{64}$') { $failures.Add("expiry candidate $($digestFact.Name) is not one lowercase SHA-256 identity.") }
        }
        if ([string]$report.identity.catalog.cohort -cne $ExpectedCohort -or
            [string]$report.identity.release.identity -cne $ExpectedRelease -or
            [string]$report.identity.release.artifact_digest -cne $ExpectedEndpointSHA256 -or
            [string]$report.identity.release.build_identity -cnotmatch '^[A-Za-z0-9._:-]+$' -or
            [string]$report.identity.release.protocol -cnotmatch '^[A-Za-z0-9._:-]+$' -or
            [string]$report.identity.network.profile -cnotmatch '^[A-Za-z0-9._:-]+$') {
            $failures.Add('expiry candidate release/network identity is not the selected maintained result.')
        }
        [void](& $requirePositiveInteger $report.identity.catalog.generation 'catalog generation')
        [void](& $requirePositiveInteger $report.identity.network.epoch 'network epoch')

        $referenceTime = ConvertFrom-ExactUTCSecond $ReferenceAt
        $reportedReference = ConvertFrom-ExactUTCSecond ([string]$report.bounds.reference_at)
        $noNew = ConvertFrom-ExactUTCSecond ([string]$report.bounds.no_new_work_after)
        $terminal = ConvertFrom-ExactUTCSecond ([string]$report.bounds.terminal)
        $catalogNotBefore = ConvertFrom-ExactUTCSecond ([string]$report.identity.catalog.not_before)
        $catalogNotAfter = ConvertFrom-ExactUTCSecond ([string]$report.identity.catalog.not_after)
        $networkValidUntil = ConvertFrom-ExactUTCSecond ([string]$report.bounds.network_valid_until)
        $timestampExpiry = ConvertFrom-ExactUTCSecond ([string]$report.bounds.tuf_expires.timestamp)
        $snapshotExpiry = ConvertFrom-ExactUTCSecond ([string]$report.bounds.tuf_expires.snapshot)
        $targetsExpiry = ConvertFrom-ExactUTCSecond ([string]$report.bounds.tuf_expires.targets)
        if (@($referenceTime, $reportedReference, $noNew, $terminal, $catalogNotBefore, $catalogNotAfter,
                $networkValidUntil, $timestampExpiry, $snapshotExpiry, $targetsExpiry) -contains $null) {
            $failures.Add('expiry candidate bounds must all be exact whole-second UTC instants.')
        }
        elseif ([string]$report.bounds.reference_at -cne $ReferenceAt -or $reportedReference -ne $referenceTime -or
            $catalogNotBefore -gt $referenceTime -or $referenceTime -ge $noNew -or $noNew -ge $terminal -or
            $catalogNotAfter -ne $terminal -or $networkValidUntil -ne $terminal -or $timestampExpiry -ne $terminal -or
            $snapshotExpiry -ne $terminal -or $targetsExpiry -ne $terminal) {
            $failures.Add('expiry candidate bounds do not satisfy reference < no-new < terminal and every authenticated terminal owner.')
        }
        else {
            $noNewText = $noNew.ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
            $terminalText = $terminal.ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
            $expectedPointTimes = [ordered]@{
                reference = $ReferenceAt
                no_new_minus_one = $noNew.AddSeconds(-1).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
                no_new_exact = $noNewText
                terminal_minus_one = $terminal.AddSeconds(-1).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
                terminal_exact = $terminalText
                terminal_plus_one = $terminal.AddSeconds(1).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
            }
            foreach ($pointName in $expectedPointTimes.Keys) {
                if ([string]$report.points.$pointName.at -cne [string]$expectedPointTimes[$pointName] -or
                    $null -eq (ConvertFrom-ExactUTCSecond ([string]$report.points.$pointName.at))) {
                    $failures.Add("expiry candidate point $pointName is not its exact required instant.")
                }
            }
        }

        $catalogPath = Join-Path $BundleRoot 'catalog.ac1'
        if (-not (Test-Path -LiteralPath $catalogPath -PathType Leaf) -or
            (Get-FileHash -LiteralPath $catalogPath -Algorithm SHA256).Hash.ToLowerInvariant() -cne [string]$report.identity.catalog.digest) {
            $failures.Add('expiry candidate catalog identity is not SHA-256 of exact catalog.ac1.')
        }
        $components = @($report.identity.components)
        $classes = @('release', 'network', 'compatibility')
        if ($components.Count -ne 3) { $failures.Add('expiry candidate report must contain exactly three ordered components.') }
        else {
            for ($index = 0; $index -lt 3; $index++) {
                $component = $components[$index]
                $class = $classes[$index]
                & $requireProperties $component @('class', 'root_id', 'generation', 'digest', 'not_before', 'not_after') "identity.components[$index]"
                if ([string]$component.class -cne $class) {
                    $failures.Add("expiry candidate component $index is not ordered $class.")
                }
                foreach ($identity in @('root_id', 'digest')) {
                    if ([string]$component.$identity -cnotmatch '^[0-9a-f]{64}$') { $failures.Add("expiry candidate $class $identity is not lowercase SHA-256.") }
                }
                [void](& $requirePositiveInteger $component.generation "$class generation")
                $componentNotBefore = ConvertFrom-ExactUTCSecond ([string]$component.not_before)
                $componentNotAfter = ConvertFrom-ExactUTCSecond ([string]$component.not_after)
                if ($null -eq $componentNotBefore -or $null -eq $componentNotAfter -or $null -eq $referenceTime -or $null -eq $terminal -or
                    $componentNotBefore -gt $referenceTime -or $componentNotAfter -ne $terminal) {
                    $failures.Add("expiry candidate $class validity is not reference-current and bound to terminal.")
                }
                $rootPath = Join-Path $BundleRoot "$class.pub"
                $componentPath = Join-Path $BundleRoot "$class.ac1"
                if (-not (Test-Path -LiteralPath $rootPath -PathType Leaf) -or
                    (Get-FileHash -LiteralPath $rootPath -Algorithm SHA256).Hash.ToLowerInvariant() -cne [string]$component.root_id -or
                    -not (Test-Path -LiteralPath $componentPath -PathType Leaf) -or
                    (Get-FileHash -LiteralPath $componentPath -Algorithm SHA256).Hash.ToLowerInvariant() -cne [string]$component.digest) {
                    $failures.Add("expiry candidate $class identities do not match the exact bundle files.")
                }
            }
        }

        & $requireProperties $report.points.reference @('at', 'inspection', 'direct_release') 'points.reference'
        & $requireProperties $report.points.no_new_minus_one @('at', 'inspection', 'direct_release') 'points.no_new_minus_one'
        & $requireProperties $report.points.no_new_exact @('at', 'fresh_inspection', 'persisted_inspection', 'persisted_retry', 'direct_release', 'direct_retry') 'points.no_new_exact'
        & $requireProperties $report.points.terminal_minus_one @('at', 'inspection', 'direct_release') 'points.terminal_minus_one'
        & $requireProperties $report.points.terminal_exact @('at', 'fresh_catalog_outcome', 'fresh_full_refused', 'persisted_catalog_outcome', 'persisted_full_refused', 'direct_component_outcomes', 'direct_release') 'points.terminal_exact'
        & $requireProperties $report.points.terminal_plus_one @('at', 'direct_release') 'points.terminal_plus_one'

        $acceptedComponents = @('accepted', 'accepted', 'accepted')
        $noNewComponents = @('expired', 'accepted', 'unavailable')
        & $requireInspection $report.points.reference.inspection 'points.reference.inspection' 'release-accepted' $true $acceptedComponents
        & $requireInspection $report.points.no_new_minus_one.inspection 'points.no_new_minus_one.inspection' 'release-accepted' $true $acceptedComponents
        & $requireInspection $report.points.no_new_exact.fresh_inspection 'points.no_new_exact.fresh_inspection' 'update-required' $false $noNewComponents
        & $requireInspection $report.points.no_new_exact.persisted_inspection 'points.no_new_exact.persisted_inspection' 'update-required' $false $noNewComponents
        & $requireInspection $report.points.no_new_exact.persisted_retry 'points.no_new_exact.persisted_retry' 'update-required' $false $noNewComponents
        & $requireInspection $report.points.terminal_minus_one.inspection 'points.terminal_minus_one.inspection' 'update-required' $false $noNewComponents

        $requireDirect = {
            param([object]$Value, [string]$Owner, [string]$Outcome, [string]$BuildSafety, [bool]$Authorized, [bool]$AuthenticatedBounds)
            & $requireProperties $Value @('outcome', 'build_safety', 'authorized', 'no_new_work_after', 'terminate_after') $Owner
            if ([string]$Value.outcome -cne $Outcome -or [string]$Value.build_safety -cne $BuildSafety) {
                $failures.Add("expiry candidate $Owner outcome/build_safety differ from $Outcome/$BuildSafety.")
            }
            & $requireBoolean $Value.authorized $Authorized "$Owner authorized"
            if ($AuthenticatedBounds) {
                if ([string]$Value.no_new_work_after -cne $noNewText -or [string]$Value.terminate_after -cne $terminalText) {
                    $failures.Add("expiry candidate $Owner lacks the exact authenticated Release bounds.")
                }
            }
            elseif ([string]$Value.no_new_work_after -cne '' -or [string]$Value.terminate_after -cne '') {
                $failures.Add("expiry candidate $Owner unexpectedly reports target-authenticated Release bounds.")
            }
        }
        & $requireDirect $report.points.reference.direct_release 'points.reference.direct_release' 'release-accepted' 'release-accepted' $true $true
        & $requireDirect $report.points.no_new_minus_one.direct_release 'points.no_new_minus_one.direct_release' 'release-accepted' 'release-accepted' $true $true
        & $requireDirect $report.points.no_new_exact.direct_release 'points.no_new_exact.direct_release' 'update-required' 'update-required' $false $true
        & $requireDirect $report.points.no_new_exact.direct_retry 'points.no_new_exact.direct_retry' 'update-required' 'update-required' $false $true
        & $requireDirect $report.points.terminal_minus_one.direct_release 'points.terminal_minus_one.direct_release' 'update-required' 'update-required' $false $true

        if ([string]$report.points.terminal_exact.fresh_catalog_outcome -cne 'invalid' -or
            [string]$report.points.terminal_exact.persisted_catalog_outcome -cne 'invalid') {
            $failures.Add('expiry candidate terminal fresh/persisted catalog outcomes are not invalid.')
        }
        & $requireBoolean $report.points.terminal_exact.fresh_full_refused $true 'points.terminal_exact.fresh_full_refused'
        & $requireBoolean $report.points.terminal_exact.persisted_full_refused $true 'points.terminal_exact.persisted_full_refused'
        & $requireClassOutcomes $report.points.terminal_exact.direct_component_outcomes @('expired', 'expired', 'expired') 'points.terminal_exact.direct_component_outcomes'
        & $requireDirect $report.points.terminal_exact.direct_release 'points.terminal_exact.direct_release' 'release-revoked' 'release-revoked' $false $true
        & $requireDirect $report.points.terminal_plus_one.direct_release 'points.terminal_plus_one.direct_release' 'release-expired' '' $false $false
    }
    catch { $failures.Add("expiry candidate report could not be validated: $($_.Exception.Message)") }
    return [pscustomobject]@{ Passed = $failures.Count -eq 0; Marker = $marker; Report = $report; Failures = @($failures) }
}

function Invoke-SelfTests {
    $assertions = New-Object System.Collections.Generic.List[string]
    $stopFixture = $null
    $plan = @(Get-A11Attempts 50000)
    if ($plan.Count -ne 10 -or @($plan.Cell | Select-Object -Unique).Count -ne 6) { $assertions.Add('plan must contain ten invocations in six cells') }
    if (@($plan | Where-Object Attempt -eq 'canary').Count -ne 4 -or @($plan | Where-Object Remote).Count -ne 9) { $assertions.Add('plan must contain four remote canaries and one local expiry invocation') }
    if (($plan | Select-Object -Last 1).Test -cne 'TestH48A11DeterministicExpiryBoundaries') { $assertions.Add('expiry test name drifted') }
    $campaignLimit = 125L * 60L * 1000L
    $cleanupReserve = 30000L
    if ((Get-BoundedAttemptSeconds 2700 0 $campaignLimit $cleanupReserve) -ne 2700 -or
        (Get-BoundedAttemptSeconds 2700 (100L * 60L * 1000L) $campaignLimit $cleanupReserve) -ne 1470 -or
        (Get-BoundedAttemptSeconds 600 $campaignLimit $campaignLimit $cleanupReserve) -ne 0 -or
        (Get-BoundedAttemptSeconds 600 ($campaignLimit - 30999) $campaignLimit $cleanupReserve) -ne 0 -or
        (Get-BoundedAttemptSeconds 600 ($campaignLimit - 31000) $campaignLimit $cleanupReserve) -ne 1) {
        $assertions.Add('campaign deadline arithmetic does not reserve cleanup and bound work to exact remaining whole seconds')
    }
    if ((Get-BoundedCleanupDeadlineMilliseconds 7400000 $campaignLimit 15000) -ne 7415000 -or
        (Get-BoundedCleanupDeadlineMilliseconds 7495000 $campaignLimit 15000) -ne $campaignLimit -or
        (Get-BoundedCleanupDeadlineMilliseconds $campaignLimit $campaignLimit 15000) -ne $campaignLimit) {
        $assertions.Add('cleanup deadline arithmetic can exceed the exact campaign deadline')
    }
    $validStopReceipt = [pscustomobject]@{
        schema = 'ardents-h4-8-a11-process-stop-v1'
        requested_root_pid = 321
        owned_pids = @(321, 322)
        stopped_pids = @(321, 322)
        survivor_pids = @()
        stop_errors = @()
        deadline_met = $true
        passed = $true
    }
    if (-not (Test-ProcessStopReceipt $validStopReceipt).Passed) {
        $assertions.Add('valid bounded process-stop receipt was rejected')
    }
    foreach ($mutation in @(
        @{ Name = 'survivor'; Property = 'survivor_pids'; Value = @(322) },
        @{ Name = 'stop error'; Property = 'stop_errors'; Value = @('access denied') },
        @{ Name = 'elapsed deadline'; Property = 'deadline_met'; Value = $false },
        @{ Name = 'false disposition'; Property = 'passed'; Value = $false }
    )) {
        $mutatedStopReceipt = ($validStopReceipt | ConvertTo-Json -Depth 4 | ConvertFrom-Json)
        $mutatedStopReceipt.($mutation.Property) = $mutation.Value
        if ((Test-ProcessStopReceipt $mutatedStopReceipt).Passed) {
            $assertions.Add("bounded process-stop receipt accepted $($mutation.Name) mutation")
        }
    }
    $runnerFileSource = [IO.File]::ReadAllText($PSCommandPath)
    $attemptStart = $runnerFileSource.LastIndexOf('function Invoke-A11Attempt(', [StringComparison]::Ordinal)
    $attemptEnd = $runnerFileSource.IndexOf('if ($PSCmdlet.ParameterSetName', $attemptStart, [StringComparison]::Ordinal)
    $runnerSource = $runnerFileSource.Substring($attemptStart, $attemptEnd - $attemptStart)
    if (-not (Test-A11BoundedProcessSource $runnerSource).Passed) {
        $assertions.Add('A11 runner lacks its exact bounded Go/observer process completion seams')
    }
    $unboundedWaitMutation = '$goProcess.WaitFor' + 'Exit()'
    if ((Test-A11BoundedProcessSource ($runnerSource + "`n$unboundedWaitMutation`n")).Passed) {
        $assertions.Add('A11 runner source oracle accepted an unbounded process wait mutation')
    }
    if ((Test-A11BoundedProcessSource ($runnerSource + "`nStop-Process -Id `$observer.Process.Id -Force -ErrorAction SilentlyContinue`n")).Passed) {
        $assertions.Add('A11 runner source oracle accepted an error-silenced observer stop mutation')
    }
    $entrypointSource = [IO.File]::ReadAllText((Join-Path $PSScriptRoot 'invoke-windows.ps1'))
    if (-not (Test-A11EntrypointCaptureSource $entrypointSource).Passed) {
        $assertions.Add('valid A11 entrypoint stream/exit capture seams were rejected')
    }
    if ((Test-A11EntrypointCaptureSource ($entrypointSource.Replace("'entrypoint.stderr.log'", "'entrypoint.stream.log'"))).Passed) {
        $assertions.Add('A11 entrypoint capture oracle accepted missing stderr retention')
    }
    $temporary = Join-Path ([IO.Path]::GetTempPath()) ('ardents-a11-self-test-' + [Guid]::NewGuid().ToString('N'))
    [IO.Directory]::CreateDirectory($temporary) | Out-Null
    try {
        $nativeStdout = Join-Path $temporary 'native-stdout.txt'
        $nativeStderr = Join-Path $temporary 'native-stderr.txt'
        $nativeExit = Invoke-NativeCaptured $env:ComSpec @('/d', '/s', '/c', 'echo native-out&echo native-err 1>&2&exit /b 0') $nativeStdout $nativeStderr $temporary
        if ($nativeExit -ne 0 -or (Get-Content -LiteralPath $nativeStdout -Raw).Trim() -cne 'native-out' -or
            (Get-Content -LiteralPath $nativeStderr -Raw).Trim() -cne 'native-err') {
            $assertions.Add('successful native stderr was not captured independently from exit status')
        }
        $stopStdout = Join-Path $temporary 'stop-fixture.stdout.log'
        $stopStderr = Join-Path $temporary 'stop-fixture.stderr.log'
        $stopFixture = Start-Process -FilePath (Get-Process -Id $PID).Path `
            -ArgumentList (Join-NativeArguments @('-NoProfile', '-Command', 'Start-Sleep -Seconds 30')) `
            -RedirectStandardOutput $stopStdout -RedirectStandardError $stopStderr -WindowStyle Hidden -PassThru
        $stopClock = [Diagnostics.Stopwatch]::StartNew()
        $stopDeadline = Get-BoundedCleanupDeadlineMilliseconds $stopClock.ElapsedMilliseconds 10000 9000
        $stopReceiptPath = Join-Path $temporary 'bounded-stop.json'
        $stopResult = Stop-OwnedProcessTree $stopFixture.Id $stopReceiptPath $stopClock $stopDeadline
        $stopWait = Wait-ProcessBounded $stopFixture $stopClock $stopDeadline
        $retainedStop = Get-Content -LiteralPath $stopReceiptPath -Raw | ConvertFrom-Json
        if (-not $stopResult.Passed -or -not $stopWait.Exited -or -not (Test-ProcessStopReceipt $retainedStop).Passed -or
            $stopClock.ElapsedMilliseconds -gt 10000 -or $null -ne (Get-Process -Id $stopFixture.Id -ErrorAction SilentlyContinue)) {
            $assertions.Add("owned process tree was not stopped, retained, and waited within the exact bounded deadline (stop=$($stopResult.Passed), wait=$($stopWait.Exited), elapsed=$($stopClock.ElapsedMilliseconds), receipt=$((Test-ProcessStopReceipt $retainedStop).Passed), facts=$($stopResult.Receipt | ConvertTo-Json -Depth 4 -Compress))")
        }
        $image = 'sha256:' + ('a' * 64)
        $containerID = 'b' * 64
        $header = $script:ubuntuHeader -join "`t"
        $row = @('1000', 'ardents-h4-8-a11-aaaaaaaaaaaa', $containerID, $image, 'true', 'false', 'false', '0', '1000000000', '1073741824', '128', 'host', 'no', '/docker/test', '1', '0', '0', '4096', '1073741824', '0', '0', '0', '2', '128', '2', '0', '2', '4', '10', '20') -join "`t"
        $row2 = $row -replace '^1000', '3000'
        $valid = Join-Path $temporary 'valid.tsv'
        Write-Utf8 $valid "$header`n$row`n$row2`n"
        $parsed = Test-UbuntuResourceSeries $valid 'ardents-h4-8-a11-aaaaaaaaaaaa' $image
        if (-not $parsed.Passed -or $parsed.Samples -ne 2) { $assertions.Add('valid observer series was rejected') }
        $gap = Join-Path $temporary 'gap.tsv'
        Write-Utf8 $gap "$header`n$row`n$($row -replace '^1000', '3001')`n"
        if ((Test-UbuntuResourceSeries $gap 'ardents-h4-8-a11-aaaaaaaaaaaa' $image).Passed) { $assertions.Add('observer gap above two seconds was accepted') }
        $oom = Join-Path $temporary 'oom.tsv'
        $oomRow = ($row.Split("`t")); $oomRow[20] = '1'
        Write-Utf8 $oom "$header`n$($oomRow -join "`t")`n$($oomRow -join "`t" -replace '^1000', '2000')`n"
        if ((Test-UbuntuResourceSeries $oom 'ardents-h4-8-a11-aaaaaaaaaaaa' $image).Passed) { $assertions.Add('OOM-kill evidence was accepted') }

        $windows = Join-Path $temporary 'windows.jsonl'
        Add-JsonLine $windows ([ordered]@{ timestamp_ms = 1000; monotonic_ms = 0; root_pid = 321; processes = @([ordered]@{ pid = 321; role = 'go-test-root' }); error = $null })
        Add-JsonLine $windows ([ordered]@{ timestamp_ms = 3000; monotonic_ms = 2000; root_pid = 321; processes = @([ordered]@{ pid = 321; role = 'go-test-root' }); error = $null })
        if (-not (Test-WindowsResourceSeries $windows 321 $true).Passed) { $assertions.Add('valid Windows observer series was rejected') }
        $windowsGap = Join-Path $temporary 'windows-gap.jsonl'
        Add-JsonLine $windowsGap ([ordered]@{ timestamp_ms = 1000; monotonic_ms = 0; root_pid = 321; processes = @([ordered]@{ pid = 321; role = 'go-test-root' }); error = $null })
        Add-JsonLine $windowsGap ([ordered]@{ timestamp_ms = 3001; monotonic_ms = 2001; root_pid = 321; processes = @([ordered]@{ pid = 321; role = 'go-test-root' }); error = $null })
        if ((Test-WindowsResourceSeries $windowsGap 321 $true).Passed) { $assertions.Add('Windows observer gap above two seconds was accepted') }
        $expiryOutput = (@(Get-ExpiryMarkers) -join "`n") + "`n"
        if (-not (Test-ExpiryOutput $expiryOutput).Passed) { $assertions.Add('valid deterministic expiry markers were rejected') }
        if ((Test-ExpiryOutput (($expiryOutput.Split("`n") | Select-Object -Skip 1) -join "`n")).Passed) { $assertions.Add('missing deterministic expiry owner marker was accepted') }
        $candidateBundle = Join-Path $temporary 'candidate-bundle'
        [IO.Directory]::CreateDirectory($candidateBundle) | Out-Null
        foreach ($name in @('catalog.ac1', 'release.pub', 'release.ac1', 'network.pub', 'network.ac1', 'compatibility.pub', 'compatibility.ac1')) {
            Write-Utf8 (Join-Path $candidateBundle $name) "exact-$name`n"
        }
        $candidateEndpoint = 'd' * 64
        $candidateControl = 'e' * 64
        $componentReports = @()
        foreach ($class in @('release', 'network', 'compatibility')) {
            $componentReports += [ordered]@{
                class = $class
                root_id = (Get-FileHash -LiteralPath (Join-Path $candidateBundle "$class.pub") -Algorithm SHA256).Hash.ToLowerInvariant()
                generation = 1
                digest = (Get-FileHash -LiteralPath (Join-Path $candidateBundle "$class.ac1") -Algorithm SHA256).Hash.ToLowerInvariant()
                not_before = '2030-01-02T03:00:00Z'
                not_after = '2030-01-02T04:00:00Z'
            }
        }
        $acceptedComponents = @(
            [ordered]@{ class = 'release'; outcome = 'accepted' },
            [ordered]@{ class = 'network'; outcome = 'accepted' },
            [ordered]@{ class = 'compatibility'; outcome = 'accepted' }
        )
        $noNewComponents = @(
            [ordered]@{ class = 'release'; outcome = 'expired' },
            [ordered]@{ class = 'network'; outcome = 'accepted' },
            [ordered]@{ class = 'compatibility'; outcome = 'unavailable' }
        )
        $expiredComponents = @(
            [ordered]@{ class = 'release'; outcome = 'expired' },
            [ordered]@{ class = 'network'; outcome = 'expired' },
            [ordered]@{ class = 'compatibility'; outcome = 'expired' }
        )
        $acceptedInspection = [ordered]@{ catalog_outcome = 'accepted'; component_outcomes = $acceptedComponents; release_outcome = 'release-accepted'; release_authorized = $true }
        $noNewInspection = [ordered]@{ catalog_outcome = 'accepted'; component_outcomes = $noNewComponents; release_outcome = 'update-required'; release_authorized = $false }
        $acceptedDirect = [ordered]@{ outcome = 'release-accepted'; build_safety = 'release-accepted'; authorized = $true; no_new_work_after = '2030-01-02T03:30:00Z'; terminate_after = '2030-01-02T04:00:00Z' }
        $noNewDirect = [ordered]@{ outcome = 'update-required'; build_safety = 'update-required'; authorized = $false; no_new_work_after = '2030-01-02T03:30:00Z'; terminate_after = '2030-01-02T04:00:00Z' }
        $candidateReport = [ordered]@{
            schema = 'ardents-h4-8-a11-expiry-candidate-v2'
            endpoint_sha256 = $candidateEndpoint
            control_sha256 = $candidateControl
            identity = [ordered]@{
                catalog = [ordered]@{ cohort = 'cohort-1'; generation = 1; digest = (Get-FileHash -LiteralPath (Join-Path $candidateBundle 'catalog.ac1') -Algorithm SHA256).Hash.ToLowerInvariant(); not_before = '2030-01-02T03:00:00Z'; not_after = '2030-01-02T04:00:00Z' }
                components = $componentReports
                release = [ordered]@{ identity = 'release-1'; build_identity = 'build-1'; artifact_digest = $candidateEndpoint; protocol = 'announced' }
                network = [ordered]@{ id = 'a' * 64; epoch = 1; digest = 'b' * 64; profile = 'ardents-interactive-route-v1' }
            }
            bounds = [ordered]@{
                reference_at = '2030-01-02T03:04:05Z'
                no_new_work_after = '2030-01-02T03:30:00Z'
                terminal = '2030-01-02T04:00:00Z'
                network_valid_until = '2030-01-02T04:00:00Z'
                tuf_expires = [ordered]@{ timestamp = '2030-01-02T04:00:00Z'; snapshot = '2030-01-02T04:00:00Z'; targets = '2030-01-02T04:00:00Z' }
            }
            points = [ordered]@{
                reference = [ordered]@{ at = '2030-01-02T03:04:05Z'; inspection = $acceptedInspection; direct_release = $acceptedDirect }
                no_new_minus_one = [ordered]@{ at = '2030-01-02T03:29:59Z'; inspection = $acceptedInspection; direct_release = $acceptedDirect }
                no_new_exact = [ordered]@{ at = '2030-01-02T03:30:00Z'; fresh_inspection = $noNewInspection; persisted_inspection = $noNewInspection; persisted_retry = $noNewInspection; direct_release = $noNewDirect; direct_retry = $noNewDirect }
                terminal_minus_one = [ordered]@{ at = '2030-01-02T03:59:59Z'; inspection = $noNewInspection; direct_release = $noNewDirect }
                terminal_exact = [ordered]@{
                    at = '2030-01-02T04:00:00Z'; fresh_catalog_outcome = 'invalid'; fresh_full_refused = $true
                    persisted_catalog_outcome = 'invalid'; persisted_full_refused = $true; direct_component_outcomes = $expiredComponents
                    direct_release = [ordered]@{ outcome = 'release-revoked'; build_safety = 'release-revoked'; authorized = $false; no_new_work_after = '2030-01-02T03:30:00Z'; terminate_after = '2030-01-02T04:00:00Z' }
                }
                terminal_plus_one = [ordered]@{ at = '2030-01-02T04:00:01Z'; direct_release = [ordered]@{ outcome = 'release-expired'; build_safety = ''; authorized = $false; no_new_work_after = ''; terminate_after = '' } }
            }
            status = 'accepted'
        }
        $candidateJSON = $candidateReport | ConvertTo-Json -Depth 16 -Compress
        $candidateMarker = 'A11_EXPIRY_CANDIDATE_REPORT ' + $candidateJSON
        $candidateGoOutput = "    candidate_expiry_test.go:12: $candidateMarker`n"
        if (-not (Test-CandidateExpiryReport $candidateGoOutput $candidateBundle '2030-01-02T03:04:05Z' 'cohort-1' 'release-1' $candidateEndpoint $candidateControl).Passed) {
            $assertions.Add('valid exact-candidate expiry report was rejected')
        }
        if ((Test-CandidateExpiryReport $expiryOutput $candidateBundle '2030-01-02T03:04:05Z' 'cohort-1' 'release-1' $candidateEndpoint $candidateControl).Passed) {
            $assertions.Add('missing exact-candidate expiry report marker was accepted')
        }
        if ((Test-CandidateExpiryReport ($candidateGoOutput + $candidateGoOutput) $candidateBundle '2030-01-02T03:04:05Z' 'cohort-1' 'release-1' $candidateEndpoint $candidateControl).Passed) {
            $assertions.Add('duplicate exact-candidate expiry report markers were accepted')
        }
        $testCandidateMutation = {
            param([string]$Name, [scriptblock]$Mutate)
            $mutated = $candidateJSON | ConvertFrom-Json
            & $Mutate $mutated
            $mutatedOutput = "    candidate_expiry_test.go:12: A11_EXPIRY_CANDIDATE_REPORT $($mutated | ConvertTo-Json -Depth 16 -Compress)`n"
            if ((Test-CandidateExpiryReport $mutatedOutput $candidateBundle '2030-01-02T03:04:05Z' 'cohort-1' 'release-1' $candidateEndpoint $candidateControl).Passed) {
                $assertions.Add("$Name exact-candidate expiry mutation was accepted")
            }
        }
        & $testCandidateMutation 'collapsed no-new/terminal' { param($value) $value.bounds.no_new_work_after = $value.bounds.terminal }
        & $testCandidateMutation 'no-new authorization' { param($value) $value.points.no_new_exact.fresh_inspection.release_authorized = $true }
        & $testCandidateMutation 'wrong terminal direct Release' { param($value) $value.points.terminal_exact.direct_release.outcome = 'update-required' }
        & $testCandidateMutation 'mismatched TUF expiry' { param($value) $value.bounds.tuf_expires.timestamp = '2030-01-02T04:00:01Z' }
        & $testCandidateMutation 'missing persisted retry' { param($value) [void]$value.points.no_new_exact.PSObject.Properties.Remove('persisted_retry') }
        & $testCandidateMutation 'extra terminal component' { param($value) $value.points.terminal_exact.direct_component_outcomes += [pscustomobject]@{ class = 'compatibility'; outcome = 'expired' } }

        $statusRoot = Join-Path $temporary 'status'
        $remoteEvidence = Join-Path $statusRoot 'remote-evidence'
        $userEvidence = Join-Path $statusRoot 'user-evidence'
        [IO.Directory]::CreateDirectory($remoteEvidence) | Out-Null
        [IO.Directory]::CreateDirectory($userEvidence) | Out-Null
        $testName = 'TestH48A11MultiHostTenCycleCanary'
        $container = 'ardents-h4-8-a11-aaaaaaaaaaaa'
        $remoteRoot = '/tmp/' + $container
        Write-Utf8 (Join-Path $statusRoot 'contract.ready') "test=$testName`ncontainer=$container`nremote_directory=$remoteRoot`n"
        Write-Utf8 (Join-Path $statusRoot 'remote.ready') "container=$container`nremote_directory=$remoteRoot`n"
        Write-Utf8 (Join-Path $statusRoot 'user.ready') "pid=123`n"
        Write-Utf8 (Join-Path $statusRoot 'complete') "passed=true`n"
        Write-Utf8 (Join-Path $statusRoot 'remote-evidence.complete') "schema=ardents-h4-8-a11-remote-evidence-v1`nretained=remote-evidence/capture.txt`n"
        Write-Utf8 (Join-Path $statusRoot 'user-evidence.complete') "schema=ardents-h4-8-a11-user-evidence-v1`nretained=user-evidence/stdout.log,user-evidence/stderr.log,user-evidence/process.txt`n"
        Write-Utf8 (Join-Path $userEvidence 'stdout.log') "bounded`n"
        Write-Utf8 (Join-Path $userEvidence 'stderr.log') "bounded-error`n"
        Write-Utf8 (Join-Path $userEvidence 'process.txt') "schema=ardents-h4-8-a11-user-evidence-v1`nstreams=separate`ndisposition=success`nexit_code=0`n"
        $capture = "schema=ardents-h4-8-a11-remote-evidence-v1`n[container-state]`n{`"OOMKilled`":false,`"Restarting`":false,`"Dead`":false,`"ExitCode`":0,`"Status`":`"exited`"}`n[staged-inventory-sha256]`n"
        foreach ($name in @('ardents-node', 'reference-c2', 'reference-c2.json', 'run.sh', 'topology.json', 'rendezvous-node.pid', 'carrier-relay.pid', 'publisher.pid', 'publisher-app.pid', 'carrier-relay-ready.json')) {
            $capture += ('c' * 64) + "`t1`t$name`n"
        }
        $capture += "[role-output]`n"
        foreach ($role in @('source-a', 'source-b', 'rendezvous-node', 'carrier-relay', 'initiator', 'introduction', 'responder', 'gateway', 'publisher', 'alpha-gateway', 'alpha-relay', 'publisher-app')) {
            $capture += "===$role.log===`n===$role.err===`n"
        }
        Write-Utf8 (Join-Path $remoteEvidence 'capture.txt') $capture
        $status = Test-A11Status $statusRoot $testName $container $remoteRoot ([pscustomobject]@{ ObservedPIDs = @(123) }) $true
        if (-not $status.Passed) { $assertions.Add('valid retained status/evidence contract was rejected') }
        $captureWithoutPublisherPIDs = $capture -replace "(?m)^[0-9a-f]{64}`t[0-9]+`tpublisher(?:-app)?\.pid`n", ''
        Write-Utf8 (Join-Path $remoteEvidence 'capture.txt') $captureWithoutPublisherPIDs
        if ((Test-A11Status $statusRoot $testName $container $remoteRoot ([pscustomobject]@{ ObservedPIDs = @(123) }) $true).Passed) {
            $assertions.Add('common remote status without exact Publisher PID inventories was accepted')
        }
        Write-Utf8 (Join-Path $remoteEvidence 'capture.txt') $capture
        $applicationTest = 'TestH48A11MultiHostPublisherApplicationLossAfterWarmup'
        Write-Utf8 (Join-Path $statusRoot 'contract.ready') "test=$applicationTest`ncontainer=$container`nremote_directory=$remoteRoot`n"
        if ((Test-A11Status $statusRoot $applicationTest $container $remoteRoot ([pscustomobject]@{ ObservedPIDs = @(123) }) $true).Passed) {
            $assertions.Add('Application fault status without exact ready/fault/reset receipts was accepted')
        }
        $carrierTest = 'TestH48A11MultiHostCarrierLossAfterWarmup'
        Write-Utf8 (Join-Path $statusRoot 'contract.ready') "test=$carrierTest`ncontainer=$container`nremote_directory=$remoteRoot`n"
        if ((Test-A11Status $statusRoot $carrierTest $container $remoteRoot ([pscustomobject]@{ ObservedPIDs = @(123) }) $true).Passed) {
            $assertions.Add('Carrier fault status without exact fault/reset receipts was accepted')
        }
        $endpointTest = 'TestH48A11MultiHostPublisherEndpointLossAfterWarmup'
        Write-Utf8 (Join-Path $statusRoot 'contract.ready') "test=$endpointTest`ncontainer=$container`nremote_directory=$remoteRoot`n"
        if ((Test-A11Status $statusRoot $endpointTest $container $remoteRoot ([pscustomobject]@{ ObservedPIDs = @(123) }) $true).Passed) {
            $assertions.Add('Endpoint fault status without exact crash/kill receipts was accepted')
        }
        $nodeTest = 'TestH48A11MultiHostProductNodeLossAfterWarmup'
        Write-Utf8 (Join-Path $statusRoot 'contract.ready') "test=$nodeTest`ncontainer=$container`nremote_directory=$remoteRoot`n"
        if ((Test-A11Status $statusRoot $nodeTest $container $remoteRoot ([pscustomobject]@{ ObservedPIDs = @(123) }) $true).Passed) {
            $assertions.Add('product Node fault status without exact ready/fault/kill receipts was accepted')
        }
        $serviceRoot = Join-Path $PSScriptRoot '..\..\e2e\service'
        $qualificationSource = [IO.File]::ReadAllText((Join-Path $serviceRoot 'reference_c2_a11_qualification_test.go'))
        $productEvidenceSource = [IO.File]::ReadAllText((Join-Path $serviceRoot 'reference_c2_multihost_product_evidence_test.go'))
        $productRunnerSource = [IO.File]::ReadAllText((Join-Path $serviceRoot 'reference_c2_multihost_runner_test.go'))
        if (-not (Test-A11ProductHarnessSeams $qualificationSource $productEvidenceSource $productRunnerSource).Passed) {
            $assertions.Add('valid exact product/fault harness seams were rejected')
        }
        $weakenedApplicationEvidence = $productEvidenceSource.Replace('!reset.TransitRolesLiveBefore', 'reset.TransitRolesLiveBefore')
        if ((Test-A11ProductHarnessSeams $qualificationSource $weakenedApplicationEvidence $productRunnerSource).Passed) {
            $assertions.Add('Application receipt oracle without transit liveness was accepted')
        }
        $weakenedCarrierEvidence = $productEvidenceSource.Replace('reset.ResetBridges != 2', 'reset.ResetBridges != 1')
        if ((Test-A11ProductHarnessSeams $qualificationSource $weakenedCarrierEvidence $productRunnerSource).Passed) {
            $assertions.Add('Carrier receipt oracle that does not require both bridges to reset was accepted')
        }
        $weakenedCarrierSelection = $productEvidenceSource.Replace('reset.SelectedBridgeID != 0', 'reset.SelectedBridgeID == 0')
        if ((Test-A11ProductHarnessSeams $qualificationSource $weakenedCarrierSelection $productRunnerSource).Passed) {
            $assertions.Add('Carrier receipt oracle without the exact selected bridge was accepted')
        }
    }
    finally {
        if ($null -ne $stopFixture) {
            try {
                $stopFixture.Refresh()
                if (-not $stopFixture.HasExited) { Stop-Process -Id $stopFixture.Id -Force -ErrorAction Stop }
                if (-not $stopFixture.WaitForExit(2000)) { $assertions.Add('self-test stop fixture survived bounded fallback cleanup') }
            }
            catch { $assertions.Add("self-test stop fixture cleanup failed: $($_.Exception.Message)") }
        }
        $resolved = (Resolve-Path -LiteralPath $temporary).Path
        $prefix = [IO.Path]::GetTempPath().TrimEnd('\', '/') + [IO.Path]::DirectorySeparatorChar + 'ardents-a11-self-test-'
        if ($resolved.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase)) { Remove-Item -LiteralPath $resolved -Recurse -Force }
    }
    if ($assertions.Count -ne 0) { throw ('A11 runner self-test failed: ' + ($assertions -join ' | ')) }
    Write-Output 'A11 runner self-tests passed.'
}

function Test-LiteralIPv4([string]$Address) {
    if ($Address -notmatch '^(\d{1,3}\.){3}\d{1,3}$') { return $false }
    foreach ($octet in $Address.Split('.')) { if ([int]$octet -gt 255) { return $false } }
    return $true
}

function Invoke-ArchiveValidation([string]$ArchivePath, [string]$EvidencePath, [string]$RetainedExtractionRoot = '') {
    $actualArchive = (Get-FileHash -LiteralPath $ArchivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    Write-Utf8 (Join-Path $EvidencePath 'archive-sha256.txt') "$actualArchive  $([IO.Path]::GetFileName($ArchivePath))`n"
    if ($actualArchive -cne $script:ArchiveSHA256) { throw 'archive digest differs from the exact campaign input.' }

    $listing = @(& tar.exe -tzf $ArchivePath)
    if ($LASTEXITCODE -ne 0 -or $listing.Count -eq 0) { throw 'release archive inventory cannot be read.' }
    Write-Utf8 (Join-Path $EvidencePath 'archive-inventory.txt') (($listing -join "`n") + "`n")
    $normalized = New-Object System.Collections.Generic.List[string]
    $topNames = New-Object 'System.Collections.Generic.HashSet[string]' ([StringComparer]::Ordinal)
    foreach ($raw in $listing) {
        $entry = [string]$raw
        if ($entry.StartsWith('./', [StringComparison]::Ordinal)) { $entry = $entry.Substring(2) }
        if ([string]::IsNullOrEmpty($entry) -or $entry.StartsWith('/', [StringComparison]::Ordinal) -or
            $entry.Contains('\') -or $entry.Contains('//') -or $entry -match '(^|/)\.\.(/|$)') {
            throw 'release archive contains an unsafe path.'
        }
        $trimmed = $entry.TrimEnd('/')
        $parts = $trimmed.Split('/')
        if ($parts.Count -gt 2 -or $parts[0] -cnotmatch '^[A-Za-z0-9._-]+$' -or
            ($parts.Count -eq 2 -and $parts[1] -cnotmatch '^[A-Za-z0-9._-]+$')) {
            throw 'release archive is not one simple top-level bundle.'
        }
        [void]$topNames.Add($parts[0])
        $normalized.Add($trimmed)
    }
    if ($topNames.Count -ne 1 -or ($normalized | Group-Object -CaseSensitive | Where-Object Count -gt 1)) {
        throw 'release archive has ambiguous or duplicate inventory.'
    }

    $retainExtraction = -not [string]::IsNullOrEmpty($RetainedExtractionRoot)
    if ($retainExtraction) {
        $validationRoot = [IO.Path]::GetFullPath($RetainedExtractionRoot)
        if (-not (Test-PathWithin $validationRoot $EvidencePath) -or (Test-Path -LiteralPath $validationRoot)) {
            throw 'retained candidate extraction root must be previously absent inside this evidence attempt.'
        }
    }
    else {
        $validationRoot = Join-Path ([IO.Path]::GetTempPath()) ('ardents-h4-8-a11-archive-' + [Guid]::NewGuid().ToString('N'))
    }
    [IO.Directory]::CreateDirectory($validationRoot) | Out-Null
    $validatedBundle = $null
    $validatedDescriptor = $null
    try {
        & tar.exe -xzf $ArchivePath -C $validationRoot
        if ($LASTEXITCODE -ne 0) { throw 'release archive extraction failed.' }
        $top = @($topNames)[0]
        $bundle = Join-Path $validationRoot $top
        $topEntries = @(Get-ChildItem -LiteralPath $validationRoot -Force)
        if ($topEntries.Count -ne 1 -or -not $topEntries[0].PSIsContainer -or -not (Test-Path -LiteralPath $bundle -PathType Container)) {
            throw 'release archive extracted an ambiguous top level.'
        }
        $bundleEntries = @(Get-ChildItem -LiteralPath $bundle -Force)
        if ($bundleEntries.Count -eq 0 -or ($bundleEntries | Where-Object {
            $_.PSIsContainer -or (($_.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0)
        })) { throw 'release bundle contains a non-regular top-level entry.' }

        $manifestPath = Join-Path $bundle 'SHA256SUMS'
        if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) { throw 'release bundle manifest is absent.' }
        $actualPin = (Get-FileHash -LiteralPath $manifestPath -Algorithm SHA256).Hash.ToLowerInvariant()
        Write-Utf8 (Join-Path $EvidencePath 'manifest-pin.txt') "$actualPin  SHA256SUMS`n"
        if ($actualPin -cne $script:ManifestPin) { throw 'release bundle manifest differs from the exact Enrollment Pin.' }
        $manifest = @{}
        foreach ($line in [IO.File]::ReadAllLines($manifestPath)) {
            if ($line -cnotmatch '^([0-9a-f]{64})  ([A-Za-z0-9._-]+)$') { throw 'release manifest line is not canonical.' }
            if ($manifest.ContainsKey($Matches[2])) { throw 'release manifest inventory contains a duplicate.' }
            $manifest[$Matches[2]] = $Matches[1]
        }
        $actualNames = @($bundleEntries | ForEach-Object Name)
        $expectedNames = @($manifest.Keys) + @('SHA256SUMS')
        if ($actualNames.Count -ne $expectedNames.Count) { throw 'release bundle and manifest inventory counts differ.' }
        foreach ($name in $actualNames) {
            if ($expectedNames -cnotcontains $name) { throw "release bundle inventory has unexpected entry $name." }
        }
        $verified = foreach ($name in $manifest.Keys | Sort-Object) {
            $digest = (Get-FileHash -LiteralPath (Join-Path $bundle $name) -Algorithm SHA256).Hash.ToLowerInvariant()
            if ($digest -cne $manifest[$name]) { throw "release bundle digest mismatch for $name." }
            [ordered]@{ name = $name; sha256 = $digest }
        }
        Write-Json (Join-Path $EvidencePath 'manifested-files.json') ([ordered]@{ schema = 'ardents-a11-release-files-v1'; files = @($verified) })
        Copy-Item -LiteralPath $manifestPath -Destination (Join-Path $EvidencePath 'SHA256SUMS')

        $descriptorPath = Join-Path $bundle 'RELEASE'
        if (-not (Test-Path -LiteralPath $descriptorPath -PathType Leaf)) { throw 'release descriptor is absent.' }
        $descriptor = Read-ReleaseDescriptor $descriptorPath
        foreach ($field in @('schema', 'cohort', 'release', 'platform', 'environment', 'network', 'target_path', 'artifact', 'control_artifact')) {
            if (-not $descriptor.ContainsKey($field)) { throw "release descriptor lacks $field." }
        }
        if ($descriptor.schema -cne 'ardents-closed-alpha-enrollment-v3' -or
            $descriptor.cohort -cne $script:Cohort -or $descriptor.release -cne $script:ReleaseTag -or
            $descriptor.platform -cne 'linux-amd64' -or $descriptor.target_path -cne 'ardents/linux-amd64/endpoint' -or
            $descriptor.artifact -cne 'ardents-linux-amd64' -or $descriptor.control_artifact -cne 'ardents-control-linux-amd64') {
            throw 'release descriptor differs from the selected A11 identity.'
        }
        $endpoint = (Get-FileHash -LiteralPath (Join-Path $bundle $descriptor.artifact) -Algorithm SHA256).Hash.ToLowerInvariant()
        $control = (Get-FileHash -LiteralPath (Join-Path $bundle $descriptor.control_artifact) -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($endpoint -cne $script:EndpointSHA256 -or $control -cne $script:ControlSHA256) {
            throw 'release Endpoint or control-companion digest differs from the selected A11 identity.'
        }
        Write-Json (Join-Path $EvidencePath 'release-descriptor.json') ([ordered]@{
            schema = 'ardents-a11-release-descriptor-v1'
            fields = $descriptor
            endpoint_sha256 = $endpoint
            control_sha256 = $control
        })
        $validatedBundle = $bundle
        $validatedDescriptor = $descriptor
    }
    finally {
        if (-not $retainExtraction -and (Test-Path -LiteralPath $validationRoot -PathType Container)) {
            $resolved = (Resolve-Path -LiteralPath $validationRoot).Path
            $prefix = [IO.Path]::GetTempPath().TrimEnd('\', '/') + [IO.Path]::DirectorySeparatorChar + 'ardents-h4-8-a11-archive-'
            if (-not $resolved.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase)) {
                throw 'archive validation root failed exact-path cleanup validation.'
            }
            Remove-Item -LiteralPath $resolved -Recurse -Force
        }
    }
    return [pscustomobject]@{ BundleRoot = $validatedBundle; Descriptor = $validatedDescriptor }
}

function Get-SSHOptions([string]$KnownHosts) {
    return @('-n', '-T', '-i', $script:keyPath, '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=15', '-o',
        'ConnectionAttempts=1', '-o', 'ServerAliveInterval=5', '-o', 'ServerAliveCountMax=6', '-o',
        'StrictHostKeyChecking=accept-new', '-o', "UserKnownHostsFile=$KnownHosts", '-o', 'LogLevel=ERROR')
}

function Invoke-SSH([string]$Command, [string]$Stdout, [string]$Stderr) {
    # Feed the exact retained script over stdin so multiline shell text never
    # depends on Windows command-line quoting. Remove -n because stdin is the
    # script channel for this invocation.
    $commandPath = $Stdout + '.command.sh'
    $normalized = $Command.Replace("`r`n", "`n").Replace("`r", "`n")
    Write-Utf8 $commandPath ($(if ($normalized.EndsWith("`n", [StringComparison]::Ordinal)) { $normalized } else { $normalized + "`n" }))
    $arguments = (Get-SSHOptions $script:knownHosts | Where-Object { $_ -ne '-n' }) + @($script:remote, 'sh', '-s')
    return Invoke-NativeCaptured $script:sshPath $arguments $Stdout $Stderr $script:repository $commandPath
}

function Get-WindowsProcessSample([int]$RootPID, [Diagnostics.Stopwatch]$Clock) {
    $captured = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
    try {
        $all = @(Get-CimInstance Win32_Process -ErrorAction Stop)
        $children = @{}
        foreach ($process in $all) {
            $parent = [int]$process.ParentProcessId
            if (-not $children.ContainsKey($parent)) { $children[$parent] = New-Object System.Collections.Generic.List[object] }
            $children[$parent].Add($process)
        }
        $selected = New-Object System.Collections.Generic.List[object]
        $queue = New-Object System.Collections.Generic.Queue[int]
        $queue.Enqueue($RootPID)
        $visited = New-Object 'System.Collections.Generic.HashSet[int]'
        while ($queue.Count -gt 0) {
            $processID = $queue.Dequeue()
            if (-not $visited.Add($processID)) { continue }
            $current = $all | Where-Object { [int]$_.ProcessId -eq $processID } | Select-Object -First 1
            if ($null -ne $current) { $selected.Add($current) }
            if ($children.ContainsKey($processID)) {
                foreach ($child in $children[$processID]) { $queue.Enqueue([int]$child.ProcessId) }
            }
        }
        $qualificationIDs = New-Object 'System.Collections.Generic.HashSet[int]'
        foreach ($process in $selected) { [void]$qualificationIDs.Add([int]$process.ProcessId) }
        $queue.Enqueue($PID)
        while ($queue.Count -gt 0) {
            $processID = $queue.Dequeue()
            if (-not $visited.Add($processID)) { continue }
            $current = $all | Where-Object { [int]$_.ProcessId -eq $processID } | Select-Object -First 1
            if ($null -ne $current) { $selected.Add($current) }
            if ($children.ContainsKey($processID)) {
                foreach ($child in $children[$processID]) { $queue.Enqueue([int]$child.ProcessId) }
            }
        }
        $facts = foreach ($process in $selected | Sort-Object ProcessId) {
            $processID = [int]$process.ProcessId
            [ordered]@{
                pid = $processID
                parent_pid = [int]$process.ParentProcessId
                role = $(if ($processID -eq $PID) { 'campaign-observer' } elseif ($processID -eq $RootPID) { 'go-test-root' } elseif ($qualificationIDs.Contains($processID)) { 'qualification-descendant' } else { 'campaign-helper' })
                name = [string]$process.Name
                creation_time_utc_ticks = $(if ($null -ne $process.CreationDate) { ([datetime]$process.CreationDate).ToUniversalTime().Ticks } else { 0L })
                cpu_100ns = [long]$process.KernelModeTime + [long]$process.UserModeTime
                working_set_bytes = [long]$process.WorkingSetSize
                private_bytes = [long]$process.PrivatePageCount
                handles = [long]$process.HandleCount
                threads = [long]$process.ThreadCount
            }
        }
        return [ordered]@{ timestamp_ms = $captured; monotonic_ms = $Clock.ElapsedMilliseconds; root_pid = $RootPID; processes = @($facts); error = $null }
    }
    catch {
        return [ordered]@{ timestamp_ms = $captured; monotonic_ms = $Clock.ElapsedMilliseconds; root_pid = $RootPID; processes = @(); error = $_.Exception.Message }
    }
}

function Test-WindowsProcessCleanup([string]$ResourcePath, [string]$EvidencePath) {
    $failures = New-Object System.Collections.Generic.List[string]
    $observed = @{}
    try {
        foreach ($line in [IO.File]::ReadAllLines($ResourcePath)) {
            if ([string]::IsNullOrWhiteSpace($line)) { continue }
            $row = $line | ConvertFrom-Json
            foreach ($process in @($row.processes)) {
                if ([string]$process.role -in @('campaign-observer', 'campaign-helper')) { continue }
                $observed[[int]$process.pid] = [long]$process.creation_time_utc_ticks
            }
        }
        $residue = New-Object System.Collections.Generic.List[object]
        foreach ($process in @(Get-CimInstance Win32_Process -ErrorAction Stop)) {
            $processID = [int]$process.ProcessId
            if (-not $observed.ContainsKey($processID)) { continue }
            $created = $(if ($null -ne $process.CreationDate) { ([datetime]$process.CreationDate).ToUniversalTime().Ticks } else { 0L })
            if ($created -eq $observed[$processID]) {
                $residue.Add([ordered]@{ pid = $processID; parent_pid = [int]$process.ParentProcessId; name = [string]$process.Name; creation_time_utc_ticks = $created })
            }
        }
        if ($residue.Count -ne 0) {
            $failures.Add('one or more observed qualification processes remained after go test exit.')
            foreach ($process in $residue) { Stop-Process -Id $process.pid -Force -ErrorAction SilentlyContinue }
        }
        Write-Json $EvidencePath ([ordered]@{
            schema = 'ardents-h4-8-a11-windows-process-cleanup-v1'
            observed_process_identities = $observed.Count
            residue = @($residue)
            emergency_stop_attempted = $residue.Count -ne 0
            failures = @($failures)
        })
    }
    catch {
        $failures.Add("Windows process residue inventory failed: $($_.Exception.Message)")
        Write-Json $EvidencePath ([ordered]@{ schema = 'ardents-h4-8-a11-windows-process-cleanup-v1'; residue = @(); failures = @($failures) })
    }
    return [pscustomobject]@{ Passed = $failures.Count -eq 0; Failures = @($failures) }
}

function Wait-ProcessBounded(
    [Diagnostics.Process]$Process,
    [Diagnostics.Stopwatch]$CampaignClock,
    [long]$DeadlineMilliseconds
) {
    try {
        $Process.Refresh()
        if ($Process.HasExited) {
            $exited = $Process.WaitForExit(0)
            return [pscustomobject]@{ Exited = $exited; ExitCode = $(if ($exited) { $Process.ExitCode } else { $null }); Error = $null }
        }
        $remainingMilliseconds = $DeadlineMilliseconds - $CampaignClock.ElapsedMilliseconds
        if ($remainingMilliseconds -le 0) {
            return [pscustomobject]@{ Exited = $false; ExitCode = $null; Error = 'bounded process wait had no remaining time.' }
        }
        $waitMilliseconds = [int][Math]::Min([long][int]::MaxValue, $remainingMilliseconds)
        $exited = $Process.WaitForExit($waitMilliseconds)
        if ($exited) { $Process.Refresh() }
        return [pscustomobject]@{ Exited = $exited; ExitCode = $(if ($exited) { $Process.ExitCode } else { $null }); Error = $null }
    }
    catch {
        return [pscustomobject]@{ Exited = $false; ExitCode = $null; Error = $_.Exception.Message }
    }
}

function Stop-OwnedProcessTree(
    [int]$RootPID,
    [string]$EvidencePath,
    [Diagnostics.Stopwatch]$CampaignClock,
    [long]$DeadlineMilliseconds,
    [int[]]$KnownPIDs = @()
) {
    $owned = New-Object 'System.Collections.Generic.HashSet[int]'
    $stopErrors = New-Object System.Collections.Generic.List[string]
    [void]$owned.Add($RootPID)
    foreach ($knownPID in $KnownPIDs) {
        if ($knownPID -gt 1) { [void]$owned.Add($knownPID) }
    }
    $taskkillExit = $null
    $taskkillFailure = $null
    $fallbackUsed = $false
    $taskkillProcess = $null
    $taskkillStdout = [IO.Path]::ChangeExtension($EvidencePath, 'taskkill.stdout.log')
    $taskkillStderr = [IO.Path]::ChangeExtension($EvidencePath, 'taskkill.stderr.log')
    try {
        if ($CampaignClock.ElapsedMilliseconds -ge $DeadlineMilliseconds) {
            $stopErrors.Add('process-tree stop had no remaining bounded cleanup time.')
        }
        else {
            $taskkillPath = Join-Path $env:WINDIR 'System32\taskkill.exe'
            $taskkillProcess = Start-Process -FilePath $taskkillPath -ArgumentList (Join-NativeArguments @('/PID', [string]$RootPID, '/T', '/F')) `
                -RedirectStandardOutput $taskkillStdout -RedirectStandardError $taskkillStderr -WindowStyle Hidden -PassThru
            $taskkillWait = Wait-ProcessBounded $taskkillProcess $CampaignClock $DeadlineMilliseconds
            if (-not $taskkillWait.Exited) {
                $taskkillFailure = 'taskkill did not exit within the bounded cleanup deadline.'
                try { Stop-Process -Id $taskkillProcess.Id -Force -ErrorAction Stop }
                catch { $stopErrors.Add("taskkill process fallback stop: $($_.Exception.Message)") }
            }
            else {
                $taskkillExit = $taskkillWait.ExitCode
                if ($taskkillExit -ne 0) { $taskkillFailure = "taskkill exited $taskkillExit." }
            }
        }
    }
    catch { $stopErrors.Add("process-tree stop: $($_.Exception.Message)") }
    foreach ($path in @($taskkillStdout, $taskkillStderr)) {
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { Write-Utf8 $path '' }
    }

    if ($null -ne $taskkillFailure) {
        $fallbackUsed = $true
        $ordered = @($owned | Where-Object { $_ -ne $RootPID } | Sort-Object -Descending) + @($RootPID)
        foreach ($processID in $ordered) {
            if ($CampaignClock.ElapsedMilliseconds -ge $DeadlineMilliseconds) {
                $stopErrors.Add("direct known-PID fallback lacked time for pid $processID.")
                continue
            }
            if ($null -eq (Get-Process -Id $processID -ErrorAction SilentlyContinue)) { continue }
            try { Stop-Process -Id $processID -Force -ErrorAction Stop }
            catch { $stopErrors.Add("direct known-PID fallback for pid $processID`: $($_.Exception.Message)") }
        }
    }

    $survivors = @()
    do {
        $survivors = @($owned | Where-Object { $null -ne (Get-Process -Id $_ -ErrorAction SilentlyContinue) } | Sort-Object)
        if ($survivors.Count -eq 0 -or $CampaignClock.ElapsedMilliseconds -ge $DeadlineMilliseconds) { break }
        $remainingMilliseconds = $DeadlineMilliseconds - $CampaignClock.ElapsedMilliseconds
        Start-Sleep -Milliseconds ([Math]::Min(100, [Math]::Max(1, $remainingMilliseconds)))
    } while ($true)

    $receipt = [ordered]@{
        schema = 'ardents-h4-8-a11-process-stop-v1'
        requested_root_pid = $RootPID
        owned_pids = @($owned | Sort-Object)
        stopped_pids = @($owned | Where-Object { $survivors -notcontains $_ } | Sort-Object)
        survivor_pids = @($survivors)
        stop_errors = @($stopErrors)
        taskkill_exit = $taskkillExit
        taskkill_failure = $taskkillFailure
        direct_known_pid_fallback_used = $fallbackUsed
        deadline_met = $CampaignClock.ElapsedMilliseconds -le $DeadlineMilliseconds
        passed = $survivors.Count -eq 0 -and $stopErrors.Count -eq 0 -and $CampaignClock.ElapsedMilliseconds -le $DeadlineMilliseconds
    }
    Write-Json $EvidencePath $receipt
    $validation = Test-ProcessStopReceipt ([pscustomobject]$receipt)
    return [pscustomobject]@{ Passed = $validation.Passed; Receipt = $receipt; Failures = @($validation.Failures) }
}

function Start-RemoteObserver([string]$Container, [string]$AttemptPath, [int]$LiveSeconds) {
    $stdout = Join-Path $AttemptPath 'ubuntu-resources.tsv'
    $stderr = Join-Path $AttemptPath 'ubuntu-observer.stderr.log'
    $arguments = (Get-SSHOptions $script:knownHosts | Where-Object { $_ -ne '-n' }) + @(
        $script:remote,
        "sh -s -- $(ConvertTo-ShellLiteral $Container) $(ConvertTo-ShellLiteral $script:RemoteImageID) 300 $LiveSeconds"
    )
    $process = Start-Process -FilePath $script:sshPath -ArgumentList (Join-NativeArguments $arguments) -WorkingDirectory $script:repository `
        -RedirectStandardInput (Join-Path $PSScriptRoot 'observe-ubuntu.sh') -RedirectStandardOutput $stdout -RedirectStandardError $stderr `
        -WindowStyle Hidden -PassThru
    return [pscustomobject]@{ Process = $process; Stdout = $stdout; Stderr = $stderr }
}

function Set-A11Environment([hashtable]$Values) {
    $prior = @{}
    foreach ($name in $Values.Keys) {
        $prior[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
        [Environment]::SetEnvironmentVariable($name, [string]$Values[$name], 'Process')
    }
    return $prior
}

function Restore-A11Environment([hashtable]$Prior) {
    foreach ($name in $Prior.Keys) { [Environment]::SetEnvironmentVariable($name, $Prior[$name], 'Process') }
}

function Invoke-RemoteAttemptPreflight([object]$Attempt, [string]$Container, [string]$RemoteRoot, [string]$AttemptPath) {
    $ports = for ($offset = 0; $offset -lt 8; $offset++) { $Attempt.Port + $offset }
    $portPattern = ':(' + (($ports | ForEach-Object { [string]$_ }) -join '|') + ')[[:space:]]'
    $command = @"
set -eu
root=$(ConvertTo-ShellLiteral $RemoteRoot)
container=$(ConvertTo-ShellLiteral $Container)
printf 'remote_root=%s\ncontainer=%s\nports=%s\n' "`$root" "`$container" $(ConvertTo-ShellLiteral (($ports -join ',')))
docker info >/dev/null
if [ -e "`$root" ]; then printf '%s\n' 'remote_root_state=preexisting' >&2; exit 20; fi
if docker container inspect "`$container" >/dev/null 2>&1; then printf '%s\n' 'container_state=preexisting' >&2; exit 21; fi
if ss -ltnH | grep -E $(ConvertTo-ShellLiteral $portPattern) >/dev/null; then printf '%s\n' 'selected_port_state=occupied' >&2; exit 22; fi
printf '%s\n' 'remote_root_state=absent' 'container_state=absent' 'selected_ports_state=free'
"@
    $stdout = Join-Path $AttemptPath 'remote-preflight.stdout.log'
    $stderr = Join-Path $AttemptPath 'remote-preflight.stderr.log'
    $exit = Invoke-SSH $command $stdout $stderr
    Write-Utf8 (Join-Path $AttemptPath 'remote-preflight.exitcode') "$exit`n"
    return $exit
}

function Invoke-RemotePostInventory([object]$Attempt, [string]$Container, [string]$RemoteRoot, [string]$AttemptPath) {
    $ports = for ($offset = 0; $offset -lt 8; $offset++) { $Attempt.Port + $offset }
    $portPattern = ':(' + (($ports | ForEach-Object { [string]$_ }) -join '|') + ')[[:space:]]'
    $command = @"
set +e
root=$(ConvertTo-ShellLiteral $RemoteRoot)
container=$(ConvertTo-ShellLiteral $Container)
residue=0
docker info >/dev/null 2>&1 || { printf '%s\n' 'docker_daemon_state=unavailable' >&2; exit 50; }
printf 'remote_root=%s\ncontainer=%s\n' "`$root" "`$container"
if docker container inspect "`$container" >/dev/null 2>&1; then
  printf '%s\n' 'container_state=present'
  docker container inspect "`$container"
  residue=1
else
  printf '%s\n' 'container_state=absent'
fi
if [ -e "`$root" ]; then
  printf '%s\n' 'remote_root_state=present'
  find "`$root" -xdev -printf '%y %m %s %p\n' | LC_ALL=C sort
  find "`$root" -xdev -type f -exec sha256sum {} \; | LC_ALL=C sort
  printf '%s\n' '[residue-role-output]'
  for file in "`$root"/*.log "`$root"/*.err; do
    if [ -f "`$file" ]; then printf '===%s===\n' "`$(basename "`$file")"; cat "`$file"; fi
  done
  residue=1
else
  printf '%s\n' 'remote_root_state=absent'
fi
if ss -ltnH | grep -E $(ConvertTo-ShellLiteral $portPattern) >/dev/null; then
  printf '%s\n' 'selected_ports_state=listening'
  ss -ltnp | grep -E $(ConvertTo-ShellLiteral $portPattern) || true
  residue=1
else
  printf '%s\n' 'selected_ports_state=free'
fi
exit "`$residue"
"@
    $stdout = Join-Path $AttemptPath 'remote-post-inventory.stdout.log'
    $stderr = Join-Path $AttemptPath 'remote-post-inventory.stderr.log'
    $exit = Invoke-SSH $command $stdout $stderr
    Write-Utf8 (Join-Path $AttemptPath 'remote-post-inventory.exitcode') "$exit`n"
    return $exit
}

function Invoke-ExactRemoteCleanup([object]$Attempt, [string]$Suffix, [string]$Container, [string]$RemoteRoot, [string]$AttemptPath) {
    $ports = for ($offset = 0; $offset -lt 8; $offset++) { $Attempt.Port + $offset }
    $portPattern = ':(' + (($ports | ForEach-Object { [string]$_ }) -join '|') + ')[[:space:]]'
    $command = @"
set -eu
suffix=$(ConvertTo-ShellLiteral $Suffix)
root=$(ConvertTo-ShellLiteral $RemoteRoot)
container=$(ConvertTo-ShellLiteral $Container)
case "`$suffix" in *[!0-9a-f]*|'') exit 30 ;; esac
[ "`${#suffix}" -ge 12 ] && [ "`${#suffix}" -le 32 ]
[ "`$root" = "/tmp/ardents-h4-8-a11-`$suffix" ]
[ "`$container" = "ardents-h4-8-a11-`$suffix" ]
docker info >/dev/null
docker rm -f "`$container" >/dev/null 2>&1 || true
if [ -e "`$root" ]; then rm -rf -- "`$root"; fi
test ! -e "`$root"
! docker container inspect "`$container" >/dev/null 2>&1
docker info >/dev/null
! ss -ltnH | grep -E $(ConvertTo-ShellLiteral $portPattern) >/dev/null
printf '%s\n' 'remote_root=absent' 'container=absent' 'selected_ports=free'
"@
    $stdout = Join-Path $AttemptPath 'remote-cleanup.stdout.log'
    $stderr = Join-Path $AttemptPath 'remote-cleanup.stderr.log'
    $exit = Invoke-SSH $command $stdout $stderr
    Write-Utf8 (Join-Path $AttemptPath 'remote-cleanup.exitcode') "$exit`n"
    return $exit
}

function Test-A11Status(
    [string]$StatusRoot,
    [string]$TestName,
    [string]$Container,
    [string]$RemoteRoot,
    [object]$WindowsResult,
    [bool]$RemoteAttempt
) {
    $failures = New-Object System.Collections.Generic.List[string]
    try {
    if ($RemoteAttempt) {
        $expected = @{
            'contract.ready' = "test=$TestName`ncontainer=$Container`nremote_directory=$RemoteRoot`n"
            'remote.ready' = "container=$Container`nremote_directory=$RemoteRoot`n"
            'complete' = "passed=true`n"
        }
        foreach ($name in $expected.Keys) {
            $path = Join-Path $StatusRoot $name
            if (-not (Test-Path -LiteralPath $path -PathType Leaf) -or [IO.File]::ReadAllText($path) -cne $expected[$name]) {
                $failures.Add("A11 status $name is absent or differs from the exact attempt identity.")
            }
        }
        $userPath = Join-Path $StatusRoot 'user.ready'
        if (-not (Test-Path -LiteralPath $userPath -PathType Leaf) -or [IO.File]::ReadAllText($userPath) -cnotmatch '^pid=([1-9][0-9]*)\n$') {
            $failures.Add('A11 status user.ready is absent or malformed.')
        }
        else {
            $userPID = [int]$Matches[1]
            if (@($WindowsResult.ObservedPIDs) -notcontains $userPID) {
                $failures.Add("Windows observer never recorded the declared User PID $userPID.")
            }
        }
        $userEvidenceRoot = Join-Path $StatusRoot 'user-evidence'
        $userEvidenceMarker = Join-Path $StatusRoot 'user-evidence.complete'
        if (-not (Test-Path -LiteralPath $userEvidenceRoot -PathType Container) -or
            -not (Test-Path -LiteralPath $userEvidenceMarker -PathType Leaf) -or
            [IO.File]::ReadAllText($userEvidenceMarker) -cne "schema=ardents-h4-8-a11-user-evidence-v1`nretained=user-evidence/stdout.log,user-evidence/stderr.log,user-evidence/process.txt`n") {
            $failures.Add('service harness did not retain the exact local User evidence contract.')
        }
        else {
            $userStdout = Join-Path $userEvidenceRoot 'stdout.log'
            $userStderr = Join-Path $userEvidenceRoot 'stderr.log'
            $userProcess = Join-Path $userEvidenceRoot 'process.txt'
            if (-not (Test-Path -LiteralPath $userStdout -PathType Leaf) -or
                -not (Test-Path -LiteralPath $userStderr -PathType Leaf) -or
                ((Get-Item -LiteralPath $userStdout).Length + (Get-Item -LiteralPath $userStderr).Length) -gt 1048576 -or
                -not (Test-Path -LiteralPath $userProcess -PathType Leaf) -or
                [IO.File]::ReadAllText($userProcess) -cne "schema=ardents-h4-8-a11-user-evidence-v1`nstreams=separate`ndisposition=success`nexit_code=0`n") {
                $failures.Add('retained local User output/status is incomplete, oversized, or not a successful exit.')
            }
        }
        # The service harness owns this copy-before-cleanup seam. Without it a
        # successful go test would erase the role logs before the checked
        # orchestrator can retain the frozen evidence contract.
        $remoteEvidenceMarker = Join-Path $StatusRoot 'remote-evidence.complete'
        $remoteEvidenceRoot = Join-Path $StatusRoot 'remote-evidence'
        if (-not (Test-Path -LiteralPath $remoteEvidenceMarker -PathType Leaf) -or
            -not (Test-Path -LiteralPath $remoteEvidenceRoot -PathType Container)) {
            $failures.Add('service harness did not retain complete remote evidence before cleanup.')
        }
        elseif ([IO.File]::ReadAllText($remoteEvidenceMarker) -cne "schema=ardents-h4-8-a11-remote-evidence-v1`nretained=remote-evidence/capture.txt`n") {
            $failures.Add('service harness remote-evidence marker is not the exact v1 contract.')
        }
        else {
            $capturePath = Join-Path $remoteEvidenceRoot 'capture.txt'
            if (-not (Test-Path -LiteralPath $capturePath -PathType Leaf)) {
                $failures.Add('service harness remote-evidence capture is absent.')
            }
            else {
                $captureInfo = Get-Item -LiteralPath $capturePath
                $capture = [IO.File]::ReadAllText($capturePath)
                if ($captureInfo.Length -lt 1 -or $captureInfo.Length -gt 1048576 -or
                    -not $capture.StartsWith("schema=ardents-h4-8-a11-remote-evidence-v1`n[container-state]`n", [StringComparison]::Ordinal) -or
                    $capture.IndexOf("`n[staged-inventory-sha256]`n", [StringComparison]::Ordinal) -lt 0 -or
                    $capture.IndexOf("`n[role-output]`n", [StringComparison]::Ordinal) -lt 0) {
                    $failures.Add('service harness remote-evidence capture is malformed or outside its 1 MiB bound.')
                }
                else {
                    $inventoryMarker = "`n[staged-inventory-sha256]`n"
                    $roleMarker = "`n[role-output]`n"
                    $statePrefix = "schema=ardents-h4-8-a11-remote-evidence-v1`n[container-state]`n"
                    $stateEnd = $capture.IndexOf($inventoryMarker, [StringComparison]::Ordinal)
                    try { $containerState = $capture.Substring($statePrefix.Length, $stateEnd - $statePrefix.Length).Trim() | ConvertFrom-Json }
                    catch { $containerState = $null; $failures.Add('remote retained Docker state is not valid JSON.') }
                    if ($null -ne $containerState -and ([bool]$containerState.OOMKilled -or [bool]$containerState.Restarting -or
                        [bool]$containerState.Dead -or [int]$containerState.ExitCode -ne 0 -or [string]$containerState.Status -cne 'exited')) {
                        $failures.Add('remote retained Docker terminal state is not a clean non-OOM exit.')
                    }
                    $inventoryStart = $stateEnd + $inventoryMarker.Length
                    $inventoryEnd = $capture.IndexOf($roleMarker, $inventoryStart, [StringComparison]::Ordinal)
                    $inventoryNames = New-Object 'System.Collections.Generic.HashSet[string]' ([StringComparer]::Ordinal)
                    $inventorySizes = @{}
                    foreach ($line in $capture.Substring($inventoryStart, $inventoryEnd - $inventoryStart).Split("`n", [StringSplitOptions]::RemoveEmptyEntries)) {
                        if ($line -cnotmatch '^([0-9a-f]{64})\t([0-9]+)\t([A-Za-z0-9._-]+(?:/[A-Za-z0-9._-]+)*)$' -or
                            -not $inventoryNames.Add($Matches[3])) {
                            $failures.Add('remote staged inventory has a malformed or duplicate digest row.')
                            break
                        }
                        $inventorySizes[$Matches[3]] = [long]$Matches[2]
                    }
                    $requiredInventory = New-Object System.Collections.Generic.List[string]
                    foreach ($requiredName in @('ardents-node', 'reference-c2', 'reference-c2.json', 'run.sh', 'topology.json', 'rendezvous-node.pid', 'carrier-relay.pid', 'publisher.pid', 'publisher-app.pid', 'carrier-relay-ready.json')) {
                        $requiredInventory.Add($requiredName)
                    }
                    if ($TestName -ceq 'TestH48A11MultiHostPublisherApplicationLossAfterWarmup') {
                        foreach ($requiredName in @('publisher-application-fault-ready', 'fault-injection.json', 'publisher-application-reset.json')) { $requiredInventory.Add($requiredName) }
                    }
                    if ($TestName -ceq 'TestH48A11MultiHostCarrierLossAfterWarmup') {
                        foreach ($requiredName in @('transit-fault-ready', 'fault-injection.json', 'carrier-relay-reset.json')) { $requiredInventory.Add($requiredName) }
                    }
                    if ($TestName -ceq 'TestH48A11MultiHostProductNodeLossAfterWarmup') {
                        foreach ($requiredName in @('transit-fault-ready', 'fault-injection.json', 'rendezvous-node-kill.json')) { $requiredInventory.Add($requiredName) }
                    }
                    if ($TestName -ceq 'TestH48A11MultiHostPublisherEndpointLossAfterWarmup') {
                        foreach ($requiredName in @('publisher-crash-ready', 'fault-injection.json', 'publisher-endpoint-kill.json')) { $requiredInventory.Add($requiredName) }
                    }
                    foreach ($requiredName in $requiredInventory) {
                        if (-not $inventoryNames.Contains($requiredName) -or [long]$inventorySizes[$requiredName] -le 0) {
                            $failures.Add("remote staged inventory lacks non-empty $requiredName digest evidence.")
                        }
                    }
                }
                foreach ($role in @('source-a', 'source-b', 'rendezvous-node', 'carrier-relay', 'initiator', 'introduction', 'responder', 'gateway', 'publisher', 'alpha-gateway', 'alpha-relay', 'publisher-app')) {
                    foreach ($extension in @('log', 'err')) {
                        if ($capture.IndexOf("===$role.$extension===`n", [StringComparison]::Ordinal) -lt 0) {
                            $failures.Add("remote evidence lacks complete $role.$extension section.")
                        }
                    }
                }
            }
        }
    }
    else {
        foreach ($remoteMarker in @('contract.ready', 'remote.ready', 'user.ready', 'remote-evidence.complete', 'remote-evidence')) {
            if (Test-Path -LiteralPath (Join-Path $StatusRoot $remoteMarker)) {
                $failures.Add("deterministic expiry unexpectedly claimed live-remote status $remoteMarker.")
            }
        }
    }
    }
    catch {
        $failures.Add("retained A11 status/evidence could not be parsed: $($_.Exception.Message)")
    }
    return [pscustomobject]@{ Passed = $failures.Count -eq 0; Failures = @($failures) }
}

function Test-A11ProductHarnessSeams(
    [string]$QualificationSource,
    [string]$EvidenceSource,
    [string]$RunnerSource
) {
    $failures = New-Object System.Collections.Generic.List[string]
    $requirements = [ordered]@{
        qualification = @(
            'func TestH48A11MultiHostPublisherApplicationLossAfterWarmup(t *testing.T)',
            'productRendezvousRelay: true'
        )
        evidence = @(
            'publisherAppPID := h48A11RemotePID(t, remote, "publisher-app.pid")',
            'fault.Fault != "publisher-application-loss"',
            'fault.TargetRole != "publisher-app"',
            'fault.TargetPID != publisherAppPID',
            'fault.Signal != "RESET"',
            'fault.ReadyMarker != "publisher-application-fault-ready"',
            'fault.WarmupCycles != scenario.dynamicWorkload.Cycles',
            'reset.Schema != "ardents-h4-8-a11-publisher-application-reset-v1"',
            'reset.Fault != "publisher-application-loss"',
            'reset.Action != "RESET"',
            'reset.PID != publisherAppPID',
            '!reset.PublisherLiveBefore',
            '!reset.PublisherAppLiveBefore',
            '!reset.RendezvousNodeLiveBefore',
            '!reset.CarrierRelayLiveBefore',
            '!reset.TransitRolesLiveBefore',
            '!reset.InjectedAfterReady',
            'h48A11AssertKillReceipt(t, killed, "ardents-h4-8-a11-publisher-endpoint-kill-v1", "publisher-endpoint-loss", fault.TargetPID)',
            'reset.Schema != "ardents-h4-8-a11-carrier-relay-reset-v1"',
            'reset.ResetCount != 1',
            'reset.ResetBridges != 2',
            'reset.ActiveBefore != 2',
            'reset.SelectedBridgeID != 0',
            'reset.ActiveBridges != 0',
            '!reset.ListenerLive',
            'relay.ResetBridges != 2',
            'relay.ActiveBefore != 2',
            'relay.SelectedBridgeID != 0',
            'relay.AcceptedAfterReset != 0',
            '!relay.ListenerLiveAfterReset',
            'h48A11AssertKillReceipt(t, killed, "ardents-h4-8-a11-rendezvous-kill-v1", "product-node-loss", nodePID)',
            'receipt.Schema != "ardents-h4-8-a11-fault-injection-v1"',
            'receipt.Signal != "KILL"',
            'receipt.ExitStatus != 137',
            '!receipt.PublisherLiveBefore',
            '!receipt.PublisherAppLiveBefore',
            '!receipt.RendezvousNodeLiveBefore',
            '!receipt.CarrierRelayLiveBefore',
            '!receipt.InjectedAfterReady'
        )
        runner = @(
            '"$publisher" >"$work/publisher.pid"',
            '"$publisher_app" >"$work/publisher-app.pid"',
            'wait_fault_file "$work/publisher-application-fault-ready"',
            'write_fault_injection publisher-application-loss publisher-app "$publisher_app" RESET publisher-application-fault-ready',
            '"schema":"ardents-h4-8-a11-publisher-application-reset-v1"',
            '"fault":"publisher-application-loss"',
            '"action":"RESET"',
            '"publisher_live_before":true',
            '"publisher_app_live_before":true',
            '"rendezvous_node_live_before":true',
            '"carrier_relay_live_before":true',
            '"transit_roles_live_before":true',
            '"injected_after_ready":true',
            '>"$work/publisher-application-reset.json"',
            '>"$work/publisher-application-fault-release"'
        )
    }
    $sources = @{ qualification = $QualificationSource; evidence = $EvidenceSource; runner = $RunnerSource }
    $required = New-Object System.Collections.Generic.List[string]
    foreach ($group in $requirements.Keys) {
        foreach ($literal in $requirements[$group]) {
            $label = "$group`:$literal"
            $required.Add($label)
            if ($sources[$group].IndexOf($literal, [StringComparison]::Ordinal) -lt 0) {
                $failures.Add("checked A11 $group source lacks exact product seam: $literal")
            }
        }
    }
    $application = [regex]::Match($QualificationSource,
        '(?s)func TestH48A11MultiHostPublisherApplicationLossAfterWarmup\(t \*testing\.T\) \{(?<body>.*?)\r?\n\}')
    if (-not $application.Success -or $application.Groups['body'].Value -cnotmatch 'Cycles:\s*60(?:\s|,)') {
        $failures.Add('Application-loss primary is not bound to exactly 60 warmup cycles.')
    }
    return [pscustomobject]@{ Passed = $failures.Count -eq 0; Required = @($required); Failures = @($failures) }
}

function Write-EvidenceInventory([string]$Root) {
    $inventoryPath = Join-Path $Root 'evidence-sha256.txt'
    $records = foreach ($file in Get-ChildItem -LiteralPath $Root -Recurse -File |
        Where-Object FullName -ne $inventoryPath | Sort-Object FullName) {
        $relative = $file.FullName.Substring($Root.Length).TrimStart('\', '/').Replace('\', '/')
        '{0}  {1}' -f (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant(), $relative
    }
    Write-Utf8 $inventoryPath (($records -join "`n") + "`n")
}

function Invoke-A11Attempt(
    [object]$Attempt,
    [int]$Ordinal,
    [string]$EvidenceRoot,
    [Diagnostics.Stopwatch]$CampaignClock,
    [long]$CampaignLimitMilliseconds
) {
    $attemptName = '{0:D2}-{1}' -f ($Ordinal + 1), $Attempt.Attempt
    $attemptPath = Join-Path (Join-Path $EvidenceRoot $Attempt.Cell) $attemptName
    $statusRoot = Join-Path $attemptPath 'status'
    [IO.Directory]::CreateDirectory($statusRoot) | Out-Null
    $suffix = [Guid]::NewGuid().ToString('N').Substring(0, 16)
    $container = "ardents-h4-8-a11-$suffix"
    $remoteRoot = "/tmp/$container"
    $startedAt = [DateTimeOffset]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
    $failures = New-Object System.Collections.Generic.List[string]
    $preflightExit = $null
    $observer = $null
    $observerExit = $null
    $observerStopped = $false
    $goProcess = $null
    $goExit = $null
    $timedOut = $false
    $priorEnvironment = $null
    $windowsResult = [pscustomobject]@{ Passed = $false; Samples = 0; ObservedPIDs = @(); Failures = @('Windows process was not started.') }
    $windowsCleanupResult = [pscustomobject]@{ Passed = $false; Failures = @('Windows process cleanup was not inspected.') }
    $windowsCleanupInspected = $false
    $ubuntuResult = [pscustomobject]@{ Passed = -not $Attempt.Remote; Samples = 0; MaximumMemory = 0; MaximumPIDs = 0; Failures = @() }
    $statusResult = [pscustomobject]@{ Passed = $false; Failures = @('status was not validated.') }
    $postExit = $null
    $cleanupExit = $null
    $ownershipEstablished = $false
    $effectiveDeadlineSeconds = 0
    $goNaturalDeadline = $CampaignLimitMilliseconds - $script:campaignCleanupReserveMilliseconds
    $goCompletionDeadline = $CampaignLimitMilliseconds

    Write-Json (Join-Path $attemptPath 'attempt-input.json') ([ordered]@{
        schema = 'ardents-h4-8-a11-sub-attempt-v1'
        ordinal = $Ordinal + 1
        denominator_cell = $Attempt.Cell
        attempt_role = $Attempt.Attempt
        exact_test = $Attempt.Test
        build_tag = 'h4_8_a11'
        suffix = $suffix
        status_root = 'status'
        remote = [bool]$Attempt.Remote
        remote_container = $(if ($Attempt.Remote) { $container } else { $null })
        remote_directory = $(if ($Attempt.Remote) { $remoteRoot } else { $null })
        candidate_reference_at = $(if ($Attempt.Remote) { $null } else { $script:At })
        exact_candidate_bundle = $(if ($Attempt.Remote) { $null } else { $script:expiryBundleRoot.Substring($attemptPath.Length).TrimStart('\', '/').Replace('\', '/') })
        selected_ports = $(if ($Attempt.Remote) { @($Attempt.Port..($Attempt.Port + 7)) } else { @() })
        go_timeout = $Attempt.GoTimeout
        orchestration_deadline_seconds = $Attempt.DeadlineSeconds
        started_at_utc = $startedAt
        retry_ordinal = 0
    })

    if (-not $Attempt.Remote) {
        Write-Json (Join-Path $attemptPath 'remote-scope.json') ([ordered]@{
            schema = 'ardents-h4-8-a11-remote-scope-v1'
            applicable = $false
            reason = 'deterministic-expiry-boundary-has-no-live-remote-topology'
            container = $null
            remote_directory = $null
            ubuntu_resource_series = 'not-applicable'
            cleanup_inventory = 'not-applicable'
        })
    }

    try {
        $effectiveDeadlineSeconds = Get-BoundedAttemptSeconds $Attempt.DeadlineSeconds $CampaignClock.ElapsedMilliseconds `
            $CampaignLimitMilliseconds $script:campaignCleanupReserveMilliseconds
        if ($effectiveDeadlineSeconds -lt 1) {
            $failures.Add('the fixed 125-minute campaign deadline elapsed before this distinct sub-attempt could start.')
            throw 'A11_CAMPAIGN_DEADLINE'
        }
        $attemptCandidateDirty = @(& git -C $script:candidateRepository status --porcelain=v1 --untracked-files=all)
        $candidateStatusExit = $LASTEXITCODE
        $attemptCandidateHead = (& git -C $script:candidateRepository rev-parse --verify HEAD).Trim()
        $candidateHeadExit = $LASTEXITCODE
        $attemptTag = (& git -C $script:candidateRepository rev-parse --verify "refs/tags/$($script:ReleaseTag)^{commit}").Trim()
        $tagExit = $LASTEXITCODE
        $attemptHarnessDirty = @(& git -C $script:repository status --porcelain=v1 --untracked-files=all)
        $harnessStatusExit = $LASTEXITCODE
        $attemptHarnessHead = (& git -C $script:repository rev-parse --verify HEAD).Trim()
        $harnessHeadExit = $LASTEXITCODE
        Write-Json (Join-Path $attemptPath 'source-preflight.json') ([ordered]@{
            candidate_status_exit = $candidateStatusExit
            candidate_dirty_entries = @($attemptCandidateDirty)
            candidate_head_exit = $candidateHeadExit
            candidate_head = $attemptCandidateHead
            tag_exit = $tagExit
            tag_commit = $attemptTag
            expected_candidate = $script:SourceRevision
            harness_status_exit = $harnessStatusExit
            harness_dirty_entries = @($attemptHarnessDirty)
            harness_head_exit = $harnessHeadExit
            harness_head = $attemptHarnessHead
        })
        if ($candidateStatusExit -ne 0 -or $attemptCandidateDirty.Count -ne 0 -or $candidateHeadExit -ne 0 -or $tagExit -ne 0 -or
            $attemptCandidateHead -cne $script:SourceRevision -or $attemptTag -cne $script:SourceRevision -or
            $harnessStatusExit -ne 0 -or $attemptHarnessDirty.Count -ne 0 -or $harnessHeadExit -ne 0) {
            $failures.Add('exact candidate or committed harness identity changed before this sub-attempt; no topology was launched.')
            throw 'A11_ATTEMPT_SOURCE'
        }
        if ($Attempt.Remote) {
            $preflightExit = Invoke-RemoteAttemptPreflight $Attempt $container $remoteRoot $attemptPath
            if ($preflightExit -ne 0) {
                $failures.Add("remote attempt preflight failed with exit $preflightExit; no topology was launched.")
                throw 'A11_ATTEMPT_PREFLIGHT'
            }
            $ownershipEstablished = $true
            $effectiveDeadlineSeconds = Get-BoundedAttemptSeconds $Attempt.DeadlineSeconds $CampaignClock.ElapsedMilliseconds `
                $CampaignLimitMilliseconds $script:campaignCleanupReserveMilliseconds
            if ($effectiveDeadlineSeconds -lt 1) {
                $failures.Add('the fixed 125-minute campaign deadline elapsed after remote ownership preflight.')
                throw 'A11_CAMPAIGN_DEADLINE'
            }
            $observer = Start-RemoteObserver $container $attemptPath $effectiveDeadlineSeconds
            Write-Utf8 (Join-Path $attemptPath 'ubuntu-observer.pid') "$($observer.Process.Id)`n"
        }

        $environmentValues = @{
            'ARDENTS_H4_8_A11_SUFFIX' = $suffix
            'ARDENTS_H4_8_A11_STATUS_ROOT' = $statusRoot
            'GOENV' = 'off'
            'GOTOOLCHAIN' = 'local'
            'GOFLAGS' = '-mod=readonly'
        }
        if ($Attempt.Remote) {
            $environmentValues['ARDENTS_H4_3B_VPS'] = $script:VPS
            $environmentValues['ARDENTS_H4_3B_SSH_KEY'] = $script:keyPath
            $environmentValues['ARDENTS_H4_3B_SSH'] = $script:sshPath
            $environmentValues['ARDENTS_H4_3B_VPS_PORT'] = [string]$Attempt.Port
            $environmentValues['ARDENTS_H4_3B_VPS_USER'] = $script:User
        }
        else {
            $environmentValues['ARDENTS_H4_8_A11_CANDIDATE_BUNDLE_ROOT'] = $script:expiryBundleRoot
            $environmentValues['ARDENTS_H4_8_A11_CANDIDATE_MANIFEST_PIN'] = $script:ManifestPin
            $environmentValues['ARDENTS_H4_8_A11_CANDIDATE_COHORT'] = $script:Cohort
            $environmentValues['ARDENTS_H4_8_A11_CANDIDATE_RELEASE'] = $script:ReleaseTag
            $environmentValues['ARDENTS_H4_8_A11_CANDIDATE_PLATFORM'] = $script:candidateDescriptor.platform
            $environmentValues['ARDENTS_H4_8_A11_CANDIDATE_ENVIRONMENT'] = $script:candidateDescriptor.environment
            $environmentValues['ARDENTS_H4_8_A11_CANDIDATE_NETWORK'] = $script:candidateDescriptor.network
            $environmentValues['ARDENTS_H4_8_A11_CANDIDATE_TARGET_PATH'] = $script:candidateDescriptor.target_path
            $environmentValues['ARDENTS_H4_8_A11_CANDIDATE_ARCHITECTURE'] = 'amd64'
            $environmentValues['ARDENTS_H4_8_A11_CANDIDATE_REFERENCE_AT'] = $script:At
            $environmentValues['ARDENTS_H4_8_A11_CANDIDATE_ENDPOINT_SHA256'] = $script:EndpointSHA256
            $environmentValues['ARDENTS_H4_8_A11_CANDIDATE_CONTROL_SHA256'] = $script:ControlSHA256
        }
        $priorEnvironment = Set-A11Environment $environmentValues
        $goArguments = @('test', '-v', '-tags=h4_8_a11', '-run', "^$($Attempt.Test)`$", '-count=1', "-timeout=$($Attempt.GoTimeout)", './tests/e2e/service')
        Write-Json (Join-Path $attemptPath 'go-invocation.json') ([ordered]@{
            executable = [IO.Path]::GetFileName($script:goPath)
            arguments = $goArguments
            working_tree_revision = $attemptHarnessHead
            candidate_source_revision = $script:SourceRevision
            environment_names = @($environmentValues.Keys | Sort-Object)
            ssh_key_value_retained = $false
        })
        $effectiveDeadlineSeconds = Get-BoundedAttemptSeconds $Attempt.DeadlineSeconds $CampaignClock.ElapsedMilliseconds `
            $CampaignLimitMilliseconds $script:campaignCleanupReserveMilliseconds
        if ($effectiveDeadlineSeconds -lt 1) {
            $failures.Add('the fixed 125-minute campaign deadline elapsed before the exact Go invocation.')
            throw 'A11_CAMPAIGN_DEADLINE'
        }
        $goStdout = Join-Path $attemptPath 'go-test.stdout.log'
        $goStderr = Join-Path $attemptPath 'go-test.stderr.log'
        $goProcess = Start-Process -FilePath $script:goPath -ArgumentList (Join-NativeArguments $goArguments) -WorkingDirectory $script:repository `
            -RedirectStandardOutput $goStdout -RedirectStandardError $goStderr -WindowStyle Hidden -PassThru
        Write-Utf8 (Join-Path $attemptPath 'go-test.pid') "$($goProcess.Id)`n"
        $resourcePath = Join-Path $attemptPath 'windows-resources.jsonl'
        $clock = [Diagnostics.Stopwatch]::StartNew()
        $nextSample = 0L
        while ($true) {
            $goProcess.Refresh()
            if ($goProcess.HasExited) { break }
            Add-JsonLine $resourcePath (Get-WindowsProcessSample $goProcess.Id $clock)
            $nextSample += 1000
            if ($clock.Elapsed.TotalSeconds -gt $effectiveDeadlineSeconds -or $CampaignClock.ElapsedMilliseconds -ge $goNaturalDeadline) {
                $timedOut = $true
                $goStopResult = Stop-OwnedProcessTree $goProcess.Id (Join-Path $attemptPath 'forced-process-cleanup.json') $CampaignClock $goCompletionDeadline
                foreach ($failure in $goStopResult.Failures) { $failures.Add("go process cleanup: $failure") }
                break
            }
            while ($clock.ElapsedMilliseconds -lt $nextSample) {
                $goProcess.Refresh()
                if ($goProcess.HasExited) { break }
                $remaining = $nextSample - $clock.ElapsedMilliseconds
                Start-Sleep -Milliseconds ([Math]::Min(200, [Math]::Max(1, $remaining)))
            }
        }
        $goCompletion = Wait-ProcessBounded $goProcess $CampaignClock $goCompletionDeadline
        if (-not $goCompletion.Exited) {
            $failures.Add("go test did not exit before the fixed $($CampaignLimitMilliseconds / 1000)-second campaign deadline: $($goCompletion.Error)")
        }
        else { $goExit = $goCompletion.ExitCode }
        Write-Utf8 (Join-Path $attemptPath 'go-test.exitcode') "$goExit`n"
        if ($timedOut) { $failures.Add("go test exceeded its $effectiveDeadlineSeconds-second bounded orchestration/campaign deadline.") }
        if ($goExit -ne 0) { $failures.Add("exact go test exited $goExit.") }
        if ($Attempt.Test -ceq 'TestH48A11DeterministicExpiryBoundaries') {
            $expiryOutput = $(if (Test-Path -LiteralPath $goStdout -PathType Leaf) { [IO.File]::ReadAllText($goStdout) } else { '' })
            $expiryResult = Test-ExpiryOutput $expiryOutput
            foreach ($failure in $expiryResult.Failures) { $failures.Add($failure) }
            $candidateExpiryResult = Test-CandidateExpiryReport $expiryOutput $script:expiryBundleRoot $script:At $script:Cohort $script:ReleaseTag $script:EndpointSHA256 $script:ControlSHA256
            foreach ($failure in $candidateExpiryResult.Failures) { $failures.Add($failure) }
            Write-Json (Join-Path $attemptPath 'status.json') ([ordered]@{
                schema = 'ardents-h4-8-a11-expiry-runner-status-v1'
                command = [IO.Path]::GetFileName($script:goPath)
                arguments = $goArguments
                exact_test = $Attempt.Test
                build_tag = 'h4_8_a11'
                markers = $expiryResult.Markers
                stdout_sha256 = $(if (Test-Path -LiteralPath $goStdout -PathType Leaf) { (Get-FileHash -LiteralPath $goStdout -Algorithm SHA256).Hash.ToLowerInvariant() } else { $null })
                markers_accepted = $expiryResult.Passed
                candidate_marker = $candidateExpiryResult.Marker
                candidate_report = $candidateExpiryResult.Report
                candidate_report_accepted = $candidateExpiryResult.Passed
                exact_candidate_bundle = $script:expiryBundleRoot.Substring($attemptPath.Length).TrimStart('\', '/').Replace('\', '/')
                remote_topology = 'not-applicable'
            })
        }
        $windowsResult = Test-WindowsResourceSeries $resourcePath $goProcess.Id ([bool]$Attempt.Remote)
        foreach ($failure in $windowsResult.Failures) { $failures.Add($failure) }
        $windowsCleanupResult = Test-WindowsProcessCleanup $resourcePath (Join-Path $attemptPath 'windows-post-process-inventory.json')
        $windowsCleanupInspected = $true
        foreach ($failure in $windowsCleanupResult.Failures) { $failures.Add($failure) }
    }
    catch {
        if ($_.Exception.Message -notin @('A11_ATTEMPT_PREFLIGHT', 'A11_ATTEMPT_SOURCE', 'A11_CAMPAIGN_DEADLINE')) {
            $failures.Add("attempt orchestration error: $($_.Exception.Message)")
        }
        if ($null -ne $goProcess) {
            try {
                $goProcess.Refresh()
                if (-not $goProcess.HasExited) {
                    $goStopResult = Stop-OwnedProcessTree $goProcess.Id (Join-Path $attemptPath 'exception-process-cleanup.json') $CampaignClock $goCompletionDeadline
                    foreach ($failure in $goStopResult.Failures) { $failures.Add("go exception cleanup: $failure") }
                }
                $goCompletion = Wait-ProcessBounded $goProcess $CampaignClock $goCompletionDeadline
                if ($goCompletion.Exited) { $goExit = $goCompletion.ExitCode }
                else { $failures.Add("qualification process exit could not be collected before the campaign deadline: $($goCompletion.Error)") }
            }
            catch { $failures.Add('qualification process exit could not be collected after an orchestration error.') }
            Write-Utf8 (Join-Path $attemptPath 'go-test.exitcode') "$goExit`n"
        }
    }
    finally {
        if ($null -ne $priorEnvironment) { Restore-A11Environment $priorEnvironment }
        if ($null -ne $goProcess -and -not $windowsCleanupInspected) {
            $resourcePath = Join-Path $attemptPath 'windows-resources.jsonl'
            $windowsCleanupResult = Test-WindowsProcessCleanup $resourcePath (Join-Path $attemptPath 'windows-post-process-inventory.json')
            $windowsCleanupInspected = $true
            foreach ($failure in $windowsCleanupResult.Failures) { $failures.Add($failure) }
        }
        if ($null -ne $observer) {
            try {
                $observerNaturalDeadline = $CampaignLimitMilliseconds - $script:campaignCleanupReserveMilliseconds
                $observerCompletion = Wait-ProcessBounded $observer.Process $CampaignClock $observerNaturalDeadline
                if (-not $observerCompletion.Exited) {
                    $observerStopResult = Stop-OwnedProcessTree $observer.Process.Id (Join-Path $attemptPath 'observer-process-cleanup.json') $CampaignClock $CampaignLimitMilliseconds
                    $observerStopped = $true
                    foreach ($failure in $observerStopResult.Failures) { $failures.Add("Ubuntu observer cleanup: $failure") }
                    $observerCompletion = Wait-ProcessBounded $observer.Process $CampaignClock $CampaignLimitMilliseconds
                }
                if ($observerCompletion.Exited) { $observerExit = $observerCompletion.ExitCode }
                else { $failures.Add("Ubuntu observer exit status is unavailable before the campaign deadline: $($observerCompletion.Error)") }
                Write-Utf8 (Join-Path $attemptPath 'ubuntu-observer.exitcode') "$observerExit`n"
                if (-not $observerStopped -and $observerExit -ne 0) { $failures.Add("Ubuntu observer exited $observerExit.") }
                $ubuntuResult = Test-UbuntuResourceSeries $observer.Stdout $container $script:RemoteImageID
                foreach ($failure in $ubuntuResult.Failures) { $failures.Add($failure) }
            }
            catch {
                $failures.Add("Ubuntu observer finalization failed: $($_.Exception.Message)")
                $observerStopped = $true
            }
        }
        if ($null -ne $goProcess) {
            try {
                $statusResult = Test-A11Status $statusRoot $Attempt.Test $container $remoteRoot $windowsResult ([bool]$Attempt.Remote)
                foreach ($failure in $statusResult.Failures) { $failures.Add($failure) }
            }
            catch { $failures.Add("A11 status finalization failed: $($_.Exception.Message)") }
        }
        if ($Attempt.Remote -and $ownershipEstablished) {
            try {
                $postExit = Invoke-RemotePostInventory $Attempt $container $remoteRoot $attemptPath
                if ($postExit -ne 0) {
                    $failures.Add("remote topology left residue or inventory failed with exit $postExit.")
                    $cleanupExit = Invoke-ExactRemoteCleanup $Attempt $suffix $container $remoteRoot $attemptPath
                    if ($cleanupExit -ne 0) { $failures.Add("exact remote residue cleanup failed with exit $cleanupExit.") }
                }
                else {
                    Write-Json (Join-Path $attemptPath 'remote-cleanup-disposition.json') ([ordered]@{ required = $false; reason = 'test-owned-cleanup-left-no-residue' })
                }
            }
            catch { $failures.Add("remote post-attempt inventory/cleanup error: $($_.Exception.Message)") }
        }

        $accepted = $failures.Count -eq 0 -and $goExit -eq 0 -and $windowsResult.Passed -and $windowsCleanupResult.Passed -and $statusResult.Passed -and
            ((-not $Attempt.Remote) -or ($preflightExit -eq 0 -and $observerExit -eq 0 -and $ubuntuResult.Passed -and $postExit -eq 0))
        $classification = if ($accepted) { 'accepted' } elseif ($preflightExit -ne $null -and $preflightExit -ne 0) { 'invalid-environment' } else { 'failed' }
        Write-Json (Join-Path $attemptPath 'disposition.json') ([ordered]@{
            schema = 'ardents-h4-8-a11-sub-attempt-disposition-v1'
            denominator_cell = $Attempt.Cell
            attempt_role = $Attempt.Attempt
            exact_test = $Attempt.Test
            suffix = $suffix
            started_at_utc = $startedAt
            finished_at_utc = [DateTimeOffset]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
            go_exit = $goExit
            orchestration_timeout = $timedOut
            effective_deadline_seconds = $effectiveDeadlineSeconds
            campaign_elapsed_milliseconds = $CampaignClock.ElapsedMilliseconds
            remote_preflight_exit = $preflightExit
            ubuntu_observer_exit = $observerExit
            ubuntu_observer_forced_stop = $observerStopped
            remote_post_inventory_exit = $postExit
            emergency_cleanup_exit = $cleanupExit
            windows_resource_samples = $windowsResult.Samples
            windows_process_cleanup_passed = $windowsCleanupResult.Passed
            ubuntu_resource_samples = $ubuntuResult.Samples
            ubuntu_maximum_memory_bytes = $ubuntuResult.MaximumMemory
            ubuntu_maximum_pids = $ubuntuResult.MaximumPIDs
            status_passed = $statusResult.Passed
            outcome = $classification
            failures = @($failures)
            retries = 0
        })
        Write-EvidenceInventory $attemptPath
    }
    return [pscustomobject]@{ Cell = $Attempt.Cell; Attempt = $Attempt.Attempt; Test = $Attempt.Test; Passed = $accepted; Outcome = $classification; Path = $attemptPath; Failures = @($failures) }
}

if ($PSCmdlet.ParameterSetName -eq 'SelfTest') {
    Invoke-SelfTests
    exit 0
}
Invoke-SelfTests | Out-Null

# Input refusal occurs before EvidenceOutput is created. Once these immutable
# identities are unambiguous, every later invalid or failed campaign phase is
# retained under the caller-owned evidence root.
if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) {
    throw 'H4-8 A11 qualification is owned by its Windows orchestrator.'
}
foreach ($entry in @(
    @{ Name = 'ArchiveSHA256'; Value = $ArchiveSHA256 },
    @{ Name = 'ManifestPin'; Value = $ManifestPin },
    @{ Name = 'EndpointSHA256'; Value = $EndpointSHA256 },
    @{ Name = 'ControlSHA256'; Value = $ControlSHA256 }
)) { Assert-LowerSHA256 $entry.Name $entry.Value }
if ($SourceRevision -cnotmatch '^[0-9a-f]{40}$') { throw 'SourceRevision must be one exact lowercase 40-hex Git commit.' }
if ($ReleaseTag -cnotmatch '^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$' -or $Cohort -cnotmatch '^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$') {
    throw 'ReleaseTag and Cohort must be exact safe non-empty identifiers.'
}
if ($At -cnotmatch '^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$') {
    throw 'At must be one exact UTC RFC3339 second.'
}
$parsedAt = [DateTimeOffset]::MinValue
if (-not [DateTimeOffset]::TryParseExact($At, 'yyyy-MM-ddTHH:mm:ssZ', [Globalization.CultureInfo]::InvariantCulture,
        [Globalization.DateTimeStyles]::AssumeUniversal -bor [Globalization.DateTimeStyles]::AdjustToUniversal, [ref]$parsedAt)) {
    throw 'At is not a real UTC RFC3339 calendar time.'
}
if (-not (Test-LiteralIPv4 $VPS)) { throw 'VPS must be one exact literal IPv4 address.' }
if ($User -cnotmatch '^[A-Za-z0-9_-]+$') { throw 'User must be one exact safe SSH account name.' }
if ($RemoteImageID -cnotmatch '^sha256:[0-9a-f]{64}$') { throw 'RemoteImageID must be one exact lowercase Docker sha256 image ID.' }
if (-not [IO.Path]::IsPathRooted($CandidateArchive) -or -not [IO.Path]::IsPathRooted($SSHKey) -or -not [IO.Path]::IsPathRooted($EvidenceOutput)) {
    throw 'CandidateArchive, SSHKey, and EvidenceOutput must be absolute paths.'
}
if (-not (Test-Path -LiteralPath $CandidateArchive -PathType Leaf)) { throw 'CandidateArchive is not an available regular file.' }
if (-not (Test-Path -LiteralPath $SSHKey -PathType Leaf)) { throw 'SSHKey is not an available regular file.' }
$candidateItem = Get-Item -LiteralPath $CandidateArchive -Force
$keyItem = Get-Item -LiteralPath $SSHKey -Force
if (($candidateItem.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0 -or
    ($keyItem.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
    throw 'CandidateArchive and SSHKey must not be reparse points.'
}
$candidatePath = $candidateItem.FullName
$keyPath = $keyItem.FullName
if ([IO.Path]::GetFileName($candidatePath) -cnotmatch '^[A-Za-z0-9._-]+\.tar\.gz$') {
    throw 'CandidateArchive must have one safe unambiguous .tar.gz filename.'
}
$evidencePath = [IO.Path]::GetFullPath($EvidenceOutput)
if (Test-Path -LiteralPath $evidencePath) { throw 'EvidenceOutput must be previously absent; attempts are never overwritten.' }
$evidenceParent = Split-Path -Parent $evidencePath
if (-not (Test-Path -LiteralPath $evidenceParent -PathType Container)) { throw 'EvidenceOutput parent directory is unavailable.' }
if ((Test-PathWithin $candidatePath $candidateRepository) -or (Test-PathWithin $evidencePath $candidateRepository) -or
    (Test-PathWithin $candidatePath $harnessRepository) -or (Test-PathWithin $evidencePath $harnessRepository)) {
    throw 'CandidateArchive and EvidenceOutput must remain outside the repository.'
}

$dirty = @(& git -C $candidateRepository status --porcelain=v1 --untracked-files=all)
if ($LASTEXITCODE -ne 0) { throw 'Git worktree status is unavailable.' }
if ($dirty.Count -ne 0) { throw 'A11 requires a clean committed candidate source worktree.' }
$head = (& git -C $candidateRepository rev-parse --verify HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $head -cne $SourceRevision) { throw 'HEAD does not equal the exact SourceRevision input.' }
$tagReference = "refs/tags/$ReleaseTag"
& git -C $candidateRepository show-ref --verify --quiet $tagReference
if ($LASTEXITCODE -ne 0) { throw 'the exact local immutable release tag is absent.' }
$tagCommit = (& git -C $candidateRepository rev-parse --verify "$tagReference^{commit}").Trim()
if ($LASTEXITCODE -ne 0 -or $tagCommit -cne $SourceRevision) { throw 'release tag does not resolve to the exact SourceRevision input.' }
$tagObject = (& git -C $candidateRepository rev-parse --verify $tagReference).Trim()
if ($LASTEXITCODE -ne 0 -or $tagObject -cnotmatch '^[0-9a-f]{40}$') { throw 'release tag object identity is unavailable.' }
$harnessDirty = @(& git -C $harnessRepository status --porcelain=v1 --untracked-files=all)
if ($LASTEXITCODE -ne 0 -or $harnessDirty.Count -ne 0) { throw 'A11 requires a clean committed runner worktree.' }
$harnessRevision = (& git -C $harnessRepository rev-parse --verify HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $harnessRevision -cnotmatch '^[0-9a-f]{40}$') { throw 'A11 runner revision is unavailable.' }

[IO.Directory]::CreateDirectory($evidencePath) | Out-Null
$campaignStarted = [DateTimeOffset]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
$campaignClock = [Diagnostics.Stopwatch]::StartNew()
$campaignLimitMilliseconds = 125L * 60L * 1000L
$campaignFailures = New-Object System.Collections.Generic.List[string]
$results = New-Object System.Collections.Generic.List[object]
$cellReceipts = New-Object System.Collections.Generic.List[object]
$knownHosts = Join-Path $evidencePath 'ssh-known-hosts'
$remote = "$User@$VPS"
$sshPath = $null
$goPath = $null
$campaignPhase = 'windows-environment'
$globalFailureClass = $null

Write-Json (Join-Path $evidencePath 'inputs.json') ([ordered]@{
    schema = 'ardents-h4-8-a11-campaign-inputs-v1'
    campaign_started_at_utc = $campaignStarted
    source_revision = $SourceRevision
    candidate_repository = $candidateRepository
    harness_revision = $harnessRevision
    release_tag = $ReleaseTag
    release_tag_object = $tagObject
    archive_name = [IO.Path]::GetFileName($candidatePath)
    archive_sha256 = $ArchiveSHA256
    manifest_pin = $ManifestPin
    endpoint_sha256 = $EndpointSHA256
    control_sha256 = $ControlSHA256
    cohort = $Cohort
    candidate_reference_at = $At
    release_platform = 'linux-amd64'
    vps = $VPS
    ssh_user = $User
    ssh_key_supplied = $true
    ssh_key_path_retained = $false
    base_port = $BasePort
    remote_image = $remoteImage
    remote_image_id = $RemoteImageID
    remote_limits = [ordered]@{ nano_cpus = $remoteNanoCPUs; memory_bytes = $remoteMemoryLimit; pids = $remotePIDLimit; network = 'host'; restart_policy = 'no' }
    denominator = 6
    planned_go_test_invocations = 10
    campaign_timeout = $CampaignTimeout
    retries = 0
})

try {
    $windowsOS = Get-CimInstance Win32_OperatingSystem -ErrorAction Stop
    $windowsBuild = [int]$windowsOS.BuildNumber
    if ([int]$windowsOS.ProductType -ne 1 -or [string]$windowsOS.Version -notmatch '^10\.0\.' -or
        $windowsBuild -lt 22000 -or $env:PROCESSOR_ARCHITECTURE -cne 'AMD64') {
        throw 'selected User/observer host must be Windows 11 x86_64.'
    }
    $goCommand = Get-Command go.exe -CommandType Application -ErrorAction Stop
    $goPath = $goCommand.Source
    [void](Get-Command tar.exe -CommandType Application -ErrorAction Stop)
    $goVersion = (& $goPath version).Trim()
    if ($LASTEXITCODE -ne 0 -or $goVersion -cne 'go version go1.26.6 windows/amd64') {
        throw 'A11 requires the exact Go 1.26.6 windows/amd64 toolchain.'
    }
    $sshPath = Find-OpenSSH
    $sshVersionExit = Invoke-NativeCaptured $sshPath @('-V') (Join-Path $evidencePath 'ssh-version.stdout.log') (Join-Path $evidencePath 'ssh-version.stderr.log')
    if ($sshVersionExit -ne 0) { throw 'Windows OpenSSH version envelope is unavailable.' }
    $sshVersion = (([IO.File]::ReadAllText((Join-Path $evidencePath 'ssh-version.stdout.log')) + ' ' +
        [IO.File]::ReadAllText((Join-Path $evidencePath 'ssh-version.stderr.log'))).Trim())
    Write-Json (Join-Path $evidencePath 'windows-host-envelope.json') ([ordered]@{
        schema = 'ardents-h4-8-a11-windows-host-v1'
        captured_at_utc = [DateTimeOffset]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
        hostname = [Environment]::MachineName
        caption = [string]$windowsOS.Caption
        version = [string]$windowsOS.Version
        build = $windowsBuild
        product_type = [int]$windowsOS.ProductType
        architecture = $env:PROCESSOR_ARCHITECTURE
        logical_processors = [Environment]::ProcessorCount
        total_visible_memory_kib = [long]$windowsOS.TotalVisibleMemorySize
        powershell = $PSVersionTable.PSVersion.ToString()
        go = $goVersion
        ssh = $sshVersion
    })

    $campaignPhase = 'release-identity'
    $expiryAttemptPath = Join-Path (Join-Path $evidencePath 'expiry-boundaries') '10-primary'
    $expiryExtractionRoot = Join-Path $expiryAttemptPath 'exact-candidate'
    $candidateValidation = Invoke-ArchiveValidation $candidatePath $evidencePath $expiryExtractionRoot
    $expiryBundleRoot = $candidateValidation.BundleRoot
    $candidateDescriptor = $candidateValidation.Descriptor
    $observerScript = Join-Path $PSScriptRoot 'observe-ubuntu.sh'
    if (-not (Test-Path -LiteralPath $observerScript -PathType Leaf)) { throw 'checked Ubuntu resource observer is unavailable.' }
    $harnessFiles = @($PSCommandPath, $observerScript, (Join-Path $PSScriptRoot 'invoke-windows.ps1'), (Join-Path $PSScriptRoot 'README.md'))
    $harnessDigests = foreach ($path in $harnessFiles) {
        [ordered]@{ path = [IO.Path]::GetFileName($path); sha256 = (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant() }
    }
    Write-Json (Join-Path $evidencePath 'harness-sha256.json') ([ordered]@{ schema = 'ardents-h4-8-a11-harness-digests-v1'; files = @($harnessDigests) })
    $treeExit = Invoke-NativeCaptured 'git.exe' @('-C', $candidateRepository, 'ls-tree', '-r', '--full-tree', $SourceRevision) `
        (Join-Path $evidencePath 'source-tree.txt') (Join-Path $evidencePath 'source-tree.stderr.log')
    Write-Utf8 (Join-Path $evidencePath 'source-tree.exitcode') "$treeExit`n"
    if ($treeExit -ne 0) { throw 'exact committed source-tree inventory is unavailable.' }

    $campaignPhase = 'test-seam'
    $attemptPlan = @(Get-A11Attempts $BasePort)
    Write-Json (Join-Path $evidencePath 'campaign-plan.json') ([ordered]@{
        schema = 'ardents-h4-8-a11-plan-v1'
        cells = @('normal-soak', 'application-loss', 'endpoint-loss', 'carrier-loss', 'product-node-loss', 'expiry-boundaries')
        attempts = @($attemptPlan | ForEach-Object { [ordered]@{ cell = $_.Cell; role = $_.Attempt; test = $_.Test; remote = $_.Remote; base_port = $_.Port; go_timeout = $_.GoTimeout } })
    })
    $definitions = New-Object 'System.Collections.Generic.HashSet[string]' ([StringComparer]::Ordinal)
    $serviceGoFiles = @(Get-ChildItem -LiteralPath (Join-Path $repository 'tests\e2e\service') -Filter '*.go' -File -Recurse)
    foreach ($file in $serviceGoFiles | Where-Object { $_.DirectoryName -eq (Join-Path $repository 'tests\e2e\service') }) {
        foreach ($line in [IO.File]::ReadAllLines($file.FullName)) {
            if ($line -cmatch '^func (TestH48A11[A-Za-z0-9_]*)\(t \*testing\.T\) \{$') { [void]$definitions.Add($Matches[1]) }
        }
    }
    $requiredTests = @($attemptPlan.Test | Sort-Object -Unique)
    $missing = @($requiredTests | Where-Object { -not $definitions.Contains($_) })
    $serviceSource = ($serviceGoFiles | ForEach-Object { [IO.File]::ReadAllText($_.FullName) }) -join "`n"
    $qualificationSource = [IO.File]::ReadAllText((Join-Path $repository 'tests\e2e\service\reference_c2_a11_qualification_test.go'))
    $productEvidenceSource = [IO.File]::ReadAllText((Join-Path $repository 'tests\e2e\service\reference_c2_multihost_product_evidence_test.go'))
    $productRunnerSource = [IO.File]::ReadAllText((Join-Path $repository 'tests\e2e\service\reference_c2_multihost_runner_test.go'))
    $productHarnessSeams = Test-A11ProductHarnessSeams $qualificationSource $productEvidenceSource $productRunnerSource
    $requiredCandidateSeams = @(
        'A11_EXPIRY_CANDIDATE_REPORT',
        'ARDENTS_H4_8_A11_CANDIDATE_', '"BUNDLE_ROOT"', '"MANIFEST_PIN"', '"COHORT"', '"RELEASE"',
        '"PLATFORM"', '"ENVIRONMENT"', '"NETWORK"', '"TARGET_PATH"', '"ARCHITECTURE"', '"REFERENCE_AT"',
        '"ENDPOINT_SHA256"', '"CONTROL_SHA256"'
    )
    $missingCandidateSeams = @($requiredCandidateSeams | Where-Object { $serviceSource.IndexOf($_, [StringComparison]::Ordinal) -lt 0 })
    Write-Json (Join-Path $evidencePath 'test-seam-preflight.json') ([ordered]@{
        schema = 'ardents-h4-8-a11-test-seams-v1'
        required = $requiredTests
        discovered = @($definitions | Sort-Object)
        missing = $missing
        required_candidate_seams = $requiredCandidateSeams
        missing_candidate_seams = $missingCandidateSeams
        required_product_harness_seams = @($productHarnessSeams.Required)
        product_harness_failures = @($productHarnessSeams.Failures)
        disposition = $(if ($missing.Count -eq 0 -and $missingCandidateSeams.Count -eq 0 -and $productHarnessSeams.Passed) { 'accepted' } else { 'harness-incomplete' })
    })
    if ($missing.Count -ne 0) { throw ('checked A11 harness lacks exact tests: ' + ($missing -join ', ')) }
    if ($missingCandidateSeams.Count -ne 0) { throw ('checked A11 harness lacks exact-candidate expiry seams: ' + ($missingCandidateSeams -join ', ')) }
    if (-not $productHarnessSeams.Passed) { throw ('checked A11 harness lacks exact product/fault seams: ' + ($productHarnessSeams.Failures -join ' | ')) }

    $campaignPhase = 'remote-environment'
    $remotePreflight = @"
set -eu
test "`$(id -u)" -eq 0
for command in docker ss sha256sum find stat awk uname nproc date cat wc cut basename sleep tar grep sort; do command -v "`$command" >/dev/null; done
. /etc/os-release
test "`$ID" = ubuntu
test "`$VERSION_ID" = 22.04
test "`$(uname -m)" = x86_64
test "`$(stat -fc %T /sys/fs/cgroup)" = cgroup2fs
docker_version="`$(docker version --format '{{.Server.Version}}')"
case "`$docker_version" in 29.*) ;; *) printf 'unsupported_docker=%s\n' "`$docker_version" >&2; exit 40 ;; esac
test "`$(docker info --format '{{.CgroupVersion}}')" = 2
image_id="`$(docker image inspect $(ConvertTo-ShellLiteral $remoteImage) --format '{{.Id}}')"
test "`$image_id" = $(ConvertTo-ShellLiteral $RemoteImageID)
test "`$(docker image inspect $(ConvertTo-ShellLiteral $remoteImage) --format '{{.Os}}/{{.Architecture}}')" = linux/amd64
printf 'schema=ardents-h4-8-a11-ubuntu-host-v1\n'
printf 'os_id=%s\nos_version=%s\narchitecture=%s\nkernel=%s\n' "`$ID" "`$VERSION_ID" "`$(uname -m)" "`$(uname -srvo)"
printf 'logical_cpus=%s\n' "`$(nproc)"
awk '/MemTotal:/ {printf "memory_kib=%s\\n", `$2}' /proc/meminfo
printf 'docker_server=%s\ndocker_cgroup=2\nimage=%s\nimage_id=%s\nssh_uid=%s\n' "`$docker_version" $(ConvertTo-ShellLiteral $remoteImage) "`$image_id" "`$(id -u)"
"@
    $remoteHostExit = Invoke-SSH $remotePreflight (Join-Path $evidencePath 'ubuntu-host-envelope.txt') (Join-Path $evidencePath 'ubuntu-host-envelope.stderr.log')
    Write-Utf8 (Join-Path $evidencePath 'ubuntu-host-envelope.exitcode') "$remoteHostExit`n"
    if ($remoteHostExit -ne 0) { throw 'Ubuntu/Docker/image/cgroup observer preflight failed.' }

    $campaignPhase = 'campaign'
    for ($index = 0; $index -lt $attemptPlan.Count; $index++) {
        $result = Invoke-A11Attempt $attemptPlan[$index] $index $evidencePath $campaignClock $campaignLimitMilliseconds
        $results.Add($result)
    }

    foreach ($cell in @('normal-soak', 'application-loss', 'endpoint-loss', 'carrier-loss', 'product-node-loss', 'expiry-boundaries')) {
        $cellResults = @($results | Where-Object Cell -eq $cell)
        $expectedCount = $(if ($cell -in @('application-loss', 'endpoint-loss', 'carrier-loss', 'product-node-loss')) { 2 } else { 1 })
        $passed = $cellResults.Count -eq $expectedCount -and @($cellResults | Where-Object { -not $_.Passed }).Count -eq 0
        $receipt = [ordered]@{
            schema = 'ardents-h4-8-a11-cell-disposition-v1'
            cell = $cell
            expected_sub_attempts = $expectedCount
            observed_sub_attempts = $cellResults.Count
            primary_test = ($cellResults | Where-Object Attempt -eq 'primary' | Select-Object -First 1).Test
            canary_test = $(if ($expectedCount -eq 2) { 'TestH48A11MultiHostTenCycleCanary' } else { $null })
            outcome = $(if ($passed) { 'accepted' } else { 'failed' })
            attempts = @($cellResults | ForEach-Object { [ordered]@{ role = $_.Attempt; test = $_.Test; outcome = $_.Outcome; evidence_directory = Split-Path -Leaf $_.Path } })
            retries = 0
        }
        Write-Json (Join-Path (Join-Path $evidencePath $cell) 'cell-disposition.json') $receipt
        $cellReceipts.Add([pscustomobject]@{ Cell = $cell; Passed = $passed; Receipt = $receipt })
    }

    $finalDirty = @(& git -C $repository status --porcelain=v1 --untracked-files=all)
    if ($LASTEXITCODE -ne 0 -or $finalDirty.Count -ne 0) { $campaignFailures.Add('campaign changed the selected clean source worktree.') }
    if ($campaignClock.ElapsedMilliseconds -gt $campaignLimitMilliseconds) { $campaignFailures.Add('campaign exceeded its fixed 125-minute profile deadline.') }
}
catch {
    $campaignFailures.Add($_.Exception.Message)
    $globalFailureClass = switch ($campaignPhase) {
        'windows-environment' { 'invalid-environment' }
        'remote-environment' { 'invalid-environment' }
        'release-identity' { 'identity-failure' }
        'test-seam' { 'harness-incomplete' }
        default { 'campaign-failure' }
    }
}
finally {
    $greenCells = @($cellReceipts | Where-Object Passed).Count
    $withinCampaignDeadline = $campaignClock.ElapsedMilliseconds -le $campaignLimitMilliseconds
    $accepted = $campaignFailures.Count -eq 0 -and $results.Count -eq 10 -and $cellReceipts.Count -eq 6 -and $greenCells -eq 6 -and $withinCampaignDeadline
    Write-Json (Join-Path $evidencePath 'campaign-receipt.json') ([ordered]@{
        schema = 'ardents-h4-8-a11-campaign-receipt-v1'
        campaign_started_at_utc = $campaignStarted
        campaign_finished_at_utc = [DateTimeOffset]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
        source_revision = $SourceRevision
        release_tag = $ReleaseTag
        archive_sha256 = $ArchiveSHA256
        manifest_pin = $ManifestPin
        endpoint_sha256 = $EndpointSHA256
        control_sha256 = $ControlSHA256
        cohort = $Cohort
        candidate_reference_at = $At
        denominator_expected = 6
        denominator_green = $greenCells
        go_test_invocations_expected = 10
        go_test_invocations_observed = $results.Count
        campaign_timeout = $CampaignTimeout
        campaign_elapsed_milliseconds = $campaignClock.ElapsedMilliseconds
        campaign_deadline_met = $withinCampaignDeadline
        outcome = $(if ($accepted) { 'accepted' } elseif ($globalFailureClass -eq 'invalid-environment') { 'invalid-environment' } else { 'failed' })
        failure_class = $globalFailureClass
        terminal_phase = $campaignPhase
        global_failures = @($campaignFailures)
        cell_outcomes = @($cellReceipts | ForEach-Object { [ordered]@{ cell = $_.Cell; passed = $_.Passed } })
        rerun_policy = 'a-new-evidence-root-is-a-new-attempt-and-never-erases-this-result'
    })
    Write-EvidenceInventory $evidencePath
}

if (-not $accepted) {
    Write-Error ('H4-8 A11 campaign failed; retained evidence: ' + $evidencePath + $(if ($campaignFailures.Count -ne 0) { ' | ' + ($campaignFailures -join ' | ') } else { '' }))
    exit 1
}
Write-Output "H4-8 A11 campaign accepted 6/6 without retry; evidence retained at $evidencePath"
