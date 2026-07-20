param(
    [Parameter(Position = 0, Mandatory = $true)]
    [ValidateSet("upgrade", "rollback")]
    [string]$Action,
    [string]$NewImage,
    [string]$StateDir = "var/deployment/local-multinode",
    [string]$Project = "ardents-local",
    [ValidateRange(10, 300)]
    [int]$TimeoutSeconds = 120,
    [switch]$AllowMutableImage
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location $root
$statePath = [IO.Path]::GetFullPath($StateDir)
$manifestPath = Join-Path $statePath "cluster.json"
$composeFile = Join-Path $root "docker/docker-compose.multinode.yml"
$composePrefix = @("compose", "-p", $Project, "-f", $composeFile)
$services = @("seed", "peer2", "peer3")

if (-not (Test-Path -LiteralPath $manifestPath)) { throw "cluster manifest is missing" }
$cluster = Get-Content -Raw -LiteralPath $manifestPath | ConvertFrom-Json
$env:ARDENTS_BOOTSTRAP_PEER = [string]$cluster.bootstrap_endpoint
$env:ARDENTS_SEED_API_TOKEN_FILE = Join-Path $statePath "secrets/seed.token"
$env:ARDENTS_PEER2_API_TOKEN_FILE = Join-Path $statePath "secrets/peer2.token"
$env:ARDENTS_PEER3_API_TOKEN_FILE = Join-Path $statePath "secrets/peer3.token"

function Invoke-Compose([string[]]$Arguments) {
    & docker @composePrefix @Arguments
    if ($LASTEXITCODE -ne 0) { throw "docker compose failed: $($Arguments -join ' ')" }
}

function Invoke-ArdJson([string]$Service, [string[]]$Arguments) {
    $raw = & docker @composePrefix exec -T $Service ard --token-file /run/ardents/api-token --output json @Arguments
    if ($LASTEXITCODE -ne 0) { throw "ard command failed for $Service" }
    return (($raw -join "`n") | ConvertFrom-Json)
}

function Wait-Service([string]$Service) {
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    $lastError = ""
    while ([DateTime]::UtcNow -lt $deadline) {
        try {
            $status = Invoke-ArdJson $Service @("network", "status")
            if ($Service -eq "seed" -and $status.network.state -eq "ready") { return }
            if ($Service -ne "seed" -and $status.network.joined -eq $true) { return }
        } catch { $lastError = $_.Exception.Message }
        Start-Sleep -Seconds 2
    }
    throw "timed out waiting for $Service after image change; $lastError"
}

function Set-Image([string]$Image) {
    if ([string]::IsNullOrWhiteSpace($Image)) { throw "image reference is empty" }
    if (-not $AllowMutableImage -and $Image -notmatch "@sha256:[0-9a-fA-F]{64}$") {
        throw "an immutable digest image is required; use -AllowMutableImage only for a local development image"
    }
    $env:ARDENTS_DOCKER_IMAGE = $Image
}

function Recreate-Service([string]$Service) {
    Invoke-Compose @("up", "-d", "--no-deps", "--force-recreate", $Service)
    Wait-Service $Service
}

function Write-ClusterManifest([string]$Image, [string]$PreviousImage) {
    $updated = [ordered]@{
        schema = "ardents.deployment/v1"
        project = [string]$cluster.project
        image = $Image
        previous_image = $PreviousImage
        bootstrap_endpoint = [string]$cluster.bootstrap_endpoint
        services = $services
        updated_at = [DateTime]::UtcNow.ToString("O")
    }
    $updated | ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 -LiteralPath $manifestPath
}

function Invoke-RollingChange([string]$TargetImage, [string]$FallbackImage) {
    Set-Image $TargetImage
    $changed = [Collections.Generic.List[string]]::new()
    try {
        foreach ($service in $services) {
            Recreate-Service $service
            $changed.Add($service)
        }
    } catch {
        $changeError = $_.Exception.Message
        Set-Image $FallbackImage
        $rollbackErrors = [Collections.Generic.List[string]]::new()
        foreach ($service in $changed) {
            try { Recreate-Service $service } catch { $rollbackErrors.Add($_.Exception.Message) }
        }
        if ($rollbackErrors.Count -gt 0) {
            throw "image change failed: $changeError; automatic rollback also failed: $($rollbackErrors -join '; ')"
        }
        throw "image change failed and changed nodes were rolled back: $changeError"
    }
}

function New-UpgradeBackups {
    foreach ($service in $services) {
        & (Join-Path $PSScriptRoot "cluster-data.ps1") backup -Node $service -StateDir $statePath -Project $Project -TimeoutSeconds $TimeoutSeconds
        if ($LASTEXITCODE -ne 0) { throw "pre-upgrade backup failed for $service" }
    }
}

if ($Action -eq "upgrade") {
    if ([string]::IsNullOrWhiteSpace($NewImage)) { throw "-NewImage is required for upgrade" }
    $previous = [string]$cluster.image
    New-UpgradeBackups
    Invoke-RollingChange $NewImage $previous
    Write-ClusterManifest $NewImage $previous
    Write-Host "Rolling upgrade accepted: $previous -> $NewImage"
} else {
    $previous = [string]$cluster.previous_image
    if ([string]::IsNullOrWhiteSpace($previous)) { throw "cluster manifest has no previous image for rollback" }
    $current = [string]$cluster.image
    Invoke-RollingChange $previous $current
    Write-ClusterManifest $previous $current
    Write-Host "Rolling rollback accepted: $current -> $previous"
}
