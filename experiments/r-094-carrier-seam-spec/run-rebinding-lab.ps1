[CmdletBinding()]
param(
    [string]$BaseImage = 'ubuntu:24.04',
    [string]$ExpectedBaseImageID = 'sha256:33ceb71981b602c1a7443a53469e4dba065f7503eab3078a2d7a57a2ab987517'
)

$ErrorActionPreference = 'Stop'
$runID = 'r094-rebind-' + [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
$label = 'io.ardents.r094.rebind=' + $runID
$clientNetwork = 'ardents-' + $runID + '-client'
$serverNetwork = 'ardents-' + $runID + '-server'
$createdContainers = [System.Collections.Generic.List[string]]::new()
$clientResults = [System.Collections.Generic.List[object]]::new()
$serverResults = [System.Collections.Generic.List[object]]::new()
$relayResults = [System.Collections.Generic.List[object]]::new()
$repository = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '../..')).Path
$temporaryRoot = (Resolve-Path -LiteralPath $env:TEMP).Path
$binary = Join-Path $temporaryRoot ('ardents-' + $runID + '-linux')

function Invoke-CheckedDocker {
    param([string[]]$Arguments)

    $output = @(& docker @Arguments 2>&1)
    if ($LASTEXITCODE -ne 0) {
        throw "docker $($Arguments -join ' ') failed:`n$($output -join [Environment]::NewLine)"
    }
    return $output
}

function Get-LabRecords {
    param([object[]]$Lines, [string]$Schema)

    $records = [System.Collections.Generic.List[object]]::new()
    foreach ($line in $Lines) {
        $text = [string]$line
        if (-not $text.TrimStart().StartsWith('{')) {
            continue
        }
        $record = $text | ConvertFrom-Json
        if ($record.schema -eq $Schema) {
            $records.Add($record)
        }
    }
    return $records
}

function Wait-LabReady {
    param([string]$Container, [string]$Schema)

    $logs = @()
    for ($attempt = 0; $attempt -lt 100; $attempt++) {
        $logs = @(& docker logs $Container 2>&1)
        $records = @(Get-LabRecords -Lines $logs -Schema $Schema |
            Where-Object { $_.event -eq 'ready' })
        if ($records.Count -eq 1) {
            return
        }
        Start-Sleep -Milliseconds 50
    }
    $state = @(& docker inspect --format '{{json .State}}' $Container 2>&1)
    throw "$Container did not report readiness; state=$($state -join ' '); logs=$($logs -join [Environment]::NewLine)"
}

function Get-NetworkAddress {
    param([string]$Container, [string]$Network)

    $inspect = (Invoke-CheckedDocker -Arguments @('inspect', $Container) | ConvertFrom-Json)[0]
    $property = $inspect.NetworkSettings.Networks.PSObject.Properties[$Network]
    if ($null -eq $property -or [string]::IsNullOrWhiteSpace($property.Value.IPAddress)) {
        throw "$Container has no address on $Network"
    }
    return [string]$property.Value.IPAddress
}

function Complete-LabContainer {
    param([string]$Container, [string]$Schema, [string]$Role)

    $exitCode = ([string](Invoke-CheckedDocker -Arguments @('wait', $Container))).Trim()
    $logs = @(& docker logs $Container 2>&1)
    $logs | ForEach-Object { Write-Output $_ }
    if ($exitCode -ne '0') {
        throw "$Container exited $exitCode"
    }
    $records = @(Get-LabRecords -Lines $logs -Schema $Schema |
        Where-Object { $_.event -eq 'outcome' })
    if ($records.Count -ne 1 -or -not [bool]$records[0].passed) {
        throw "$Container failed its outcome oracle"
    }
    switch ($Role) {
        'server' { $serverResults.Add($records[0]) }
        'relay' { $relayResults.Add($records[0]) }
        default { throw "unknown lab role $Role" }
    }
}

function Remove-OwnedContainer {
    param([string]$Container)

    $existing = @(& docker ps -aq --filter ('name=^/' + $Container + '$') 2>$null)
    if ($existing.Count -eq 0) {
        return
    }
    $containerLabel = ([string](& docker inspect -f '{{index .Config.Labels "io.ardents.r094.rebind"}}' $Container 2>$null)).Trim()
    if ($containerLabel -ne $runID) {
        throw "refusing to remove unowned container $Container"
    }
    & docker rm -f $Container 1>$null
}

function Invoke-RebindingCase {
    param([string]$Case, [bool]$ExpectRebind)

    $deadline = [DateTimeOffset]::UtcNow.AddSeconds(30).ToUnixTimeSeconds()
    $server = 'ardents-' + $runID + '-' + $Case + '-server'
    $relay = 'ardents-' + $runID + '-' + $Case + '-relay'
    $client = 'ardents-' + $runID + '-' + $Case + '-client'
    $createdContainers.Add($server)
    $createdContainers.Add($relay)
    $createdContainers.Add($client)

    [void](Invoke-CheckedDocker -Arguments @(
        'run', '-d', '--name', $server, '--label', $label, '--network', $serverNetwork,
        '--user', '65534:65534', '-v', "${binary}:/lab/r094:ro", '--entrypoint', '/lab/r094', $BaseImage,
        'fault-server', '-profile', 'r094-quic-v1', '-endpoint', '0.0.0.0:19443',
        '-case', $Case, '-token', $Case, '-deadline-unix', [string]$deadline, '-expect', 'success'
    ))
    Wait-LabReady -Container $server -Schema 'ardents-r094-fault-case-v1'
    $serverAddress = Get-NetworkAddress -Container $server -Network $serverNetwork

    $relayArguments = @(
        'create', '--name', $relay, '--label', $label, '--network', $clientNetwork,
        '--user', '65534:65534', '-v', "${binary}:/lab/r094:ro", '--entrypoint', '/lab/r094', $BaseImage,
        'fault-relay', '-listen', '0.0.0.0:19443', '-control', '0.0.0.0:19444',
        '-upstream', ($serverAddress + ':19443'), '-case', $Case, '-token', $Case,
        '-deadline-unix', [string]$deadline,
        ('-expect-rebind=' + $ExpectRebind.ToString().ToLowerInvariant())
    )
    [void](Invoke-CheckedDocker -Arguments $relayArguments)
    [void](Invoke-CheckedDocker -Arguments @('network', 'connect', $serverNetwork, $relay))
    [void](Invoke-CheckedDocker -Arguments @('start', $relay))
    Wait-LabReady -Container $relay -Schema 'ardents-r094-rebind-relay-v1'
    $relayAddress = Get-NetworkAddress -Container $relay -Network $clientNetwork

    $clientArguments = @(
        'run', '--rm', '--name', $client, '--label', $label, '--network', $clientNetwork,
        '--user', '65534:65534', '-v', "${binary}:/lab/r094:ro", '--entrypoint', '/lab/r094', $BaseImage,
        'fault-client', '-profile', 'r094-quic-v1', '-endpoint', ($relayAddress + ':19443'),
        '-case', $Case, '-token', $Case, '-deadline-unix', [string]$deadline, '-expect', 'success',
        '-relay-control', ($relayAddress + ':19444')
    )
    if ($ExpectRebind) {
        $clientArguments += '-request-rebind=true'
    }
    $clientOutput = Invoke-CheckedDocker -Arguments $clientArguments
    $clientOutput | ForEach-Object { Write-Output $_ }
    $clientRecords = @(Get-LabRecords -Lines $clientOutput -Schema 'ardents-r094-fault-case-v1' |
        Where-Object { $_.role -eq 'client' -and $_.event -eq 'outcome' })
    if ($clientRecords.Count -ne 1 -or -not [bool]$clientRecords[0].passed -or
        $clientRecords[0].adapter_calls -ne 'tcp=0,quic=1') {
        throw "$Case failed its client or single-Adapter oracle"
    }
    $clientResults.Add($clientRecords[0])

    Complete-LabContainer -Container $server -Schema 'ardents-r094-fault-case-v1' -Role 'server'
    Complete-LabContainer -Container $relay -Schema 'ardents-r094-rebind-relay-v1' -Role 'relay'

    $relayRecord = $relayResults[$relayResults.Count - 1]
    if ($ExpectRebind -and ($relayRecord.old_upstream_local -eq $relayRecord.new_upstream_local -or
        [int64]$relayRecord.packets_after -le 0)) {
        throw "$Case did not prove an upstream tuple change with post-change traffic"
    }
    Remove-OwnedContainer -Container $server
    Remove-OwnedContainer -Container $relay
}

try {
    $actualBaseImageID = ([string](Invoke-CheckedDocker -Arguments @(
        'image', 'inspect', $BaseImage, '--format', '{{.Id}}'
    ))).Trim()
    if ($actualBaseImageID -ne $ExpectedBaseImageID) {
        throw "base image mismatch: expected $ExpectedBaseImageID, observed $actualBaseImageID"
    }

    Push-Location $repository
    try {
        $previousGOOS, $previousGOARCH = $env:GOOS, $env:GOARCH
        $env:GOOS, $env:GOARCH = 'linux', 'amd64'
        $sources = @(
            'experiments/r-094-carrier-seam-spec/fault_main.go',
            'experiments/r-094-carrier-seam-spec/fault_control.go',
            'experiments/r-094-carrier-seam-spec/fault_path.go',
            'experiments/r-094-carrier-seam-spec/fault_relay.go',
            'experiments/r-094-carrier-seam-spec/fault_server.go',
            'experiments/r-094-carrier-seam-spec/fault_transcript.go',
            'experiments/r-094-carrier-seam-spec/contract.go',
            'experiments/r-094-carrier-seam-spec/identity.go',
            'experiments/r-094-carrier-seam-spec/peer.go',
            'experiments/r-094-carrier-seam-spec/quic_adapter.go',
            'experiments/r-094-carrier-seam-spec/tcp_adapter.go'
        )
        & go build -trimpath -o $binary @sources
        if ($LASTEXITCODE -ne 0) {
            throw 'rebinding binary build failed'
        }
    } finally {
        $env:GOOS, $env:GOARCH = $previousGOOS, $previousGOARCH
        Pop-Location
    }
    $binarySHA256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $binary).Hash.ToLowerInvariant()

    [void](Invoke-CheckedDocker -Arguments @('network', 'create', '--label', $label, $clientNetwork))
    [void](Invoke-CheckedDocker -Arguments @('network', 'create', '--label', $label, $serverNetwork))

    Invoke-RebindingCase -Case 'relay-baseline' -ExpectRebind $false
    Invoke-RebindingCase -Case 'nat-port-rebinding' -ExpectRebind $true

    if ($clientResults.Count -ne 2 -or $serverResults.Count -ne 2 -or $relayResults.Count -ne 2) {
        throw "incomplete rebinding matrix: client=$($clientResults.Count), server=$($serverResults.Count), relay=$($relayResults.Count)"
    }
    $changed = $relayResults | Where-Object { [bool]$_.rebound } | Select-Object -First 1
    [pscustomobject]@{
        schema = 'ardents-r094-rebinding-lab-v1'
        passed = $true
        client_cases = $clientResults.Count
        server_cases = $serverResults.Count
        relay_cases = $relayResults.Count
        old_upstream_local = $changed.old_upstream_local
        new_upstream_local = $changed.new_upstream_local
        packets_before = $changed.packets_before
        packets_after = $changed.packets_after
        binary_sha256 = $binarySHA256
        base_image = $actualBaseImageID
    } | ConvertTo-Json -Compress
} finally {
    foreach ($container in $createdContainers) {
        Remove-OwnedContainer -Container $container
    }
    foreach ($network in @($clientNetwork, $serverNetwork)) {
        $networkID = @(& docker network ls -q --filter ('name=^' + $network + '$') 2>$null)
        if ($networkID.Count -eq 0) {
            continue
        }
        $networkLabel = ([string](& docker network inspect -f '{{index .Labels "io.ardents.r094.rebind"}}' $network 2>$null)).Trim()
        if ($networkLabel -eq $runID) {
            & docker network rm $network 1>$null
        }
    }
    if (([System.IO.Path]::GetDirectoryName($binary) -eq $temporaryRoot) -and (Test-Path -LiteralPath $binary)) {
        Remove-Item -LiteralPath $binary -Force
    }
}
