param([Parameter(Mandatory=$true)][ValidatePattern('^r171-[a-z0-9-]{1,32}$')][string]$RunName,
      [Parameter(Mandatory=$true)][string]$EvidenceRoot,
      [ValidateSet('synthetic','node')][string]$SourceProfile='node')
$ErrorActionPreference='Stop'
$taskRepo=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../..'))
$taskRoot=[IO.Path]::GetFullPath($EvidenceRoot)
if(Test-Path -LiteralPath $taskRoot){throw 'Refuse existing storage evidence'}
if($taskRoot.Equals($taskRepo,[StringComparison]::OrdinalIgnoreCase) -or $taskRoot.StartsWith($taskRepo+[IO.Path]::DirectorySeparatorChar,[StringComparison]::OrdinalIgnoreCase)){throw 'Storage evidence must remain outside Git'}
$taskAncestor=[IO.DirectoryInfo]::new($taskRoot)
while($null -ne $taskAncestor){
    if($taskAncestor.Exists -and ($taskAncestor.Attributes -band [IO.FileAttributes]::ReparsePoint)){throw 'Refuse redirected evidence root'}
    $taskAncestor=$taskAncestor.Parent
}
[IO.Directory]::CreateDirectory($taskRoot) | Out-Null
$taskAcl=[Security.AccessControl.DirectorySecurity]::new()
$taskAcl.SetAccessRuleProtection($true,$false)
$taskSid=[Security.Principal.WindowsIdentity]::GetCurrent().User
$taskAcl.SetOwner($taskSid)
foreach($taskPrincipal in @($taskSid,[Security.Principal.SecurityIdentifier]::new('S-1-5-18'))){
    $taskAcl.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new($taskPrincipal,'FullControl','ContainerInherit,ObjectInherit','None','Allow'))
}
[IO.Directory]::SetAccessControl($taskRoot,$taskAcl)
$taskHelper='sha256:0ecc73f220e154f40bdf3db718f9ff66890d6b381adaaf05fc3cbe989441c3ad'
$taskRoles=@('fixture','prometheus','alertmanager','loki','alloy','grafana')
if($SourceProfile -eq 'node'){$taskRoles += 'browser-tunnel'}else{$taskRoles += 'state-anchor'}
$taskAllowed=@('/state','/tmp','/dev/shm','/events','/fixture','/metrics-certs','/prometheus','/alertmanager')
$taskResults=@();$taskUnavailable=@()
$taskScript=@"
import json,os,signal,stat,time
def expire(*_): raise RuntimeError('Selected filesystem observation exceeded cooperative budget')
signal.signal(signal.SIGALRM,expire)
signal.alarm(10)
roots=os.environ['R171_SELECTED_FS_ROOTS'].split(',')
if not isinstance(roots,list) or len(roots)>16: raise RuntimeError('Invalid selected root set')
result=[]
for root in roots:
    if root not in ('/state','/tmp','/dev/shm','/events','/fixture','/metrics-certs','/prometheus','/alertmanager'):
        raise RuntimeError('Unselected filesystem root')
    try:
        path='/proc/1/root'+root
        entry=os.stat(path)
        if not stat.S_ISDIR(entry.st_mode): raise RuntimeError('Selected root is not a directory')
        info=os.statvfs(path)
        result.append({'root':root,'available':True,'filesystem_device':entry.st_dev,
                       'capacity_bytes':info.f_blocks*info.f_frsize,
                       'used_bytes':(info.f_blocks-info.f_bfree)*info.f_frsize,
                       'available_bytes':info.f_bavail*info.f_frsize})
    except OSError as error:
        result.append({'root':root,'available':False,'errno':error.errno})
print(json.dumps({'at':time.time(),'filesystems':result}))
"@
foreach($taskRole in $taskRoles){
    $taskName=$RunName+'-'+$taskRole+'-1'
    $taskInspect=docker inspect $taskName | ConvertFrom-Json
    if($LASTEXITCODE -ne 0 -or @($taskInspect).Count -ne 1){throw 'Selected container unavailable'}
    $taskItem=$taskInspect[0]
    if($taskItem.Config.Labels.'com.docker.compose.project' -cne $RunName -or $taskItem.Config.Labels.'com.docker.compose.service' -cne $taskRole -or -not $taskItem.State.Running -or $taskItem.State.Paused -or $taskItem.State.Restarting -or $taskItem.State.OOMKilled -or ($taskItem.Config.User -cne '10001:10001' -and -not ($taskRole -eq 'browser-tunnel' -and $taskItem.Config.User -ceq '10002:10002'))){throw 'Refuse unowned or unavailable container'}
    if($taskItem.Id -cnotmatch '^[0-9a-f]{64}$'){throw 'Invalid selected container identity'}
    $taskRoots=@('/dev/shm')
    $taskExternal=@()
    foreach($taskProperty in $taskItem.HostConfig.Tmpfs.psobject.Properties){
        if($taskProperty.Name -cnotin $taskAllowed){throw 'Unselected writable temporary mount'}
        $taskRoots += $taskProperty.Name
    }
    foreach($taskMount in $taskItem.Mounts){
        if($taskMount.Type -eq 'volume'){
            if($taskMount.Destination -cin $taskAllowed){$taskRoots += $taskMount.Destination}
            elseif($taskMount.RW){$taskExternal += $taskMount.Destination}
        }elseif($taskMount.RW){$taskExternal += $taskMount.Destination}
    }
    $taskRoots=@($taskRoots | Sort-Object -Unique)
    $taskRootsArgument=$taskRoots -join ','
    # Share only the explicitly selected container PID namespace. No host walk,
    # root user, ptrace capability, file content or producer raw data.
    $taskObservation=docker run --rm --pull never --network none --pid ('container:'+$taskItem.Id) --read-only --user $taskItem.Config.User --cap-drop ALL --security-opt no-new-privileges --memory 128m --cpus 0.25 --pids-limit 16 --shm-size 1m --env ('R171_SELECTED_FS_ROOTS='+$taskRootsArgument) --entrypoint python3 $taskHelper -c $taskScript
    if($LASTEXITCODE -ne 0){throw 'Selected filesystem observation failed'}
    $taskAfter=docker inspect $taskName | ConvertFrom-Json
    if($LASTEXITCODE -ne 0 -or $taskAfter[0].Id -cne $taskItem.Id -or $taskAfter[0].State.StartedAt -cne $taskItem.State.StartedAt -or $taskAfter[0].RestartCount -ne $taskItem.RestartCount -or -not $taskAfter[0].State.Running -or $taskAfter[0].State.Paused){throw 'Container changed during filesystem observation'}
    $taskObserved=$taskObservation | ConvertFrom-Json
    if(@($taskObserved.filesystems).Count -ne $taskRoots.Count){throw 'Incomplete selected filesystem observation'}
    foreach($taskFilesystem in $taskObserved.filesystems){
        if(-not $taskFilesystem.available){$taskUnavailable += [pscustomobject]@{role=$taskRole;root=$taskFilesystem.root;errno=$taskFilesystem.errno}}
        elseif($taskFilesystem.capacity_bytes -le 0 -or $taskFilesystem.used_bytes -lt 0 -or $taskFilesystem.used_bytes -gt $taskFilesystem.capacity_bytes){throw 'Invalid native filesystem quantities'}
    }
    $taskResults += [pscustomobject]@{role=$taskRole;observed_at=$taskObserved.at;filesystems=$taskObserved.filesystems;external_writable_mounts=$taskExternal;docker_log_driver=$taskItem.HostConfig.LogConfig.Type;docker_log_options=$taskItem.HostConfig.LogConfig.Config}
}
$taskUnique=@{}
foreach($taskResult in $taskResults){foreach($taskFilesystem in $taskResult.filesystems){
    if($taskFilesystem.available){
        $taskKey=[string]$taskFilesystem.filesystem_device
        if(-not $taskUnique.ContainsKey($taskKey)){$taskUnique[$taskKey]=$taskFilesystem}
    }
}}
$taskCapacity=0L;$taskUsed=0L
foreach($taskFilesystem in $taskUnique.Values){$taskCapacity += [long]$taskFilesystem.capacity_bytes;$taskUsed += [long]$taskFilesystem.used_bytes}
$taskReceipt=[ordered]@{run=$RunName;source_profile=$SourceProfile;filesystem_observations=$taskResults;unavailable=$taskUnavailable;unique_filesystems=$taskUnique.Count;observed_capacity_bytes=$taskCapacity;observed_filesystem_used_bytes=$taskUsed;capacity_budget_bytes=2147483648;within_selected_filesystem_budget=($taskUnavailable.Count -eq 0 -and $taskCapacity -le 2147483648);scope='live non-atomic selected-container filesystems, including index/WAL/temporary allocations; used bytes cover the entire filesystem, including metadata and other allocations on a shared backing filesystem, not selected-directory or process usage';complete_disk_budget_verified=$false;excluded='Docker log bytes and external writable bind mounts require separate bounded evidence; no backend admission or complete disk-budget claim'}
$taskReceipt | ConvertTo-Json -Depth 9 | Out-File (Join-Path $taskRoot 'storage-inventory.json') -Encoding utf8
if($taskUnavailable.Count){throw 'Selected filesystem measurements unavailable; receipt retained'}
if($taskCapacity -gt 2147483648){throw 'Selected filesystem capacity exceeds two-GiB budget'}
Write-Output 'Selected native filesystem quantities recorded; external storage coverage remains explicit.'
