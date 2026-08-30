[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$PrimaryVPS,
    [Parameter(Mandatory = $true)][string]$PrimarySSHKey,
    [Parameter(Mandatory = $true)][string]$PrimaryUser,
    [Parameter(Mandatory = $true)][ValidateRange(1024, 65470)][int]$PrimaryBasePort,
    [Parameter(Mandatory = $true)][string]$SecondaryVPS,
    [AllowEmptyString()][string]$SecondarySSHKey = '',
    [AllowEmptyString()][string]$SecondaryPasswordFile = '',
    [Parameter(Mandatory = $true)][string]$SecondaryHostKey,
    [Parameter(Mandatory = $true)][string]$SecondaryUser,
    [Parameter(Mandatory = $true)][string]$EvidenceOutput,
    [Parameter(Mandatory = $true)][string]$OperatorHistoryBase64,
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

function Invoke-NativeCaptured([string]$Executable, [string[]]$Arguments) {
    $preference = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        $output = @(& $Executable @Arguments 2>&1 | ForEach-Object { $_.ToString() })
        return @{ ExitCode=$LASTEXITCODE; Output=$output }
    } finally {
        $ErrorActionPreference = $preference
    }
}

function Resolve-WindowsOpenSSHCommand([string]$Name) {
    $command = Get-Command $Name -ErrorAction SilentlyContinue
    if ($null -ne $command) { return $command.Source }
    foreach ($root in @('Sysnative', 'System32')) {
        $candidate = Join-Path $env:WINDIR "$root\OpenSSH\$Name"
        if (Test-Path -LiteralPath $candidate -PathType Leaf) { return $candidate }
    }
    throw "required Windows OpenSSH command is unavailable: $Name"
}

function Resolve-RemoteAccess([string]$Key, [string]$PasswordFile, [string]$HostKey, [string]$User, [string]$RemoteHost) {
    $target = "$User@$RemoteHost"
    if ($PasswordFile -ne '') {
        $ssh = (Get-Command 'plink.exe' -ErrorAction Stop).Source
        $scp = (Get-Command 'pscp.exe' -ErrorAction Stop).Source
        return @{ SSH=$ssh; SSHBase=@('-batch','-noagent','-hostkey',$HostKey,'-pwfile',$PasswordFile,$target); SCP=$scp; SCPBase=@('-batch','-noagent','-hostkey',$HostKey,'-pwfile',$PasswordFile) }
    }
    if ([IO.Path]::GetExtension($Key) -ieq '.ppk') {
        $ssh = (Get-Command 'plink.exe' -ErrorAction Stop).Source
        $scp = (Get-Command 'pscp.exe' -ErrorAction Stop).Source
        return @{ SSH=$ssh; SSHBase=@('-batch','-noagent','-hostkey',$HostKey,'-i',$Key,$target); SCP=$scp; SCPBase=@('-batch','-noagent','-hostkey',$HostKey,'-i',$Key) }
    }
    $ssh = Resolve-WindowsOpenSSHCommand 'ssh.exe'
    $scp = Resolve-WindowsOpenSSHCommand 'scp.exe'
    $options = @('-i',$Key,'-o','BatchMode=yes','-o','IdentitiesOnly=yes','-o','ConnectTimeout=10','-o','StrictHostKeyChecking=accept-new')
    return @{ SSH=$ssh; SSHBase=($options + @($target)); SCP=$scp; SCPBase=$options }
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
foreach ($key in @($PrimarySSHKey)) {
    if (-not [IO.Path]::IsPathRooted($key) -or -not (Test-Path -LiteralPath $key -PathType Leaf)) {
        throw 'H4-5 SSH keys must be existing absolute files'
    }
}
if (($SecondarySSHKey -eq '') -eq ($SecondaryPasswordFile -eq '')) {
    throw 'H4-5 requires exactly one SecondarySSHKey or SecondaryPasswordFile'
}
if ($SecondarySSHKey -ne '' -and (-not [IO.Path]::IsPathRooted($SecondarySSHKey) -or -not (Test-Path -LiteralPath $SecondarySSHKey -PathType Leaf))) {
    throw 'H4-5 secondary SSH key must be one existing absolute file'
}
if ($SecondarySSHKey -ne '' -and [IO.Path]::GetExtension($SecondarySSHKey) -ine '.ppk') {
    throw 'H4-5 secondary SSH key must be one PuTTY PPK so its declared fingerprint is enforced'
}
if ($SecondaryPasswordFile -ne '' -and (-not [IO.Path]::IsPathRooted($SecondaryPasswordFile) -or -not (Test-Path -LiteralPath $SecondaryPasswordFile -PathType Leaf))) {
    throw 'H4-5 secondary password file must be one existing absolute file'
}
if ($SecondaryPasswordFile -ne '') {
    $passwordPath = [IO.Path]::GetFullPath($SecondaryPasswordFile)
    $repositoryPrefix = $repository.TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar
    if ($passwordPath.StartsWith($repositoryPrefix, [StringComparison]::OrdinalIgnoreCase)) {
        throw 'H4-5 secondary password file must remain outside the repository'
    }
    $passwordItem = Get-Item -LiteralPath $passwordPath -Force
    if (($passwordItem.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw 'H4-5 secondary password file cannot be a reparse point'
    }
    $passwordACL = Get-Acl -LiteralPath $passwordPath
    $currentWindowsIdentity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $currentIdentity = $currentWindowsIdentity.Name
    $currentSID = $currentWindowsIdentity.User.Value
    $ownerSID = $passwordACL.GetOwner([Security.Principal.SecurityIdentifier]).Value
    if ($ownerSID -cne $currentSID) {
        throw 'H4-5 secondary password file is not owned by the current operator'
    }
    foreach ($rule in $passwordACL.Access) {
        $sid = $rule.IdentityReference.Translate([Security.Principal.SecurityIdentifier]).Value
        if ($rule.AccessControlType -eq [Security.AccessControl.AccessControlType]::Allow -and $sid -cne $currentSID) {
            throw 'H4-5 secondary password file grants another principal access'
        }
    }
}
if ([string]::IsNullOrWhiteSpace($SecondaryHostKey)) { throw 'H4-5 secondary host-key fingerprint is absent' }
if ([IO.Path]::GetExtension($PrimarySSHKey) -ieq '.ppk') {
    throw 'PrimarySSHKey must be OpenSSH-compatible because the installed-host Go oracle owns its SSH calls; PPK is supported for SecondarySSHKey'
}
if (-not [IO.Path]::IsPathRooted($EvidenceOutput)) { throw 'H4-5 evidence output must be absolute' }
if (Test-Path -LiteralPath $EvidenceOutput) { throw 'H4-5 evidence output must not already exist' }
try {
    $operatorHistoryRaw = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($OperatorHistoryBase64))
    $operatorHistory = $operatorHistoryRaw | ConvertFrom-Json
} catch {
    throw 'H4-5 operator history must be one UTF-8 Base64 JSON document'
}
$historyProperties = @($operatorHistory.PSObject.Properties.Name)
$historyRequired = @('schema','manual_commands','failed_attempts','clarifications','repairs','provisioning','input_verification','preparation_active_human_seconds','active_human_limitation')
if (@($historyRequired | Where-Object { $historyProperties -cnotcontains $_ }).Count -ne 0 -or
    $operatorHistory.schema -cne 'ardents-h4-5-operator-history-v1') {
    throw 'H4-5 operator history schema is invalid'
}
if ($null -ne $operatorHistory.preparation_active_human_seconds -and [int64]$operatorHistory.preparation_active_human_seconds -lt 0) {
    throw 'H4-5 preparation active-human time is invalid'
}
if ([string]::IsNullOrWhiteSpace([string]$operatorHistory.active_human_limitation)) {
    throw 'H4-5 active-human measurement limitation is absent'
}

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
    $primaryAccess = Resolve-RemoteAccess $PrimarySSHKey '' '' $PrimaryUser $PrimaryVPS
    $secondaryAccess = Resolve-RemoteAccess $SecondarySSHKey $SecondaryPasswordFile $SecondaryHostKey $SecondaryUser $SecondaryVPS
    Invoke-Checked $primaryAccess.SSH ($primaryAccess.SSHBase + @('test "$(id -u)" -eq 0; test "$(ps -p 1 -o comm=)" = systemd; docker image inspect golang:1.26.6 >/dev/null')) 'primary VPS preflight failed'
    Invoke-Checked $secondaryAccess.SSH ($secondaryAccess.SSHBase + @('test "$(id -u)" -eq 0; docker image inspect golang:1.26.6 >/dev/null')) 'secondary VPS preflight failed'

    [void](New-Item -ItemType Directory -Path $EvidenceOutput -Force)
    Write-Utf8NoBom (Join-Path $EvidenceOutput 'operator-history.json') ($operatorHistoryRaw.TrimEnd() + "`n")
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
        param($Name, $Repository, $Evidence, $Artifacts, $Revision, $PrimaryHost, $PrimaryKey, $PrimaryLogin, $PrimarySSH, $PrimarySSHBase, $BasePort,
            $SecondaryHost, $SecondaryLogin, $SecondarySSH, $SecondarySSHBase, $SecondarySCP, $SecondarySCPBase, $CampaignID)
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
            $hostFacts = 'set -eu; printf "boot_id="; cat /proc/sys/kernel/random/boot_id; uname -srmo; printf "vcpus="; nproc; head -n 1 /proc/meminfo; df -Pk / /var /tmp; ip -s link; systemctl --version | head -n 1; docker image inspect golang:1.26.6 --format "image_id={{.Id}}"'
            if ((Run-Cell 'primary-host-envelope' $PrimarySSH ($PrimarySSHBase + @($hostFacts))) -ne 0) { $failed++ }
            $common = @{ ARDENTS_H4_3B_VPS=$PrimaryHost; ARDENTS_H4_3B_SSH_KEY=$PrimaryKey; ARDENTS_H4_3B_VPS_USER=$PrimaryLogin; ARDENTS_H4_3B_VPS_PORT="$BasePort"; ARDENTS_H4_8_A11_SUFFIX=$CampaignID }
            $soak = @{} + $common
            $soak.ARDENTS_H4_5_EVIDENCE_DIR = Join-Path $Evidence 'primary-bounded-mixed-workload'
            if ((Run-Cell 'primary-bounded-mixed-workload' 'go' @('test','-tags','h4_3b_multihost h4_5_rendezvous','./tests/e2e/service','-run','^TestH45InstalledRendezvousBoundedMixedWorkload$','-count=1','-timeout=3m') $soak) -ne 0) { $failed++ }
            $reboot = @{} + $common
            $reboot.ARDENTS_H4_5_EVIDENCE_DIR = Join-Path $Evidence 'primary-kill-reboot-recovery'
            if ((Run-Cell 'primary-kill-reboot-recovery' 'go' @('test','-tags','h4_3b_multihost h4_5_rendezvous','./tests/e2e/service','-run','^TestH45InstalledRendezvousRecoversFromKillAndHostReboot$','-count=1','-timeout=4m') $reboot) -ne 0) { $failed++ }
            $smoke = @{} + $common
            $smoke.ARDENTS_H4_5_EVIDENCE_DIR = Join-Path $Evidence 'primary-final-smoke'
            if ((Run-Cell 'primary-final-smoke' 'go' @('test','-tags','h4_3b_multihost h4_5_rendezvous','./tests/e2e/service','-run','^TestH45InstalledRendezvousPublisherToUserLifecycle$','-count=1','-timeout=4m') $smoke) -ne 0) { $failed++ }
        } elseif ($Name -ceq 'local') {
            if ((Run-Cell 'local-capacity-hostile' 'go' @('test','./internal/node','-run','^TestRendezvous(PairsExactAuthenticatedLegsAndDrains|DrainPreservesActivePairInsideLease|RejectsDuplicateSideWithoutDisplacingWaitingLeg|RejectsUnauthorizedOrMismatchedBoundIdentity|ExpiresUnpairedLeg|BoundsEachPairedDirection|ReservesHandshakeWaitingAndPairSlots)$','-count=1','-timeout=2m')) -ne 0) { $failed++ }
            if ((Run-Cell 'local-pressure-storage' 'go' @('test','./internal/resource','-run','^TestRendezvousDedicatedHost(ProfileProtectsRecoversAndDrains|StorageCeilingDrains)$','-count=1','-timeout=2m')) -ne 0) { $failed++ }
            if ((Run-Cell 'local-update-rollback' 'go' @('test','./internal/contributor','-run','^Test(PinnedSuccessorUpdatesAndRestartsSameDeployment|FailedSuccessorRestoresPreviousReadyGeneration|AmbiguousUpdateStopFailureRestartsPreviousReadyGeneration|DrainStopsWorkAndWithdrawalAlsoDisablesService|RemovalRequiresExactWithdrawnDeploymentAndLeavesBundle)$','-count=1','-timeout=2m')) -ne 0) { $failed++ }
            if ((Run-Cell 'local-source-state' 'go' @('test','./internal/network/state','-run','^TestRefresh(WaitsForTwoAuthenticatedSourcesAndRestarts|PersistsTLSFailureBackoff|RejectsUncertainClockBeforeContact|StagesFutureEpochAndActivatesAfterRestart)$','-count=1','-timeout=2m')) -ne 0) { $failed++ }
            if ((Run-Cell 'local-stream-semantics' 'go' @('test','./internal/endpoint','-run','^Test(SlowConsumersApplyBackpressureUntilLocalCancellation|LogicalQueueBackpressuresAtFrozenDirectionalCap|OrderlyHalfCloseReportsObservedPartialCounts)$','-count=1','-timeout=2m')) -ne 0) { $failed++ }
            $mount = ($Artifacts -replace '\\','/') + ':/artifacts:ro'
            $dockerTest = "/artifacts/e2e-node.test -test.run='^Test(NativeDutyProcessesUseTheirExactStateAssignments|RendezvousNodeProcessPairsOnlyStateAuthorizedLegs|RendezvousNodeProcessKeepsActivePairPastAdmissionDeadline)$' -test.count=1 -test.timeout=3m"
            if ((Run-Cell 'local-docker-listener-pressure' 'docker' @('run','--rm','--name',"ardents-h4-5-local-$CampaignID",'--network','none','--read-only','--tmpfs','/tmp:rw,nosuid,nodev,size=256m','--memory','512m','--cpus','1','--pids-limit','128','-e','ARDENTS_E2E_COMMAND_ROOT=/artifacts','-v',$mount,'golang:1.26.6','sh','-c',$dockerTest)) -ne 0) { $failed++ }
        } elseif ($Name -ceq 'secondary-linux') {
            $remoteRoot = "/tmp/ardents-h4-5-$CampaignID"
            $target = "$SecondaryLogin@$SecondaryHost"
            $hostFacts = 'set -eu; printf "boot_id="; cat /proc/sys/kernel/random/boot_id; uname -srmo; printf "vcpus="; nproc; head -n 1 /proc/meminfo; df -Pk / /var /tmp; ip -s link; docker image inspect golang:1.26.6 --format "image_id={{.Id}}"'
            if ((Run-Cell 'secondary-host-envelope' $SecondarySSH ($SecondarySSHBase + @($hostFacts))) -ne 0) { $failed++ }
            if ((Run-Cell 'secondary-stage' $SecondarySSH ($SecondarySSHBase + @("set -eu; umask 077; test ! -e '$remoteRoot'; mkdir -m 700 '$remoteRoot'"))) -ne 0) { $failed++ }
            $uploads = $SecondarySCPBase + @((Join-Path $Artifacts 'ardents'),(Join-Path $Artifacts 'ardents-node'),(Join-Path $Artifacts 'e2e-node.test'),"${target}:$remoteRoot/")
            if ($failed -eq 0 -and (Run-Cell 'secondary-upload' $SecondarySCP $uploads) -ne 0) { $failed++ }
            $remoteTest = "set -eu; chmod 700 '$remoteRoot/ardents' '$remoteRoot/ardents-node' '$remoteRoot/e2e-node.test'; docker run --rm --name 'ardents-h4-5-secondary-$CampaignID' --network none --read-only --tmpfs /tmp:rw,nosuid,nodev,size=256m --memory 512m --cpus 1 --pids-limit 128 -e ARDENTS_E2E_COMMAND_ROOT=/artifacts -v '${remoteRoot}:/artifacts:ro' golang:1.26.6 /artifacts/e2e-node.test '-test.run=^Test(RendezvousNodeProcessBoundsIncompleteTLSHandshakes|RendezvousNodeProcessExpiresIncompleteTLSAdmission|RendezvousNodeProcessDrainsActivePairOnSIGTERM|TwoNodeProcessesRefreshWithdrawRestartAndReassign)$' -test.count=1 -test.timeout=3m"
            if ($failed -eq 0 -and (Run-Cell 'secondary-linux-process' $SecondarySSH ($SecondarySSHBase + @($remoteTest))) -ne 0) { $failed++ }
            if ((Run-Cell 'secondary-cleanup' $SecondarySSH ($SecondarySSHBase + @("set -eu; test -d '$remoteRoot'; rm -f '$remoteRoot/ardents' '$remoteRoot/ardents-node' '$remoteRoot/e2e-node.test'; rmdir '$remoteRoot'; test ! -e '$remoteRoot'"))) -ne 0) { $failed++ }
        }
        [pscustomobject]@{ shard=$Name; failures=$failed }
    }

    $jobs = @(
        Start-Job -Name 'h45-primary' -ScriptBlock $shard -ArgumentList 'primary-installed',$repository,$EvidenceOutput,$artifactRoot,$revision,$PrimaryVPS,$PrimarySSHKey,$PrimaryUser,$primaryAccess.SSH,$primaryAccess.SSHBase,$PrimaryBasePort,$SecondaryVPS,$SecondaryUser,$secondaryAccess.SSH,$secondaryAccess.SSHBase,$secondaryAccess.SCP,$secondaryAccess.SCPBase,$campaignID
        Start-Job -Name 'h45-local' -ScriptBlock $shard -ArgumentList 'local',$repository,$EvidenceOutput,$artifactRoot,$revision,$PrimaryVPS,$PrimarySSHKey,$PrimaryUser,$primaryAccess.SSH,$primaryAccess.SSHBase,$PrimaryBasePort,$SecondaryVPS,$SecondaryUser,$secondaryAccess.SSH,$secondaryAccess.SSHBase,$secondaryAccess.SCP,$secondaryAccess.SCPBase,$campaignID
        Start-Job -Name 'h45-secondary' -ScriptBlock $shard -ArgumentList 'secondary-linux',$repository,$EvidenceOutput,$artifactRoot,$revision,$PrimaryVPS,$PrimarySSHKey,$PrimaryUser,$primaryAccess.SSH,$primaryAccess.SSHBase,$PrimaryBasePort,$SecondaryVPS,$SecondaryUser,$secondaryAccess.SSH,$secondaryAccess.SSHBase,$secondaryAccess.SCP,$secondaryAccess.SCPBase,$campaignID
    )
    [void](Wait-Job -Job $jobs -Timeout 3000)
    $timedOut = @($jobs | Where-Object State -eq 'Running')
    foreach ($job in $timedOut) { Stop-Job -Job $job }
    [void]@($jobs | Receive-Job -Wait -AutoRemoveJob -ErrorAction SilentlyContinue)

    $cleanupRoot = Join-Path $EvidenceOutput 'controller-final-cleanup'
    [void](New-Item -ItemType Directory -Path $cleanupRoot -Force)
    $cleanupStarted = [DateTimeOffset]::UtcNow
    $cleanupOutput = [Collections.Generic.List[string]]::new()
    $cleanupFailures = 0
    $fixtureName = "ardents-h4-8-a11-$campaignID"
    $primaryCleanup = @'
set -eu
fixture='__FIXTURE__'
for work in "/tmp/$fixture" "/var/tmp/$fixture"; do
  if [ -x "$work/bundle-1/ardents-node" ] && [ -f "$work/bundle-1/manifest.json" ]; then
    pin=$(sha256sum "$work/bundle-1/manifest.json" | cut -d' ' -f1)
    "$work/bundle-1/ardents-node" contributor apply --bundle "$work/bundle-1" --manifest-pin "$pin" >/dev/null 2>&1 || true
  fi
done
program=/usr/lib/ardents-contributor/current/ardents-node
record=/var/lib/private/ardents-contributor/installation.json
if [ -x "$program" ] && [ -f "$record" ]; then
  deployment=$(sed -n 's/.*"deployment_id":"\([^"]*\)".*/\1/p' "$record")
  case "$deployment" in ''|*[!0-9a-f]*) exit 1 ;; esac
  [ "${#deployment}" -eq 64 ]
  "$program" contributor withdraw >/dev/null 2>&1 || true
  "$program" contributor remove --confirm "$deployment" >/dev/null
fi
docker rm -f "$fixture" >/dev/null 2>&1 || true
for work in "/tmp/$fixture" "/var/tmp/$fixture"; do
  if [ -e "$work" ]; then rm -rf -- "$work"; fi
  test ! -e "$work"
done
test ! -e /etc/systemd/system/ardents-rendezvous-contributor.service
test ! -e /usr/lib/ardents-contributor
test ! -e /var/lib/private/ardents-contributor
! systemctl is-active --quiet ardents-rendezvous-contributor.service
! ss -ltnH "sport = :__PORT__" | grep -q .
'@
    $primaryCleanup = $primaryCleanup.Replace('__FIXTURE__',$fixtureName).Replace('__PORT__',"$($PrimaryBasePort + 1)")
    $secondaryRoot = "/tmp/ardents-h4-5-$campaignID"
    $secondaryContainer = "ardents-h4-5-secondary-$campaignID"
    $secondaryCleanup = "set -eu; docker rm -f '$secondaryContainer' >/dev/null 2>&1 || true; if [ -d '$secondaryRoot' ]; then rm -f '$secondaryRoot/ardents' '$secondaryRoot/ardents-node' '$secondaryRoot/e2e-node.test'; rmdir '$secondaryRoot'; fi; test ! -e '$secondaryRoot'; ! docker container inspect '$secondaryContainer' >/dev/null 2>&1"
    $cleanupSteps = @(
        @{ Label='primary'; Executable=$primaryAccess.SSH; Arguments=($primaryAccess.SSHBase + @($primaryCleanup)) },
        @{ Label='secondary'; Executable=$secondaryAccess.SSH; Arguments=($secondaryAccess.SSHBase + @($secondaryCleanup)) }
    )
    foreach ($step in $cleanupSteps) {
        $cleanupOutput.Add("[$($step.Label)]")
        try {
            $stepArguments = @($step.Arguments)
            $output = @(& $step.Executable @stepArguments 2>&1 | ForEach-Object { $_.ToString() })
            foreach ($line in $output) { $cleanupOutput.Add($line) }
            if ($LASTEXITCODE -ne 0) { $cleanupFailures++ }
        } catch {
            $cleanupOutput.Add($_.Exception.Message)
            $cleanupFailures++
        }
    }
    $localContainer = "ardents-h4-5-local-$campaignID"
    $cleanupOutput.Add('[local]')
    $inspection = Invoke-NativeCaptured 'docker' @('container','inspect',$localContainer)
    if ($inspection.ExitCode -eq 0) {
        $removal = Invoke-NativeCaptured 'docker' @('rm','-f',$localContainer)
        foreach ($line in $removal.Output) { $cleanupOutput.Add($line) }
        if ($removal.ExitCode -ne 0) { $cleanupFailures++ }
    }
    $inspection = Invoke-NativeCaptured 'docker' @('container','inspect',$localContainer)
    if ($inspection.ExitCode -eq 0) { $cleanupFailures++ } else { $cleanupOutput.Add('container_absent=true') }
    $cleanupFinished = [DateTimeOffset]::UtcNow
    Write-Utf8NoBom (Join-Path $cleanupRoot 'output.log') (($cleanupOutput -join "`n") + "`n")
    $cleanupRecord = [ordered]@{ schema='ardents-h4-5-cell-v1'; cell='controller-final-cleanup'; shard='controller'; source_revision=$revision;
        started_at=$cleanupStarted.ToString('o'); finished_at=$cleanupFinished.ToString('o'); duration_ms=[int64]($cleanupFinished-$cleanupStarted).TotalMilliseconds;
        exit_code=$cleanupFailures; outcome=$(if ($cleanupFailures -eq 0) {'passed'} else {'failed'}) }
    Write-Utf8NoBom (Join-Path $cleanupRoot 'result.json') (($cleanupRecord | ConvertTo-Json) + "`n")
    $finished = [DateTimeOffset]::UtcNow

    $expected = [ordered]@{
        'primary-host-envelope'='primary-installed'; 'primary-bounded-mixed-workload'='primary-installed';
        'primary-kill-reboot-recovery'='primary-installed'; 'primary-final-smoke'='primary-installed';
        'local-capacity-hostile'='local'; 'local-pressure-storage'='local'; 'local-update-rollback'='local';
        'local-source-state'='local'; 'local-stream-semantics'='local'; 'local-docker-listener-pressure'='local';
        'secondary-host-envelope'='secondary-linux'; 'secondary-stage'='secondary-linux'; 'secondary-upload'='secondary-linux';
        'secondary-linux-process'='secondary-linux'; 'secondary-cleanup'='secondary-linux'; 'controller-final-cleanup'='controller'
    }
    $records = @()
    $validated = 0
    foreach ($cell in $expected.Keys) {
        $path = Join-Path $EvidenceOutput "$cell\result.json"
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { continue }
        try { $record = Get-Content -LiteralPath $path -Raw | ConvertFrom-Json } catch { continue }
        $records += $record
        $properties = @($record.PSObject.Properties.Name)
        $required = @('schema','cell','shard','source_revision','duration_ms','exit_code','outcome')
        if (@($required | Where-Object { $properties -cnotcontains $_ }).Count -ne 0) { continue }
        try {
            if ($record.schema -ceq 'ardents-h4-5-cell-v1' -and $record.cell -ceq $cell -and
                $record.shard -ceq $expected[$cell] -and $record.source_revision -ceq $revision -and
                [int64]$record.duration_ms -ge 0 -and [int]$record.exit_code -eq 0 -and $record.outcome -ceq 'passed') { $validated++ }
        } catch { continue }
    }
    $summary = [ordered]@{ schema='ardents-h4-5-campaign-v1'; source_revision=$revision; campaign_id=$campaignID;
        primary_vps=$PrimaryVPS; secondary_vps=$SecondaryVPS; started_at=$controllerStarted.ToString('o'); finished_at=$finished.ToString('o');
        duration_seconds=[int64]($finished-$controllerStarted).TotalSeconds; deadline_seconds=3600; expected_cells=$expected.Count;
        observed_cells=$records.Count; passed_cells=$validated; failed_cells=($expected.Count-$validated); timed_out_shards=$timedOut.Count;
        outcome=$(if ($records.Count -eq $expected.Count -and $validated -eq $expected.Count -and $timedOut.Count -eq 0 -and ($finished-$controllerStarted).TotalMinutes -le 60) {'accepted'} else {'rejected'}) }
    Write-Utf8NoBom (Join-Path $EvidenceOutput 'campaign-summary.json') (($summary | ConvertTo-Json -Depth 4) + "`n")
    $burden = [ordered]@{ schema='ardents-h4-5-operator-burden-v1'; source_revision=$revision; campaign_id=$campaignID;
        measurement='one non-interactive controller invocation; all campaign remote commands and cleanup are runner-owned';
        wall_seconds=$summary.duration_seconds; campaign_active_human_seconds=0;
        preparation_active_human_seconds=$operatorHistory.preparation_active_human_seconds;
        active_human_limitation=$operatorHistory.active_human_limitation; controller_invocations=1;
        manual_command_count=@($operatorHistory.manual_commands).Count; failed_attempt_count=@($operatorHistory.failed_attempts).Count;
        clarification_count=@($operatorHistory.clarifications).Count; repair_count=@($operatorHistory.repairs).Count;
        automated_cells=$expected.Count; failed_cells=$summary.failed_cells; history='operator-history.json' }
    Write-Utf8NoBom (Join-Path $EvidenceOutput 'operator-burden.json') (($burden | ConvertTo-Json -Depth 4) + "`n")
    if ($summary.outcome -cne 'accepted') { throw "H4-5 campaign rejected: $validated/$($expected.Count) cells passed" }
    Write-Output "H4-5 campaign accepted: $validated/$($expected.Count) cells in $($summary.duration_seconds)s"
} finally {
    Pop-Location
}
