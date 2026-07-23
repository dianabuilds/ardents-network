param(
    [double]$MaxGiB = 8,
    [switch]$Force,
    [switch]$StatusOnly,
    [switch]$BoundBuildKit
)

$ErrorActionPreference = "Stop"

$cacheVolumes = @(
    "ardents-go-build-cache",
    "ardents-go-mod-cache"
)

function Convert-DockerSizeToBytes {
    param([string]$Size)

    if ([string]::IsNullOrWhiteSpace($Size) -or $Size -eq "0B") { return 0L }
    if ($Size -notmatch '^([0-9]+(?:\.[0-9]+)?)([kMGT]?i?B)$') {
        throw "unsupported Docker size: $Size"
    }

    $value = [double]$Matches[1]
    $factor = switch ($Matches[2]) {
        "B" { 1 }
        "kB" { 1e3 }
        "MB" { 1e6 }
        "GB" { 1e9 }
        "TB" { 1e12 }
        "KiB" { 1KB }
        "MiB" { 1MB }
        "GiB" { 1GB }
        "TiB" { 1TB }
        default { throw "unsupported Docker size unit: $($Matches[2])" }
    }
    return [long]($value * $factor)
}

$rawReport = & docker system df --verbose --format json
if ($LASTEXITCODE -ne 0) { throw "unable to inspect Docker storage" }
$report = $rawReport | ConvertFrom-Json

$totalBytes = 0L
foreach ($name in $cacheVolumes) {
    $volume = @($report.Volumes | Where-Object { $_.Name -eq $name }) | Select-Object -First 1
    if ($null -eq $volume) {
        Write-Host "Docker cache volume: absent ($name)"
        continue
    }
    $bytes = Convert-DockerSizeToBytes $volume.Size
    $totalBytes += $bytes
    Write-Host ("Docker cache volume: {0} ({1}, links={2})" -f $name, $volume.Size, $volume.Links)
}

$totalGiB = $totalBytes / 1GB
Write-Host ("Ardents Docker cache total: {0:N2} GiB; cleanup threshold: {1:N2} GiB" -f $totalGiB, $MaxGiB)

if (-not $StatusOnly) {
    if (-not $Force -and $totalGiB -le $MaxGiB) {
        Write-Host "Docker caches retained: size is within the threshold."
    } else {
        foreach ($name in $cacheVolumes) {
            $exists = & docker volume ls --quiet --filter "name=^$name$"
            if ($LASTEXITCODE -ne 0) { throw "unable to inspect Docker volume $name" }
            if ($exists -contains $name) {
                & docker volume rm $name
                if ($LASTEXITCODE -ne 0) {
                    throw "unable to remove Docker cache volume $name; stop any test runner using it and retry"
                }
            }
        }
        Write-Host "Removed only the two Ardents Go cache volumes. They will be recreated on demand."
    }
}

if ($BoundBuildKit -and -not $StatusOnly) {
    # BuildKit storage is shared by all repositories, so this is deliberately
    # opt-in. It retains the most useful cache up to the requested bound.
    $limit = "{0}gb" -f [math]::Max(1, [math]::Ceiling($MaxGiB))
    & docker buildx prune --force --max-used-space $limit
    if ($LASTEXITCODE -ne 0) { throw "unable to bound Docker BuildKit cache" }
}
