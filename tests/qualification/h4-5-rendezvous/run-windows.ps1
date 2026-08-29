[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$PrimaryVPS,
    [Parameter(Mandatory = $true)][string]$PrimarySSHKey,
    [Parameter(Mandatory = $true)][string]$PrimaryUser,
    [Parameter(Mandatory = $true)][ValidateRange(1024, 65470)][int]$PrimaryBasePort,
    [Parameter(Mandatory = $true)][string]$SecondaryVPS,
    [Parameter(Mandatory = $true)][string]$SecondarySSHKey,
    [Parameter(Mandatory = $true)][string]$SecondaryUser,
    [Parameter(Mandatory = $true)][string]$EvidenceOutput,
    [switch]$ValidateOnly
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$controllerStarted = [DateTimeOffset]::UtcNow

function Write-Utf8NoBom([string]$Path, [string]$Contents) {
    [IO.File]::WriteAllText($Path, $Contents, [Text.UTF8Encoding]::new($false))
}

function Assert-LiteralIPv4([string]$Value, [string]$Name) {
    $parsed = $null
    if (-not [Net.IPAddress]::TryParse($Value, [ref]$parsed) -or $parsed.AddressFamily -ne [Net.Sockets.AddressFamily]::InterNetwork) {
        throw "$Name must be one literal IPv4 address"
    }
}

function Invoke-Checked([string]$Executable, [string[]]$Arguments, [string]$Failure) {
    & $Executable @Arguments
    if ($LASTEXITCODE -ne 0) { throw $Failure }
}

$repository = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
if ((Resolve-Path $MyInvocation.MyCommand.Path).Path -cne (Join-Path $repository 'tests\qualification\h4-5-rendezvous\run-windows.ps1')) {
    throw 'H4-5 qualification is owned by its repository runner'
}
Assert-LiteralIPv4 $PrimaryVPS 'PrimaryVPS'
Assert-LiteralIPv4 $SecondaryVPS 'SecondaryVPS'
if ($PrimaryVPS -ceq $SecondaryVPS) { throw 'H4-5 requires two distinct declared VPS addresses' }
foreach ($value in @($PrimaryUser, $SecondaryUser)) {
    if ($value -cnotmatch '^[a-z_][a-z0-9_-]{0,31}$') { throw 'H4-5 VPS user is invalid' }
}
foreach ($key in @($PrimarySSHKey, $SecondarySSHKey)) {
    if (-not [IO.Path]::IsPathRooted($key) -or -not (Test-Path -LiteralPath $key -PathType Leaf)) {
        throw 'H4-5 SSH keys must be existing absolute files'
    }
}
if (-not [IO.Path]::IsPathRooted($EvidenceOutput)) { throw 'H4-5 evidence output must be absolute' }
if (Test-Path -LiteralPath $EvidenceOutput) { throw 'H4-5 evidence output must not already exist' }

Push-Location $repository
try {
    $revision = (& git rev-parse HEAD).Trim()
    if ($LASTEXITCODE -ne 0 -or $revision -cnotmatch '^[0-9a-f]{40}$') { throw 'read H4-5 source revision failed' }
    $dirty = @(& git status --porcelain=v1 --untracked-files=all)
    if ($LASTEXITCODE -ne 0 -or $dirty.Count -ne 0) { throw 'H4-5 qualification requires one clean committed worktree' }
    if ($ValidateOnly) {
        [void][ScriptBlock]::Create((Get-Content -LiteralPath $MyInvocation.MyCommand.Path -Raw))
        Write-Output "validated H4-5 runner at $revision"
        exit 0
    }

    Invoke-Checked 'docker' @('image', 'inspect', 'golang:1.26.6', '--format', '{{.Id}}') 'local pinned Docker image is unavailable'
    $sshOptions = @('-o', 'BatchMode=yes', '-o', 'ConnectTimeout=10', '-o', 'StrictHostKeyChecking=accept-new')
    Invoke-Checked 'ssh' (@('-i', $PrimarySSHKey) + $sshOptions + @("$PrimaryUser@$PrimaryVPS", 'test "$(id -u)" -eq 0; test "$(ps -p 1 -o comm=)" = systemd; docker image inspect golang:1.26.6 >/dev/null')) 'primary VPS preflight failed'
    Invoke-Checked 'ssh' (@('-i', $SecondarySSHKey) + $sshOptions + @("$SecondaryUser@$SecondaryVPS", 'test "$(id -u)" -eq 0; docker image inspect golang:1.26.6 >/dev/null')) 'secondary VPS preflight failed'

    [void](New-Item -ItemType Directory -Path $EvidenceOutput -Force)
    $artifactRoot = Join-Path $EvidenceOutput 'artifacts'
    [void](New-Item -ItemType Directory -Path $artifactRoot)
    $oldGOOS, $oldGOARCH, $oldCGO = $env:GOOS, $env:GOARCH, $env:CGO_ENABLED
    try {
        $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = 'linux', 'amd64', '0'
        Invoke-Checked 'go' @('build', '-trimpath', '-buildvcs=false', '-o', (Join-Path $artifactRoot 'ardents'), './cmd/ardents') 'build Linux ardents oracle command failed'
        Invoke-Checked 'go' @('build', '-trimpath', '-buildvcs=false', '-o', (Join-Path $artifactRoot 'ardents-node'), './cmd/ardents-node') 'build Linux ardents-node oracle command failed'
        Invoke-Checked 'go' @('test', '-c', '-o', (Join-Path $artifactRoot 'e2e-node.test'), './tests/e2e/node') 'build Linux Node-process oracle failed'
    } finally {
        $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = $oldGOOS, $oldGOARCH, $oldCGO
    }
    Get-ChildItem -LiteralPath $artifactRoot -File | Get-FileHash -Algorithm SHA256 |
        Select-Object @{n='file';e={Split-Path $_.Path -Leaf}}, @{n='sha256';e={$_.Hash.ToLowerInvariant()}} |
        ConvertTo-Json | ForEach-Object { Write-Utf8NoBom (Join-Path $artifactRoot 'sha256.json') ($_ + "`n") }

    $campaignID = [Guid]::NewGuid().ToString('N')
    $shard = {
        param($Name, $Repository, $Evidence, $Artifacts, $Revision, $PrimaryHost, $PrimaryKey, $PrimaryLogin, $BasePort,
            $SecondaryHost, $SecondaryKey, $SecondaryLogin, $CampaignID)
        Set-StrictMode -Version Latest
        $ErrorActionPreference = 'Stop'
        function Write-Text([string]$Path, [string]$Contents) {
            [IO.File]::WriteAllText($Path, $Contents, [Text.UTF8Encoding]::new($false))
        }
        function Run-Cell([string]$Cell, [string]$Executable, [string[]]$Arguments, [hashtable]$Environment = @{}) {
            $root = Join-Path $Evidence $Cell
            [void](New-Item -ItemType Directory -Path $root -Force)
            $before = @{}
            foreach ($key in $Environment.Keys) {
                $before[$key] = [Environment]::GetEnvironmentVariable($key, 'Process')
                [Environment]::SetEnvironmentVariable($key, $Environment[$key], 'Process')
            }
            $cellStarted = [DateTimeOffset]::UtcNow
            try {
                $output = @(& $Executable @Arguments 2>&1 | ForEach-Object { $_.ToString() })
                $exitCode = $LASTEXITCODE
            } catch {
                $output = @($_.Exception.Message)
                $exitCode = 255
            } finally {
                foreach ($key in $Environment.Keys) { [Environment]::SetEnvironmentVariable($key, $before[$key], 'Process') }
            }
            $finished = [DateTimeOffset]::UtcNow
            Write-Text (Join-Path $root 'output.log') (($output -join "`n") + "`n")
            $record = [ordered]@{ schema='ardents-h4-5-cell-v1'; cell=$Cell; shard=$Name; source_revision=$Revision;
                started_at=$cellStarted.ToString('o'); finished_at=$finished.ToString('o'); duration_ms=[int64]($finished-$cellStarted).TotalMilliseconds;
                exit_code=$exitCode; outcome=$(if ($exitCode -eq 0) {'passed'} else {'failed'}) }
            Write-Text (Join-Path $root 'result.json') (($record | ConvertTo-Json) + "`n")
            return $exitCode
        }
        Set-Location $Repository
        $failed = 0
        if ($Name -ceq 'primary-installed') {
            $target = "$PrimaryLogin@$PrimaryHost"
            $ssh = @('-i',$PrimaryKey,'-o','BatchMode=yes','-o','ConnectTimeout=10','-o','StrictHostKeyChecking=accept-new')
            $hostFacts = 'set -eu; printf "boot_id="; cat /proc/sys/kernel/random/boot_id; uname -srmo; printf "vcpus="; nproc; awk ''/MemTotal:/ {printf "memory_kib=%s\n", $2}'' /proc/meminfo; df -Pk / /var /tmp; ip -s link; systemctl --version | head -n 1; docker image inspect golang:1.26.6 --format "image_id={{.Id}}"'
            if ((Run-Cell 'primary-host-envelope' 'ssh' ($ssh + @($target,$hostFacts))) -ne 0) { $failed++ }
            $common = @{ ARDENTS_H4_3B_VPS=$PrimaryHost; ARDENTS_H4_3B_SSH_KEY=$PrimaryKey; ARDENTS_H4_3B_VPS_USER=$PrimaryLogin; ARDENTS_H4_3B_VPS_PORT="$BasePort" }
            $soak = @{} + $common
            $soak.ARDENTS_H4_5_EVIDENCE_DIR = Join-Path $Evidence 'primary-eight-minute-soak'
            if ((Run-Cell 'primary-eight-minute-soak' 'go' @('test','-tags','h4_3b_multihost h4_5_rendezvous','./tests/e2e/service','-run','^TestH45InstalledRendezvousEightMinuteMixedSoak$','-count=1','-timeout=12m') $soak) -ne 0) { $failed++ }
            $reboot = @{} + $common
            $reboot.ARDENTS_H4_5_EVIDENCE_DIR = Join-Path $Evidence 'primary-kill-reboot-recovery'
            if ((Run-Cell 'primary-kill-reboot-recovery' 'go' @('test','-tags','h4_3b_multihost h4_5_rendezvous','./tests/e2e/service','-run','^TestH45InstalledRendezvousRecoversFromKillAndHostReboot$','-count=1','-timeout=4m') $reboot) -ne 0) { $failed++ }
            $smoke = @{} + $common
            $smoke.ARDENTS_H4_5_EVIDENCE_DIR = Join-Path $Evidence 'primary-final-smoke'
            if ((Run-Cell 'primary-final-smoke' 'go' @('test','-tags','h4_3b_multihost h4_5_rendezvous','./tests/e2e/service','-run','^TestH45InstalledRendezvousPublisherToUserLifecycle$','-count=1','-timeout=4m') $smoke) -ne 0) { $failed++ }
        } elseif ($Name -ceq 'local') {
            if ((Run-Cell 'local-capacity-hostile' 'go' @('test','./internal/node','-run','^TestRendezvous(PairsExactAuthenticatedLegsAndDrains|DrainPreservesActivePairInsideLease|RejectsDuplicateSideWithoutDisplacingWaitingLeg|RejectsUnauthorizedOrMismatchedBoundIdentity|ExpiresUnpairedLeg|BoundsEachPairedDirection|ReservesHandshakeWaitingAndPairSlots)$','-count=1','-timeout=2m')) -ne 0) { $failed++ }
            if ((Run-Cell 'local-pressure-storage' 'go' @('test','./internal/resource','-run','^TestRendezvousFunctionalAlpha(ProfileProtectsRecoversAndDrains|StorageCeilingDrains)$','-count=1','-timeout=2m')) -ne 0) { $failed++ }
            if ((Run-Cell 'local-update-rollback' 'go' @('test','./internal/contributor','-run','^Test(PinnedSuccessorUpdatesAndRestartsSameDeployment|FailedSuccessorRestoresPreviousReadyGeneration|AmbiguousUpdateStopFailureRestartsPreviousReadyGeneration|DrainStopsWorkAndWithdrawalAlsoDisablesService|RemovalRequiresExactWithdrawnDeploymentAndLeavesBundle)$','-count=1','-timeout=2m')) -ne 0) { $failed++ }
            if ((Run-Cell 'local-source-state' 'go' @('test','./internal/network/state','-run','^TestRefresh(WaitsForTwoAuthenticatedSourcesAndRestarts|PersistsTLSFailureBackoff|RejectsUncertainClockBeforeContact|StagesFutureEpochAndActivatesAfterRestart)$','-count=1','-timeout=2m')) -ne 0) { $failed++ }
            if ((Run-Cell 'local-stream-semantics' 'go' @('test','./internal/endpoint','-run','^Test(SlowConsumersApplyBackpressureUntilLocalCancellation|LogicalQueueBackpressuresAtFrozenDirectionalCap|OrderlyHalfCloseReportsObservedPartialCounts)$','-count=1','-timeout=2m')) -ne 0) { $failed++ }
            $mount = ($Artifacts -replace '\\','/') + ':/artifacts:ro'
            $dockerTest = "/artifacts/e2e-node.test -test.run='^Test(NativeDutyProcessesUseTheirExactStateAssignments|RendezvousNodeProcessPairsOnlyStateAuthorizedLegs|RendezvousNodeProcessKeepsActivePairPastAdmissionDeadline)$' -test.count=1 -test.timeout=3m"
            if ((Run-Cell 'local-docker-listener-pressure' 'docker' @('run','--rm','--network','none','--read-only','--tmpfs','/tmp:rw,nosuid,nodev,size=256m','--memory','512m','--cpus','1','--pids-limit','128','-e','ARDENTS_E2E_COMMAND_ROOT=/artifacts','-v',$mount,'golang:1.26.6','sh','-c',$dockerTest)) -ne 0) { $failed++ }
        } elseif ($Name -ceq 'secondary-linux') {
            $remoteRoot = "/tmp/ardents-h4-5-$CampaignID"
            $target = "$SecondaryLogin@$SecondaryHost"
            $ssh = @('-i',$SecondaryKey,'-o','BatchMode=yes','-o','ConnectTimeout=10','-o','StrictHostKeyChecking=accept-new')
            $hostFacts = 'set -eu; printf "boot_id="; cat /proc/sys/kernel/random/boot_id; uname -srmo; printf "vcpus="; nproc; awk ''/MemTotal:/ {printf "memory_kib=%s\n", $2}'' /proc/meminfo; df -Pk / /var /tmp; ip -s link; docker image inspect golang:1.26.6 --format "image_id={{.Id}}"'
            if ((Run-Cell 'secondary-host-envelope' 'ssh' ($ssh + @($target,$hostFacts))) -ne 0) { $failed++ }
            if ((Run-Cell 'secondary-stage' 'ssh' ($ssh + @($target,"set -eu; umask 077; test ! -e '$remoteRoot'; mkdir -m 700 '$remoteRoot'"))) -ne 0) { $failed++ }
            $uploads = @('-i',$SecondaryKey,'-o','BatchMode=yes','-o','ConnectTimeout=10',(Join-Path $Artifacts 'ardents'),(Join-Path $Artifacts 'ardents-node'),(Join-Path $Artifacts 'e2e-node.test'),"${target}:$remoteRoot/")
            if ($failed -eq 0 -and (Run-Cell 'secondary-upload' 'scp' $uploads) -ne 0) { $failed++ }
            $remoteTest = "set -eu; chmod 700 '$remoteRoot/ardents' '$remoteRoot/ardents-node' '$remoteRoot/e2e-node.test'; docker run --rm --network none --read-only --tmpfs /tmp:rw,nosuid,nodev,size=256m --memory 512m --cpus 1 --pids-limit 128 -e ARDENTS_E2E_COMMAND_ROOT=/artifacts -v '${remoteRoot}:/artifacts:ro' golang:1.26.6 /artifacts/e2e-node.test '-test.run=^Test(RendezvousNodeProcessBoundsIncompleteTLSHandshakes|RendezvousNodeProcessExpiresIncompleteTLSAdmission|RendezvousNodeProcessDrainsActivePairOnSIGTERM|TwoNodeProcessesRefreshWithdrawRestartAndReassign)$' -test.count=1 -test.timeout=3m"
            if ($failed -eq 0 -and (Run-Cell 'secondary-linux-process' 'ssh' ($ssh + @($target,$remoteTest))) -ne 0) { $failed++ }
            if ((Run-Cell 'secondary-cleanup' 'ssh' ($ssh + @($target,"set -eu; test -d '$remoteRoot'; rm -f '$remoteRoot/ardents' '$remoteRoot/ardents-node' '$remoteRoot/e2e-node.test'; rmdir '$remoteRoot'; test ! -e '$remoteRoot'"))) -ne 0) { $failed++ }
        }
        [pscustomobject]@{ shard=$Name; failures=$failed }
    }

    $jobs = @(
        Start-Job -Name 'h45-primary' -ScriptBlock $shard -ArgumentList 'primary-installed',$repository,$EvidenceOutput,$artifactRoot,$revision,$PrimaryVPS,$PrimarySSHKey,$PrimaryUser,$PrimaryBasePort,$SecondaryVPS,$SecondarySSHKey,$SecondaryUser,$campaignID
        Start-Job -Name 'h45-local' -ScriptBlock $shard -ArgumentList 'local',$repository,$EvidenceOutput,$artifactRoot,$revision,$PrimaryVPS,$PrimarySSHKey,$PrimaryUser,$PrimaryBasePort,$SecondaryVPS,$SecondarySSHKey,$SecondaryUser,$campaignID
        Start-Job -Name 'h45-secondary' -ScriptBlock $shard -ArgumentList 'secondary-linux',$repository,$EvidenceOutput,$artifactRoot,$revision,$PrimaryVPS,$PrimarySSHKey,$PrimaryUser,$PrimaryBasePort,$SecondaryVPS,$SecondarySSHKey,$SecondaryUser,$campaignID
    )
    [void](Wait-Job -Job $jobs -Timeout 3000)
    $timedOut = @($jobs | Where-Object State -eq 'Running')
    foreach ($job in $timedOut) { Stop-Job -Job $job }
    [void]@($jobs | Receive-Job -Wait -AutoRemoveJob)
    $finished = [DateTimeOffset]::UtcNow

    $expected = @('primary-host-envelope','primary-eight-minute-soak','primary-kill-reboot-recovery','primary-final-smoke','local-capacity-hostile','local-pressure-storage',
        'local-update-rollback','local-source-state','local-stream-semantics','local-docker-listener-pressure',
        'secondary-host-envelope','secondary-stage','secondary-upload','secondary-linux-process','secondary-cleanup')
    $records = @()
    foreach ($cell in $expected) {
        $path = Join-Path $EvidenceOutput "$cell\result.json"
        if (Test-Path -LiteralPath $path) { $records += Get-Content -LiteralPath $path -Raw | ConvertFrom-Json }
    }
    $passed = @($records | Where-Object outcome -eq 'passed').Count
    $summary = [ordered]@{ schema='ardents-h4-5-campaign-v1'; source_revision=$revision; campaign_id=$campaignID;
        primary_vps=$PrimaryVPS; secondary_vps=$SecondaryVPS; started_at=$controllerStarted.ToString('o'); finished_at=$finished.ToString('o');
        duration_seconds=[int64]($finished-$controllerStarted).TotalSeconds; deadline_seconds=3600; expected_cells=$expected.Count;
        observed_cells=$records.Count; passed_cells=$passed; failed_cells=($records.Count-$passed); timed_out_shards=$timedOut.Count;
        outcome=$(if ($records.Count -eq $expected.Count -and $passed -eq $expected.Count -and $timedOut.Count -eq 0 -and ($finished-$controllerStarted).TotalMinutes -le 60) {'accepted'} else {'rejected'}) }
    Write-Utf8NoBom (Join-Path $EvidenceOutput 'campaign-summary.json') (($summary | ConvertTo-Json -Depth 4) + "`n")
    if ($summary.outcome -cne 'accepted') { throw "H4-5 campaign rejected: $passed/$($expected.Count) cells passed" }
    Write-Output "H4-5 campaign accepted: $passed/$($expected.Count) cells in $($summary.duration_seconds)s"
} finally {
    Pop-Location
}
