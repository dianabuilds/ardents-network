param(
    [string]$Label = "manual",
    [string]$OutputPath = "",
    [ValidateRange(1, 10)]
    [int]$SampleSeconds = 2
)

$ErrorActionPreference = "Stop"
$before = @{}
Get-Process -ErrorAction SilentlyContinue | ForEach-Object {
    if ($null -ne $_.CPU) { $before[$_.Id] = $_.CPU }
}
Start-Sleep -Seconds $SampleSeconds

$cores = [Environment]::ProcessorCount
$processes = foreach ($process in Get-Process -ErrorAction SilentlyContinue) {
    if (-not $before.ContainsKey($process.Id) -or $null -eq $process.CPU) { continue }
    $delta = $process.CPU - $before[$process.Id]
    if ($delta -le 0.01) { continue }
    [pscustomobject][ordered]@{
        name = $process.ProcessName
        pid = $process.Id
        cpu_percent = [math]::Round(100 * $delta / ($SampleSeconds * $cores), 2)
        memory_mb = [math]::Round($process.WorkingSet64 / 1MB, 1)
    }
}
$processes = @($processes | Sort-Object cpu_percent -Descending | Select-Object -First 15)

$memory = [ordered]@{ total_mb = $null; available_mb = $null; used_percent = $null }
try {
    $os = Get-CimInstance Win32_OperatingSystem
    $memory.total_mb = [math]::Round($os.TotalVisibleMemorySize / 1KB, 1)
    $memory.available_mb = [math]::Round($os.FreePhysicalMemory / 1KB, 1)
    $memory.used_percent = [math]::Round(100 * (1 - $os.FreePhysicalMemory / $os.TotalVisibleMemorySize), 1)
} catch {}

$drive = [IO.DriveInfo]::new([IO.Path]::GetPathRoot($PSScriptRoot))
$vmmem = Get-Process -Name vmmemWSL -ErrorAction SilentlyContinue | Select-Object -First 1
$snapshot = [ordered]@{
    label = $Label
    captured_at = [DateTime]::UtcNow.ToString("o")
    sample_seconds = $SampleSeconds
    logical_processors = $cores
    memory = $memory
    disk = if ($drive.IsReady) { [ordered]@{
        name = $drive.Name
        free_gb = [math]::Round($drive.AvailableFreeSpace / 1GB, 2)
        used_gb = [math]::Round(($drive.TotalSize - $drive.TotalFreeSpace) / 1GB, 2)
    }} else { $null }
    vmmem_wsl_mb = if ($vmmem) { [math]::Round($vmmem.WorkingSet64 / 1MB, 1) } else { 0 }
    top_processes = $processes
}

Write-Host ("resource snapshot [{0}]: memory={1}% available={2}MB vmmemWSL={3}MB disk_free={4}GB" -f `
    $Label, $memory.used_percent, $memory.available_mb, $snapshot.vmmem_wsl_mb, $snapshot.disk.free_gb)
$processes | Select-Object -First 8 | Format-Table name, pid, cpu_percent, memory_mb -AutoSize

if ($OutputPath) {
    $fullPath = [IO.Path]::GetFullPath($OutputPath)
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $fullPath) | Out-Null
    $snapshot | ConvertTo-Json -Depth 8 | Set-Content -Encoding utf8 $fullPath
    Write-Host "==> resources $fullPath"
}
