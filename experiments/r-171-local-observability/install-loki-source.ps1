param([Parameter(Mandatory=$true)][string]$PrimaryRoot,[Parameter(Mandatory=$true)][string]$EvidenceRoot,[Parameter(Mandatory=$true)][string]$PluginRoot)
$ErrorActionPreference='Stop'
$taskPrimary=[IO.Path]::GetFullPath($PrimaryRoot); $taskRoot=[IO.Path]::GetFullPath($EvidenceRoot); $taskPlugin=[IO.Path]::GetFullPath($PluginRoot)
foreach($taskPath in @($taskPrimary,$taskRoot,$taskPlugin)) {
    $taskAncestor=[IO.DirectoryInfo]::new($taskPath)
    while($null -ne $taskAncestor) {
        if($taskAncestor.Exists -and ($taskAncestor.Attributes -band [IO.FileAttributes]::ReparsePoint)){throw 'Refuse redirected evidence root'}
        if(Test-Path -LiteralPath (Join-Path $taskAncestor.FullName '.git')){throw 'Source/cache/evidence must stay outside Git'}
        $taskAncestor=$taskAncestor.Parent
    }
}
if(Test-Path -LiteralPath $taskRoot){throw 'Refuse evidence reuse'}
New-Item -ItemType Directory -Path $taskRoot | Out-Null
$taskHelper='sha256:0ecc73f220e154f40bdf3db718f9ff66890d6b381adaaf05fc3cbe989441c3ad'
docker image inspect $taskHelper --format '{{.Id}}' | Out-File (Join-Path $taskRoot 'helper-image.txt') -Encoding utf8
if($LASTEXITCODE -ne 0){throw 'Explicitly installed exact helper required'}
docker run --rm --read-only --cap-drop ALL --security-opt no-new-privileges --user 10001:10001 --memory 2g --cpus 1 --pids-limit 64 --tmpfs /tmp:rw,nosuid,nodev,noexec,size=67108864,uid=10001,gid=10001,mode=0700 --tmpfs /cache:rw,nosuid,nodev,noexec,size=1073741824,uid=10001,gid=10001,mode=0700 -e HOME=/tmp -e GOPATH=/cache/modules -e GOCACHE=/cache/build -e GOTMPDIR=/cache/work -e GOMAXPROCS=2 -e GOTOOLCHAIN=local -e CGO_ENABLED=0 -e GOOS=linux -e GOARCH=amd64 -e GOFLAGS=-p=2 -e GOPROXY=https://proxy.golang.org -e GOSUMDB=sum.golang.org --mount "type=bind,source=$taskPrimary,target=/primary,readonly" --mount "type=bind,source=$taskPlugin,target=/artifact,readonly" --mount "type=bind,source=$taskRoot,target=/evidence" --mount "type=bind,source=$PSScriptRoot,target=/probe,readonly" --entrypoint python3 $taskHelper /probe/inspect-loki-source.py
if($LASTEXITCODE -ne 0){throw 'Exact source inspection failed; preserve original evidence'}
