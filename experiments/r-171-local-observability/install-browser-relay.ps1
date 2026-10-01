param([Parameter(Mandatory=$true)][string]$EvidenceRoot)
$ErrorActionPreference='Stop'
$taskRoot=[IO.Path]::GetFullPath($EvidenceRoot)
$taskAncestor=[IO.DirectoryInfo]::new($taskRoot)
while($null -ne $taskAncestor){
    if($taskAncestor.Exists -and ($taskAncestor.Attributes -band [IO.FileAttributes]::ReparsePoint)){throw 'Refuse redirected binary output'}
    if(Test-Path -LiteralPath (Join-Path $taskAncestor.FullName '.git')){throw 'Probe binary must stay outside Git'}
    $taskAncestor=$taskAncestor.Parent
}
if(Test-Path -LiteralPath $taskRoot){throw 'Refuse evidence reuse'}
New-Item -ItemType Directory -Path $taskRoot | Out-Null
go version | Out-File (Join-Path $taskRoot 'toolchain.txt') -Encoding utf8
go build -o (Join-Path $taskRoot 'browser-relay.exe') (Join-Path $PSScriptRoot 'browser-relay.go')
if($LASTEXITCODE -ne 0){throw 'Explicit browser probe tool build failed'}
$taskBinary=Join-Path $taskRoot 'browser-relay.exe'
$taskHasher=[Security.Cryptography.SHA256]::Create()
$taskStream=[IO.File]::OpenRead($taskBinary)
try { $taskHash=[BitConverter]::ToString($taskHasher.ComputeHash($taskStream)).Replace('-','').ToLowerInvariant() }
finally { $taskStream.Dispose(); $taskHasher.Dispose() }
@{sha256=$taskHash;path=$taskBinary;bytes=(Get-Item -LiteralPath $taskBinary).Length} | ConvertTo-Json | Out-File (Join-Path $taskRoot 'binary.json') -Encoding utf8

