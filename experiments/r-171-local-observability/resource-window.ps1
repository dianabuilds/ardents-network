param([Parameter(Mandatory=$true)][ValidatePattern('^r171-[a-z0-9-]{1,32}$')][string]$RunName,
      [Parameter(Mandatory=$true)][string]$ReportsRoot,
      [ValidateSet('synthetic','node')][string]$SourceProfile='synthetic')
$ErrorActionPreference='Stop'
$taskDestination=Join-Path $ReportsRoot 'resource-window'
if (Test-Path -LiteralPath $taskDestination) { throw 'Refuse overwrite of resource evidence.' }
New-Item -ItemType Directory -Path $taskDestination | Out-Null
# Apply a finite ceiling below one core for the selected observation workload.
# Configure the complete seven-container ceiling; no query helper runs in the window.
$taskCaps=[ordered]@{fixture=0.05;prometheus=0.25;alertmanager=0.10;loki=0.25;alloy=0.15;grafana=0.15;'state-anchor'=0.01}
if ($SourceProfile -eq 'node') {
    $taskCaps=[ordered]@{fixture=0.15;prometheus=0.20;alertmanager=0.10;loki=0.20;alloy=0.15;grafana=0.15;'browser-tunnel'=0.01}
}
$taskNames=@(); $taskLimits=@(); $taskIdentities=@{}
function Assert-WindowIdentity($Item, [string]$Name, $Expected) {
    if ($Item.Name -cne ('/'+$Name) -or $Item.Config.Labels.'com.docker.compose.project' -cne $RunName -or
        -not $Item.State.Running -or $Item.State.Paused -or $Item.State.Restarting -or $Item.State.OOMKilled) {
        throw 'Selected container is unavailable, paused, restarting, OOM-killed or outside this project.'
    }
    if ($null -ne $Expected -and ($Item.Id -cne $Expected.id -or $Item.State.StartedAt -cne $Expected.started -or
        $Item.RestartCount -ne $Expected.restarts)) { throw 'Selected container changed or restarted during resource window.' }
}
# Validate every selected identity before changing any CPU cap.
foreach ($taskRole in $taskCaps.Keys) {
    $taskName=$RunName+'-'+$taskRole+'-1'
    $taskInspect=docker inspect $taskName | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0 -or @($taskInspect).Count -ne 1) { throw 'Selected resource-window container unavailable.' }
    Assert-WindowIdentity $taskInspect[0] $taskName $null
    if ($taskInspect[0].Config.Labels.'com.docker.compose.service' -cne $taskRole) { throw 'Selected role identity mismatch.' }
}
foreach ($taskRole in $taskCaps.Keys) {
    $taskName=$RunName+'-'+$taskRole+'-1'
    docker update --cpus ([string]$taskCaps[$taskRole]) $taskName | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Failed to set selected resource-window CPU cap.' }
    $taskInspect=docker inspect $taskName | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0 -or @($taskInspect).Count -ne 1) { throw 'Selected resource-window container unavailable.' }
    Assert-WindowIdentity $taskInspect[0] $taskName $null
    $taskIdentities[$taskName]=@{id=$taskInspect[0].Id;started=$taskInspect[0].State.StartedAt;restarts=$taskInspect[0].RestartCount}
    if ($taskInspect[0].HostConfig.NanoCpus -ne [long]($taskCaps[$taskRole]*1000000000)) { throw 'CPU cap not applied.' }
    $taskNames += $taskName
    $taskLimits += [pscustomobject]@{role=$taskRole;cpu_limit=$taskCaps[$taskRole];memory_limit=$taskInspect[0].HostConfig.Memory;image=$taskInspect[0].Image}
}
$taskLimits | ConvertTo-Json -Depth 4 | Out-File (Join-Path $taskDestination 'limits.json') -Encoding utf8
function Assert-WindowContainers {
    $taskCurrent=docker inspect @taskNames | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0 -or $taskCurrent.Count -ne $taskNames.Count) { throw 'Selected container identity set unavailable.' }
    foreach ($taskItem in $taskCurrent) {
        $taskIdentityName=$taskItem.Name.TrimStart('/')
        if (-not $taskIdentities.ContainsKey($taskIdentityName)) { throw 'Unexpected container identity.' }
        Assert-WindowIdentity $taskItem $taskIdentityName $taskIdentities[$taskIdentityName]
        if ($taskItem.HostConfig.NanoCpus -ne [long]($taskCaps[$taskItem.Config.Labels.'com.docker.compose.service']*1000000000)) {
            throw 'Resource-window CPU cap changed.'
        }
    }
}
$taskStarted=(Get-Date).ToUniversalTime()
$taskTimer=[Diagnostics.Stopwatch]::StartNew()
$taskSamples=@(); $taskPeak=0L
while ($true) {
    Assert-WindowContainers
    $taskStats=docker stats --no-stream --format '{{json .}}' @taskNames
    if ($LASTEXITCODE -ne 0) { throw 'Resource-window Docker stats failed.' }
    $taskRows=@($taskStats | ForEach-Object { $_ | ConvertFrom-Json })
    if ($taskRows.Count -ne $taskNames.Count) { throw 'Resource-window container set incomplete.' }
    $taskRss=0L; $taskCPU=0.0; $taskObservations=@()
    foreach ($taskName in $taskNames) {
        # Docker top needs PID to filter its own ps result, but only RSS/count leave this scope.
        # No argv, process names or host process walk is retained.
        $taskTop=docker top $taskName -eo pid,rss
        if ($LASTEXITCODE -ne 0) { throw 'Selected-container RSS observation failed.' }
        $taskRoleRSS=0L; $taskProcessCount=0
        foreach ($taskLine in $taskTop) {
            if ($taskLine -match '^\s*[0-9]+\s+([0-9]+)\s*$') {
                $taskRoleRSS += [long]$Matches[1]*1024
                $taskProcessCount++
            }
        }
        if ($taskProcessCount -eq 0) { throw 'Selected-container RSS observation missing.' }
        $taskRow=@($taskRows | Where-Object {$_.Name -eq $taskName})
        if ($taskRow.Count -ne 1) { throw 'Selected-container stats identity mismatch.' }
        $taskPercent=[double]::Parse($taskRow[0].CPUPerc.TrimEnd('%'),[Globalization.CultureInfo]::InvariantCulture)
        if ([double]::IsNaN($taskPercent) -or [double]::IsInfinity($taskPercent) -or $taskPercent -lt 0) { throw 'Invalid CPU observation.' }
        $taskRss += $taskRoleRSS; $taskCPU += $taskPercent
        $taskObservations += [pscustomobject]@{container=$taskName;rss_bytes=$taskRoleRSS;process_count=$taskProcessCount;cpu_percent=$taskPercent}
    }
    if ($taskRss -gt $taskPeak) { $taskPeak=$taskRss }
    $taskSample=[pscustomobject]@{at=(Get-Date).ToUniversalTime().ToString('o');elapsed_seconds=$taskTimer.Elapsed.TotalSeconds;aggregate_rss_bytes=$taskRss;aggregate_cpu_percent=$taskCPU;containers=$taskObservations}
    $taskSamples += $taskSample
    $taskSample | ConvertTo-Json -Compress -Depth 5 | Out-File (Join-Path $taskDestination 'samples.ndjson') -Append -Encoding utf8
    if ($taskRss -gt 1073741824) { throw 'Observed aggregate RSS exceeded declared one-GiB target.' }
    Assert-WindowContainers
    if ($taskTimer.Elapsed.TotalSeconds -ge 300) { break }
    Start-Sleep -Seconds 5
}
$taskEnded=(Get-Date).ToUniversalTime()
$taskDuration=$taskTimer.Elapsed.TotalSeconds
$taskMean=($taskSamples | Measure-Object -Property aggregate_cpu_percent -Average).Average
$taskSummary=[ordered]@{started_utc=$taskStarted.ToString('o');ended_utc=$taskEnded.ToString('o');duration_seconds=$taskDuration;sample_count=$taskSamples.Count;sampled_peak_rss_bytes=$taskPeak;mean_sampled_cpu_percent=$taskMean;source_profile=$SourceProfile;combined_enforced_cpu_limit=0.96;cpu_percent_basis='100 percent is one core';rss_scope='sum of selected-container process RSS including Grafana plugin children';limits='RSS peak is sampled; Docker daemon and CLI host overhead are outside these container measurements';passed=$true}
if ($taskSamples.Count -lt 20) { throw 'Insufficient resource-window samples.' }
if ($taskMean -gt 100) { throw 'Observed mean sampled CPU exceeded one core.' }
# Container survival is not source freshness or product readiness. A separate native
# history query must verify the selected source over these exact UTC bounds.
$taskSummary | ConvertTo-Json -Depth 4 | Out-File (Join-Path $taskDestination 'summary.json') -Encoding utf8
Write-Output 'Completed selected-container five-minute resource window; source history verification still required.'