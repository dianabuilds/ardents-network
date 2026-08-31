[CmdletBinding(DefaultParameterSetName = 'Run')]
param(
    [Parameter(Mandatory = $true, ParameterSetName = 'Run')][string]$CandidateArchive,
    [Parameter(Mandatory = $true, ParameterSetName = 'Run')][string]$ArchiveSHA256,
    [Parameter(Mandatory = $true, ParameterSetName = 'Run')][string]$ManifestPin,
    [Parameter(Mandatory = $true, ParameterSetName = 'Run')][string]$EndpointSHA256,
    [Parameter(Mandatory = $true, ParameterSetName = 'Run')][string]$ControlSHA256,
    [Parameter(Mandatory = $true, ParameterSetName = 'Run')][string]$Cohort,
    [Parameter(Mandatory = $true, ParameterSetName = 'Run')][string]$Release,
    [Parameter(Mandatory = $true, ParameterSetName = 'Run')][string]$At,
    [Parameter(Mandatory = $true, ParameterSetName = 'Run')][string]$VPS,
    [Parameter(Mandatory = $true, ParameterSetName = 'Run')][string]$SSHKey,
    [Parameter(Mandatory = $true, ParameterSetName = 'Run')][string]$User,
    [Parameter(Mandatory = $true, ParameterSetName = 'Run')][string]$EvidenceOutput,
    [Parameter(Mandatory = $true, ParameterSetName = 'SelfTest')][switch]$SelfTest
)

$ErrorActionPreference = 'Stop'
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
$repository = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..\..\..')).Path
$attemptStartedAt = [DateTimeOffset]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
$failures = New-Object System.Collections.Generic.List[string]
$stageRoot = $null
$remoteRoot = $null
$remoteCreated = $false
$remoteOwnershipPossible = $false
$remoteRunExit = $null
$remoteCopyExit = $null
$remoteCleanupExit = $null
$harnessRevision = $null
$ubuntuEvidenceValidated = $false
$candidateArchiveName = $null

function Write-Utf8([string]$Path, [string]$Value) {
    [System.IO.File]::WriteAllText($Path, $Value, $script:utf8NoBom)
}

function Write-Json([string]$Path, [object]$Value) {
    Write-Utf8 $Path (($Value | ConvertTo-Json -Depth 8) + "`n")
}

function Assert-LowerSHA256([string]$Name, [string]$Value) {
    if ($Value -cnotmatch '^[0-9a-f]{64}$') {
        throw "$Name must be exactly 64 lowercase hexadecimal characters."
    }
}

function Test-PathWithin([string]$Child, [string]$Parent) {
    $parentPrefix = $Parent.TrimEnd([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar
    return $Child.Equals($Parent, [StringComparison]::OrdinalIgnoreCase) -or $Child.StartsWith($parentPrefix, [StringComparison]::OrdinalIgnoreCase)
}

function Find-OpenSSH {
    foreach ($root in @(
        (Join-Path $env:WINDIR 'System32\OpenSSH'),
        (Join-Path $env:WINDIR 'Sysnative\OpenSSH')
    )) {
        $ssh = Join-Path $root 'ssh.exe'
        $scp = Join-Path $root 'scp.exe'
        if ([IO.File]::Exists($ssh) -and [IO.File]::Exists($scp)) {
            return @{ SSH = $ssh; SCP = $scp }
        }
    }
    $sshCommand = Get-Command ssh.exe -CommandType Application -ErrorAction SilentlyContinue
    $scpCommand = Get-Command scp.exe -CommandType Application -ErrorAction SilentlyContinue
    if ($null -ne $sshCommand -and $null -ne $scpCommand) {
        return @{ SSH = $sshCommand.Source; SCP = $scpCommand.Source }
    }
    throw 'Windows OpenSSH ssh.exe and scp.exe are required.'
}

function Invoke-NativeCaptured([string]$Executable, [string[]]$Arguments, [string]$StdoutPath, [string]$StderrPath) {
    & $Executable @Arguments 1> $StdoutPath 2> $StderrPath
    $code = $LASTEXITCODE
    foreach ($path in @($StdoutPath, $StderrPath)) {
        if (Test-Path -LiteralPath $path -PathType Leaf) {
            $text = Get-Content -LiteralPath $path -Raw
            Write-Utf8 $path $text
        }
        else {
            Write-Utf8 $path ''
        }
    }
    return $code
}

function Read-ReleaseDescriptor([string]$Path) {
    $fields = @{}
    foreach ($line in [IO.File]::ReadAllLines($Path)) {
        if ($line -notmatch '^([a-z_]+)=(.+)$') {
            throw 'RELEASE contains a malformed or empty descriptor line.'
        }
        if ($fields.ContainsKey($Matches[1])) {
            throw "RELEASE duplicates field $($Matches[1])."
        }
        $fields[$Matches[1]] = $Matches[2]
    }
    return $fields
}

function Get-AlphaControlRequiredUbuntuEvidenceFiles {
    return @(
        'host-envelope.txt', 'archive-sha256.txt', 'archive-inventory.txt', 'manifest-pin.txt', 'manifest-check.txt',
        'SHA256SUMS', 'RELEASE', 'product-sha256.txt', 'expected-control-identities.txt', 'freshness-preflight.txt',
        'identical-authenticated-statements.txt', 'endpoint-a-lifecycle.stdout.log', 'endpoint-a-lifecycle.stderr.log',
        'endpoint-b-lifecycle.stdout.log', 'endpoint-b-lifecycle.stderr.log', 'lifecycle-release-outcomes.txt',
        'endpoint-a-release-floor-inventory.txt', 'endpoint-b-release-floor-inventory.txt',
        'endpoint-a-control.stdout.json', 'endpoint-a-control.stderr.log', 'endpoint-a-control.exitcode',
        'endpoint-b-control.stdout.json', 'endpoint-b-control.stderr.log', 'endpoint-b-control.exitcode',
        'endpoint-a-control-release-floor-inventory.txt', 'endpoint-b-control-release-floor-inventory.txt',
        'control-report-comparison.txt', 'qualification-summary.txt', 'endpoint-a.exitcode', 'endpoint-b.exitcode',
        'lifecycle-verdict.txt', 'verdict.txt', 'process-residue.txt', 'work-tree-residue.txt',
        'remote-exit-code.txt', 'outcome.txt', 'ssh-observed-run-exitcode.txt', 'runner.stdout.log', 'runner.stderr.log'
    )
}

function Test-AlphaControlUbuntuEvidence([string]$Root, [string]$ExpectedCohort, [string]$ExpectedRelease) {
    $issues = New-Object System.Collections.Generic.List[string]
    if (-not (Test-Path -LiteralPath $Root -PathType Container)) {
        $issues.Add('retained Ubuntu evidence directory is absent.')
        return @($issues)
    }
    foreach ($relative in Get-AlphaControlRequiredUbuntuEvidenceFiles) {
        if (-not (Test-Path -LiteralPath (Join-Path $Root $relative) -PathType Leaf)) {
            $issues.Add("retained Ubuntu evidence lacks $relative.")
        }
    }
    if ($issues.Count -ne 0) {
        return @($issues)
    }
    $summary = "schema=ardents-alpha-control-two-endpoints-summary-v1`nendpoint_a_release=release-accepted`nendpoint_b_release=release-accepted`nendpoint_a_control_release=release-accepted`nendpoint_b_control_release=release-accepted`nrelease_floor_inventories=canonical-byte-for-byte-equal`nlifecycle_identity=selected-cohort-release`n"
    $exactText = [ordered]@{
        'qualification-summary.txt' = $summary
        'lifecycle-release-outcomes.txt' = "endpoint-a=release-accepted`nendpoint-b=release-accepted`n"
        'control-report-comparison.txt' = "reports=byte-for-byte-equal`nendpoint-a=release-accepted`nendpoint-b=release-accepted`n"
        'lifecycle-verdict.txt' = "endpoint-a=stopped-exit-0-socket-absent`nendpoint-b=stopped-exit-0-socket-absent`n"
        'verdict.txt' = "alpha-control-two-fresh-endpoints=accepted`n"
        'remote-exit-code.txt' = "0`n"
        'outcome.txt' = "accepted`n"
        'ssh-observed-run-exitcode.txt' = "0`n"
        'endpoint-a.exitcode' = "0`n"
        'endpoint-b.exitcode' = "0`n"
        'endpoint-a-control.exitcode' = "0`n"
        'endpoint-b-control.exitcode' = "0`n"
    }
    foreach ($entry in $exactText.GetEnumerator()) {
        $actual = [IO.File]::ReadAllText((Join-Path $Root $entry.Key)).Replace("`r`n", "`n")
        if ($actual -cne $entry.Value) {
            $issues.Add("retained Ubuntu evidence $($entry.Key) is not the exact accepted receipt.")
        }
    }
    $freshness = [IO.File]::ReadAllText((Join-Path $Root 'freshness-preflight.txt')).Replace("`r`n", "`n")
    $expectedFreshness = "endpoint-a_endpoint_release_floor=absent`nendpoint-a_control_inspection_root=absent`nendpoint-b_endpoint_release_floor=absent`nendpoint-b_control_inspection_root=absent`n"
    if ($freshness -cne $expectedFreshness) {
        $issues.Add('retained freshness preflight does not prove four absent roots.')
    }
    $decision = '"kind":"release-decision","outcome":"release-accepted","cohort":"' + $ExpectedCohort + '","release":"' + $ExpectedRelease + '"'
    foreach ($name in @('endpoint-a', 'endpoint-b')) {
        $lifecycle = [IO.File]::ReadAllText((Join-Path $Root "$name-lifecycle.stdout.log"))
        if ($lifecycle.IndexOf($decision, [StringComparison]::Ordinal) -lt 0) {
            $issues.Add("retained $name lifecycle is not bound to the selected cohort/release.")
        }
    }
    $reportA = [IO.File]::ReadAllText((Join-Path $Root 'endpoint-a-control.stdout.json')).Replace("`r`n", "`n")
    $reportB = [IO.File]::ReadAllText((Join-Path $Root 'endpoint-b-control.stdout.json')).Replace("`r`n", "`n")
    if ($reportA -cne $reportB -or $reportA.IndexOf('"release":"release-accepted"', [StringComparison]::Ordinal) -lt 0) {
        $issues.Add('retained fresh control reports are unequal or do not contain exact release-accepted.')
    }
    $floorFiles = @('endpoint-a-release-floor-inventory.txt', 'endpoint-b-release-floor-inventory.txt',
        'endpoint-a-control-release-floor-inventory.txt', 'endpoint-b-control-release-floor-inventory.txt')
    $floor = [IO.File]::ReadAllText((Join-Path $Root $floorFiles[0])).Replace("`r`n", "`n")
    if ([string]::IsNullOrWhiteSpace($floor)) {
        $issues.Add('retained Release floor inventory is empty.')
    }
    foreach ($name in $floorFiles[1..($floorFiles.Count - 1)]) {
        if ([IO.File]::ReadAllText((Join-Path $Root $name)).Replace("`r`n", "`n") -cne $floor) {
            $issues.Add("retained Release floor inventory $name differs.")
        }
    }
    $residue = [IO.File]::ReadAllText((Join-Path $Root 'process-residue.txt')).Replace("`r`n", "`n")
    if (($residue -split "`n" | Where-Object { $_ -match 'running=no' -and $_ -match 'socket_present=no' }).Count -ne 2 -or
        $residue -match 'running=yes|socket_present=yes') {
        $issues.Add('retained process/socket residue is not empty.')
    }
    return @($issues)
}

function Invoke-AlphaControlSelfTest {
    $root = Join-Path ([IO.Path]::GetTempPath()) ('ardents-alpha-control-selftest-' + [Guid]::NewGuid().ToString('N'))
    [IO.Directory]::CreateDirectory($root) | Out-Null
    try {
        foreach ($relative in Get-AlphaControlRequiredUbuntuEvidenceFiles) {
            Write-Utf8 (Join-Path $root $relative) "placeholder`n"
        }
        Write-Utf8 (Join-Path $root 'qualification-summary.txt') "schema=ardents-alpha-control-two-endpoints-summary-v1`nendpoint_a_release=release-accepted`nendpoint_b_release=release-accepted`nendpoint_a_control_release=release-accepted`nendpoint_b_control_release=release-accepted`nrelease_floor_inventories=canonical-byte-for-byte-equal`nlifecycle_identity=selected-cohort-release`n"
        Write-Utf8 (Join-Path $root 'lifecycle-release-outcomes.txt') "endpoint-a=release-accepted`nendpoint-b=release-accepted`n"
        Write-Utf8 (Join-Path $root 'control-report-comparison.txt') "reports=byte-for-byte-equal`nendpoint-a=release-accepted`nendpoint-b=release-accepted`n"
        Write-Utf8 (Join-Path $root 'lifecycle-verdict.txt') "endpoint-a=stopped-exit-0-socket-absent`nendpoint-b=stopped-exit-0-socket-absent`n"
        Write-Utf8 (Join-Path $root 'verdict.txt') "alpha-control-two-fresh-endpoints=accepted`n"
        foreach ($name in @('remote-exit-code.txt', 'ssh-observed-run-exitcode.txt', 'endpoint-a.exitcode', 'endpoint-b.exitcode', 'endpoint-a-control.exitcode', 'endpoint-b-control.exitcode')) {
            Write-Utf8 (Join-Path $root $name) "0`n"
        }
        Write-Utf8 (Join-Path $root 'outcome.txt') "accepted`n"
        Write-Utf8 (Join-Path $root 'freshness-preflight.txt') "endpoint-a_endpoint_release_floor=absent`nendpoint-a_control_inspection_root=absent`nendpoint-b_endpoint_release_floor=absent`nendpoint-b_control_inspection_root=absent`n"
        $event = '{"kind":"release-decision","outcome":"release-accepted","cohort":"cohort-1","release":"alpha-1"}' + "`n"
        Write-Utf8 (Join-Path $root 'endpoint-a-lifecycle.stdout.log') $event
        Write-Utf8 (Join-Path $root 'endpoint-b-lifecycle.stdout.log') $event
        $report = '{"release":"release-accepted"}' + "`n"
        Write-Utf8 (Join-Path $root 'endpoint-a-control.stdout.json') $report
        Write-Utf8 (Join-Path $root 'endpoint-b-control.stdout.json') $report
        foreach ($name in @('endpoint-a-release-floor-inventory.txt', 'endpoint-b-release-floor-inventory.txt',
            'endpoint-a-control-release-floor-inventory.txt', 'endpoint-b-control-release-floor-inventory.txt')) {
            Write-Utf8 (Join-Path $root $name) "f 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef  current`n"
        }
        Write-Utf8 (Join-Path $root 'process-residue.txt') "endpoint_a_pid=none running=no socket=/tmp/a socket_present=no`nendpoint_b_pid=none running=no socket=/tmp/b socket_present=no`n"
        $valid = @(Test-AlphaControlUbuntuEvidence $root 'cohort-1' 'alpha-1')
        if ($valid.Count -ne 0) { throw ('valid retained evidence was rejected: ' + ($valid -join ' | ')) }
        Write-Utf8 (Join-Path $root 'qualification-summary.txt') "endpoint_a_release=no-update`n"
        if (@(Test-AlphaControlUbuntuEvidence $root 'cohort-1' 'alpha-1').Count -eq 0) { throw 'no-update summary was accepted.' }
        Write-Utf8 (Join-Path $root 'qualification-summary.txt') "schema=ardents-alpha-control-two-endpoints-summary-v1`nendpoint_a_release=release-accepted`nendpoint_b_release=release-accepted`nendpoint_a_control_release=release-accepted`nendpoint_b_control_release=release-accepted`nrelease_floor_inventories=canonical-byte-for-byte-equal`nlifecycle_identity=selected-cohort-release`n"
        Remove-Item -LiteralPath (Join-Path $root 'endpoint-b-control-release-floor-inventory.txt') -Force
        if (@(Test-AlphaControlUbuntuEvidence $root 'cohort-1' 'alpha-1').Count -eq 0) { throw 'missing retained evidence was accepted.' }
        Write-Output 'alpha-control-run-windows-self-test=accepted'
    }
    finally {
        if (Test-Path -LiteralPath $root -PathType Container) {
            $resolved = (Resolve-Path -LiteralPath $root).Path
            $prefix = [IO.Path]::GetTempPath().TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar + 'ardents-alpha-control-selftest-'
            if (-not $resolved.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase)) {
                throw 'self-test temporary root failed exact-path cleanup validation.'
            }
            Remove-Item -LiteralPath $resolved -Recurse -Force
        }
    }
}

if ($SelfTest) {
    Invoke-AlphaControlSelfTest
    exit 0
}

# A caller does not enter the denominator until this exact external path can
# safely own a new immutable attempt. Every later failure runs through finally.
if (-not [IO.Path]::IsPathRooted($EvidenceOutput)) {
    throw 'EvidenceOutput must be an absolute path before an attempt can become eligible.'
}
$evidencePath = [IO.Path]::GetFullPath($EvidenceOutput)
if (Test-Path -LiteralPath $evidencePath) {
    throw 'EvidenceOutput must be previously absent; attempts are never overwritten.'
}
$evidenceParent = Split-Path -Parent $evidencePath
if (-not (Test-Path -LiteralPath $evidenceParent -PathType Container)) {
    throw 'EvidenceOutput parent directory is unavailable.'
}
if (Test-PathWithin $evidencePath $repository) {
    throw 'EvidenceOutput must remain outside the repository.'
}
[IO.Directory]::CreateDirectory($evidencePath) | Out-Null
try {
$candidateArchiveName = [IO.Path]::GetFileName($CandidateArchive)
Write-Json (Join-Path $evidencePath 'requested-inputs.json') ([ordered]@{
    schema = 'ardents-alpha-control-two-endpoints-request-v1'
    attempt_started_at_utc = $attemptStartedAt
    candidate_archive_name = $candidateArchiveName
    archive_sha256 = $ArchiveSHA256
    manifest_pin = $ManifestPin
    endpoint_sha256 = $EndpointSHA256
    control_sha256 = $ControlSHA256
    cohort = $Cohort
    release = $Release
    decision_at = $At
    vps = $VPS
    ssh_user = $User
    ssh_key_supplied = $true
})
Write-Json (Join-Path $evidencePath 'windows-host-envelope.json') ([ordered]@{
    schema = 'ardents-qualification-host-envelope-v1'
    captured_at_utc = [DateTimeOffset]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
    hostname = [Environment]::MachineName
    os = [Environment]::OSVersion.VersionString
    process_architecture = $env:PROCESSOR_ARCHITECTURE
    logical_processors = [Environment]::ProcessorCount
    powershell = $PSVersionTable.PSVersion.ToString()
})

if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) {
    throw 'Alpha-control two-Endpoint qualification is owned by its Windows orchestrator.'
}
foreach ($entry in @(
    @{ Name = 'ArchiveSHA256'; Value = $ArchiveSHA256 },
    @{ Name = 'ManifestPin'; Value = $ManifestPin },
    @{ Name = 'EndpointSHA256'; Value = $EndpointSHA256 },
    @{ Name = 'ControlSHA256'; Value = $ControlSHA256 }
)) {
    Assert-LowerSHA256 $entry.Name $entry.Value
}
if ($Cohort -cnotmatch '^[A-Za-z0-9._-]+$' -or $Release -cnotmatch '^[A-Za-z0-9._-]+$') {
    throw 'Cohort and Release must be exact non-empty safe identifiers.'
}
if ($At -cnotmatch '^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$') {
    throw 'At must be one exact UTC RFC3339 second, for example 2026-08-29T12:00:00Z.'
}
$parsedAt = [DateTimeOffset]::MinValue
if (-not [DateTimeOffset]::TryParseExact($At, 'yyyy-MM-ddTHH:mm:ssZ', [Globalization.CultureInfo]::InvariantCulture,
        [Globalization.DateTimeStyles]::AssumeUniversal -bor [Globalization.DateTimeStyles]::AdjustToUniversal, [ref]$parsedAt)) {
    throw 'At is not a real UTC RFC3339 calendar time.'
}
if ($VPS -notmatch '^(\d{1,3}\.){3}\d{1,3}$') {
    throw 'VPS must be one literal IPv4 address.'
}
foreach ($octet in $VPS.Split('.')) {
    if ([int]$octet -gt 255) { throw 'VPS contains an IPv4 octet above 255.' }
}
if ($User -cnotmatch '^[A-Za-z0-9_-]+$') {
    throw 'User must be one exact safe SSH account name.'
}
if (-not [IO.Path]::IsPathRooted($CandidateArchive) -or -not [IO.Path]::IsPathRooted($SSHKey)) {
    throw 'CandidateArchive and SSHKey must be absolute paths.'
}
if (-not (Test-Path -LiteralPath $CandidateArchive -PathType Leaf)) {
    throw 'CandidateArchive is not an available regular file.'
}
if (-not (Test-Path -LiteralPath $SSHKey -PathType Leaf)) {
    throw 'SSHKey is not an available regular file.'
}
$candidateItem = Get-Item -LiteralPath $CandidateArchive -Force
$keyItem = Get-Item -LiteralPath $SSHKey -Force
if (($candidateItem.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0 -or ($keyItem.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
    throw 'CandidateArchive and SSHKey must not be reparse points.'
}
$candidatePath = $candidateItem.FullName
$keyPath = $keyItem.FullName
$candidateName = [IO.Path]::GetFileName($candidatePath)
if ($candidateName -cnotmatch '^[A-Za-z0-9._-]+\.tar\.gz$' -or $candidateName -ceq 'run-ubuntu.sh') {
    throw 'CandidateArchive must have one safe unambiguous .tar.gz filename.'
}
if (Test-PathWithin $candidatePath $repository) {
    throw 'CandidateArchive must remain outside the repository.'
}

$dirty = @(& git -C $repository status --porcelain=v1 --untracked-files=all)
if ($LASTEXITCODE -ne 0) { throw 'Git worktree status is unavailable.' }
if ($dirty.Count -ne 0) { throw 'Alpha-control live qualification requires a clean Git worktree.' }
$harnessRevision = (& git -C $repository rev-parse HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $harnessRevision -cnotmatch '^[0-9a-f]{40}$') {
    throw 'Alpha-control harness revision is unavailable.'
}

Write-Json (Join-Path $evidencePath 'inputs.json') ([ordered]@{
    schema = 'ardents-alpha-control-two-endpoints-inputs-v1'
    attempt_started_at_utc = $attemptStartedAt
    harness_revision = $harnessRevision
    candidate_archive_name = [IO.Path]::GetFileName($candidatePath)
    archive_sha256 = $ArchiveSHA256
    manifest_pin = $ManifestPin
    endpoint_sha256 = $EndpointSHA256
    control_sha256 = $ControlSHA256
    cohort = $Cohort
    release = $Release
    platform = 'linux-amd64'
    decision_at = $At
    vps = $VPS
    ssh_user = $User
    ssh_key_supplied = $true
    worktree = 'clean'
})
$runScript = Join-Path $PSScriptRoot 'run-ubuntu.sh'
if (-not (Test-Path -LiteralPath $runScript -PathType Leaf)) {
    throw 'checked alpha-control Ubuntu runner is unavailable.'
}
$harnessFiles = @($PSCommandPath, $runScript, (Join-Path $PSScriptRoot 'README.md'))
$harnessDigests = foreach ($path in $harnessFiles) {
    [ordered]@{
        name = [IO.Path]::GetFileName($path)
        sha256 = (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant()
    }
}
Write-Json (Join-Path $evidencePath 'harness-sha256.json') ([ordered]@{ schema = 'ardents-alpha-control-harness-digests-v1'; files = @($harnessDigests) })

    $openSSH = Find-OpenSSH
    $suffix = [Guid]::NewGuid().ToString('N').Substring(0, 16)
    $remoteRoot = "/tmp/ardents-alpha-control-two-endpoints-$suffix"
    $remote = "$User@$VPS"
    $knownHosts = Join-Path $evidencePath 'ssh-known-hosts'
    $sshOptions = @('-n', '-i', $keyPath, '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=15', '-o', 'ConnectionAttempts=1',
        '-o', 'ServerAliveInterval=5', '-o', 'ServerAliveCountMax=6', '-o', 'StrictHostKeyChecking=accept-new',
        '-o', "UserKnownHostsFile=$knownHosts", '-o', 'LogLevel=ERROR')
    $scpOptions = @('-i', $keyPath, '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=15', '-o', 'ConnectionAttempts=1',
        '-o', 'StrictHostKeyChecking=accept-new', '-o', "UserKnownHostsFile=$knownHosts", '-o', 'LogLevel=ERROR')
    $stageRoot = Join-Path ([IO.Path]::GetTempPath()) ("ardents-alpha-control-local-$suffix")
    [IO.Directory]::CreateDirectory($stageRoot) | Out-Null
    $actualArchiveSHA256 = (Get-FileHash -LiteralPath $candidatePath -Algorithm SHA256).Hash.ToLowerInvariant()
    Write-Utf8 (Join-Path $evidencePath 'local-archive-sha256.txt') "$actualArchiveSHA256  $([IO.Path]::GetFileName($candidatePath))`n"
    if ($actualArchiveSHA256 -cne $ArchiveSHA256) { throw 'local archive digest differs from the approved input before upload.' }

    $archiveListing = @(& tar.exe -tzf $candidatePath)
    if ($LASTEXITCODE -ne 0 -or $archiveListing.Count -eq 0) { throw 'local archive inventory cannot be read before upload.' }
    Write-Utf8 (Join-Path $evidencePath 'local-archive-inventory.txt') (($archiveListing -join "`n") + "`n")
    $normalized = New-Object System.Collections.Generic.List[string]
    $topNames = New-Object 'System.Collections.Generic.HashSet[string]' ([StringComparer]::Ordinal)
    foreach ($rawEntry in $archiveListing) {
        $entry = [string]$rawEntry
        if ($entry.StartsWith('./', [StringComparison]::Ordinal)) { $entry = $entry.Substring(2) }
        if ([string]::IsNullOrEmpty($entry) -or $entry.StartsWith('/', [StringComparison]::Ordinal) -or
            $entry.Contains('\') -or $entry.Contains('//') -or $entry -match '(^|/)\.\.(/|$)') {
            throw 'local archive inventory contains an unsafe path before upload.'
        }
        $trimmed = $entry.TrimEnd('/')
        $parts = $trimmed.Split('/')
        if ($parts.Count -gt 2 -or $parts[0] -cnotmatch '^[A-Za-z0-9._-]+$' -or
            ($parts.Count -eq 2 -and $parts[1] -cnotmatch '^[A-Za-z0-9._-]+$')) {
            throw 'local archive inventory is not one simple top-level bundle.'
        }
        if (-not $topNames.Add($parts[0])) { }
        $normalized.Add($trimmed)
    }
    if ($topNames.Count -ne 1 -or ($normalized | Group-Object -CaseSensitive | Where-Object Count -gt 1)) {
        throw 'local archive has ambiguous or duplicate top-level inventory.'
    }
    $top = @($topNames)[0]
    & tar.exe -xzf $candidatePath -C $stageRoot
    if ($LASTEXITCODE -ne 0) { throw 'local archive extraction failed before upload.' }
    $bundle = Join-Path $stageRoot $top
    if (-not (Test-Path -LiteralPath $bundle -PathType Container)) { throw 'local extracted bundle root is absent.' }
    $topEntries = @(Get-ChildItem -LiteralPath $stageRoot -Force)
    if ($topEntries.Count -ne 1 -or -not $topEntries[0].PSIsContainer) { throw 'local archive extracted an ambiguous top level.' }
    $bundleEntries = @(Get-ChildItem -LiteralPath $bundle -Force)
    if ($bundleEntries.Count -eq 0 -or ($bundleEntries | Where-Object { $_.PSIsContainer -or (($_.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) })) {
        throw 'local bundle contains a non-regular top-level entry.'
    }
    $manifestPath = Join-Path $bundle 'SHA256SUMS'
    if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) { throw 'local bundle manifest is absent.' }
    $actualManifestPin = (Get-FileHash -LiteralPath $manifestPath -Algorithm SHA256).Hash.ToLowerInvariant()
    Write-Utf8 (Join-Path $evidencePath 'local-manifest-pin.txt') "$actualManifestPin  SHA256SUMS`n"
    if ($actualManifestPin -cne $ManifestPin) { throw 'local manifest differs from the approved Enrollment Pin before upload.' }
    $manifestEntries = @{}
    foreach ($line in [IO.File]::ReadAllLines($manifestPath)) {
        if ($line -cnotmatch '^([0-9a-f]{64})  ([A-Za-z0-9._-]+)$') { throw 'local manifest line is not canonical.' }
        if ($manifestEntries.ContainsKey($Matches[2])) { throw 'local manifest inventory contains a duplicate.' }
        $manifestEntries[$Matches[2]] = $Matches[1]
    }
    $actualNames = @($bundleEntries | ForEach-Object Name)
    $expectedNames = @($manifestEntries.Keys) + @('SHA256SUMS')
    if ($actualNames.Count -ne $expectedNames.Count) { throw 'local bundle and manifest inventory counts differ.' }
    foreach ($name in $actualNames) {
        if ($expectedNames -cnotcontains $name) { throw "local bundle inventory has unexpected entry $name." }
    }
    foreach ($name in $manifestEntries.Keys) {
        $digest = (Get-FileHash -LiteralPath (Join-Path $bundle $name) -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($digest -cne $manifestEntries[$name]) { throw "local manifested byte verification failed for $name." }
    }
    Copy-Item -LiteralPath $manifestPath -Destination (Join-Path $evidencePath 'local-SHA256SUMS')
    $descriptorPath = Join-Path $bundle 'RELEASE'
    $descriptor = Read-ReleaseDescriptor $descriptorPath
    foreach ($field in @('schema', 'cohort', 'release', 'platform', 'environment', 'network', 'target_path', 'artifact', 'control_artifact')) {
        if (-not $descriptor.ContainsKey($field)) { throw "local RELEASE lacks field $field." }
    }
    if ($descriptor.schema -cne 'ardents-closed-alpha-enrollment-v3' -or $descriptor.cohort -cne $Cohort -or
        $descriptor.release -cne $Release -or $descriptor.platform -cne 'linux-amd64' -or
        $descriptor.target_path -cne 'ardents/linux-amd64/endpoint' -or
        $descriptor.artifact -cne 'ardents-linux-amd64' -or $descriptor.control_artifact -cne 'ardents-control-linux-amd64') {
        throw 'local RELEASE differs from the exact selected enrollment contract.'
    }
    $actualEndpointSHA256 = (Get-FileHash -LiteralPath (Join-Path $bundle $descriptor.artifact) -Algorithm SHA256).Hash.ToLowerInvariant()
    $actualControlSHA256 = (Get-FileHash -LiteralPath (Join-Path $bundle $descriptor.control_artifact) -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actualEndpointSHA256 -cne $EndpointSHA256 -or $actualControlSHA256 -cne $ControlSHA256) {
        throw 'local Endpoint or control companion digest differs from the approved input before upload.'
    }
    Write-Utf8 (Join-Path $evidencePath 'local-product-sha256.txt') "$actualEndpointSHA256  $($descriptor.artifact)`n$actualControlSHA256  $($descriptor.control_artifact)`n"
    Write-Utf8 (Join-Path $evidencePath 'local-validation-verdict.txt') "archive-pin-inventory-product-bytes=accepted-before-upload`n"

    $createCommand = "set -eu; id -u | grep -x 0 >/dev/null; test ! -e '$remoteRoot'; umask 077; mkdir -m 700 '$remoteRoot'; printf '%s\n' '$suffix' >'$remoteRoot/.qualification-owner'; mkdir -m 700 '$remoteRoot/ubuntu'"
    $remoteOwnershipPossible = $true
    $createExit = Invoke-NativeCaptured $openSSH.SSH ($sshOptions + @($remote, $createCommand)) `
        (Join-Path $evidencePath 'remote-create.stdout.log') (Join-Path $evidencePath 'remote-create.stderr.log')
    if ($createExit -ne 0) { throw 'remote exact-root creation or UID-0 preflight failed.' }
    $remoteCreated = $true

    $uploadExit = Invoke-NativeCaptured $openSSH.SCP ($scpOptions + @($candidatePath, $runScript, "$remote`:$remoteRoot/")) `
        (Join-Path $evidencePath 'upload.stdout.log') (Join-Path $evidencePath 'upload.stderr.log')
    Write-Utf8 (Join-Path $evidencePath 'upload.exitcode') "$uploadExit`n"
    if ($uploadExit -ne 0) { throw 'uploading the already validated archive and checked runner failed.' }
    $renameCommand = "set -eu; mv '$remoteRoot/$([IO.Path]::GetFileName($candidatePath))' '$remoteRoot/candidate.tar.gz'; chmod 600 '$remoteRoot/candidate.tar.gz'; chmod 700 '$remoteRoot/run-ubuntu.sh'"
    # If the source archive already had the fixed staged name, omit the
    # otherwise-invalid self move while retaining one unambiguous remote name.
    if ([IO.Path]::GetFileName($candidatePath) -ceq 'candidate.tar.gz') {
        $renameCommand = "set -eu; test -f '$remoteRoot/candidate.tar.gz'; chmod 600 '$remoteRoot/candidate.tar.gz'; chmod 700 '$remoteRoot/run-ubuntu.sh'"
    }
    $renameExit = Invoke-NativeCaptured $openSSH.SSH ($sshOptions + @($remote, $renameCommand)) `
        (Join-Path $evidencePath 'remote-stage.stdout.log') (Join-Path $evidencePath 'remote-stage.stderr.log')
    if ($renameExit -ne 0) { throw 'normalizing the exact staged archive name failed.' }

    $runCommand = "set +e; sh '$remoteRoot/run-ubuntu.sh' '$remoteRoot' '$ArchiveSHA256' '$ManifestPin' '$EndpointSHA256' '$ControlSHA256' '$Cohort' '$Release' '$At' 'candidate.tar.gz' >'$remoteRoot/ubuntu/runner.stdout.log' 2>'$remoteRoot/ubuntu/runner.stderr.log'; code=`$?; printf '%s\n' `$code >'$remoteRoot/ubuntu/ssh-observed-run-exitcode.txt'; exit `$code"
    $remoteRunExit = Invoke-NativeCaptured $openSSH.SSH ($sshOptions + @($remote, $runCommand)) `
        (Join-Path $evidencePath 'remote-run.stdout.log') (Join-Path $evidencePath 'remote-run.stderr.log')
    Write-Utf8 (Join-Path $evidencePath 'remote-run.exitcode') "$remoteRunExit`n"
    if ($remoteRunExit -ne 0) { throw "remote alpha-control qualifier failed with exit $remoteRunExit." }
}
catch {
    $failures.Add($_.Exception.Message)
}
finally {
    if ($remoteCreated) {
        try {
            $remoteCopyExit = Invoke-NativeCaptured $openSSH.SCP ($scpOptions + @('-r', "$remote`:$remoteRoot/ubuntu", $evidencePath)) `
                (Join-Path $evidencePath 'evidence-copy.stdout.log') (Join-Path $evidencePath 'evidence-copy.stderr.log')
            Write-Utf8 (Join-Path $evidencePath 'evidence-copy.exitcode') "$remoteCopyExit`n"
            if ($remoteCopyExit -ne 0) { $failures.Add('copying retained Ubuntu evidence failed.') }
            if ($remoteCopyExit -eq 0 -and $remoteRunExit -eq 0) {
                $ubuntuIssues = @(Test-AlphaControlUbuntuEvidence (Join-Path $evidencePath 'ubuntu') $Cohort $Release)
                if ($ubuntuIssues.Count -eq 0) {
                    $ubuntuEvidenceValidated = $true
                }
                else {
                    foreach ($issue in $ubuntuIssues) { $failures.Add($issue) }
                }
            }
        }
        catch {
            $failures.Add('copying retained Ubuntu evidence raised an orchestration error.')
        }
    }
    if ($remoteOwnershipPossible) {
        try {
            $cleanupCommand = "set -eu; case '$remoteRoot' in /tmp/ardents-alpha-control-two-endpoints-[0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]) ;; *) exit 2 ;; esac; test -f '$remoteRoot/.qualification-owner'; grep -x '$suffix' '$remoteRoot/.qualification-owner' >/dev/null; rm -rf -- '$remoteRoot'; test ! -e '$remoteRoot'; printf 'remote_root=absent\n'"
            $remoteCleanupExit = Invoke-NativeCaptured $openSSH.SSH ($sshOptions + @($remote, $cleanupCommand)) `
                (Join-Path $evidencePath 'remote-cleanup.stdout.log') (Join-Path $evidencePath 'remote-cleanup.stderr.log')
            Write-Utf8 (Join-Path $evidencePath 'remote-cleanup.exitcode') "$remoteCleanupExit`n"
            if ($remoteCleanupExit -ne 0) { $failures.Add('exact remote-root cleanup or absence verification failed.') }
        }
        catch {
            $failures.Add('exact remote-root cleanup raised an orchestration error.')
        }
    }
    if ($null -ne $stageRoot -and (Test-Path -LiteralPath $stageRoot -PathType Container)) {
        $resolvedStage = (Resolve-Path -LiteralPath $stageRoot).Path
        $tempPrefix = [IO.Path]::GetTempPath().TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar + 'ardents-alpha-control-local-'
        if ($resolvedStage.StartsWith($tempPrefix, [StringComparison]::OrdinalIgnoreCase)) {
            Remove-Item -LiteralPath $resolvedStage -Recurse -Force
        }
        else {
            $failures.Add('local temporary root failed exact-path cleanup validation.')
        }
    }
    if (Test-Path -LiteralPath $evidencePath -PathType Container) {
        $accepted = $failures.Count -eq 0 -and $remoteRunExit -eq 0 -and $remoteCopyExit -eq 0 -and
            $remoteCleanupExit -eq 0 -and $ubuntuEvidenceValidated
        Write-Json (Join-Path $evidencePath 'attempt-receipt.json') ([ordered]@{
            schema = 'ardents-alpha-control-two-endpoints-attempt-v1'
            attempt_started_at_utc = $attemptStartedAt
            attempt_finished_at_utc = [DateTimeOffset]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
            harness_revision = $harnessRevision
            candidate_archive_name = $candidateArchiveName
            archive_sha256 = $ArchiveSHA256
            manifest_pin = $ManifestPin
            endpoint_sha256 = $EndpointSHA256
            control_sha256 = $ControlSHA256
            cohort = $Cohort
            release = $Release
            decision_at = $At
            remote_run_exit = $remoteRunExit
            evidence_copy_exit = $remoteCopyExit
            remote_cleanup_exit = $remoteCleanupExit
            ubuntu_evidence_validated = $ubuntuEvidenceValidated
            outcome = $(if ($accepted) { 'accepted' } else { 'failed' })
            failures = @($failures)
            rerun_policy = 'separate-attempt-never-erases-failure'
        })
        $inventory = foreach ($file in Get-ChildItem -LiteralPath $evidencePath -Recurse -File | Where-Object Name -ne 'evidence-sha256.txt' | Sort-Object FullName) {
            $relative = $file.FullName.Substring($evidencePath.Length).TrimStart('\', '/').Replace('\', '/')
            '{0}  {1}' -f (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant(), $relative
        }
        Write-Utf8 (Join-Path $evidencePath 'evidence-sha256.txt') (($inventory -join "`n") + "`n")
    }
}

if ($failures.Count -ne 0) {
    throw ('Alpha-control two-Endpoint qualification failed: ' + ($failures -join ' | '))
}
Write-Output "Alpha-control two fresh Endpoints accepted; evidence retained at $evidencePath"
