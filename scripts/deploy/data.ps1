param(
    [Parameter(Position = 0, Mandatory = $true)]
    [ValidateSet("backup", "restore")]
    [string]$Action,
    [Parameter(Mandatory = $true)]
    [ValidateSet("seed", "peer2", "peer3")]
    [string]$Node,
    [string]$Archive,
    [string]$StateDir = "var/deployment/local-multinode",
    [string]$Project = "ardents-local",
    [ValidateRange(10, 300)]
    [int]$TimeoutSeconds = 90,
    [switch]$ConfirmReplace
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location $root
$statePath = [IO.Path]::GetFullPath($StateDir)
$manifestPath = Join-Path $statePath "cluster.json"
$repositoryComposeFile = Join-Path $root "deploy/docker/compose/docker-compose.multinode.yml"
$bundleComposeFile = Join-Path $root "docker/docker-compose.multinode.yml"
$composeFile = if (Test-Path -LiteralPath $repositoryComposeFile) {
    $repositoryComposeFile
} elseif (Test-Path -LiteralPath $bundleComposeFile) {
    $bundleComposeFile
} else {
    throw "multinode Compose file is missing from the repository or distribution bundle"
}
$composePrefix = @("compose", "-p", $Project, "-f", $composeFile)

if (-not (Test-Path -LiteralPath $manifestPath)) {
    throw "cluster manifest is missing; form the cluster before data operations"
}
$cluster = Get-Content -Raw -LiteralPath $manifestPath | ConvertFrom-Json
$image = [string]$cluster.image
$env:ARDENTS_DOCKER_IMAGE = $image
$env:ARDENTS_BOOTSTRAP_PEER = [string]$cluster.bootstrap_endpoint
$nodePrincipalProperty = $cluster.node_principals.PSObject.Properties[$Node]
if ($null -eq $nodePrincipalProperty -or [string]::IsNullOrWhiteSpace([string]$nodePrincipalProperty.Value)) {
    throw "cluster manifest has no node Principal for $Node"
}
$nodePrincipal = [string]$nodePrincipalProperty.Value
$operatorDevice = "/operator-identity/device.json"

function Invoke-Compose([string[]]$Arguments) {
    & docker @composePrefix @Arguments
    if ($LASTEXITCODE -ne 0) { throw "docker compose failed: $($Arguments -join ' ')" }
}

function Invoke-ArdJson([string[]]$Arguments) {
    $raw = & docker @composePrefix --profile tools run --rm --no-deps "$Node-operator" `
        --output json --signer-file $operatorDevice --principal $nodePrincipal @Arguments
    if ($LASTEXITCODE -ne 0) { throw "ardentsctl command failed for $Node" }
    return (($raw -join "`n") | ConvertFrom-Json)
}

function Wait-Condition([string]$Description, [scriptblock]$Condition) {
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    $lastError = ""
    while ([DateTime]::UtcNow -lt $deadline) {
        try { if (& $Condition) { return } } catch { $lastError = $_.Exception.Message }
        Start-Sleep -Seconds 7
    }
    throw "timed out waiting for $Description; $lastError"
}

function Wait-Node {
    Wait-Condition "$Node restored network participation" {
        $status = Invoke-ArdJson @("network", "status")
        if ($Node -eq "seed") { return $status.network.state -eq "ready" }
        return $status.network.joined -eq $true
    }
}

function Get-ContinuityRecord {
    $response = Invoke-ArdJson @("network", "records", "list")
    $record = $response.records |
        Where-Object { $null -ne $_.nodeFacts -and $_.sourceV1 -eq "local" } |
        Select-Object -First 1
    if ($null -eq $record) { throw "$Node has no local continuity record" }
    $endpoint = $record.nodeFacts.endpoints | Where-Object { $_ -match "/p2p/" } | Select-Object -First 1
    if ([string]::IsNullOrWhiteSpace($endpoint)) { throw "$Node has no Waku peer endpoint" }
    $peerID = ($endpoint -split "/p2p/", 2)[1]
    return [ordered]@{
        node_principal = [string]$record.nodeFacts.principal
        peer_id = [string]$peerID
    }
}

function Get-DataVolume {
    $container = (& docker @composePrefix ps -aq $Node).Trim()
    if ([string]::IsNullOrWhiteSpace($container)) { throw "$Node container is missing" }
    $inspect = ((& docker inspect $container) -join "`n") | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0) { throw "cannot inspect $Node container" }
    $mount = $inspect[0].Mounts | Where-Object { $_.Destination -eq "/var/lib/ardents" } | Select-Object -First 1
    $volume = [string]$mount.Name
    if ([string]::IsNullOrWhiteSpace($volume)) {
        throw "cannot resolve $Node persistent data volume"
    }
    if ($volume -notmatch "_$([regex]::Escape($Node))_data$") {
        throw "refusing unexpected data volume: $volume"
    }
    return $volume
}

function Invoke-Backup {
    $continuity = Get-ContinuityRecord
    $archivePath = if ([string]::IsNullOrWhiteSpace($Archive)) {
        Join-Path $statePath ("backups/{0}-{1}.tar.gz" -f $Node, [DateTime]::UtcNow.ToString("yyyyMMddTHHmmssZ"))
    } else { [IO.Path]::GetFullPath($Archive) }
    $archiveDir = [IO.Path]::GetDirectoryName($archivePath)
    [IO.Directory]::CreateDirectory($archiveDir) | Out-Null
    Invoke-Compose @("stop", $Node)
    try {
        $volume = Get-DataVolume
        & docker run --rm --mount "source=$volume,target=/data,readonly" --mount "type=bind,source=$archiveDir,target=/backup" --entrypoint /bin/tar $image -czf "/backup/$([IO.Path]::GetFileName($archivePath))" -C /data .
        if ($LASTEXITCODE -ne 0) { throw "backup archive creation failed" }
        & docker run --rm --mount "type=bind,source=$archiveDir,target=/backup,readonly" --entrypoint /bin/tar $image -tzf "/backup/$([IO.Path]::GetFileName($archivePath))" | Out-Null
        if (-not $?) { throw "backup archive verification failed" }
        $sidecar = [ordered]@{
            schema = "ardents.backup/v1"
            node = $Node
            project = $Project
            image = $image
            created_at = [DateTime]::UtcNow.ToString("O")
            consistency = $continuity
            archive_sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $archivePath).Hash.ToLowerInvariant()
        }
        $sidecar | ConvertTo-Json -Depth 6 | Set-Content -Encoding utf8 -LiteralPath "$archivePath.json"
    } finally {
        Invoke-Compose @("start", $Node)
        Wait-Node
    }
    Write-Host "Stopped-node backup accepted: $archivePath"
}

function Invoke-Restore {
    if (-not $ConfirmReplace) {
        throw "restore replaces the selected node volume; rerun with -ConfirmReplace"
    }
    if ([string]::IsNullOrWhiteSpace($Archive)) { throw "-Archive is required for restore" }
    $archivePath = [IO.Path]::GetFullPath($Archive)
    $sidecarPath = "$archivePath.json"
    if (-not (Test-Path -LiteralPath $archivePath) -or -not (Test-Path -LiteralPath $sidecarPath)) {
        throw "archive and its .json continuity manifest are required"
    }
    $backup = Get-Content -Raw -LiteralPath $sidecarPath | ConvertFrom-Json
    if ($backup.node -ne $Node) { throw "backup belongs to $($backup.node), not $Node" }
    $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $archivePath).Hash.ToLowerInvariant()
    if ($hash -ne $backup.archive_sha256) { throw "backup checksum mismatch" }
    $archiveDir = [IO.Path]::GetDirectoryName($archivePath)
    $archiveName = [IO.Path]::GetFileName($archivePath)
    Invoke-Compose @("stop", $Node)
    $volume = Get-DataVolume
    & docker run --rm --mount "source=$volume,target=/data" --mount "type=bind,source=$archiveDir,target=/backup,readonly" --entrypoint /bin/sh $image -ec 'find /data -mindepth 1 -maxdepth 1 -exec rm -rf -- {} +; tar -xzf "/backup/$1" -C /data' restore $archiveName
    if ($LASTEXITCODE -ne 0) { throw "restore extraction failed; node remains stopped" }
    Invoke-Compose @("start", $Node)
    Wait-Node
    $actual = Get-ContinuityRecord
    foreach ($field in @("node_principal", "peer_id")) {
        if ($actual[$field] -ne $backup.consistency.$field) {
            throw "restore continuity mismatch: $field"
        }
    }
    Write-Host "Restore accepted with matching Ardents and Waku identity: $Node"
}

if ($Action -eq "backup") { Invoke-Backup } else { Invoke-Restore }
