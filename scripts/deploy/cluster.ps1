param(
    [Parameter(Position = 0)]
    [ValidateSet("up", "status", "start", "stop", "down")]
    [string]$Action = "status",
    [string]$StateDir = "var/deployment/local-multinode",
    [string]$Project = "ardents-local",
    [string]$Image = "ardents/ardd-local:dev",
    [ValidateRange(10, 300)]
    [int]$TimeoutSeconds = 90,
    [switch]$Build,
    [switch]$RemoveVolumes
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location $root
$statePath = [IO.Path]::GetFullPath($StateDir)
$composeFile = Join-Path $root "docker/docker-compose.multinode.yml"
$manifestPath = Join-Path $statePath "cluster.json"
$services = @("seed", "peer2", "peer3")
$composePrefix = @("compose", "-p", $Project, "-f", $composeFile)

function Invoke-Compose([string[]]$Arguments) {
    & docker @composePrefix @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose failed: $($Arguments -join ' ')"
    }
}

function New-RandomToken {
    $bytes = [byte[]]::new(32)
    $rng = [Security.Cryptography.RandomNumberGenerator]::Create()
    try { $rng.GetBytes($bytes) } finally { $rng.Dispose() }
    return [Convert]::ToBase64String($bytes)
}

function Ensure-PrivateToken([string]$Path) {
    if (Test-Path -LiteralPath $Path) { return }
    [IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($Path)) | Out-Null
    [IO.File]::WriteAllText($Path, (New-RandomToken) + "`n", [Text.UTF8Encoding]::new($false))
}

function Set-DeploymentEnvironment([string]$Bootstrap = "pending") {
    $secretDir = Join-Path $statePath "secrets"
    $env:ARDENTS_SEED_API_TOKEN_FILE = Join-Path $secretDir "seed.token"
    $env:ARDENTS_PEER2_API_TOKEN_FILE = Join-Path $secretDir "peer2.token"
    $env:ARDENTS_PEER3_API_TOKEN_FILE = Join-Path $secretDir "peer3.token"
    $env:ARDENTS_DOCKER_IMAGE = $Image
    $env:ARDENTS_BOOTSTRAP_PEER = $Bootstrap
}

function Ensure-LocalSecrets {
    foreach ($service in $services) {
        Ensure-PrivateToken (Join-Path $statePath "secrets/$service.token")
    }
}

function Assert-NodesStopped {
    foreach ($service in $services) {
        $container = ((& docker @composePrefix ps -q $service) -join "").Trim()
        if ($container -and ((& docker inspect -f "{{.State.Running}}" $container).Trim() -eq "true")) {
            throw "$service is already running; use status, stop, or start instead of up"
        }
    }
}

function Invoke-ProvisionNode([string]$Service, [int]$Port, [string]$Bootstrap = "") {
    $arguments = @(
        "run", "--rm", "--no-deps", "realm-provisioner",
        "-authority-dir", "/authority",
        "-node-dir", "/nodes/$Service",
        "-secret-dir", "/secrets/$Service",
        "-node-name", $Service,
        "-transport-port", "$Port"
    )
    if ($Bootstrap) { $arguments += @("-bootstrap-peer", $Bootstrap) }
    Invoke-Compose $arguments
}

function Invoke-ArdJson([string]$Service, [string[]]$Arguments) {
    $raw = & docker @composePrefix exec -T $Service ard --token-file /run/ardents/api-token --output json @Arguments
    if ($LASTEXITCODE -ne 0) { throw "ard command failed for $Service" }
    return (($raw -join "`n") | ConvertFrom-Json)
}

function Wait-Condition([string]$Description, [scriptblock]$Condition) {
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    $lastError = ""
    while ([DateTime]::UtcNow -lt $deadline) {
        try {
            if (& $Condition) { return }
        } catch { $lastError = $_.Exception.Message }
        Start-Sleep -Seconds 2
    }
    throw "timed out waiting for $Description; $lastError"
}

function Wait-API([string]$Service) {
    Wait-Condition "$Service local API" {
        $null = Invoke-ArdJson $Service @("node", "status")
        return $true
    }
}

function Wait-NetworkJoined([string]$Service) {
    Wait-Condition "$Service network participation" {
        $status = Invoke-ArdJson $Service @("network", "status")
        return $status.network.joined -eq $true
    }
}

function Wait-ProductReady([string]$Service) {
    Wait-Condition "$Service product readiness" {
        $status = Invoke-ArdJson $Service @("node", "status")
        return $status.snapshot.node.ready -eq $true
    }
}

function Wait-SeedReady {
    Wait-Condition "seed network listener and publication" {
        $status = Invoke-ArdJson "seed" @("network", "status")
        if ($status.network.state -ne "ready") { return $false }
        try { return -not [string]::IsNullOrWhiteSpace((Get-SeedEndpoint)) }
        catch { return $false }
    }
}

function Get-SeedEndpoint {
    $response = Invoke-ArdJson "seed" @("network", "records", "list")
    $record = $response.records |
        Where-Object { $_.kind -eq "node" -and $_.endpoints.Count -gt 0 } |
        Select-Object -First 1
    if ($null -eq $record) { throw "seed has no published node record" }
    $endpoint = $record.endpoints |
        Where-Object {
            ($_ -match "/tcp/61001/.*/p2p/" -or $_ -match "/tcp/61001/p2p/") -and
            $_ -notmatch "^/ip4/127\." -and $_ -notmatch "^/ip6/(::1|0*:0*:0*:0*:0*:0*:0*:1)/"
        } |
        Select-Object -First 1
    if ([string]::IsNullOrWhiteSpace($endpoint) -or $endpoint -notmatch "^/(ip4|ip6|dns4|dns6)/.+/tcp/61001/.*/p2p/[A-Za-z0-9]+$" -and $endpoint -notmatch "^/(ip4|ip6|dns4|dns6)/.+/tcp/61001/p2p/[A-Za-z0-9]+$") {
        throw "seed published no valid TCP Waku bootstrap endpoint"
    }
    return $endpoint
}

function Read-Manifest {
    if (-not (Test-Path -LiteralPath $manifestPath)) {
        throw "cluster manifest is missing; run ./ardents.ps1 up"
    }
    return (Get-Content -Raw -LiteralPath $manifestPath | ConvertFrom-Json)
}

function Write-Manifest([string]$Bootstrap) {
    $manifest = [ordered]@{
        schema = "ardents.deployment/v1"
        project = $Project
        image = $Image
        bootstrap_endpoint = $Bootstrap
        services = $services
        updated_at = [DateTime]::UtcNow.ToString("O")
    }
    [IO.Directory]::CreateDirectory($statePath) | Out-Null
    $manifest | ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 -LiteralPath $manifestPath
}

function Start-CleanCluster {
    Ensure-LocalSecrets
    Set-DeploymentEnvironment
    Assert-NodesStopped
    if ($Build) { Invoke-Compose @("build", "realm-provisioner") }
    Invoke-ProvisionNode "seed" 61001
    Invoke-ProvisionNode "peer2" 61002
    Invoke-ProvisionNode "peer3" 61003
    # Re-open earlier stopped stores after all realm members exist so every
    # receiver retains the signed sender grants of every other member.
    Invoke-ProvisionNode "seed" 61001
    Invoke-ProvisionNode "peer2" 61002
    $up = @("up", "-d")
    Invoke-Compose ($up + "seed")
    Wait-API "seed"
    Wait-SeedReady
    Wait-ProductReady "seed"
    $bootstrap = Get-SeedEndpoint
    Set-DeploymentEnvironment $bootstrap
    Invoke-ProvisionNode "peer2" 61002 $bootstrap
    Invoke-ProvisionNode "peer3" 61003 $bootstrap
    Write-Manifest $bootstrap
    Invoke-Compose ($up + @("peer2", "peer3"))
    foreach ($service in @("peer2", "peer3")) {
        Wait-NetworkJoined $service
        Wait-ProductReady $service
    }
    Write-Host "Ardents local cluster is product-ready without manual bootstrap values."
}

function Show-Status {
    $manifest = Read-Manifest
    Set-DeploymentEnvironment $manifest.bootstrap_endpoint
    $items = foreach ($service in $services) {
        try {
            $node = Invoke-ArdJson $service @("node", "status")
            $network = Invoke-ArdJson $service @("network", "status")
            $workloads = Invoke-ArdJson $service @("workload", "list")
            $inventory = Invoke-ArdJson $service @("data", "inventory")
            [ordered]@{
                service = $service
                lifecycle = $node.snapshot.node.state
                ready = $node.snapshot.node.ready
                network_state = $network.network.state
                joined = $network.network.joined
                active_mode = $network.network.activeMode
                workload_responsive = $null -ne $workloads.workloads
                data_responsive = $null -ne $inventory
            }
        } catch {
            [ordered]@{ service = $service; error = $_.Exception.Message }
        }
    }
    $items | ConvertTo-Json -Depth 5
}

switch ($Action) {
    "up" { Start-CleanCluster }
    "status" { Show-Status }
    "start" {
        $manifest = Read-Manifest
        Set-DeploymentEnvironment $manifest.bootstrap_endpoint
        Invoke-Compose @("start")
        Wait-SeedReady
        Wait-ProductReady "seed"
        foreach ($service in @("peer2", "peer3")) {
            Wait-NetworkJoined $service
            Wait-ProductReady $service
        }
    }
    "stop" {
        $manifest = Read-Manifest
        Set-DeploymentEnvironment $manifest.bootstrap_endpoint
        Invoke-Compose @("stop")
    }
    "down" {
        $manifest = Read-Manifest
        Set-DeploymentEnvironment $manifest.bootstrap_endpoint
        $down = @("down", "--remove-orphans")
        if ($RemoveVolumes) {
            $down = @("--profile", "provisioning") + $down + "--volumes"
        }
        Invoke-Compose $down
        if ($RemoveVolumes) {
            Write-Host "Containers and project volumes removed; deployment state files retained."
        } else {
            Write-Host "Containers removed; persistent volumes and deployment state retained."
        }
    }
}
