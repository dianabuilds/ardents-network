param(
    [string]$ReportDir = "tests/.artifacts/reports/stb-307-multihost",
    [ValidateSet("Never", "IfMissing", "Always")]
    [string]$BuildMode = "IfMissing",
    [ValidateRange(1, 4)]
    [int]$BuildParallelism = 1,
    [switch]$Keep
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root
$reportPath = [IO.Path]::GetFullPath($ReportDir)
$project = "ardents-stb307-$PID"
$secretPath = Join-Path ([IO.Path]::GetTempPath()) "$project-secrets"
New-Item -ItemType Directory -Force -Path $reportPath | Out-Null
New-Item -ItemType Directory -Force -Path $secretPath | Out-Null
$composeFile = Join-Path $root "docker/docker-compose.testnet.yml"
$dockerfile = Join-Path $root "docker/ardd.Dockerfile"
$composePrefix = @("compose", "-p", $project, "-f", $composeFile)
$services = @("seed", "bridge", "a1", "a2", "b1", "b2", "recovery")
$results = [Collections.Generic.List[object]]::new()
$snapshots = [ordered]@{}

function Invoke-Compose([string[]]$CommandArgs) {
    & docker @composePrefix @CommandArgs
    if ($LASTEXITCODE -ne 0) { throw "docker compose failed: $($CommandArgs -join ' ')" }
}

function Test-ImageExists([string]$Image) {
    $previousPreference = $ErrorActionPreference
    $ErrorActionPreference = "SilentlyContinue"
    & docker image inspect $Image 2>$null | Out-Null
    $exitCode = $LASTEXITCODE
    $ErrorActionPreference = $previousPreference
    return $exitCode -eq 0
}

function Ensure-TestnetImage {
    $image = if ([string]::IsNullOrWhiteSpace($env:ARDENTS_TESTNET_IMAGE)) { "ardents/ardd-testnet:dev" } else { $env:ARDENTS_TESTNET_IMAGE }
    $exists = Test-ImageExists $image
    $shouldBuild = $BuildMode -eq "Always" -or ($BuildMode -eq "IfMissing" -and -not $exists)
    if ($shouldBuild) {
        $previousGitInfo = $env:BUILDX_GIT_INFO
        try {
            $env:BUILDX_GIT_INFO = "false"
            & docker build --build-arg "GO_BUILD_PARALLELISM=$BuildParallelism" -t $image -f $dockerfile $root
            if ($LASTEXITCODE -ne 0) { throw "testnet image build failed: $image" }
        } finally {
            if ($null -eq $previousGitInfo) { Remove-Item Env:BUILDX_GIT_INFO -ErrorAction SilentlyContinue }
            else { $env:BUILDX_GIT_INFO = $previousGitInfo }
        }
        return
    }
    if (-not $exists) {
        throw "testnet image is missing: $image; build it separately or rerun with -BuildMode IfMissing"
    }
}

function Write-PrivateText([string]$Path, [string]$Value) {
    [IO.File]::WriteAllText($Path, $Value + "`n", [Text.UTF8Encoding]::new($false))
}

function Remove-TestSecrets {
    $resolved = [IO.Path]::GetFullPath($secretPath)
    $tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
    if (-not $resolved.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase) -or
        [IO.Path]::GetFileName($resolved) -ne "$project-secrets") {
        throw "refusing to remove unexpected test secret path: $resolved"
    }
    if (Test-Path -LiteralPath $resolved) {
        Remove-Item -LiteralPath $resolved -Recurse -Force
    }
}

function Invoke-OpenSSL([string[]]$OpenSSLArgs) {
    $previousPreference = $ErrorActionPreference
    $ErrorActionPreference = "SilentlyContinue"
    & openssl @OpenSSLArgs 2>$null | Out-Null
    $exitCode = $LASTEXITCODE
    $ErrorActionPreference = $previousPreference
    if ($exitCode -ne 0) { throw "openssl failed: $($OpenSSLArgs[0])" }
}

function New-TestCertificate([string]$Name, [string[]]$Names) {
    $key = Join-Path $secretPath "$Name.key"
    $csr = Join-Path $secretPath "$Name.csr"
    $cert = Join-Path $secretPath "$Name.crt"
    $ext = Join-Path $secretPath "$Name.ext"
    $san = ($Names | ForEach-Object { "DNS:$_" }) -join ","
    Write-PrivateText $ext "subjectAltName=$san`nextendedKeyUsage=serverAuth"
    Invoke-OpenSSL @("req", "-newkey", "rsa:2048", "-nodes", "-keyout", $key, "-out", $csr, "-subj", "/CN=$Name")
    Invoke-OpenSSL @("x509", "-req", "-in", $csr, "-CA", $env:ARDENTS_WSS_CA_FILE, "-CAkey", (Join-Path $secretPath "ca.key"), "-CAcreateserial", "-out", $cert, "-days", "2", "-sha256", "-extfile", $ext)
    return @{ Cert = $cert; Key = $key }
}

function Invoke-ArdJson([string]$Service, [string[]]$CommandArgs) {
    $raw = & docker @composePrefix exec -T $Service ard --token-file /run/secrets/ardents-api-token --output json @CommandArgs
    if ($LASTEXITCODE -ne 0) { throw "ard command failed for $Service" }
    return (($raw -join "`n") | ConvertFrom-Json)
}

function Invoke-StoreProbe([string]$Service, [string[]]$ProbeArgs) {
    & docker @composePrefix exec -T $Service ard-store-probe @ProbeArgs
    if ($LASTEXITCODE -ne 0) { throw "Store probe failed for $Service" }
}

function Wait-Condition([string]$Description, [scriptblock]$Condition, [int]$Seconds = 45) {
    $deadline = [DateTime]::UtcNow.AddSeconds($Seconds)
    $last = ""
    while ([DateTime]::UtcNow -lt $deadline) {
        try {
            if (& $Condition) { return }
        } catch { $last = $_.Exception.Message }
        Start-Sleep -Milliseconds 500
    }
    throw "timed out waiting for $Description; $last"
}

function Wait-API([string]$Service) {
    Wait-Condition "$Service API" { $null = Invoke-ArdJson $Service @("node", "status"); return $true } 60
}

function Wait-Joined([string]$Service, [int]$Seconds = 45) {
    Wait-Condition "$Service joined/ready" {
        $status = Invoke-ArdJson $Service @("network", "status")
        return $status.network.joined -and $status.network.state -eq "ready"
    } $Seconds
}

function Get-NodeRecord([string]$Service) {
    $response = Invoke-ArdJson $Service @("network", "records", "list")
    $record = $response.records | Where-Object { $_.kind -eq "node" -and $_.endpoints.Count -gt 0 } | Select-Object -First 1
    if ($null -eq $record) { throw "no published node record for $Service" }
    return $record
}

function Select-Endpoint([object]$Record, [string]$Pattern) {
    $endpoint = $Record.endpoints | Where-Object { $_ -match $Pattern } | Select-Object -First 1
    if ([string]::IsNullOrWhiteSpace($endpoint)) { throw "endpoint matching $Pattern was not published" }
    return $endpoint
}

function Capture-Snapshot([string]$Label) {
    $items = [ordered]@{}
    foreach ($service in $services) {
        try {
            $items[$service] = [ordered]@{
                network = Invoke-ArdJson $service @("network", "status")
                peers = Invoke-ArdJson $service @("network", "peers")
                records = Invoke-ArdJson $service @("network", "records", "list")
            }
        } catch { $items[$service] = @{ error = $_.Exception.Message } }
    }
    $snapshots[$Label] = $items
}

function Run-Step([string]$Name, [scriptblock]$Body) {
    $started = [DateTime]::UtcNow
    try {
        & $Body
        $results.Add([ordered]@{ name = $Name; status = "passed"; started_at = $started; ended_at = [DateTime]::UtcNow })
    } catch {
        $results.Add([ordered]@{ name = $Name; status = "failed"; error = $_.Exception.Message; started_at = $started; ended_at = [DateTime]::UtcNow })
        throw
    }
}

foreach ($service in $services) {
    $path = Join-Path $secretPath "$service.token"
    Write-PrivateText $path ([Guid]::NewGuid().ToString("N"))
    Set-Item "Env:ARDENTS_$($service.ToUpper())_TOKEN_FILE" $path
}
$env:ARDENTS_SEED_TOKEN_FILE = Join-Path $secretPath "seed.token"
$env:ARDENTS_BRIDGE_TOKEN_FILE = Join-Path $secretPath "bridge.token"
$env:ARDENTS_A1_TOKEN_FILE = Join-Path $secretPath "a1.token"
$env:ARDENTS_A2_TOKEN_FILE = Join-Path $secretPath "a2.token"
$env:ARDENTS_B1_TOKEN_FILE = Join-Path $secretPath "b1.token"
$env:ARDENTS_B2_TOKEN_FILE = Join-Path $secretPath "b2.token"
$env:ARDENTS_RECOVERY_TOKEN_FILE = Join-Path $secretPath "recovery.token"
$env:ARDENTS_WSS_CA_FILE = Join-Path $secretPath "ca.crt"
Invoke-OpenSSL @("req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", (Join-Path $secretPath "ca.key"), "-out", $env:ARDENTS_WSS_CA_FILE, "-days", "2", "-sha256", "-subj", "/CN=Ardents STB-307 Test CA")
$bridgeCert = New-TestCertificate "bridge" @("bridge", "localhost")
$b2Cert = New-TestCertificate "b2" @("b2", "localhost")
$env:ARDENTS_BRIDGE_CERT_FILE = $bridgeCert.Cert
$env:ARDENTS_BRIDGE_KEY_FILE = $bridgeCert.Key
$env:ARDENTS_B2_CERT_FILE = $b2Cert.Cert
$env:ARDENTS_B2_KEY_FILE = $b2Cert.Key
$env:ARDENTS_SEED_ZONE_A = "pending"
$env:ARDENTS_ZONE_A_BOOTSTRAP = "pending"
$env:ARDENTS_ZONE_B_BOOTSTRAP = "pending"
$env:ARDENTS_BRIDGE_WSS = "pending"
$env:ARDENTS_A2_IP = "172.31.10.12"

try {
    Run-Step "verify testnet image" {
        Ensure-TestnetImage
    }
    Run-Step "start seed" {
        Invoke-Compose @("up", "-d", "seed")
        Wait-API "seed"
        $seed = Get-NodeRecord "seed"
        $env:ARDENTS_SEED_ZONE_A = Select-Endpoint $seed "/ip4/172\.31\.10\.10/tcp/61001/"
        $script:seedZoneB = Select-Endpoint $seed "/ip4/172\.31\.20\.10/tcp/61001/"
    }
    Run-Step "start dual-homed bridge" {
        Invoke-Compose @("up", "-d", "bridge")
        Wait-Joined "bridge"
        $bridge = Get-NodeRecord "bridge"
        $bridgeA = Select-Endpoint $bridge "/ip4/172\.31\.10\.20/tcp/61002/"
        $bridgeB = Select-Endpoint $bridge "/ip4/172\.31\.20\.20/tcp/61002/"
        $script:bridgeZoneA = $bridgeA
        $script:bridgeZoneB = $bridgeB
        $env:ARDENTS_BRIDGE_WSS = Select-Endpoint $bridge "/dns4/bridge/tcp/61443/.*/p2p/"
        $env:ARDENTS_ZONE_A_BOOTSTRAP = "$($env:ARDENTS_SEED_ZONE_A),$bridgeA"
        $env:ARDENTS_ZONE_B_BOOTSTRAP = "$script:seedZoneB,$bridgeB"
    }
    Run-Step "start two segments and WSS-only client path" {
        Invoke-Compose @("up", "-d", "a1", "a2", "b1", "b2", "recovery")
        foreach ($service in @("a1", "a2", "b1", "b2", "recovery")) { Wait-Joined $service 60 }
        Capture-Snapshot "initial"
    }
    Run-Step "kill node process and recover" {
        Invoke-Compose @("kill", "-s", "SIGKILL", "a1")
        Invoke-Compose @("up", "-d", "a1")
        Wait-Joined "a1" 60
    }
    Run-Step "change address through controlled recreation" {
        $before = Get-NodeRecord "a2"
        $env:ARDENTS_A2_IP = "172.31.10.14"
        Invoke-Compose @("up", "-d", "--force-recreate", "a2")
        Wait-Joined "a2" 60
        $after = Get-NodeRecord "a2"
        if (($before.endpoints -join ",") -eq ($after.endpoints -join ",")) { throw "a2 endpoint did not change" }
        if (-not (($after.endpoints -join ",") -match "172.31.10.14")) { throw "a2 did not publish the new address" }
    }
    Run-Step "lose one bootstrap without losing healthy participation" {
        Invoke-Compose @("stop", "seed")
        foreach ($service in @("bridge", "a1", "a2", "b1", "b2", "recovery")) { Wait-Joined $service 45 }
        Capture-Snapshot "bootstrap-loss"
    }
    Run-Step "partition zone B while preserving local participation" {
        $bridgeID = (& docker @composePrefix ps -q bridge).Trim()
        & docker network disconnect "${project}_zone_b" $bridgeID
        if ($LASTEXITCODE -ne 0) { throw "failed to disconnect zone B" }
        foreach ($service in @("b1", "b2")) { Wait-Joined $service 45 }
        foreach ($service in @("b1", "b2")) {
            $status = Invoke-ArdJson $service @("network", "status")
            if ($status.network.activeMode -ne "steady") {
                throw "$service left steady mode despite healthy local segment participation"
            }
        }
        Capture-Snapshot "segment-partitioned"
    }
    Run-Step "isolate one node and enter restricted defense" {
        $b1ID = (& docker @composePrefix ps -q b1).Trim()
        & docker network disconnect "${project}_zone_b" $b1ID
        if ($LASTEXITCODE -ne 0) { throw "failed to isolate b1" }
        Wait-Condition "zone B restricted defense" {
            $status = Invoke-ArdJson "b1" @("network", "status")
            return $status.network.activeMode -eq "restricted_defense"
        } 45
        Capture-Snapshot "peer-collapsed"
    }
    Run-Step "rejoin partition and recover steady provider shape" {
        $bridgeID = (& docker @composePrefix ps -q bridge).Trim()
        & docker network connect --alias bridge --ip 172.31.20.20 "${project}_zone_b" $bridgeID
        if ($LASTEXITCODE -ne 0) { throw "failed to reconnect zone B" }
        $b1ID = (& docker @composePrefix ps -q b1).Trim()
        & docker network connect --alias b1 --ip 172.31.20.11 "${project}_zone_b" $b1ID
        if ($LASTEXITCODE -ne 0) { throw "failed to reconnect b1" }
        foreach ($service in @("b1", "b2")) { Wait-Joined $service 75 }
        Wait-Condition "zone B steady mode" {
            $status = Invoke-ArdJson "b1" @("network", "status")
            return $status.network.activeMode -eq "steady"
        } 75
        Capture-Snapshot "recovered"
    }
    Run-Step "peer churn remains recoverable" {
        foreach ($iteration in 1..3) {
            Invoke-Compose @("restart", "b1")
            Wait-Joined "b1" 60
            Wait-Joined "b2" 60
        }
    }
    Run-Step "fresh constrained client recovers retained opaque history from Store" {
        $storeTopic = [Guid]::NewGuid().ToString("N")
        $storePayload = [Guid]::NewGuid().ToString("N")
        Invoke-Compose @("stop", "recovery")
        Invoke-StoreProbe "b1" @("-action", "publish", "-provider", $script:bridgeZoneB, "-topic", $storeTopic, "-payload", $storePayload)
        Invoke-Compose @("start", "recovery")
        Wait-Joined "recovery" 60
        Invoke-StoreProbe "recovery" @("-action", "fetch", "-provider", $script:bridgeZoneA, "-topic", $storeTopic, "-payload", $storePayload)
    }
    Run-Step "retain resource and topology evidence" {
        Capture-Snapshot "final"
        $ids = & docker @composePrefix ps -q
        $stats = & docker stats --no-stream --format "{{json .}}" $ids
        [IO.File]::WriteAllLines((Join-Path $reportPath "docker-stats.jsonl"), $stats)
        $versions = [ordered]@{
            docker = (& docker version --format "{{.Server.Version}}")
            compose = (& docker compose version --short)
            image_build = (& docker @composePrefix exec -T bridge ardd --version)
            project = $project
        }
        $versions | ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path $reportPath "versions.json")
    }
} finally {
    $summary = [ordered]@{
        scenario_id = "NFM-001"
        generated_at = [DateTime]::UtcNow
        project = $project
        topology = @{ nodes = $services; networks = @("zone_a", "zone_b") }
        results = $results
        passed = @($results | Where-Object status -eq "passed").Count
        failed = @($results | Where-Object status -eq "failed").Count
    }
    $summary | ConvertTo-Json -Depth 20 | Set-Content -Encoding utf8 (Join-Path $reportPath "summary.json")
    $snapshots | ConvertTo-Json -Depth 30 | Set-Content -Encoding utf8 (Join-Path $reportPath "snapshots.json")
    try { & docker @composePrefix logs --no-color 2>&1 | Set-Content -Encoding utf8 (Join-Path $reportPath "compose.log") } catch {}
    if (-not $Keep) {
        try { Invoke-Compose @("down", "-v", "--remove-orphans") } catch {}
        finally { Remove-TestSecrets }
    } else {
        Write-Warning "test secrets retained for kept project at $secretPath"
    }
}

if (@($results | Where-Object status -eq "failed").Count -gt 0) { exit 1 }
Write-Host "STB-307 multi-host scenario passed; report: $reportPath"
