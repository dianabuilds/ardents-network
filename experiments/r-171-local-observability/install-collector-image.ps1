param([Parameter(Mandatory=$true)][string]$ArtifactRoot,
      [Parameter(Mandatory=$true)][string]$EvidenceRoot)
$ErrorActionPreference='Stop'
function Get-PinnedHash([string]$Path) {
    $taskHasher=[Security.Cryptography.SHA256]::Create()
    $taskFile=[IO.File]::OpenRead($Path)
    try { return [BitConverter]::ToString($taskHasher.ComputeHash($taskFile)).Replace('-','').ToLowerInvariant() }
    finally { $taskFile.Dispose(); $taskHasher.Dispose() }
}
$taskArtifact=[IO.Path]::GetFullPath($ArtifactRoot)
$taskRoot=[IO.Path]::GetFullPath($EvidenceRoot)
foreach ($taskPath in @($taskArtifact,$taskRoot)) {
    $taskAncestor=[IO.DirectoryInfo]::new($taskPath)
    while ($null -ne $taskAncestor) {
        if ($taskAncestor.Exists -and ($taskAncestor.Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw 'Refuse redirected installation roots.' }
        if (Test-Path -LiteralPath (Join-Path $taskAncestor.FullName '.git')) { throw 'Installation inputs/context/evidence must be outside Git.' }
        $taskAncestor=$taskAncestor.Parent
    }
}
if (Test-Path -LiteralPath $taskRoot) { throw 'Refuse installation evidence reuse.' }
$taskBinary=Join-Path $taskArtifact 'otelcol-contrib'
$taskInfo=Get-Item -LiteralPath $taskBinary
if ($taskInfo.Length -ne 407498914 -or ($taskInfo.Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw 'Unexpected binary input.' }
$taskExpected='2425bdf5f89042cd71f56cf5a66b41681d340cbe14c0b1899bcfe2a3a685a064'
if ((Get-PinnedHash $taskBinary) -ne $taskExpected) { throw 'Pinned binary identity mismatch.' }
New-Item -ItemType Directory -Path $taskRoot | Out-Null
$taskContext=Join-Path $taskRoot 'context'
New-Item -ItemType Directory -Path $taskContext | Out-Null
Copy-Item -LiteralPath $taskBinary -Destination (Join-Path $taskContext 'otelcol-contrib')
if ((Get-PinnedHash (Join-Path $taskContext 'otelcol-contrib')) -ne $taskExpected) { throw 'Copied binary identity mismatch.' }
$taskDockerfile=@('FROM scratch','COPY --chown=10001:10001 --chmod=0555 otelcol-contrib /otelcol-contrib','USER 10001:10001','ENTRYPOINT ["/otelcol-contrib"]') -join "`n"
[IO.File]::WriteAllText((Join-Path $taskContext 'Dockerfile'),$taskDockerfile,[Text.UTF8Encoding]::new($false))
[IO.File]::WriteAllText((Join-Path $taskContext '.dockerignore'),"*`n!otelcol-contrib`n!Dockerfile`n",[Text.UTF8Encoding]::new($false))
$taskReceipt=[ordered]@{started_utc=(Get-Date).ToUniversalTime().ToString('o');scope='synthetic investigation only';binary_sha256=$taskExpected;binary_bytes=$taskInfo.Length;complete=$false}
try {
    $taskPriorPreference=$ErrorActionPreference
    try {
        # Windows PowerShell treats native progress on stderr as ErrorRecord.
        # Retain both streams and decide from the actual native exit code.
        $ErrorActionPreference='Continue'
        docker build --network none --pull=false --platform linux/amd64 --tag ardents-r171-otel:0.162.0 $taskContext *> (Join-Path $taskRoot 'build.txt')
        $taskBuildExit=$LASTEXITCODE
    } finally { $ErrorActionPreference=$taskPriorPreference }
    $taskReceipt.build_exit=$taskBuildExit
    if ($taskBuildExit -ne 0) { throw 'Collector image build failed; retain original build evidence.' }
    $taskImage=docker image inspect ardents-r171-otel:0.162.0 | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0 -or $taskImage[0].Architecture -ne 'amd64' -or $taskImage[0].Os -ne 'linux') { throw 'Unexpected built target.' }
    $taskReceipt.image_config_id=$taskImage[0].Id
    $taskReceipt.complete=$true
} finally {
    $taskReceipt.ended_utc=(Get-Date).ToUniversalTime().ToString('o')
    $taskReceipt | ConvertTo-Json -Depth 5 | Out-File (Join-Path $taskRoot 'receipt.json') -Encoding utf8
}
Write-Output $taskReceipt.image_config_id
