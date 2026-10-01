param([Parameter(Mandatory=$true)][string]$SourceRoot,
      [Parameter(Mandatory=$true)][string]$EvidenceRoot)
$ErrorActionPreference='Stop'
$taskRepo=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../..'))
$taskSource=[IO.Path]::GetFullPath($SourceRoot)
if($taskSource.Equals([IO.Path]::GetPathRoot($taskSource),[StringComparison]::OrdinalIgnoreCase)){throw 'Refuse whole-volume artifact source'}
$taskSource=$taskSource.TrimEnd([IO.Path]::DirectorySeparatorChar)
$taskOutput=[IO.Path]::GetFullPath($EvidenceRoot)
if($taskOutput.Equals([IO.Path]::GetPathRoot($taskOutput),[StringComparison]::OrdinalIgnoreCase)){throw 'Refuse whole-volume observation output'}
$taskOutput=$taskOutput.TrimEnd([IO.Path]::DirectorySeparatorChar)
function Assert-ExternalRoot([string]$Path) {
    if($Path.Equals($taskRepo,[StringComparison]::OrdinalIgnoreCase) -or $Path.StartsWith($taskRepo+[IO.Path]::DirectorySeparatorChar,[StringComparison]::OrdinalIgnoreCase)){throw 'Artifact observations must remain outside Git'}
    $taskAncestor=[IO.DirectoryInfo]::new($Path)
    while($null -ne $taskAncestor){
        if($taskAncestor.Exists -and ($taskAncestor.Attributes -band [IO.FileAttributes]::ReparsePoint)){throw 'Refuse redirected artifact root'}
        $taskAncestor=$taskAncestor.Parent
    }
}
Assert-ExternalRoot $taskSource
Assert-ExternalRoot $taskOutput
if(-not [IO.Directory]::Exists($taskSource)){throw 'Selected artifact root unavailable'}
if(Test-Path -LiteralPath $taskOutput){throw 'Refuse existing artifact observation output'}
if($taskOutput.Equals($taskSource,[StringComparison]::OrdinalIgnoreCase) -or $taskOutput.StartsWith($taskSource+[IO.Path]::DirectorySeparatorChar,[StringComparison]::OrdinalIgnoreCase) -or $taskSource.StartsWith($taskOutput+[IO.Path]::DirectorySeparatorChar,[StringComparison]::OrdinalIgnoreCase)){throw 'Observation and selected artifact roots must be disjoint'}
$taskSid=[Security.Principal.WindowsIdentity]::GetCurrent().User
$taskSourceAcl=Get-Acl -LiteralPath $taskSource
if($taskSourceAcl.GetOwner([Security.Principal.SecurityIdentifier]).Value -ne $taskSid.Value){throw 'Refuse artifact root owned by another account'}
[IO.Directory]::CreateDirectory($taskOutput) | Out-Null
$taskAcl=[Security.AccessControl.DirectorySecurity]::new()
$taskAcl.SetAccessRuleProtection($true,$false)
$taskAcl.SetOwner($taskSid)
foreach($taskPrincipal in @($taskSid,[Security.Principal.SecurityIdentifier]::new('S-1-5-18'))){
    $taskAcl.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new($taskPrincipal,'FullControl','ContainerInherit,ObjectInherit','None','Allow'))
}
[IO.Directory]::SetAccessControl($taskOutput,$taskAcl)
$taskClock=[Diagnostics.Stopwatch]::StartNew()
function Read-ArtifactMetadata {
    $taskEntries=[Collections.Generic.Dictionary[string,string]]::new([StringComparer]::OrdinalIgnoreCase)
    $taskPending=[Collections.Generic.Stack[IO.DirectoryInfo]]::new()
    $taskPending.Push([IO.DirectoryInfo]::new($taskSource))
    $taskBytes=0L;$taskFiles=0;$taskDirectories=0
    while($taskPending.Count -gt 0){
        if($taskClock.Elapsed.TotalSeconds -ge 10){throw 'Artifact observation exceeded cooperative ten-second budget'}
        $taskDirectory=$taskPending.Pop()
        $taskDirectory.Refresh()
        if(-not $taskDirectory.Exists -or ($taskDirectory.Attributes -band [IO.FileAttributes]::ReparsePoint)){throw 'Selected directory disappeared or became redirected'}
        $taskDirectories++
        if($taskEntries.Count -ge 10000){throw 'Artifact observation exceeds ten-thousand-entry budget'}
        $taskEntries.Add($taskDirectory.FullName,('directory:'+ $taskDirectory.CreationTimeUtc.Ticks+':'+$taskDirectory.LastWriteTimeUtc.Ticks))
        foreach($taskEntry in $taskDirectory.EnumerateFileSystemInfos()){
            if($taskClock.Elapsed.TotalSeconds -ge 10){throw 'Artifact observation exceeded cooperative ten-second budget'}
            $taskEntry.Refresh()
            if(-not $taskEntry.Exists -or ($taskEntry.Attributes -band [IO.FileAttributes]::ReparsePoint)){throw 'Refuse missing or redirected artifact entry'}
            if($taskEntry -is [IO.DirectoryInfo]){if(($taskEntries.Count+$taskPending.Count) -ge 10000){throw 'Artifact observation exceeds ten-thousand-entry budget'};$taskPending.Push($taskEntry);continue}
            if($taskEntry -isnot [IO.FileInfo]){throw 'Refuse unsupported artifact entry'}
            if($taskEntries.Count -ge 10000){throw 'Artifact observation exceeds ten-thousand-entry budget'}
            $taskLength=$taskEntry.Length
            if($taskLength -lt 0 -or $taskBytes -gt ([long]::MaxValue-$taskLength)){throw 'Invalid artifact byte total'}
            $taskEntries.Add($taskEntry.FullName,('file:'+$taskLength+':'+$taskEntry.CreationTimeUtc.Ticks+':'+$taskEntry.LastWriteTimeUtc.Ticks))
            $taskBytes += $taskLength;$taskFiles++
        }
    }
    return [pscustomobject]@{entries=$taskEntries;logical_bytes=$taskBytes;files=$taskFiles;directories=$taskDirectories}
}
$taskReceipt=[ordered]@{schema='r171-private-artifact-storage';complete=$false;logical_bytes=$null;files=$null;directories=$null;budget_bytes=134217728;within_logical_budget=$null;metadata_stable=$false;observation_seconds=$null;failure=$null;scope='explicit account-owned selected root; source privacy is a prerequisite, not verified; metadata only; two cooperative non-atomic observations; directory entries counted separately even for hard-linked files';excluded='physical allocated bytes, filesystem metadata, Docker logs, alternate data streams, files outside the selected root, crash durability and continuously enforced quota';source_privacy_verified=$false;complete_disk_budget_verified=$false}
$taskFailure=$null
try {
    $taskBefore=Read-ArtifactMetadata
    $taskReceipt.logical_bytes=$taskBefore.logical_bytes
    $taskReceipt.files=$taskBefore.files
    $taskReceipt.directories=$taskBefore.directories
    $taskAfter=Read-ArtifactMetadata
    if($taskBefore.entries.Count -ne $taskAfter.entries.Count){throw 'Artifact tree changed during observation'}
    foreach($taskName in $taskBefore.entries.Keys){
        if(-not $taskAfter.entries.ContainsKey($taskName) -or $taskBefore.entries[$taskName] -cne $taskAfter.entries[$taskName]){throw 'Artifact metadata changed during observation'}
    }
    $taskReceipt.metadata_stable=$true
    $taskReceipt.within_logical_budget=($taskBefore.logical_bytes -le $taskReceipt.budget_bytes)
    if(-not $taskReceipt.within_logical_budget){throw 'Selected artifact logical bytes exceed 128-MiB budget'}
    $taskReceipt.complete=$true
} catch {
    $taskFailure=$_
    # Retain a bounded category, never source paths or native exception text.
    $taskReceipt.failure='observation-refused'
} finally {
    $taskReceipt.observation_seconds=$taskClock.Elapsed.TotalSeconds
    $taskReceipt | ConvertTo-Json -Depth 4 | Out-File (Join-Path $taskOutput 'artifact-storage.json') -Encoding utf8 -ErrorAction Stop
}
if($null -ne $taskFailure){throw $taskFailure}
Write-Output 'Selected private artifact logical bytes recorded; physical disk coverage remains unverified.'
