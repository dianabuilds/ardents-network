param(
    [Parameter(Mandatory = $true)][string]$EndpointHost,
    [Parameter(Mandatory = $true)][string]$SSHKey,
    [Parameter(Mandatory = $true)][string]$RunnerBinary,
    [Parameter(Mandatory = $true)][string]$SourceCommit,
    [Parameter(Mandatory = $true)][string]$Plan,
    [Parameter(Mandatory = $true)][string]$AuthorityInventory,
    [Parameter(Mandatory = $true)][string]$EvidenceOutput,
    [string]$User = 'root'
)

$ErrorActionPreference = 'Stop'
$repository = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..\..'))
$packageRoot = Join-Path $repository 'packaging\stream-qualification-worker'

function Resolve-InputFile([string]$Path, [string]$Name) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { throw "$Name is absent." }
    return (Resolve-Path -LiteralPath $Path).Path
}
function Assert-Host([string]$Value, [string]$Name) {
    if ($Value -notmatch '^(?:[A-Za-z0-9](?:[A-Za-z0-9.-]{0,251}[A-Za-z0-9])?|(?:\d{1,3}\.){3}\d{1,3})$') { throw "$Name is invalid." }
}
function Assert-RemotePath([string]$Value, [string]$Name) {
    if ($Value -notmatch '^/[A-Za-z0-9._/-]+$' -or $Value.Contains('//') -or $Value.Contains('/../')) { throw "$Name is not a canonical safe remote path." }
}
function Assert-Hex([string]$Value, [string]$Name) {
    if ($Value -cnotmatch '^[0-9a-f]{64}$') { throw "$Name must be lowercase SHA-256-width hex." }
}
function Write-Utf8([string]$Path, [string]$Body) {
    [IO.File]::WriteAllText($Path, $Body, [Text.UTF8Encoding]::new($false))
}
function Convert-Digest([object]$Value, [string]$Name) {
    $bytes = @($Value)
    if ($bytes.Count -ne 32) { throw "$Name is not a 32-byte observation." }
    $hex = -join ($bytes | ForEach-Object { $number = [int]$_; if ($number -lt 0 -or $number -gt 255) { throw "$Name contains an invalid byte." }; $number.ToString('x2') })
    Assert-Hex $hex $Name
    return $hex
}
function Invoke-Native([string]$Program, [string[]]$Arguments, [string]$Label) {
    & $Program @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$Label failed with exit code $LASTEXITCODE." }
}

Assert-Host $EndpointHost 'EndpointHost'
Assert-Host $User 'User'
$keyPath = Resolve-InputFile $SSHKey 'SSHKey'
$runnerPath = Resolve-InputFile $RunnerBinary 'RunnerBinary'
if ($SourceCommit -cnotmatch '^[0-9a-f]{40}$') { throw 'SourceCommit must be a full lowercase Git commit.' }
$actualCommit = ((& git -C $repository rev-parse HEAD 2>$null) -join '').Trim()
if ($LASTEXITCODE -ne 0 -or $actualCommit -cne $SourceCommit) { throw 'SourceCommit must equal the checked-out candidate commit.' }
$sourceStatus = @(& git -C $repository status --porcelain --untracked-files=normal)
if ($LASTEXITCODE -ne 0 -or $sourceStatus.Count -ne 0) { throw 'Qualification requires a clean source commit.' }
$planPath = Resolve-InputFile $Plan 'Plan'
$authorityPath = Resolve-InputFile $AuthorityInventory 'AuthorityInventory'
$installPath = Resolve-InputFile (Join-Path $packageRoot 'install.py') 'install.py'
$endpointInstallPath = Resolve-InputFile (Join-Path $packageRoot 'install_endpoint.py') 'install_endpoint.py'
$unitPath = Resolve-InputFile (Join-Path $packageRoot 'ardents-endpoint.service') 'ardents-endpoint.service'
$planObject = Get-Content -LiteralPath $planPath -Raw | ConvertFrom-Json
if ([string]$planObject.Mode -cne 'net32-idle' -or @($planObject.Participants).Count -ne 1) { throw 'Plan must be one explicit net32-idle participant.' }
$participant = $planObject.Participants[0]
if ([int]$participant.Role -ne 1 -or [int]$participant.Profile -ne 1 -or [int]$participant.Condition -ne 1 -or [int]$participant.ReaderIndex -ne 0 -or -not [string]::IsNullOrEmpty([string]$participant.Link)) { throw 'NET-32 participant shape is invalid.' }
$authority = Get-Content -LiteralPath $authorityPath -Raw | ConvertFrom-Json
foreach ($field in @('Host', 'Binary', 'VaultRoot', 'RecordID', 'EnvironmentCommitment', 'NetworkCommitment', 'RootCommitment', 'IDCommitment')) {
    if (-not $authority.PSObject.Properties[$field] -or [string]::IsNullOrWhiteSpace([string]$authority.$field)) { throw "Authority inventory lacks $field." }
}
Assert-Host $authority.Host 'authority Host'
foreach ($field in @('Binary', 'VaultRoot')) { Assert-RemotePath ([string]$authority.$field) "authority $field" }
if ([string]$authority.RecordID -notmatch '^[A-Za-z0-9._-]{1,160}$') { throw 'authority RecordID is invalid.' }
foreach ($field in @('EnvironmentCommitment', 'NetworkCommitment', 'RootCommitment', 'IDCommitment')) { Assert-Hex ([string]$authority.$field) "authority $field" }

$evidence = [IO.Path]::GetFullPath($EvidenceOutput)
if ($evidence.StartsWith($repository + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) { throw 'EvidenceOutput must remain outside the repository.' }
if (Test-Path -LiteralPath $evidence) { throw 'EvidenceOutput must be a new directory.' }
[IO.Directory]::CreateDirectory($evidence) | Out-Null
$ssh = (Get-Command ssh.exe -CommandType Application).Source
$scp = (Get-Command scp.exe -CommandType Application).Source
$knownHosts = Join-Path $evidence 'ssh-known-hosts'
$sshOptions = @('-i', $keyPath, '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=15', '-o', 'ConnectionAttempts=1', '-o', "UserKnownHostsFile=$knownHosts", '-o', 'StrictHostKeyChecking=accept-new')
$scpOptions = @('-i', $keyPath, '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=15', '-o', "UserKnownHostsFile=$knownHosts", '-o', 'StrictHostKeyChecking=accept-new')
$attempt = [Guid]::NewGuid().ToString('N')
$remoteRoot = "/var/tmp/ardents-net32-$attempt"
Assert-RemotePath $remoteRoot 'remote attempt root'
function Remote([string]$HostName) { return "$User@$HostName" }
function Invoke-SSH([string]$HostName, [string]$Command, [string]$Label) {
    $output = & $ssh @sshOptions -n (Remote $HostName) $Command 2>&1
    if ($LASTEXITCODE -ne 0) { throw "$Label failed: $($output -join [Environment]::NewLine)" }
    return @($output)
}
function Send-File([string]$Source, [string]$HostName, [string]$Destination, [string]$Label) {
    Invoke-Native $scp ($scpOptions + @($Source, "$(Remote $HostName):$Destination")) $Label
}
function Receive-File([string]$HostName, [string]$Source, [string]$Destination, [string]$Label) {
    Invoke-Native $scp ($scpOptions + @("$(Remote $HostName):$Source", $Destination)) $Label
}
function Journal([string]$HostName, [string]$Invocation) {
    return @(Invoke-SSH $HostName "journalctl _SYSTEMD_INVOCATION_ID=$Invocation --no-pager -o cat" 'read Endpoint journal')
}
function Wait-Event([string]$Invocation, [string]$Kind, [DateTime]$Deadline) {
    while ([DateTime]::UtcNow -lt $Deadline) {
        foreach ($line in (Journal $EndpointHost $Invocation)) {
            try { $record = $line | ConvertFrom-Json -ErrorAction Stop } catch { continue }
            if ([int]$record.Participant -eq 0 -and [string]$record.Event.Kind -ceq $Kind) { return $record.Event }
        }
        $state = ((Invoke-SSH $EndpointHost 'systemctl show ardents-endpoint.service -p ActiveState --value' 'read Endpoint state') -join '').Trim()
        if ($state -eq 'inactive' -or $state -eq 'failed') { throw "Endpoint stopped before $Kind." }
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out waiting for $Kind."
}
function Issue-Permission([object]$Files, [string]$Digest) {
    Assert-Hex $Digest 'Reader request digest'
    Assert-RemotePath ([string]$Files.RequestPath) 'Reader request path'
    Assert-RemotePath ([string]$Files.ResponsePath) 'Reader response path'
    $requestLocal = Join-Path ([IO.Path]::GetTempPath()) "$attempt-reader.request"
    $permissionLocal = Join-Path ([IO.Path]::GetTempPath()) "$attempt-reader.permission"
    $authorityRequest = "$remoteRoot/reader.request"
    $authorityPermission = "$remoteRoot/reader.permission"
    try {
        Receive-File $EndpointHost ([string]$Files.RequestPath) $requestLocal 'download Reader request'
        if ((Get-FileHash -LiteralPath $requestLocal -Algorithm SHA256).Hash.ToLowerInvariant() -cne $Digest) { throw 'Reader request differs from the observed commitment.' }
        [void](Invoke-SSH ([string]$authority.Host) "install -d -m 700 '$remoteRoot'" 'create authority staging')
        Send-File $requestLocal ([string]$authority.Host) $authorityRequest 'upload Reader request to custody'
        Write-Host "Enter this independently observed admission request commitment when custody asks: $Digest"
        $args = @('issue-admission-permission', '--vault-root', $authority.VaultRoot, '--record', $authority.RecordID, '--request', $authorityRequest, '--permission-output', $authorityPermission, '--environment-commitment', $authority.EnvironmentCommitment, '--network-commitment', $authority.NetworkCommitment, '--root-commitment', $authority.RootCommitment, '--kind', 'admission', '--id-commitment', $authority.IDCommitment)
        $quoted = ($args | ForEach-Object { "'$($_)'" }) -join ' '
        & $ssh @sshOptions -tt (Remote ([string]$authority.Host)) "'$($authority.Binary)' $quoted"
        if ($LASTEXITCODE -ne 0) { throw 'Reader custody issuance failed.' }
        Receive-File ([string]$authority.Host) $authorityPermission $permissionLocal 'download Reader permission'
        if ((Get-Item -LiteralPath $permissionLocal).Length -ne 228) { throw 'Reader permission has the wrong size.' }
        Send-File $permissionLocal $EndpointHost "$remoteRoot/reader.permission" 'upload Reader permission'
        [void](Invoke-SSH $EndpointHost "install -o ardents-endpoint -g ardents-endpoint -m 600 '$remoteRoot/reader.permission' '$($Files.ResponsePath)'" 'publish Reader permission')
    } finally {
        if (Test-Path -LiteralPath $requestLocal) { [IO.File]::Delete($requestLocal) }
        if (Test-Path -LiteralPath $permissionLocal) { [IO.File]::Delete($permissionLocal) }
        try { [void](Invoke-SSH ([string]$authority.Host) "rm -f '$authorityRequest' '$authorityPermission'" 'remove custody staging') } catch {}
    }
}

$invocation = $null
try {
    [IO.File]::Copy($planPath, (Join-Path $evidence 'plan.installed.json'))
    [IO.File]::Copy($authorityPath, (Join-Path $evidence 'authority-public-inventory.json'))
    $inputs = [ordered]@{ SourceCommit=$SourceCommit; Runner=(Get-FileHash $runnerPath -Algorithm SHA256).Hash.ToLowerInvariant(); Plan=(Get-FileHash $planPath -Algorithm SHA256).Hash.ToLowerInvariant(); Installer=(Get-FileHash $endpointInstallPath -Algorithm SHA256).Hash.ToLowerInvariant(); Unit=(Get-FileHash $unitPath -Algorithm SHA256).Hash.ToLowerInvariant() }
    Write-Utf8 (Join-Path $evidence 'input-digests.json') (($inputs | ConvertTo-Json -Compress) + "`n")
    $envelope = Invoke-SSH $EndpointHost 'uname -a; cat /etc/os-release; systemctl --version; stat -fc %T /sys/fs/cgroup' 'capture host envelope'
    Write-Utf8 (Join-Path $evidence 'environment.txt') (($envelope -join "`n") + "`n")
    [void](Invoke-SSH $EndpointHost "install -d -m 700 '$remoteRoot' '$remoteRoot/package'" 'create Endpoint staging')
    Send-File $runnerPath $EndpointHost "$remoteRoot/runner" 'upload runner'
    Send-File $planPath $EndpointHost "$remoteRoot/plan.json" 'upload plan'
    Send-File $installPath $EndpointHost "$remoteRoot/package/install.py" 'upload installer helper'
    Send-File $endpointInstallPath $EndpointHost "$remoteRoot/package/install_endpoint.py" 'upload Endpoint installer'
    Send-File $unitPath $EndpointHost "$remoteRoot/package/ardents-endpoint.service" 'upload Endpoint unit'
    [void](Invoke-SSH $EndpointHost "chmod 700 '$remoteRoot/package/install.py' '$remoteRoot/package/install_endpoint.py'; systemctl stop ardents-endpoint.service 2>/dev/null || :; python3 '$remoteRoot/package/install_endpoint.py' '$remoteRoot/runner' '$remoteRoot/plan.json'; systemctl reset-failed ardents-endpoint.service; systemctl start --no-block ardents-endpoint.service" 'install and start Endpoint')
    for ($poll = 0; $poll -lt 30; $poll++) {
        $invocation = ((Invoke-SSH $EndpointHost 'systemctl show ardents-endpoint.service -p InvocationID --value' 'read Endpoint invocation') -join '').Trim()
        if ($invocation -match '^[0-9a-f]{32}$') { break }
        Start-Sleep -Milliseconds 200
    }
    if ($invocation -notmatch '^[0-9a-f]{32}$') { throw 'Endpoint InvocationID was not published.' }
    $deadline = [DateTime]::UtcNow.AddMinutes(13)
    $event = Wait-Event $invocation 'permission-required' $deadline
    Issue-Permission $participant.Participant.ReaderPermission (Convert-Digest $event.RequestDigest 'Reader request digest')
    [void](Wait-Event $invocation 'idle-ready' $deadline)
    while ([DateTime]::UtcNow -lt $deadline) {
        $state = ((Invoke-SSH $EndpointHost 'systemctl show ardents-endpoint.service -p ActiveState --value' 'wait for NET-32 Endpoint') -join '').Trim()
        if ($state -eq 'inactive' -or $state -eq 'failed') { break }
        Start-Sleep -Seconds 1
    }
    $status = (Invoke-SSH $EndpointHost 'systemctl show ardents-endpoint.service -p ActiveState -p MainPID -p Result -p ExecMainStatus' 'read NET-32 completion') -join "`n"
    if ($status -notmatch 'ActiveState=inactive' -or $status -notmatch 'MainPID=0' -or $status -notmatch 'Result=success' -or $status -notmatch 'ExecMainStatus=0') { throw 'NET-32 Endpoint did not complete successfully.' }
    $journal = (Journal $EndpointHost $invocation) -join "`n"
    $journalPath = Join-Path $evidence 'endpoint.jsonl'
    Write-Utf8 $journalPath ($journal + "`n")
    Send-File $journalPath $EndpointHost "$remoteRoot/endpoint.jsonl" 'upload NET-32 evidence for verification'
    $verified = Invoke-SSH $EndpointHost "/usr/lib/ardents/qualification/ardents-qualification verify-run '$remoteRoot/endpoint.jsonl'" 'verify completed NET-32 evidence'
    Write-Utf8 (Join-Path $evidence 'completed-verdict.json') (($verified -join "`n") + "`n")
    Write-Utf8 (Join-Path $evidence 'attempt.json') (([ordered]@{ Schema='ardents-net32-short-projection-attempt-v1'; Host=$EndpointHost; Invocation=$invocation; ObservedMinutes=10; Observed24Hours=$false; SourceCommit=$SourceCommit; Passed=$true; CompletedAt=[DateTime]::UtcNow.ToString('o') } | ConvertTo-Json -Compress) + "`n")
} catch {
    $failure = $_.Exception.Message
    Write-Utf8 (Join-Path $evidence 'failure.txt') ($failure + "`n")
    if ($invocation -match '^[0-9a-f]{32}$') { try { Write-Utf8 (Join-Path $evidence 'endpoint.failed.jsonl') (((Journal $EndpointHost $invocation) -join "`n") + "`n") } catch {} }
    Write-Utf8 (Join-Path $evidence 'attempt.json') (([ordered]@{ Schema='ardents-net32-short-projection-attempt-v1'; Host=$EndpointHost; Invocation=$invocation; ObservedMinutes=10; Observed24Hours=$false; SourceCommit=$SourceCommit; Passed=$false; Failure=$failure; CompletedAt=[DateTime]::UtcNow.ToString('o') } | ConvertTo-Json -Compress) + "`n")
    throw
} finally {
    try { [void](Invoke-SSH $EndpointHost 'systemctl stop ardents-endpoint.service 2>/dev/null || :; systemctl reset-failed ardents-endpoint.service' 'stop bounded NET-32 Endpoint') } catch {}
    foreach ($hostName in @($EndpointHost, [string]$authority.Host) | Select-Object -Unique) {
        try { [void](Invoke-SSH $hostName "case '$remoteRoot' in /var/tmp/ardents-net32-[0-9a-f]*) rm -rf -- '$remoteRoot';; *) exit 64;; esac" 'remove bounded NET-32 staging') } catch {}
    }
    $inventory = foreach ($file in Get-ChildItem -LiteralPath $evidence -Recurse -File | Where-Object Name -ne 'evidence-sha256.txt' | Sort-Object FullName) {
        $relative = $file.FullName.Substring($evidence.Length).TrimStart('\', '/').Replace('\', '/')
        "{0}  {1}" -f ((Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()), $relative
    }
    Write-Utf8 (Join-Path $evidence 'evidence-sha256.txt') (($inventory -join "`n") + "`n")
}

Write-Output "Installed NET-32 short projection passed; evidence: $evidence"
