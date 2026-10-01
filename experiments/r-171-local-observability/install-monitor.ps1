param([Parameter(Mandatory=$true)][string]$EvidenceRoot, [switch]$NodeFixture)
$ErrorActionPreference='Stop'
$taskRoot=[IO.Path]::GetFullPath($EvidenceRoot)
$taskRepo=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../..'))
$taskAncestor=[IO.DirectoryInfo]::new($taskRoot)
while($null -ne $taskAncestor){
    if($taskAncestor.Exists -and ($taskAncestor.Attributes -band [IO.FileAttributes]::ReparsePoint)){throw 'Refuse redirected build output'}
    if(Test-Path -LiteralPath (Join-Path $taskAncestor.FullName '.git')){throw 'Build output must stay outside Git'}
    $taskAncestor=$taskAncestor.Parent
}
if(Test-Path -LiteralPath $taskRoot){throw 'Refuse build evidence reuse'}
New-Item -ItemType Directory -Path $taskRoot | Out-Null
$taskSid=[Security.Principal.WindowsIdentity]::GetCurrent().User
$taskAcl=[Security.AccessControl.DirectorySecurity]::new()
$taskAcl.SetAccessRuleProtection($true,$false)
$taskAcl.SetOwner($taskSid)
foreach($taskPrincipal in @($taskSid,[Security.Principal.SecurityIdentifier]::new('S-1-5-18'))){
    $taskAcl.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new($taskPrincipal,'FullControl','ContainerInherit,ObjectInherit','None','Allow'))
}
[IO.Directory]::SetAccessControl($taskRoot,$taskAcl)
$taskHelper='sha256:0ecc73f220e154f40bdf3db718f9ff66890d6b381adaaf05fc3cbe989441c3ad'
docker image inspect $taskHelper | Out-File (Join-Path $taskRoot 'helper-image.json') -Encoding utf8
if($LASTEXITCODE -ne 0){throw 'Installed diagnostic build helper missing; no implicit download'}
function Get-BuildHash([string]$Path) {
    $taskHasher=[Security.Cryptography.SHA256]::Create()
    $taskStream=[IO.File]::OpenRead($Path)
    try { return [BitConverter]::ToString($taskHasher.ComputeHash($taskStream)).Replace('-','').ToLowerInvariant() }
    finally { $taskStream.Dispose(); $taskHasher.Dispose() }
}
function Get-BuildInputs {
    $taskNames=git -C $taskRepo ls-files --cached --others --exclude-standard -- '*.go' 'go.mod' 'go.sum'
    if($LASTEXITCODE -ne 0){throw 'Cannot identify selected source'}
    if(@($taskNames).Count -gt 20000){throw 'Selected source manifest exceeds bound'}
    foreach($taskName in $taskNames){
        $taskPath=Join-Path $taskRepo $taskName
        if((Get-Item -LiteralPath $taskPath).Attributes -band [IO.FileAttributes]::ReparsePoint){throw 'Refuse redirected source'}
        [pscustomobject]@{name=$taskName;sha256=(Get-BuildHash $taskPath)}
    }
}
$taskBefore=@(Get-BuildInputs)
$taskHead=git -C $taskRepo rev-parse HEAD
if($LASTEXITCODE -ne 0){throw 'Cannot identify build commit'}
$taskStatus=git -C $taskRepo status --porcelain
$taskReceipt=[ordered]@{complete=$false;head=$taskHead;dirty=[bool]$taskStatus;helper_image=$taskHelper;source_inputs=$taskBefore;started_utc=(Get-Date).ToUniversalTime().ToString('o');network='none';goos='linux';goarch='amd64';qualification=$false}
$taskGoFiles=@('diagnostic-command.go','diagnostic-capture.go','diagnostic-view.go','diagnostic-report.go','diagnostic-monitor.go','diagnostic-monitor-view.go','diagnostic-monitor-collector.go','diagnostic-log-retention.go','diagnostic-evidence.go')
$taskBuild='set -eu; umask 077; go version > /output/toolchain.txt; go build -trimpath -buildvcs=false -o /output/ardents-diagnostics'
foreach($taskFile in $taskGoFiles){$taskBuild+=' /src/scripts/diagnostics/'+$taskFile}
$taskBuild+='; go build -trimpath -buildvcs=false -o /output/ardents-node ./cmd/ardents-node; go version -m /output/ardents-diagnostics > /output/diagnostics-buildinfo.txt; go version -m /output/ardents-node > /output/node-buildinfo.txt'
$taskArtifactNames=@('ardents-diagnostics','ardents-node')
if($NodeFixture){
    foreach($taskCommand in @('ardents','ardents-control')){
        $taskBuild+='; go build -trimpath -buildvcs=false -o /output/'+$taskCommand+' ./cmd/'+$taskCommand
        $taskBuild+='; go version -m /output/'+$taskCommand+' > /output/'+$taskCommand+'-buildinfo.txt'
        $taskArtifactNames+=$taskCommand
    }
    $taskBuild+='; go build -trimpath -buildvcs=false -o /output/qualification-network ./tests/qualification/stream-network-two-host/fixturecommand/qualification-network'
    $taskBuild+='; go version -m /output/qualification-network > /output/qualification-network-buildinfo.txt'
    $taskArtifactNames+='qualification-network'
}
$taskName='r171-monitor-build-'+[Guid]::NewGuid().ToString('N').Substring(0,12)
$taskRun=@('run','--rm','--name',$taskName,'--network','none','--read-only','--cap-drop','ALL','--security-opt','no-new-privileges','--user','10001:10001','--memory','2g','--cpus','2','--pids-limit','64','--tmpfs','/tmp:rw,nosuid,nodev,exec,size=536870912,uid=10001,gid=10001,mode=0700','--tmpfs','/cache:rw,nosuid,nodev,noexec,size=536870912,uid=10001,gid=10001,mode=0700','-e','HOME=/tmp','-e','GOCACHE=/cache','-e','GOTOOLCHAIN=local','-e','GOFLAGS=-mod=readonly','-e','GOPROXY=off','-e','GOSUMDB=off','-e','CGO_ENABLED=0','-e','GOMAXPROCS=2','-e','GOOS=linux','-e','GOARCH=amd64','--mount',"type=bind,source=$taskRepo,target=/src,readonly",'--mount',"type=bind,source=$taskRoot,target=/output",'--workdir','/src','--entrypoint','timeout',$taskHelper,'180','sh','-c',$taskBuild)
try {
    docker @taskRun *> (Join-Path $taskRoot 'build.txt')
    $taskReceipt.exit_code=$LASTEXITCODE
    if($LASTEXITCODE -ne 0){throw 'Selected monitor/Node build failed; keep original receipt'}
    $taskAfter=@(Get-BuildInputs)
    if(($taskBefore | ConvertTo-Json -Depth 4 -Compress) -ne ($taskAfter | ConvertTo-Json -Depth 4 -Compress)){throw 'Selected source changed during build'}
    $taskReceipt.artifacts=@(foreach($taskBinary in $taskArtifactNames){
        $taskPath=Join-Path $taskRoot $taskBinary
        $taskSize=(Get-Item -LiteralPath $taskPath).Length
        if($taskSize -le 0 -or $taskSize -gt 64MB){throw 'Build artifact outside declared bound'}
        [pscustomobject]@{name=$taskBinary;bytes=$taskSize;sha256=(Get-BuildHash $taskPath)}
    })
    $taskReceipt.complete=$true
} finally {
    $taskReceipt.ended_utc=(Get-Date).ToUniversalTime().ToString('o')
    $taskReceipt | ConvertTo-Json -Depth 5 | Out-File (Join-Path $taskRoot 'build-receipt.json') -Encoding utf8
}
