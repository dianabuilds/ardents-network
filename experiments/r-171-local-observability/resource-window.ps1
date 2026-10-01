param([Parameter(Mandatory=$true)][ValidatePattern('^r171-[a-z0-9-]{1,32}$')][string]$RunName,
      [Parameter(Mandatory=$true)][string]$ReportsRoot)
$ErrorActionPreference='Stop'
$taskDestination=Join-Path $ReportsRoot 'resource-window'
if (Test-Path -LiteralPath $taskDestination) { throw 'Refuse overwrite of resource evidence.' }
New-Item -ItemType Directory -Path $taskDestination | Out-Null
# This selected synthetic workload uses less than one CPU during its healthy window.
# Configure the complete seven-container ceiling; no query helper runs in the window.
$taskCaps=[ordered]@{fixture=0.05;prometheus=0.25;alertmanager=0.10;loki=0.25;alloy=0.15;grafana=0.15;'state-anchor'=0.01}
$taskNames=@(); $taskLimits=@()
foreach ($taskRole in $taskCaps.Keys) {
    $taskName=$RunName+'-'+$taskRole+'-1'
    docker update --cpus ([string]$taskCaps[$taskRole]) $taskName | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Failed to set selected resource-window CPU cap.' }
    $taskInspect=docker inspect $taskName | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0 -or -not $taskInspect[0].State.Running) { throw 'Selected resource-window container unavailable.' }
    if ($taskInspect[0].HostConfig.NanoCpus -ne [long]($taskCaps[$taskRole]*1000000000)) { throw 'CPU cap not applied.' }
    $taskNames += $taskName
    $taskLimits += [pscustomobject]@{role=$taskRole;cpu_limit=$taskCaps[$taskRole];memory_limit=$taskInspect[0].HostConfig.Memory;image=$taskInspect[0].Image}
}
$taskLimits | ConvertTo-Json -Depth 4 | Out-File (Join-Path $taskDestination 'limits.json') -Encoding utf8
$taskStarted=(Get-Date).ToUniversalTime()
$taskTimer=[Diagnostics.Stopwatch]::StartNew()
$taskSamples=@(); $taskPeak=0L
while ($true) {
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
    if ($taskTimer.Elapsed.TotalSeconds -ge 300) { break }
    Start-Sleep -Seconds 5
}
$taskEnded=(Get-Date).ToUniversalTime()
$taskDuration=$taskTimer.Elapsed.TotalSeconds
$taskMean=($taskSamples | Measure-Object -Property aggregate_cpu_percent -Average).Average
$taskSummary=[ordered]@{started_utc=$taskStarted.ToString('o');ended_utc=$taskEnded.ToString('o');duration_seconds=$taskDuration;sample_count=$taskSamples.Count;sampled_peak_rss_bytes=$taskPeak;mean_sampled_cpu_percent=$taskMean;combined_enforced_cpu_limit=0.96;cpu_percent_basis='100 percent is one core';rss_scope='sum of selected-container process RSS including Grafana plugin children';limits='RSS peak is sampled; Docker daemon and CLI host overhead are outside these container measurements';passed=$true}
if ($taskSamples.Count -lt 20) { throw 'Insufficient resource-window samples.' }
$taskSummary | ConvertTo-Json -Depth 4 | Out-File (Join-Path $taskDestination 'summary.json') -Encoding utf8
Write-Output 'Completed selected-container healthy five-minute resource window.'