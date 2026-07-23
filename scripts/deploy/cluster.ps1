param(
    [Parameter(Position = 0)]
    [ValidateSet("up", "status", "start", "stop", "down")]
    [string]$Action = "status",
    [string]$StateDir = "var/deployment/local-multinode",
    [string]$Project = "ardents-local",
    [string]$Image = "ardents/node-local:dev",
    [ValidateRange(10, 300)]
    [int]$TimeoutSeconds = 90,
    [switch]$Build,
    [switch]$RemoveVolumes
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location $root
$statePath = [IO.Path]::GetFullPath($StateDir)
$composeFile = Join-Path $root "deploy/docker/compose/docker-compose.multinode.yml"
$manifestPath = Join-Path $statePath "cluster.json"
$services = @("seed", "peer2", "peer3")
$composePrefix = @("compose", "-p", $Project, "-f", $composeFile)
$operatorRoot = "/operator-identity/root.json"
$operatorDevice = "/operator-identity/device.json"
$operationalActions = @(
    "node.status",
    "node.runtime",
    "transport.network_status",
    "discovery.list_records",
    "workload.list",
    "data.inventory"
)
$nodePrincipals = @{}
$operatorPrincipal = ""

function Invoke-Compose([string[]]$Arguments) {
    & docker @composePrefix @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose failed: $($Arguments -join ' ')"
    }
}

function Invoke-OperatorTool([string]$Service, [string[]]$Arguments, [switch]$AllowFailure) {
    $raw = & docker @composePrefix --profile tools run --rm --no-deps "$Service-operator" @Arguments
    if ($LASTEXITCODE -ne 0 -and -not $AllowFailure) {
        throw "Principal-authenticated ardentsctl command failed for $Service"
    }
    return [ordered]@{ ExitCode = $LASTEXITCODE; Output = (($raw -join "`n").Trim()) }
}

function Test-OperatorFile([string]$Path) {
    & docker @composePrefix --profile tools run --rm --no-deps --entrypoint /bin/sh seed-operator -ec "test -f '$Path'"
    return $LASTEXITCODE -eq 0
}

function Ensure-OperatorIdentity {
    & docker @composePrefix --profile tools run --rm --no-deps --entrypoint /bin/sh seed-operator -ec "chmod 0700 /operator-identity"
    if ($LASTEXITCODE -ne 0) { throw "cannot protect the Operator identity volume" }
    if (-not (Test-OperatorFile $operatorRoot)) {
        $null = Invoke-OperatorTool "seed" @("--output", "json", "identity", "principal", "create", "--signer-file", $operatorRoot)
    }
    if (-not (Test-OperatorFile $operatorDevice)) {
        $null = Invoke-OperatorTool "seed" @("--output", "json", "identity", "device", "create", "--root-signer-file", $operatorRoot, "--signer-file", $operatorDevice)
    }
    $shown = Invoke-OperatorTool "seed" @("--output", "json", "identity", "principal", "show", "--signer-file", $operatorRoot)
    $identity = $shown.Output | ConvertFrom-Json
    if ([string]::IsNullOrWhiteSpace([string]$identity.principal)) {
        throw "operator Principal identity is unavailable"
    }
    return [string]$identity.principal
}

function Invoke-PrincipalJson([string]$Service, [string]$NodePrincipal, [string[]]$Arguments) {
    if ([string]::IsNullOrWhiteSpace($NodePrincipal)) {
        throw "node Principal is required for $Service"
    }
    $result = Invoke-OperatorTool $Service (@(
        "--output", "json",
        "--signer-file", $operatorDevice,
        "--principal", $NodePrincipal
    ) + $Arguments)
    if ([string]::IsNullOrWhiteSpace($result.Output)) { return $null }
    return ($result.Output | ConvertFrom-Json)
}

function Set-DeploymentEnvironment([string]$Bootstrap = "pending") {
    $env:ARDENTS_DOCKER_IMAGE = $Image
    $env:ARDENTS_BOOTSTRAP_PEER = $Bootstrap
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
        "-runtime-data-dir", "/var/lib/ardents",
        "-runtime-secret-dir", "/run/ardents",
        "-node-name", $Service,
        "-transport-port", "$Port"
    )
    if ($Bootstrap) { $arguments += @("-bootstrap-peer", $Bootstrap) }
    $raw = & docker @composePrefix @arguments
    if ($LASTEXITCODE -ne 0) {
        throw "node provisioning failed for $Service"
    }
    $text = ($raw -join "`n").Trim()
    $match = [regex]::Match($text, "(?:^|\s)principal=(p1_[A-Za-z0-9_-]+)(?:\s|$)")
    if (-not $match.Success) {
        throw "node provisioning did not report a canonical Principal for $Service"
    }
    Write-Host "Provisioned $Service as $($match.Groups[1].Value)."
    return $match.Groups[1].Value
}

function Invoke-ArdJson([string]$Service, [string[]]$Arguments) {
    return Invoke-PrincipalJson $Service ([string]$nodePrincipals[$Service]) $Arguments
}

function Wait-Condition([string]$Description, [scriptblock]$Condition) {
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    $lastError = ""
    while ([DateTime]::UtcNow -lt $deadline) {
        try {
            if (& $Condition) { return }
        } catch { $lastError = $_.Exception.Message }
        # Each disposable helper authenticates once. Keep retries below the
        # documented 10 authentication challenges/minute source limit.
        Start-Sleep -Seconds 7
    }
    throw "timed out waiting for $Description; $lastError"
}

function Test-NodeRuntimePath([string]$Service, [string]$Test, [string]$Path) {
    & docker @composePrefix --profile tools run --rm --no-deps --entrypoint /bin/sh "$Service-operator" -ec "test $Test '$Path'"
    return $LASTEXITCODE -eq 0
}

function Test-ServiceHealthy([string]$Service) {
    $container = ((& docker @composePrefix ps -q $Service) -join "").Trim()
    if ([string]::IsNullOrWhiteSpace($container)) { return $false }
    $health = ((& docker inspect --format "{{.State.Health.Status}}" $container) -join "").Trim()
    return $LASTEXITCODE -eq 0 -and $health -eq "healthy"
}

function Ensure-NodeOperator([string]$Service) {
    Wait-Condition "$Service healthy process and protected Operator socket" {
        return (Test-ServiceHealthy $Service) -and
            (Test-NodeRuntimePath $Service "-S" "/run/ardents/control.sock")
    }
    $ticket = "/run/ardents/operator-bootstrap-ticket"
    if (-not (Test-NodeRuntimePath $Service "-f" $ticket)) {
        $null = Invoke-ArdJson $Service @("node", "status")
        return
    }
    $enrollment = Invoke-OperatorTool $Service @(
        "--output", "json",
        "--signer-file", $operatorDevice,
        "--principal", [string]$nodePrincipals[$Service],
        "identity", "enroll",
        "--root-signer-file", $operatorRoot,
        "--device-signer-file", $operatorDevice,
        "--bootstrap-ticket-file", $ticket
    )
    $enrolled = $enrollment.Output | ConvertFrom-Json
    if ([string]$enrolled.principal -ne $operatorPrincipal) {
        throw "unexpected enrolled Principal on $Service"
    }
    $grantArgs = @(
        "identity", "grant", "issue",
        "--subject", $operatorPrincipal,
        "--scope", "node",
        "--request-id", "local-deployment-$Service-operational-v1",
        "--yes"
    )
    foreach ($action in $operationalActions) {
        $grantArgs += @("--action", $action)
    }
    $null = Invoke-PrincipalJson $Service ([string]$nodePrincipals[$Service]) $grantArgs
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
        Where-Object { $null -ne $_.nodeFacts -and $_.nodeFacts.endpoints.Count -gt 0 } |
        Select-Object -First 1
    if ($null -eq $record) { throw "seed has no published node record" }
    $endpoint = $record.nodeFacts.endpoints |
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

function Import-ManifestIdentities($Manifest) {
    $script:operatorPrincipal = [string]$Manifest.operator_principal
    if ([string]::IsNullOrWhiteSpace($script:operatorPrincipal) -or $null -eq $Manifest.node_principals) {
        throw "cluster manifest has no Principal identity bindings"
    }
    foreach ($service in $services) {
        $property = $Manifest.node_principals.PSObject.Properties[$service]
        if ($null -eq $property -or [string]::IsNullOrWhiteSpace([string]$property.Value)) {
            throw "cluster manifest has no node Principal for $service"
        }
        $script:nodePrincipals[$service] = [string]$property.Value
    }
}

function Write-Manifest([string]$Bootstrap) {
    $manifest = [ordered]@{
        schema = "ardents.deployment/v1"
        project = $Project
        image = $Image
        bootstrap_endpoint = $Bootstrap
        operator_principal = $operatorPrincipal
        node_principals = [ordered]@{
            seed = [string]$nodePrincipals.seed
            peer2 = [string]$nodePrincipals.peer2
            peer3 = [string]$nodePrincipals.peer3
        }
        services = $services
        updated_at = [DateTime]::UtcNow.ToString("O")
    }
    [IO.Directory]::CreateDirectory($statePath) | Out-Null
    $manifest | ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 -LiteralPath $manifestPath
}

function Start-CleanCluster {
    Set-DeploymentEnvironment
    Assert-NodesStopped
    if ($Build) { Invoke-Compose @("build", "realm-provisioner") }
    $script:operatorPrincipal = Ensure-OperatorIdentity
    try {
        Start-ClusterComponents
    } catch {
        $startupFailure = $_.Exception
        try {
            Invoke-Compose @("down", "--remove-orphans")
        } catch {
            throw "cluster startup failed: $($startupFailure.Message); partial cleanup failed: $($_.Exception.Message)"
        }
        throw $startupFailure
    }
}

function Start-ClusterComponents {
    $script:nodePrincipals.seed = Invoke-ProvisionNode "seed" 61001
    $script:nodePrincipals.peer2 = Invoke-ProvisionNode "peer2" 61002
    $script:nodePrincipals.peer3 = Invoke-ProvisionNode "peer3" 61003
    # Re-open earlier stopped stores after all realm members exist so every
    # receiver retains the signed sender grants of every other member.
    $seedPrincipal = Invoke-ProvisionNode "seed" 61001
    $peer2Principal = Invoke-ProvisionNode "peer2" 61002
    if ($seedPrincipal -ne $nodePrincipals.seed -or $peer2Principal -ne $nodePrincipals.peer2) {
        throw "node Principal changed during idempotent realm provisioning"
    }
    $up = @("up", "-d")
    Invoke-Compose ($up + "seed")
    Ensure-NodeOperator "seed"
    Wait-SeedReady
    Wait-ProductReady "seed"
    $bootstrap = Get-SeedEndpoint
    Set-DeploymentEnvironment $bootstrap
    $peer2Principal = Invoke-ProvisionNode "peer2" 61002 $bootstrap
    $peer3Principal = Invoke-ProvisionNode "peer3" 61003 $bootstrap
    if ($peer2Principal -ne $nodePrincipals.peer2 -or $peer3Principal -ne $nodePrincipals.peer3) {
        throw "node Principal changed while applying the bootstrap endpoint"
    }
    Write-Manifest $bootstrap
    Invoke-Compose ($up + @("peer2", "peer3"))
    foreach ($service in @("peer2", "peer3")) {
        Ensure-NodeOperator $service
        Wait-NetworkJoined $service
        Wait-ProductReady $service
    }
    Write-Host "Ardents local cluster is product-ready without manual bootstrap values."
}

function Show-Status {
    $manifest = Read-Manifest
    Import-ManifestIdentities $manifest
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
        Import-ManifestIdentities $manifest
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
        Import-ManifestIdentities $manifest
        Set-DeploymentEnvironment $manifest.bootstrap_endpoint
        Invoke-Compose @("stop")
    }
    "down" {
        $manifest = Read-Manifest
        Import-ManifestIdentities $manifest
        Set-DeploymentEnvironment $manifest.bootstrap_endpoint
        $down = @("down", "--remove-orphans")
        if ($RemoveVolumes) {
            $down = @("--profile", "provisioning", "--profile", "tools") + $down + "--volumes"
        }
        Invoke-Compose $down
        if ($RemoveVolumes) {
            Write-Host "Containers and project volumes removed; deployment state files retained."
        } else {
            Write-Host "Containers removed; persistent volumes and deployment state retained."
        }
    }
}
