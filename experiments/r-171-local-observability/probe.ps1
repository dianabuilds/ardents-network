param([Parameter(Mandatory=$true)][string]$EvidenceRoot,
      [Parameter(Mandatory=$true)][ValidatePattern('^r171-[a-z0-9-]{1,32}$')][string]$RunName, [switch]$RestartProbe, [switch]$ResourceProbe, [switch]$BackendProbe, [switch]$StorageProbe, [ValidateSet('alloy','otel')][string]$Collector='alloy', [switch]$MinimalCollector, [string]$PatchedPluginRoot)
$ErrorActionPreference='Stop'
if ($MinimalCollector -and $Collector -ne 'otel') { throw 'Minimal variant requires explicit OTel profile.' }
if (($ResourceProbe -or $BackendProbe -or $StorageProbe) -and -not $RestartProbe) { throw 'Resource profile requires explicit restart-state profile.' }
if ($Collector -eq 'otel' -and $StorageProbe) { throw 'OTel retry/loss counter semantics require separate validation before backend/pressure profile.' }
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
function Get-ProbeSourceSnapshot {
    $taskSelected=@('images.json','compose.yaml','compose.restart.yaml','fixture.py','prometheus.yml','alerts.yml','alertmanager.yml','loki.yml','config.alloy','prepare-private.py','query-probe.py','resource-window.ps1','storage-pressure.py','probe.ps1','otel.yml','compose.otel.yaml','prometheus.otel.yml','loki.otel.yml','install-collector-image.ps1','alerts.otel.yml','builder.yml','build-minimal.py','install-minimal.ps1','compose.plugins.yaml','stage-plugins.py','install-plugin-trees.ps1')
    foreach ($taskFile in $taskSelected) {
        $taskBytes=[IO.File]::ReadAllBytes((Join-Path $PSScriptRoot $taskFile))
        $taskHasher=[Security.Cryptography.SHA256]::Create()
        try { $taskDigest=[BitConverter]::ToString($taskHasher.ComputeHash($taskBytes)).Replace('-','').ToLowerInvariant() }
        finally { $taskHasher.Dispose() }
        [pscustomobject]@{name=$taskFile;bytes=$taskBytes.Length;sha256=$taskDigest}
    }
}
$taskSourceBefore=@(Get-ProbeSourceSnapshot)
$taskLock=Get-Content (Join-Path $PSScriptRoot 'images.json') -Raw | ConvertFrom-Json
$taskEnv=[ordered]@{R171_SOURCE=$PSScriptRoot;R171_PRIVATE=$taskPrivate}
function Get-PluginTreeSnapshot {
    if (-not $PatchedPluginRoot) { return }
    $taskPluginPath=[IO.Path]::GetFullPath($PatchedPluginRoot)
    $taskAncestor=[IO.DirectoryInfo]::new($taskPluginPath)
    while ($null -ne $taskAncestor) {
        if ($taskAncestor.Exists -and ($taskAncestor.Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw 'Refuse redirected plugin root' }
        if (Test-Path -LiteralPath (Join-Path $taskAncestor.FullName '.git')) { throw 'Plugin trees must stay outside Git' }
        $taskAncestor=$taskAncestor.Parent
    }
    $taskStage=Get-Content (Join-Path $taskPluginPath 'stage-receipt.json') -Raw | ConvertFrom-Json
    if (-not $taskStage.complete -or $taskStage.plugins.Count -ne 2) { throw 'Explicit complete plugin staging required' }
    foreach ($taskPlugin in $taskStage.plugins) {
        if (($taskPlugin.id -eq 'prometheus' -and $taskPlugin.version -eq '13.2.3') -or ($taskPlugin.id -eq 'loki' -and $taskPlugin.version -eq '13.2.1')) {} else { throw 'Unexpected staged plugin identity' }
        $taskBase=Join-Path (Join-Path $taskPluginPath 'plugins') $taskPlugin.id
        $taskFiles=@(Get-ChildItem -LiteralPath $taskBase -File -Recurse)
        if ($taskFiles.Count -gt 4096 -or $taskFiles.Count -ne @($taskPlugin.files.PSObject.Properties).Count) { throw 'Staged inventory changed' }
        foreach ($taskEntry in $taskFiles) {
            if ($taskEntry.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Refuse redirected plugin file' }
            $taskRelative=$taskEntry.FullName.Substring($taskBase.Length+1).Replace('\','/')
            $taskDigest=(Get-FileHash -LiteralPath $taskEntry.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
            if ($taskDigest -ne $taskPlugin.files.$taskRelative) { throw 'Staged plugin hash changed' }
            [pscustomobject]@{plugin=$taskPlugin.id;name=$taskRelative;sha256=$taskDigest;bytes=$taskEntry.Length}
        }
    }
}
$taskPluginBefore=@(Get-PluginTreeSnapshot)
if ($PatchedPluginRoot) { $taskEnv.R171_PATCHED_PLUGINS=[IO.Path]::GetFullPath($PatchedPluginRoot) }
$taskHelper=docker image inspect ardents-diagnostics:prebuilt-parsers-final-389 --format '{{.Id}}'
if ($LASTEXITCODE -ne 0) { throw 'Explicitly installed diagnostic helper missing.' }
$taskEnv.R171_HELPER_IMAGE=$taskHelper
$taskImageIdentities=@()
foreach ($taskImage in $taskLock.images) {
    if ($Collector -eq 'otel' -and $taskImage.name -eq 'alloy') { continue }
    $taskInstalled=docker image inspect $taskImage.image | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0) { throw 'Pinned image missing; run explicit make tools-install.' }
    $taskImageIdentities += [pscustomobject]@{role=$taskImage.name;manifest=$taskImage.image;config_id=$taskInstalled[0].Id;os=$taskInstalled[0].Os;architecture=$taskInstalled[0].Architecture}
    $taskEnv[('R171_'+$taskImage.name.ToUpper()+'_IMAGE')]=$taskImage.image
}
if ($Collector -eq 'otel') {
    $taskOtelSelector='ardents-r171-otel:0.162.0'
    if ($MinimalCollector) { $taskOtelSelector='sha256:7c345035ed2941c4df2f0e95dedcf3209297ca5ef2df8b297694584c4a0a6fc1' }
    $taskOtel=docker image inspect $taskOtelSelector | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0 -or $taskOtel[0].Architecture -ne 'amd64' -or $taskOtel[0].Os -ne 'linux') { throw 'Explicitly installed OTel image required.' }
    $taskEnv.R171_OTEL_IMAGE=$taskOtel[0].Id
    # Required interpolation in the base compose is replaced by the OTel override.
    $taskEnv.R171_ALLOY_IMAGE=$taskOtel[0].Id
    $taskImageIdentities += [pscustomobject]@{role='otel';manifest='local scratch image';config_id=$taskOtel[0].Id;os=$taskOtel[0].Os;architecture=$taskOtel[0].Architecture}
}
$taskOld=@{}; foreach ($taskKey in $taskEnv.Keys) { $taskOld[$taskKey]=[Environment]::GetEnvironmentVariable($taskKey); [Environment]::SetEnvironmentVariable($taskKey,$taskEnv[$taskKey]) }
$taskCompose=@('compose','-p',$RunName,'-f',(Join-Path $PSScriptRoot 'compose.yaml'))
if ($RestartProbe) { $taskCompose += @('-f',(Join-Path $PSScriptRoot 'compose.restart.yaml')) }
if ($Collector -eq 'otel') { $taskCompose += @('-f',(Join-Path $PSScriptRoot 'compose.otel.yaml')) }
if ($PatchedPluginRoot) { $taskCompose += @('-f',(Join-Path $PSScriptRoot 'compose.plugins.yaml')) }
$taskReceipt=[ordered]@{patched_plugins=[bool]$PatchedPluginRoot;plugin_inputs=$taskPluginBefore;collector=$Collector;minimal_distribution=[bool]$MinimalCollector;started_utc=(Get-Date).ToUniversalTime().ToString('o');run=$RunName;source_scope='synthetic fixture only';complete=$false;helper_image_id=$taskHelper;images=$taskImageIdentities;source_inputs=$taskSourceBefore}
function Invoke-ProbeQuery([string]$Mode,[string]$ReportName) {
    $taskDestination=Join-Path $taskReports $ReportName
    New-Item -ItemType Directory -Path $taskDestination | Out-Null
    docker run --rm --network ($RunName+'_probe') --read-only --cap-drop ALL --security-opt no-new-privileges --user 10001:10001 --memory 256m --cpus 0.5 --pids-limit 16 --shm-size 1m --log-driver local --log-opt max-size=2m --log-opt max-file=2 --mount "type=bind,source=$PSScriptRoot,target=/probe,readonly" --mount "type=bind,source=$taskPrivate/query,target=/certs,readonly" --mount "type=bind,source=$taskDestination,target=/reports" --mount "type=bind,source=$taskReports,target=/history,readonly" --entrypoint python3 $taskHelper /probe/query-probe.py $Mode $ReportName $Collector
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
    if ($Collector -eq 'otel') { Invoke-ProbeQuery 'otel-delivery' 'otel-delivery' }
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
    if ($PatchedPluginRoot) { Invoke-ProbeQuery 'plugin-identities' 'plugin-identities' }
    if ($StorageProbe) {
        $taskPressureReports=Join-Path $taskReports 'storage-injection'
        New-Item -ItemType Directory -Path $taskPressureReports | Out-Null
        Invoke-ProbeQuery 'storage-baseline' 'storage-baseline'
        foreach ($taskAction in @('fill','free')) {
            if ($taskAction -eq 'free') { Invoke-ProbeQuery 'storage-pressure' 'storage-pressure' }
            docker run --rm --network none --read-only --cap-drop ALL --security-opt no-new-privileges --user 10001:10001 --memory 640m --cpus 0.25 --pids-limit 4 --shm-size 1m --log-driver local --log-opt max-size=2m --log-opt max-file=2 --mount "type=volume,source=$($RunName)_loki-state,target=/state" --mount "type=bind,source=$PSScriptRoot,target=/probe,readonly" --mount "type=bind,source=$taskPressureReports,target=/reports" --entrypoint python3 $taskHelper /probe/storage-pressure.py $taskAction
            if ($LASTEXITCODE -ne 0) { throw 'Synthetic storage injection action failed; preserve evidence.' }
        }
        Invoke-ProbeQuery 'catchup' 'post-storage-pressure-logs'
        Invoke-ProbeQuery 'storage-recovered' 'storage-recovered'
    }
    if ($BackendProbe) {
        $taskBackendPrefix=if ($Collector -eq 'otel') { 'otel-' } else { '' }
        Invoke-ProbeQuery ($taskBackendPrefix+'backend-baseline') 'backend-baseline'
        docker @taskCompose stop --timeout 5 loki
        if ($LASTEXITCODE -ne 0) { throw 'Synthetic log backend stop failed.' }
        Invoke-ProbeQuery ($taskBackendPrefix+'backend-unavailable') 'backend-unavailable'
        docker @taskCompose start loki
        if ($LASTEXITCODE -ne 0) { throw 'Synthetic log backend start failed.' }
        Invoke-ProbeQuery 'ready' 'backend-recovery-readiness'
        Invoke-ProbeQuery 'catchup' 'post-backend-outage-logs'
        if ($Collector -eq 'otel') { Invoke-ProbeQuery 'otel-backend-recovered' 'backend-recovered' }
    }
    if ($RestartProbe) {
        Invoke-ProbeQuery 'restart-before' 'restart-before'
        # Drain the sender while its receiver is available, before receiver shutdown.
        docker @taskCompose stop --timeout 30 alloy
        if ($LASTEXITCODE -ne 0) { throw 'Collector drain/stop failed.' }
        $taskCollectorId=docker @taskCompose ps -a -q alloy
        $taskStopped=docker inspect $taskCollectorId | ConvertFrom-Json
        $taskStopped[0].State | ConvertTo-Json -Depth 5 | Out-File (Join-Path $taskReports 'collector-stopped-before-backends.json') -Encoding utf8
        if ($LASTEXITCODE -ne 0 -or $taskStopped[0].State.Running -or $taskStopped[0].State.OOMKilled -or $taskStopped[0].State.ExitCode -ne 0) { throw 'Collector did not stop cleanly within its finite drain budget.' }
        docker @taskCompose restart --timeout 30 alertmanager loki prometheus grafana
        if ($LASTEXITCODE -ne 0) { throw 'Synthetic backend restart failed.' }
        Invoke-ProbeQuery 'ready' 'restart-readiness'
        docker @taskCompose start alloy
        if ($LASTEXITCODE -ne 0) { throw 'Collector start after backend readiness failed.' }
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
    $taskSourceAfter=@(Get-ProbeSourceSnapshot)
    $taskReceipt.source_inputs_stable=($taskSourceBefore | ConvertTo-Json -Compress) -eq ($taskSourceAfter | ConvertTo-Json -Compress)
    if (-not $taskReceipt.source_inputs_stable) { throw 'Probe input changed; retain original observations without acceptance.' }
    $taskPluginAfter=@(Get-PluginTreeSnapshot)
    $taskReceipt.plugin_inputs_stable=($taskPluginBefore | ConvertTo-Json -Compress) -eq ($taskPluginAfter | ConvertTo-Json -Compress)
    if (-not $taskReceipt.plugin_inputs_stable) { throw 'Plugin tree changed during run' }
    $taskReceipt.complete=$true
} finally {
    $taskSourceAfter=@(Get-ProbeSourceSnapshot)
    $taskReceipt.source_inputs_stable=($taskSourceBefore | ConvertTo-Json -Compress) -eq ($taskSourceAfter | ConvertTo-Json -Compress)
    docker @taskCompose logs --no-color --tail 2000 2>&1 | Out-File (Join-Path $taskReports 'service-logs.txt') -Encoding utf8
    docker @taskCompose ps -a --format json | Out-File (Join-Path $taskReports 'container-states-before-cleanup.json') -Encoding utf8
    docker @taskCompose down --timeout 5
    $taskReceipt.cleanup_exit=$LASTEXITCODE
    $taskReceipt.ended_utc=(Get-Date).ToUniversalTime().ToString('o')
    $taskReceipt | ConvertTo-Json -Depth 8 | Out-File (Join-Path $taskRoot 'receipt.json') -Encoding utf8
    foreach ($taskKey in $taskOld.Keys) { [Environment]::SetEnvironmentVariable($taskKey,$taskOld[$taskKey]) }
}
