param([Parameter(Mandatory=$true)][string]$ArtifactRoot,[Parameter(Mandatory=$true)][string]$EvidenceRoot)
$ErrorActionPreference='Stop'
$taskArtifact=[IO.Path]::GetFullPath($ArtifactRoot)
$taskRoot=[IO.Path]::GetFullPath($EvidenceRoot)
foreach($taskPath in @($taskArtifact,$taskRoot)) {
    $taskAncestor=[IO.DirectoryInfo]::new($taskPath)
    while($null -ne $taskAncestor) {
        if($taskAncestor.Exists -and ($taskAncestor.Attributes -band [IO.FileAttributes]::ReparsePoint)){throw 'Refuse redirected build root'}
        if(Test-Path -LiteralPath (Join-Path $taskAncestor.FullName '.git')){throw 'Generated build inputs/outputs must stay outside Git'}
        $taskAncestor=$taskAncestor.Parent
    }
}
if(Test-Path -LiteralPath $taskRoot){throw 'Refuse build evidence reuse'}
if(-not(Test-Path -LiteralPath (Join-Path $taskArtifact 'ocb_0.162.0_linux_amd64') -PathType Leaf)){throw 'Explicitly installed Builder artifact required'}
New-Item -ItemType Directory -Path $taskRoot | Out-Null
$taskHelper='sha256:0ecc73f220e154f40bdf3db718f9ff66890d6b381adaaf05fc3cbe989441c3ad'
docker image inspect $taskHelper --format '{{.Id}}' | Out-File (Join-Path $taskRoot 'helper-image.txt') -Encoding utf8
if($LASTEXITCODE -ne 0){throw 'Explicitly installed exact build helper required'}
docker run --rm --read-only --cap-drop ALL --security-opt no-new-privileges --user 10001:10001 --memory 3g --cpus 2 --pids-limit 128 --tmpfs /tmp:rw,nosuid,nodev,exec,size=67108864,uid=10001,gid=10001,mode=0700 --tmpfs /cache:rw,nosuid,nodev,noexec,size=2147483648,uid=10001,gid=10001,mode=0700 -e HOME=/tmp -e GOPATH=/cache/modules -e GOCACHE=/cache/build -e GOTMPDIR=/cache/work -e GOMAXPROCS=2 -e GOTOOLCHAIN=local -e CGO_ENABLED=0 -e GOFLAGS=-p=2 -e GOPROXY=https://proxy.golang.org -e GOSUMDB=sum.golang.org --mount "type=bind,source=$taskArtifact,target=/assets,readonly" --mount "type=bind,source=$taskRoot,target=/build" --mount "type=bind,source=$PSScriptRoot,target=/probe,readonly" --entrypoint python3 $taskHelper /probe/build-minimal.py
if($LASTEXITCODE -ne 0){throw 'Bounded official-component build failed; original evidence retained'}
