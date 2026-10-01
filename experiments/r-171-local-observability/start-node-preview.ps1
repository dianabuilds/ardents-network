param([Parameter(Mandatory=$true)][string]$EvidenceRoot,
      [Parameter(Mandatory=$true)][string]$BinaryRoot,
      [Parameter(Mandatory=$true)][string]$PluginRoot,
      [Parameter(Mandatory=$true)][string]$RelayBinary,
      [ValidatePattern('^r171-[a-z0-9-]{1,32}$')][string]$RunName='r171-node-preview-a')
$ErrorActionPreference='Stop'
$taskRoot=[IO.Path]::GetFullPath($EvidenceRoot)
$taskRepo=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../..'))
if(Test-Path -LiteralPath $taskRoot){throw 'Refuse existing preview root'}
if($taskRoot.StartsWith($taskRepo,[StringComparison]::OrdinalIgnoreCase)){throw 'Preview must stay outside source'}
$taskAncestor=[IO.DirectoryInfo]::new($taskRoot)
while($null -ne $taskAncestor){
    if($taskAncestor.Exists -and ($taskAncestor.Attributes -band [IO.FileAttributes]::ReparsePoint)){throw 'Refuse redirected preview root'}
    $taskAncestor=$taskAncestor.Parent
}
[IO.Directory]::CreateDirectory($taskRoot) | Out-Null
$taskAcl=[Security.AccessControl.DirectorySecurity]::new()
$taskAcl.SetAccessRuleProtection($true,$false)
$taskSid=[Security.Principal.WindowsIdentity]::GetCurrent().User
$taskAcl.SetOwner($taskSid)
foreach($taskIdentity in @($taskSid,[Security.Principal.SecurityIdentifier]::new('S-1-5-18'))){
    $taskAcl.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new($taskIdentity,'FullControl','ContainerInherit,ObjectInherit','None','Allow'))
}
[IO.Directory]::SetAccessControl($taskRoot,$taskAcl)
$taskPrivate=Join-Path $taskRoot 'private'
$taskReports=Join-Path $taskRoot 'reports'
[IO.Directory]::CreateDirectory($taskPrivate) | Out-Null
[IO.Directory]::CreateDirectory($taskReports) | Out-Null
function Get-PreviewHash([string]$Path){
    $taskHash=[Security.Cryptography.SHA256]::Create()
    $taskStream=[IO.File]::OpenRead($Path)
    try{return [BitConverter]::ToString($taskHash.ComputeHash($taskStream)).Replace('-','').ToLowerInvariant()}
    finally{$taskStream.Dispose();$taskHash.Dispose()}
}
$taskBuild=Get-Content (Join-Path $BinaryRoot 'build-receipt.json') -Raw | ConvertFrom-Json
$taskExpected=@('ardents-diagnostics','ardents-node','ardents','ardents-control','qualification-network')
if(-not $taskBuild.complete -or $taskBuild.artifacts.Count -ne $taskExpected.Count){throw 'Complete actual Node fixture build required'}
$taskSeen=@{}
foreach($taskArtifact in $taskBuild.artifacts){
    if($taskArtifact.name -cnotin $taskExpected -or $taskSeen.ContainsKey($taskArtifact.name)){throw 'Exact Node fixture artifact set required'}
    $taskSeen[$taskArtifact.name]=$true
    $taskFile=Get-Item -LiteralPath (Join-Path $BinaryRoot $taskArtifact.name)
    if($taskFile.PSIsContainer -or ($taskFile.Attributes -band [IO.FileAttributes]::ReparsePoint) -or $taskFile.Length -le 0 -or $taskFile.Length -gt 64MB){throw 'Selected binary outside file bounds'}
    if($taskFile.Length -ne $taskArtifact.bytes -or $taskArtifact.sha256 -cnotmatch '^[0-9a-f]{64}$' -or (Get-PreviewHash $taskFile.FullName) -ne $taskArtifact.sha256){throw 'Selected binary changed'}
}
$taskPlugins=Get-Content (Join-Path $PluginRoot 'stage-receipt.json') -Raw | ConvertFrom-Json
if(-not $taskPlugins.complete -or $taskPlugins.plugins.Count -ne 2 -or @($taskPlugins.plugins.id | Sort-Object -Unique).Count -ne 2){throw 'Checked exact distinct plugin pair required'}
$taskSeenPlugins=@{}
foreach($taskPlugin in $taskPlugins.plugins){
    if($taskPlugin.id -cnotin @('prometheus','loki') -or $taskSeenPlugins.ContainsKey($taskPlugin.id)){throw 'Exact distinct plugin pair required'}
    $taskSeenPlugins[$taskPlugin.id]=$true
    $taskBase=Join-Path (Join-Path $PluginRoot 'plugins') $taskPlugin.id
    if(@(Get-ChildItem -LiteralPath $taskBase -Force -Recurse | Where-Object {$_.Attributes -band [IO.FileAttributes]::ReparsePoint}).Count){throw 'Redirected plugin tree'}
    $taskActual=@(Get-ChildItem -LiteralPath $taskBase -File -Force -Recurse)
    if($taskActual.Count -ne @($taskPlugin.files.psobject.Properties).Count){throw 'Plugin inventory changed'}
    foreach($taskEntry in $taskActual){
        if($taskEntry.Attributes -band [IO.FileAttributes]::ReparsePoint){throw 'Redirected plugin file'}
        $taskRelative=$taskEntry.FullName.Substring($taskBase.Length+1).Replace('\','/')
        if((Get-PreviewHash $taskEntry.FullName) -ne $taskPlugin.files.$taskRelative){throw 'Plugin bytes changed'}
    }
}
$taskHelper='sha256:0ecc73f220e154f40bdf3db718f9ff66890d6b381adaaf05fc3cbe989441c3ad'
$taskOtel='sha256:7c345035ed2941c4df2f0e95dedcf3209297ca5ef2df8b297694584c4a0a6fc1'
$taskEnv=[ordered]@{R171_SOURCE=$PSScriptRoot;R171_REPO=$taskRepo;R171_PRIVATE=$taskPrivate;
    R171_HELPER_IMAGE=$taskHelper;R171_OTEL_IMAGE=$taskOtel;R171_ALLOY_IMAGE=$taskOtel;
    R171_NODE_BINARIES=[IO.Path]::GetFullPath($BinaryRoot);R171_PATCHED_PLUGINS=[IO.Path]::GetFullPath($PluginRoot)}
$taskImages=@($taskHelper,$taskOtel)
$taskLock=Get-Content (Join-Path $PSScriptRoot 'images.json') -Raw | ConvertFrom-Json
foreach($taskImage in $taskLock.images){
    if($taskImage.name -eq 'alloy'){continue}
    $taskEnv[('R171_'+$taskImage.name.ToUpper()+'_IMAGE')]=$taskImage.image
    $taskImages+=$taskImage.image
}
$taskImageReceipts=@()
foreach($taskImage in $taskImages){
    $taskInfo=docker image inspect $taskImage | ConvertFrom-Json
    if($LASTEXITCODE -ne 0 -or $taskInfo[0].Os -ne 'linux' -or $taskInfo[0].Architecture -ne 'amd64'){throw 'Installed exact candidate image missing'}
    $taskImageReceipts += @{selected=$taskImage;config=$taskInfo[0].Id}
}
$taskEnvPath=Join-Path $taskRoot 'compose.env'
$taskLines=foreach($taskKey in $taskEnv.Keys){$taskKey+'='+$taskEnv[$taskKey].Replace('\','/')}
[IO.File]::WriteAllLines($taskEnvPath,$taskLines,[Text.UTF8Encoding]::new($false))
$taskCompose=@('compose','--env-file',$taskEnvPath,'-p',$RunName)
foreach($taskFile in @('compose.yaml','compose.otel.yaml','compose.plugins.yaml','compose.browser.yaml','compose.node.yaml')){
    $taskCompose+=@('-f',(Join-Path $PSScriptRoot $taskFile))
}
$taskReceipt=[ordered]@{complete=$false;research_preview=$true;backend_admission=$false;
    run=$RunName;started_utc=(Get-Date).ToUniversalTime().ToString('o');images=$taskImageReceipts;
    artifacts=$taskBuild.artifacts;relay_sha256=(Get-PreviewHash $RelayBinary);url='http://127.0.0.1:8098/d/accepted-node';
    source_scope='one actual Node and two Sources in one shared local container';source_preview_seconds=3600}
try{
    docker run --rm --network none --read-only --user 10001:10001 --cap-drop ALL --security-opt no-new-privileges --memory 128m --cpus 0.25 --pids-limit 16 --mount "type=bind,source=$PSScriptRoot,target=/probe,readonly" --mount "type=bind,source=$taskPrivate,target=/private" -e R171_COLLECTOR=otel -e R171_SOURCE_MODE=accepted-node --entrypoint python3 $taskHelper /probe/prepare-private.py
    if($LASTEXITCODE -ne 0){throw 'Private preparation failed'}
    docker @taskCompose config --quiet
    if($LASTEXITCODE -ne 0){throw 'Invalid Compose configuration'}
    docker @taskCompose create --pull never fixture
    if($LASTEXITCODE -ne 0){throw 'Source creation failed'}
    docker @taskCompose up -d --pull never browser-tunnel
    if($LASTEXITCODE -ne 0){throw 'Private RAM volume anchor failed'}
    docker run --rm --network none --read-only --user 10001:10001 --cap-drop ALL --security-opt no-new-privileges --memory 64m --cpus 0.1 --pids-limit 16 --mount "type=bind,source=$taskPrivate/node,target=/input,readonly" --mount "type=volume,source=$($RunName)_node-certs,target=/output" --entrypoint python3 $taskHelper -c "import pathlib,shutil,os; os.umask(0o077); root=pathlib.Path('/output'); root.chmod(0o700); [(shutil.copyfile('/input/'+n,root/n),(root/n).chmod(0o600)) for n in ('server.crt','server.key','client-ca.crt','client-pin.txt')]"
    if($LASTEXITCODE -ne 0){throw 'Private Linux certificate staging failed'}
    docker @taskCompose up -d --pull never alertmanager loki prometheus alloy grafana fixture browser-tunnel
    if($LASTEXITCODE -ne 0){throw 'Preview startup failed'}
    $taskRelay=Start-Process -FilePath $RelayBinary -ArgumentList @('-container',($RunName+'-browser-tunnel-1'),'-duration','3600s') -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $taskReports 'relay.stdout') -RedirectStandardError (Join-Path $taskReports 'relay.stderr')
    $taskReceipt.relay_pid=$taskRelay.Id
    Start-Sleep -Seconds 1
    if($taskRelay.HasExited){throw 'Browser relay startup failed'}
    $taskReceipt.complete=$true
}finally{
    $taskReceipt | ConvertTo-Json -Depth 6 | Out-File (Join-Path $taskRoot 'launch-receipt.json') -Encoding utf8
}
Write-Output 'Started real-source preview; live ingestion and browser verification still required.'
