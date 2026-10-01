#Requires -Version 7.0
param(
    [Parameter(Mandatory = $true)][string]$PublisherHost,
    [Parameter(Mandatory = $true)][string]$SSHKey,
    [Parameter(Mandatory = $true)][string]$RunnerBinary,
    [Parameter(Mandatory = $true)][string]$BaselineEvidence,
    [Parameter(Mandatory = $true)][string]$EpisodeEvidence,
    [Parameter(Mandatory = $true)][string]$EvidenceOutput,
    [string]$User = 'root'
)

$ErrorActionPreference = 'Stop'
$repository = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..\..'))

function Resolve-File([string]$Path, [string]$Label) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { throw "$Label is absent." }
    return (Resolve-Path -LiteralPath $Path).Path
}
function Resolve-Directory([string]$Path, [string]$Label) {
    if (-not (Test-Path -LiteralPath $Path -PathType Container)) { throw "$Label is absent." }
    return (Resolve-Path -LiteralPath $Path).Path
}
function Assert-Host([string]$Value, [string]$Label) {
    if ($Value -notmatch '^(?:[A-Za-z0-9](?:[A-Za-z0-9.-]{0,251}[A-Za-z0-9])?|(?:\d{1,3}\.){3}\d{1,3})$') { throw "$Label is invalid." }
}
function Write-Utf8([string]$Path, [string]$Body) {
    [IO.File]::WriteAllText($Path, $Body, [Text.UTF8Encoding]::new($false))
}
function Invoke-Native([string]$Program, [string[]]$Arguments, [string]$Label, [int]$TimeoutSeconds = 180) {
    $start = [Diagnostics.ProcessStartInfo]::new()
    $start.FileName = $Program
    $start.UseShellExecute = $false
    $start.RedirectStandardOutput = $true
    $start.RedirectStandardError = $true
    foreach ($argument in $Arguments) { [void]$start.ArgumentList.Add($argument) }
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = $start
    if (-not $process.Start()) { throw "$Label did not start." }
    $stdout = $process.StandardOutput.ReadToEndAsync()
    $stderr = $process.StandardError.ReadToEndAsync()
    if (-not $process.WaitForExit($TimeoutSeconds * 1000)) {
        $process.Kill($true); $process.WaitForExit()
        $captured = $stdout.Result + $stderr.Result
        if ($captured.Length -gt 65536) { $captured = $captured.Substring($captured.Length - 65536) }
        throw "$Label exceeded its $TimeoutSeconds-second local deadline: $captured"
    }
    $lines = @((($stdout.Result + $stderr.Result) -split "`r?`n") | Where-Object { $_ -ne '' })
    if ($process.ExitCode -ne 0) { throw "$Label failed with exit code $($process.ExitCode): $($lines -join [Environment]::NewLine)" }
    return $lines
}
function Remote([string]$HostName) { return "$User@$HostName" }
function Invoke-SSH([string]$Command, [string]$Label) {
    return @(Invoke-Native $script:ssh ($script:sshOptions + @('-n', (Remote $PublisherHost), $Command)) $Label)
}
function Send-File([string]$Local, [string]$RemotePath, [string]$Label) {
    [void](Invoke-Native $script:scp ($script:scpOptions + @($Local, "$(Remote $PublisherHost):$RemotePath")) $Label)
}

Assert-Host $PublisherHost 'PublisherHost'
Assert-Host $User 'User'
$key = Resolve-File $SSHKey 'SSHKey'
$runner = Resolve-File $RunnerBinary 'RunnerBinary'
$baseline = Resolve-Directory $BaselineEvidence 'BaselineEvidence'
$episode = Resolve-Directory $EpisodeEvidence 'EpisodeEvidence'
$output = [IO.Path]::GetFullPath($EvidenceOutput)
foreach ($candidate in @($baseline, $episode, $output)) {
    if ($candidate.StartsWith($repository + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
        throw 'NET-14V evidence must remain outside the repository.'
    }
}
if (Test-Path -LiteralPath $output) { throw 'EvidenceOutput must be a new directory.' }
[IO.Directory]::CreateDirectory($output) | Out-Null

$baseAttempt = Get-Content -LiteralPath (Resolve-File (Join-Path $baseline 'attempt.json') 'baseline attempt') -Raw | ConvertFrom-Json
$eventAttempt = Get-Content -LiteralPath (Resolve-File (Join-Path $episode 'attempt.json') 'episode attempt') -Raw | ConvertFrom-Json
$baseManifest = Resolve-File (Join-Path $baseline 'network-manifest.json') 'baseline manifest'
$eventManifest = Resolve-File (Join-Path $episode 'network-manifest.json') 'episode manifest'
$baseVerdict = Resolve-File (Join-Path $baseline 'paired-verdict.json') 'baseline paired verdict'
$eventPassed = [bool]$eventAttempt.Passed
$eventVerdict = if ($eventPassed) { Resolve-File (Join-Path $episode 'paired-verdict.json') 'episode paired verdict' } else { Resolve-File (Join-Path $episode 'relay-results.failed.json') 'failed episode relay evidence' }
$baseInputs = Get-Content -LiteralPath (Resolve-File (Join-Path $baseline 'input-digests.json') 'baseline inputs') -Raw | ConvertFrom-Json
$eventInputs = Get-Content -LiteralPath (Resolve-File (Join-Path $episode 'input-digests.json') 'episode inputs') -Raw | ConvertFrom-Json
$recoveryPattern = if ($eventPassed) { 'recovery-*.jsonl' } else { 'recovery-*.failed.jsonl' }
$recoveries = @(Get-ChildItem -LiteralPath $episode -Filter $recoveryPattern -File | Sort-Object Name)
if ($recoveries.Count -lt 1 -or $recoveries.Count -gt 2) { throw 'Episode must contain one or two recovery journals.' }
if (-not [bool]$baseAttempt.Passed -or
    [int]$baseAttempt.Condition -ne 1 -or [int]$eventAttempt.Condition -ne 3 -or
    [int]$baseAttempt.Profile -ne [int]$eventAttempt.Profile -or
    [string]$baseAttempt.Seed -cne [string]$eventAttempt.Seed -or
    [string]$baseAttempt.SourceCommit -cnotmatch '^[0-9a-f]{40}$' -or
    [string]$baseAttempt.SourceCommit -cne [string]$eventAttempt.SourceCommit) {
    throw 'NET-14V inputs are not one candidate-bound normal/recovery pair.'
}
$baseManifestObject = Get-Content -LiteralPath $baseManifest -Raw | ConvertFrom-Json
$eventManifestObject = Get-Content -LiteralPath $eventManifest -Raw | ConvertFrom-Json
if ([string]$baseManifestObject.Cell -cne 'net14ad' -or
    [string]$eventManifestObject.Cell -cne 'net14-recovery' -or
    [string]$baseManifestObject.Carrier -cne [string]$eventManifestObject.Carrier) {
    throw 'NET-14V manifests do not form one Carrier baseline/episode pair.'
}
$runnerHash = (Get-FileHash -LiteralPath $runner -Algorithm SHA256).Hash.ToLowerInvariant()
if ([string]$baseInputs.Files.runner -cne $runnerHash -or [string]$eventInputs.Files.runner -cne $runnerHash -or
    [string]$baseInputs.SourceCommit -cne [string]$baseAttempt.SourceCommit -or
    [string]$eventInputs.SourceCommit -cne [string]$eventAttempt.SourceCommit) {
    throw 'NET-14V evidence was produced by another runner candidate.'
}

$script:ssh = (Get-Command ssh.exe -CommandType Application).Source
$script:scp = (Get-Command scp.exe -CommandType Application).Source
$knownHosts = Join-Path $output 'ssh-known-hosts'
$script:sshOptions = @('-i', $key, '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=15',
    '-o', 'ConnectionAttempts=1', '-o', "UserKnownHostsFile=$knownHosts", '-o', 'StrictHostKeyChecking=accept-new')
$script:scpOptions = @('-i', $key, '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=15',
    '-o', "UserKnownHostsFile=$knownHosts", '-o', 'StrictHostKeyChecking=accept-new')
$attempt = [Guid]::NewGuid().ToString('N')
$remoteRoot = "/var/tmp/ardents-net14v-$attempt"
try {
    [void](Invoke-SSH "install -d -m 700 '$remoteRoot'" 'create NET-14V verifier root')
    foreach ($item in @(
        @($runner, "$remoteRoot/runner"),
        @($baseManifest, "$remoteRoot/baseline-manifest.json"),
        @($eventManifest, "$remoteRoot/episode-manifest.json"),
        @($baseVerdict, "$remoteRoot/baseline-verdict.json"),
        @($eventVerdict, "$remoteRoot/episode-verdict.json")
    )) { Send-File $item[0] $item[1] 'upload NET-14V input' }
    $remoteRecovery = @()
    for ($index = 0; $index -lt $recoveries.Count; $index++) {
        $remote = "$remoteRoot/recovery-$index.jsonl"
        Send-File $recoveries[$index].FullName $remote 'upload recovery evidence'
        $remoteRecovery += "'$remote'"
    }
    $verification = if ($eventPassed) { 'verify-net14v' } else { 'verify-failed-net14v' }
    $command = "'$remoteRoot/runner' $verification '$remoteRoot/baseline-manifest.json' " +
        "'$remoteRoot/episode-manifest.json' '$remoteRoot/baseline-verdict.json' " +
        "'$remoteRoot/episode-verdict.json' " + ($remoteRecovery -join ' ')
    $result = (Invoke-SSH "chmod 700 '$remoteRoot/runner'; $command" 'verify NET-14V pair') -join [Environment]::NewLine
    $verdictName = if ($eventPassed) { 'net14v-verdict.json' } else { 'failed-net14v-verdict.json' }
    Write-Utf8 (Join-Path $output $verdictName) ($result + [Environment]::NewLine)
    $verdict = $result | ConvertFrom-Json
    $expectedKind = if ($eventPassed) { 'net14v' } else { 'failed-net14v' }
    if ([string]$verdict.Kind -cne $expectedKind -or @($verdict.Criteria).Count -eq 0 -or
        @($verdict.Criteria | Where-Object { -not [bool]$_.Passed }).Count -ne 0) {
        throw 'NET-14V verifier returned an incomplete or failing verdict.'
    }
    $receipt = [ordered]@{ Schema='ardents-net14v-attempt-v1'; Host=$PublisherHost
        Carrier=[string]$eventManifestObject.Carrier; Profile=[int]$eventAttempt.Profile
        Seed=[string]$eventAttempt.Seed; SourceCommit=[string]$eventAttempt.SourceCommit; RunnerSHA256=$runnerHash; WorkloadPassed=$eventPassed; Passed=$true
        CompletedAt=[DateTimeOffset]::UtcNow.ToString('o') }
    Write-Utf8 (Join-Path $output 'attempt.json') (($receipt | ConvertTo-Json -Compress) + [Environment]::NewLine)
} catch {
    Write-Utf8 (Join-Path $output 'failure.txt') ($_.Exception.Message + [Environment]::NewLine)
    throw
} finally {
    try { [void](Invoke-SSH "case '$remoteRoot' in /var/tmp/ardents-net14v-[0-9a-f]*) rm -rf -- '$remoteRoot';; *) exit 64;; esac" 'remove NET-14V verifier root') } catch {}
    $inventory = foreach ($file in Get-ChildItem -LiteralPath $output -File | Where-Object Name -ne 'evidence-sha256.txt' | Sort-Object Name) {
        "{0}  {1}" -f ((Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()), $file.Name
    }
    Write-Utf8 (Join-Path $output 'evidence-sha256.txt') (($inventory -join [Environment]::NewLine) + [Environment]::NewLine)
}
Write-Output "NET-14V pair passed; evidence: $output"
