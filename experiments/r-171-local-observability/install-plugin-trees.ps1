param([Parameter(Mandatory=$true)][string]$ArtifactRoot,[Parameter(Mandatory=$true)][string]$EvidenceRoot)
$ErrorActionPreference='Stop'
$taskAssets=[IO.Path]::GetFullPath($ArtifactRoot); $taskRoot=[IO.Path]::GetFullPath($EvidenceRoot)
foreach($taskPath in @($taskAssets,$taskRoot)){
    $taskAncestor=[IO.DirectoryInfo]::new($taskPath)
    while($null -ne $taskAncestor){
        if($taskAncestor.Exists -and ($taskAncestor.Attributes -band [IO.FileAttributes]::ReparsePoint)){throw 'Refuse redirected artifact/evidence path'}
        if(Test-Path -LiteralPath (Join-Path $taskAncestor.FullName '.git')){throw 'Plugin trees must stay outside Git'}
        $taskAncestor=$taskAncestor.Parent
    }
}
if(Test-Path -LiteralPath $taskRoot){throw 'Refuse evidence reuse'}
New-Item -ItemType Directory -Path $taskRoot | Out-Null
$taskHelper='sha256:0ecc73f220e154f40bdf3db718f9ff66890d6b381adaaf05fc3cbe989441c3ad'
docker image inspect $taskHelper --format '{{.Id}}' | Out-File (Join-Path $taskRoot 'helper-image.txt') -Encoding utf8
if($LASTEXITCODE -ne 0){throw 'Explicitly installed exact helper required'}
docker run --rm --network none --read-only --cap-drop ALL --security-opt no-new-privileges --user 10001:10001 --memory 256m --cpus 0.5 --pids-limit 16 --mount "type=bind,source=$taskAssets,target=/assets,readonly" --mount "type=bind,source=$taskRoot,target=/output" --mount "type=bind,source=$PSScriptRoot,target=/probe,readonly" --entrypoint python3 $taskHelper /probe/stage-plugins.py
if($LASTEXITCODE -ne 0){throw 'Plugin staging failed; preserve original result'}
