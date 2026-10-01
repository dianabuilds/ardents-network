param([Parameter(Mandatory=$true)][string]$EvidenceRoot,
      [Parameter(Mandatory=$true)][ValidatePattern('^r171-[a-z0-9-]{1,32}$')][string]$RunName, [switch]$RestartProbe, [switch]$ResourceProbe, [switch]$BackendProbe)
$ErrorActionPreference='Stop'
if (($ResourceProbe -or $BackendProbe) -and -not $RestartProbe) { throw 'Resource profile requires explicit restart-state profile.' }
$taskRoot=[IO.Path]::GetFullPath($EvidenceRoot)
$taskRepo=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../..'))
if ($taskRoot.Equals($taskRepo,[StringComparison]::OrdinalIgnoreCase) -or
    $taskRoot.StartsWith($taskRepo+[IO.Path]::DirectorySeparatorChar,[StringComparison]::OrdinalIgnoreCase)) {
    throw 'Probe evidence must be outside Git.'
}
if (Test-Path -LiteralPath $taskRoot) { throw 'Refuse reuse of probe evidence.' }
$taskAncestor=[IO.DirectoryInfo]::new([IO.Path]::GetDirectoryName($taskRoot))
while ($null -ne $taskAncestor) {
    if ($taskAncestor.Exists -and ($taskAncestor.Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw 'No redirected evidence ancestors.' }
    $taskAncestor=$taskAncestor.Parent
}
New-Item -ItemType Directory -Path $taskRoot | Out-Null
$taskSid=[Security.Principal.WindowsIdentity]::GetCurrent().User
$taskAcl=[Security.AccessControl.DirectorySecurity]::new()
$taskAcl.SetAccessRuleProtection($true,$false)
$taskAcl.SetOwner($taskSid)
foreach ($taskPrincipal in @($taskSid,[Security.Principal.SecurityIdentifier]::new('S-1-5-18'))) {
    $taskAcl.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new($taskPrincipal,'FullControl','ContainerInherit,ObjectInherit','None','Allow'))
}
[IO.Directory]::SetAccessControl($taskRoot,$taskAcl)
$taskPrivate=Join-Path $taskRoot 'private'; $taskReports=Join-Path $taskRoot 'reports'
New-Item -ItemType Directory -Path $taskPrivate,$taskReports | Out-Null
$taskLock=Get-Content (Join-Path $PSScriptRoot 'images.json') -Raw | ConvertFrom-Json
$taskEnv=[ordered]@{R171_SOURCE=$PSScriptRoot;R171_PRIVATE=$taskPrivate}
$taskHelper=docker image inspect ardents-diagnostics:prebuilt-parsers-final-389 --format '{{.Id}}'
if ($LASTEXITCODE -ne 0) { throw 'Explicitly installed diagnostic helper missing.' }
$taskEnv.R171_HELPER_IMAGE=$taskHelper
foreach ($taskImage in $taskLock.images) {
    docker image inspect $taskImage.image | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Pinned image missing; run explicit make tools-install.' }
    $taskEnv[('R171_'+$taskImage.name.ToUpper()+'_IMAGE')]=$taskImage.image
}
$taskOld=@{}; foreach ($taskKey in $taskEnv.Keys) { $taskOld[$taskKey]=[Environment]::GetEnvironmentVariable($taskKey); [Environment]::SetEnvironmentVariable($taskKey,$taskEnv[$taskKey]) }
$taskCompose=@('compose','-p',$RunName,'-f',(Join-Path $PSScriptRoot 'compose.yaml'))
if ($RestartProbe) { $taskCompose += @('-f',(Join-Path $PSScriptRoot 'compose.restart.yaml')) }
$taskReceipt=[ordered]@{started_utc=(Get-Date).ToUniversalTime().ToString('o');run=$RunName;source_scope='synthetic fixture only';complete=$false}
function Invoke-ProbeQuery([string]$Mode,[string]$ReportName) {
    $taskDestination=Join-Path $taskReports $ReportName
    New-Item -ItemType Directory -Path $taskDestination | Out-Null
    docker run --rm --network ($RunName+'_probe') --read-only --cap-drop ALL --security-opt no-new-privileges --user 10001:10001 --memory 256m --cpus 0.5 --pids-limit 16 --shm-size 1m --log-driver local --log-opt max-size=2m --log-opt max-file=2 --mount "type=bind,source=$PSScriptRoot,target=/probe,readonly" --mount "type=bind,source=$taskPrivate/query,target=/certs,readonly" --mount "type=bind,source=$taskDestination,target=/reports" --mount "type=bind,source=$taskReports,target=/history,readonly" --entrypoint python3 $taskHelper /probe/query-probe.py $Mode $ReportName
    if ($LASTEXITCODE -ne 0) { throw "Query $Mode failed; preserve original reports." }
}
try {
    docker run --rm --network none --read-only --cap-drop ALL --security-opt no-new-privileges --user 10001:10001 --memory 128m --cpus 0.5 --pids-limit 16 --mount "type=bind,source=$PSScriptRoot,target=/probe,readonly" --mount "type=bind,source=$taskPrivate,target=/private" --entrypoint python3 $taskHelper /probe/prepare-private.py
    if ($LASTEXITCODE -ne 0) { throw 'Private preparation failed.' }
    docker @taskCompose config --quiet
    if ($LASTEXITCODE -ne 0) { throw 'Compose validation failed.' }
    docker @taskCompose up -d --pull never alertmanager loki prometheus alloy grafana
    if ($LASTEXITCODE -ne 0) { throw 'Backend startup failed.' }
    Invoke-ProbeQuery 'ready' 'readiness'
    docker @taskCompose up -d --pull never fixture
    if ($LASTEXITCODE -ne 0) { throw 'Source startup failed.' }
    Start-Sleep -Seconds 12
    Invoke-ProbeQuery 'observe' 'normal'
    Start-Sleep -Seconds 30
    Invoke-ProbeQuery 'observe' 'pressure'
    Invoke-ProbeQuery 'lifecycle' 'silenced'
    Invoke-ProbeQuery 'lifecycle' 'silence-expired'
    Start-Sleep -Seconds 40
    Invoke-ProbeQuery 'lifecycle' 'threshold-recovered'
    docker @taskCompose pause fixture
    if ($LASTEXITCODE -ne 0) { throw 'Synthetic source freeze failed.' }
    Invoke-ProbeQuery 'lifecycle' 'source-pending'
    Invoke-ProbeQuery 'lifecycle' 'source-firing'
    docker @taskCompose unpause fixture
    if ($LASTEXITCODE -ne 0) { throw 'Synthetic source resume failed.' }
    Invoke-ProbeQuery 'lifecycle' 'source-recovered'
    docker @taskCompose pause alloy
    if ($LASTEXITCODE -ne 0) { throw 'Synthetic collector freeze failed.' }
    Invoke-ProbeQuery 'lifecycle' 'collector-pending'
    Invoke-ProbeQuery 'lifecycle' 'collector-firing'
    docker @taskCompose unpause alloy
    if ($LASTEXITCODE -ne 0) { throw 'Synthetic collector resume failed.' }
    Invoke-ProbeQuery 'lifecycle' 'collector-recovered'
    Invoke-ProbeQuery 'catchup' 'post-outage-logs'
    Invoke-ProbeQuery 'shared-interval' 'grafana-shared-interval'
    if ($BackendProbe) {
        Invoke-ProbeQuery 'backend-baseline' 'backend-baseline'
        docker @taskCompose stop --timeout 5 loki
        if ($LASTEXITCODE -ne 0) { throw 'Synthetic log backend stop failed.' }
        Invoke-ProbeQuery 'backend-unavailable' 'backend-unavailable'
        docker @taskCompose start loki
        if ($LASTEXITCODE -ne 0) { throw 'Synthetic log backend start failed.' }
        Invoke-ProbeQuery 'ready' 'backend-recovery-readiness'
        Invoke-ProbeQuery 'catchup' 'post-backend-outage-logs'
    }
    if ($RestartProbe) {
        Invoke-ProbeQuery 'restart-before' 'restart-before'
        docker @taskCompose restart --timeout 5 alertmanager loki prometheus alloy grafana
        if ($LASTEXITCODE -ne 0) { throw 'Synthetic backend restart failed.' }
        Invoke-ProbeQuery 'ready' 'restart-readiness'
        Start-Sleep -Seconds 4
        Invoke-ProbeQuery 'restart-after' 'restart-after'
        Invoke-ProbeQuery 'catchup' 'post-restart-logs'
    }
    if ($ResourceProbe) {
        & (Join-Path $PSScriptRoot 'resource-window.ps1') -RunName $RunName -ReportsRoot $taskReports
        Invoke-ProbeQuery 'healthy-window' 'healthy-window'
        Invoke-ProbeQuery 'catchup' 'post-resource-logs'
    }
    docker @taskCompose stats --no-stream --format json | Out-File (Join-Path $taskReports 'resource-snapshot.json') -Encoding utf8
    if ($RestartProbe) {
        $taskStorageScript = @"
import json, os
results=[]
for name in ('prometheus','alertmanager','loki','alloy','grafana'):
    root='/state/'+name
    blocks=0
    count=0
    for directory,dirs,files in os.walk(root):
        for entry in files:
            stat=os.stat(os.path.join(directory,entry),follow_symlinks=False)
            blocks+=stat.st_blocks*512
            count+=1
    fs=os.statvfs(root)
    results.append({'scope':name,'file_count':count,'allocated_file_bytes':blocks,'filesystem_used_bytes':(fs.f_blocks-fs.f_bfree)*fs.f_frsize,'filesystem_capacity_bytes':fs.f_blocks*fs.f_frsize})
print(json.dumps({'states':results,'limit':'live non-atomic sample; excludes Grafana /tmp, fixture state and Docker service logs'}))
"@
        $taskAnchor=docker @taskCompose ps -q state-anchor
        if ($LASTEXITCODE -ne 0 -or -not $taskAnchor) { throw 'Storage observation mount holder unavailable.' }
        docker exec $taskAnchor python3 -c $taskStorageScript | Out-File (Join-Path $taskReports 'backend-storage.json') -Encoding utf8
        if ($LASTEXITCODE -ne 0) { throw 'Backend storage observation failed.' }
    }
    $taskReceipt.complete=$true
} finally {
    docker @taskCompose logs --no-color --tail 2000 2>&1 | Out-File (Join-Path $taskReports 'service-logs.txt') -Encoding utf8
    docker @taskCompose ps -a --format json | Out-File (Join-Path $taskReports 'container-states-before-cleanup.json') -Encoding utf8
    docker @taskCompose down --timeout 5
    $taskReceipt.cleanup_exit=$LASTEXITCODE
    $taskReceipt.ended_utc=(Get-Date).ToUniversalTime().ToString('o')
    $taskReceipt | ConvertTo-Json -Depth 4 | Out-File (Join-Path $taskRoot 'receipt.json') -Encoding utf8
    foreach ($taskKey in $taskOld.Keys) { [Environment]::SetEnvironmentVariable($taskKey,$taskOld[$taskKey]) }
}