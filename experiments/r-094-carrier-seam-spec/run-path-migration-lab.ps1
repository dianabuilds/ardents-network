[CmdletBinding()]
param(
    [string]$ToolImage = 'ardents-carrier-tooling:s43-85ffe2d',
    [string]$ExpectedToolImageID = 'sha256:85074e6550c563477d7a1239bab07de3a18986472c08da97058f3264076c2e16'
)

$ErrorActionPreference = 'Stop'
$runID = 'r094-path-' + [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
$labelKey = 'io.ardents.r094.path'
$label = $labelKey + '=' + $runID
$networkA = 'ardents-' + $runID + '-a'
$networkB = 'ardents-' + $runID + '-b'
$createdContainers = [System.Collections.Generic.List[string]]::new()
$caseResults = [System.Collections.Generic.List[object]]::new()
$cleanupErrors = [System.Collections.Generic.List[string]]::new()
$repository = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '../..')).Path
$temporaryRoot = (Resolve-Path -LiteralPath $env:TEMP).Path
$binary = Join-Path $temporaryRoot ('ardents-' + $runID + '-linux')
$resumePath = '/tmp/ardents-r094-path-resume'
$payloadSHA256 = '88a27acc92907475c5f16c76b14633def7715e1a1f7672fd362564229c05ce96'
$actualToolImageID = ''
$binarySHA256 = ''
$environment = @()
$runError = $null

function Invoke-CheckedDocker {
    param([string[]]$Arguments)

    $output = @(& docker @Arguments 2>&1)
    if ($LASTEXITCODE -ne 0) {
        throw "docker $($Arguments -join ' ') failed:`n$($output -join [Environment]::NewLine)"
    }
    return $output
}

function Get-LabRecords {
    param([object[]]$Lines)

    $records = [System.Collections.Generic.List[object]]::new()
    foreach ($line in $Lines) {
        $text = [string]$line
        if (-not $text.TrimStart().StartsWith('{')) {
            continue
        }
        try {
            $record = $text | ConvertFrom-Json
        } catch {
            throw "invalid path-lab JSON record: $text"
        }
        if ($record.schema -eq 'ardents-r094-fault-case-v1') {
            $records.Add($record)
        }
    }
    return $records
}

function Wait-LabEvent {
    param([string]$Container, [string]$Role, [string]$Event)

    $logs = @()
    for ($attempt = 0; $attempt -lt 400; $attempt++) {
        $logs = @(& docker logs $Container 2>&1)
        $records = @(Get-LabRecords -Lines $logs |
            Where-Object { $_.role -eq $Role -and $_.event -eq $Event })
        if ($records.Count -eq 1) {
            return $records[0]
        }
        if ($records.Count -gt 1) {
            throw "$Container emitted duplicate $Role/$Event records"
        }
        Start-Sleep -Milliseconds 50
    }
    $state = @(& docker inspect --format '{{json .State}}' $Container 2>&1)
    throw "$Container did not report $Role/$Event; state=$($state -join ' '); logs=$($logs -join [Environment]::NewLine)"
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

function Get-NetworkHostAddress {
    param([string]$Network, [int]$HostOffset)

    $inspect = (Invoke-CheckedDocker -Arguments @('network', 'inspect', $Network) | ConvertFrom-Json)[0]
    $configs = @($inspect.IPAM.Config | Where-Object { [string]$_.Subnet -match '^[0-9.]+/[0-9]+$' })
    if ($configs.Count -ne 1) {
        throw "$Network does not expose one IPv4 subnet"
    }
    $parts = ([string]$configs[0].Subnet).Split('/')
    $bytes = [System.Net.IPAddress]::Parse($parts[0]).GetAddressBytes()
    $prefix = [int]$parts[1]
    if ($bytes.Count -ne 4 -or $prefix -lt 8 -or $prefix -gt 28) {
        throw "$Network has an unsupported path-lab subnet $($configs[0].Subnet)"
    }
    $value = ([uint64]$bytes[0] -shl 24) -bor ([uint64]$bytes[1] -shl 16) -bor
        ([uint64]$bytes[2] -shl 8) -bor [uint64]$bytes[3]
    $mask = (([uint64]4294967295 -shl (32 - $prefix)) -band [uint64]4294967295)
    $networkValue = $value -band $mask
    $hostCount = [uint64]1 -shl (32 - $prefix)
    if ($HostOffset -le 1 -or [uint64]$HostOffset -ge ($hostCount - 1)) {
        throw "$HostOffset is outside the usable subnet of $Network"
    }
    $candidate = $networkValue + [uint64]$HostOffset
    $candidateBytes = [byte[]]@(
        [byte](($candidate -shr 24) -band 0xff),
        [byte](($candidate -shr 16) -band 0xff),
        [byte](($candidate -shr 8) -band 0xff),
        [byte]($candidate -band 0xff)
    )
    return ([System.Net.IPAddress]::new($candidateBytes)).ToString()
}

function Get-InterfaceForAddress {
    param([string]$Container, [string]$Address)

    $interfaces = [System.Collections.Generic.List[string]]::new()
    $lines = Invoke-CheckedDocker -Arguments @(
        'exec', '--user', '0:0', $Container, 'ip', '-o', '-4', 'addr', 'show', 'scope', 'global'
    )
    foreach ($line in $lines) {
        if ([string]$line -match '^\d+:\s+([^@\s]+)(?:@[^\s]+)?\s+inet\s+([0-9.]+)/') {
            if ($Matches[2] -eq $Address) {
                $interfaces.Add($Matches[1])
            }
        }
    }
    if ($interfaces.Count -ne 1) {
        throw "$Container maps $Address to $($interfaces.Count) interfaces"
    }
    return $interfaces[0]
}

function Get-Route {
    param([string]$Container, [string]$Destination)

    return ((Invoke-CheckedDocker -Arguments @(
        'exec', '--user', '0:0', $Container, 'ip', '-o', 'route', 'get', $Destination
    ) | ForEach-Object { [string]$_ }) -join ' ').Trim()
}

function Assert-RouteUses {
    param(
        [string]$Route,
        [string]$Device,
        [string]$Source,
        [string]$Via = ''
    )

    if ($Route -notmatch ('\bdev\s+' + [regex]::Escape($Device) + '\b') -or
        $Route -notmatch ('\bsrc\s+' + [regex]::Escape($Source) + '\b')) {
        throw "route does not use expected device/source: $Route"
    }
    if ($Via -ne '' -and $Route -notmatch ('\bvia\s+' + [regex]::Escape($Via) + '\b')) {
        throw "route does not use expected next hop ${Via}: $Route"
    }
}

function Initialize-IngressCounter {
    param(
        [string]$Server,
        [string]$Device,
        [string]$ClientB,
        [string]$ServerA
    )

    [void](Invoke-CheckedDocker -Arguments @(
        'exec', '--user', '0:0', $Server,
        'tc', 'qdisc', 'add', 'dev', $Device, 'clsact'
    ))
    [void](Invoke-CheckedDocker -Arguments @(
        'exec', '--user', '0:0', $Server,
        'tc', 'filter', 'add', 'dev', $Device, 'ingress', 'protocol', 'ip', 'pref', '1', 'u32',
        'match', 'ip', 'src', ($ClientB + '/32'),
        'match', 'ip', 'dst', ($ServerA + '/32'),
        'match', 'ip', 'protocol', '17', '0xff',
        'match', 'ip', 'dport', '19443', '0xffff',
        'action', 'pass'
    ))
}

function Get-IngressCounter {
    param([string]$Server, [string]$Device)

    $output = Invoke-CheckedDocker -Arguments @(
        'exec', '--user', '0:0', $Server,
        'tc', '-s', 'filter', 'show', 'dev', $Device, 'ingress', 'pref', '1'
    )
    $joined = ($output | ForEach-Object { [string]$_ }) -join [Environment]::NewLine
    $matches = [regex]::Matches($joined, 'Sent\s+(\d+)\s+bytes\s+(\d+)\s+pkt')
    if ($matches.Count -eq 0) {
        throw "tc did not expose an ingress action counter:`n$joined"
    }
    $packets = 0L
    $bytes = 0L
    foreach ($match in $matches) {
        $packets = [Math]::Max($packets, [int64]$match.Groups[2].Value)
        $bytes = [Math]::Max($bytes, [int64]$match.Groups[1].Value)
    }
    return [pscustomobject]@{ Packets = $packets; Bytes = $bytes }
}

function Assert-ContainerProfile {
    param([string]$Container)

    $inspect = (Invoke-CheckedDocker -Arguments @('inspect', $Container) | ConvertFrom-Json)[0]
    $hostConfig = $inspect.HostConfig
    $capAdd = @($hostConfig.CapAdd | ForEach-Object { ([string]$_).ToUpperInvariant() })
    $capDrop = @($hostConfig.CapDrop | ForEach-Object { ([string]$_).ToUpperInvariant() })
    $security = @($hostConfig.SecurityOpt | ForEach-Object { [string]$_ })
    $tmpfs = $hostConfig.Tmpfs.PSObject.Properties['/tmp']
    if ($inspect.Config.User -ne '65534:65534' -or
        -not [bool]$hostConfig.ReadonlyRootfs -or
        [int64]$hostConfig.Memory -ne 134217728 -or
        [int64]$hostConfig.NanoCpus -ne 500000000 -or
        [int64]$hostConfig.PidsLimit -ne 64 -or
        $capAdd.Count -ne 1 -or $capAdd[0] -ne 'CAP_NET_ADMIN' -or
        $capDrop -notcontains 'ALL' -or
        -not ($security | Where-Object { $_ -match '^no-new-privileges' }) -or
        $null -eq $tmpfs) {
        throw "$Container does not match the declared read-only/resource/user/capability profile"
    }
}

function Complete-LabContainer {
    param([string]$Container, [string]$Role)

    $exitCode = ([string](Invoke-CheckedDocker -Arguments @('wait', $Container))).Trim()
    $logs = @(& docker logs $Container 2>&1)
    $logs | ForEach-Object { Write-Host $_ }
    $records = @(Get-LabRecords -Lines $logs |
        Where-Object { $_.role -eq $Role -and $_.event -eq 'outcome' })
    if ($exitCode -ne '0') {
        throw "$Container exited $exitCode"
    }
    if ($records.Count -ne 1 -or -not [bool]$records[0].passed) {
        throw "$Container failed its $Role outcome oracle"
    }
    return $records[0]
}

function Get-RemoteIP {
    param([string]$Remote)

    if ($Remote -notmatch '^([0-9.]+):[0-9]+$') {
        throw "non-IPv4 remote address in path observation: $Remote"
    }
    return $Matches[1]
}

function Remove-OwnedContainer {
    param([string]$Container)

    $existing = @(& docker ps -aq --filter ('name=^/' + $Container + '$') 2>$null)
    if ($existing.Count -eq 0) {
        return
    }
    $containerLabel = ([string](& docker inspect -f '{{index .Config.Labels "io.ardents.r094.path"}}' $Container 2>$null)).Trim()
    if ($containerLabel -ne $runID) {
        throw "refusing to remove unowned container $Container"
    }
    [void](Invoke-CheckedDocker -Arguments @('rm', '-f', $Container))
}

function Remove-OwnedNetwork {
    param([string]$Network)

    $existing = @(& docker network ls -q --filter ('name=^' + $Network + '$') 2>$null)
    if ($existing.Count -eq 0) {
        return
    }
    $networkLabel = ([string](& docker network inspect -f '{{index .Labels "io.ardents.r094.path"}}' $Network 2>$null)).Trim()
    if ($networkLabel -ne $runID) {
        throw "refusing to remove unowned network $Network"
    }
    [void](Invoke-CheckedDocker -Arguments @('network', 'rm', $Network))
}

function Invoke-PathCase {
    param([string]$Case, [bool]$ChangePath)

    $deadline = [DateTimeOffset]::UtcNow.AddSeconds(30).ToUnixTimeSeconds()
    $expected = if ($ChangePath) { 'success-or-timeout' } else { 'success' }
    $server = 'ardents-' + $runID + '-' + $Case + '-server'
    $client = 'ardents-' + $runID + '-' + $Case + '-client'
    $createdContainers.Add($server)
    $createdContainers.Add($client)

    $serverA = Get-NetworkHostAddress -Network $networkA -HostOffset 10
    $clientA = Get-NetworkHostAddress -Network $networkA -HostOffset 11
    $serverB = Get-NetworkHostAddress -Network $networkB -HostOffset 10
    $clientB = Get-NetworkHostAddress -Network $networkB -HostOffset 11
    $commonServer = @(
        'create', '--name', $server, '--label', $label, '--network', $networkA, '--ip', $serverA,
        '--user', '65534:65534', '--read-only', '--memory', '128m', '--cpus', '0.50', '--pids-limit', '64',
        '--cap-drop', 'ALL', '--cap-add', 'NET_ADMIN', '--security-opt', 'no-new-privileges',
        '--tmpfs', '/tmp:rw,noexec,nosuid,nodev,size=1048576,uid=65534,gid=65534,mode=0700',
        '--sysctl', 'net.ipv4.conf.all.rp_filter=0', '--sysctl', 'net.ipv4.conf.default.rp_filter=0',
        '-v', "${binary}:/lab/r094:ro", '--entrypoint', '/lab/r094', $ToolImage,
        'fault-server', '-profile', 'r094-quic-v1', '-endpoint', ($serverA + ':19443'),
        '-case', $Case, '-token', $Case, '-deadline-unix', [string]$deadline,
        '-expect', $expected, '-hold-after-outcome=true'
    )
    [void](Invoke-CheckedDocker -Arguments $commonServer)
    [void](Invoke-CheckedDocker -Arguments @('network', 'connect', '--ip', $serverB, $networkB, $server))

    $commonClient = @(
        'create', '--name', $client, '--label', $label, '--network', $networkA, '--ip', $clientA,
        '--user', '65534:65534', '--read-only', '--memory', '128m', '--cpus', '0.50', '--pids-limit', '64',
        '--cap-drop', 'ALL', '--cap-add', 'NET_ADMIN', '--security-opt', 'no-new-privileges',
        '--tmpfs', '/tmp:rw,noexec,nosuid,nodev,size=1048576,uid=65534,gid=65534,mode=0700',
        '--sysctl', 'net.ipv4.conf.all.rp_filter=0', '--sysctl', 'net.ipv4.conf.default.rp_filter=0',
        '-v', "${binary}:/lab/r094:ro", '--entrypoint', '/lab/r094', $ToolImage,
        'fault-client', '-profile', 'r094-quic-v1', '-endpoint', ($serverA + ':19443'),
        '-case', $Case, '-token', $Case, '-deadline-unix', [string]$deadline,
        '-expect', $expected, '-resume-file', $resumePath
    )
    [void](Invoke-CheckedDocker -Arguments $commonClient)
    [void](Invoke-CheckedDocker -Arguments @('network', 'connect', '--ip', $clientB, $networkB, $client))

    if ($serverA -eq $serverB -or $clientA -eq $clientB) {
        throw "$Case did not receive distinct A/B addresses"
    }
    Assert-ContainerProfile -Container $server
    Assert-ContainerProfile -Container $client

    [void](Invoke-CheckedDocker -Arguments @('start', $server))
    $ready = Wait-LabEvent -Container $server -Role 'server' -Event 'ready'
    if ($ready.endpoint -ne ($serverA + ':19443')) {
        throw "$Case server bound $($ready.endpoint), expected ${serverA}:19443"
    }
    $serverADevice = Get-InterfaceForAddress -Container $server -Address $serverA
    $serverBDevice = Get-InterfaceForAddress -Container $server -Address $serverB
    if ($serverADevice -eq $serverBDevice) {
        throw "$Case server A/B addresses share one interface"
    }

    Initialize-IngressCounter -Server $server -Device $serverBDevice -ClientB $clientB -ServerA $serverA
    $counterBefore = Get-IngressCounter -Server $server -Device $serverBDevice
    if ($counterBefore.Packets -ne 0 -or $counterBefore.Bytes -ne 0) {
        throw "$Case server-B ingress counter was nonzero before the client started"
    }

    [void](Invoke-CheckedDocker -Arguments @('start', $client))
    $opened = Wait-LabEvent -Container $client -Role 'client' -Event 'opened'
    if ($opened.adapter_calls -ne 'tcp=0,quic=1' -or $opened.endpoint -ne ($serverA + ':19443')) {
        throw "$Case changed its Adapter count or endpoint before the route operation"
    }
    $clientADevice = Get-InterfaceForAddress -Container $client -Address $clientA
    $clientBDevice = Get-InterfaceForAddress -Container $client -Address $clientB
    if ($clientADevice -eq $clientBDevice) {
        throw "$Case client A/B addresses share one interface"
    }
    $routeBefore = Get-Route -Container $client -Destination $serverA
    Assert-RouteUses -Route $routeBefore -Device $clientADevice -Source $clientA
    $counterAfterOpen = Get-IngressCounter -Server $server -Device $serverBDevice
    if ($counterAfterOpen.Packets -ne 0 -or $counterAfterOpen.Bytes -ne 0) {
        throw "$Case used server-B before the declared post-Open route operation"
    }

    if ($ChangePath) {
        [void](Invoke-CheckedDocker -Arguments @(
            'exec', '--user', '0:0', $client,
            'ip', 'route', 'replace', ($serverA + '/32'), 'via', $serverB,
            'dev', $clientBDevice, 'src', $clientB
        ))
        $routeAfter = Get-Route -Container $client -Destination $serverA
        Assert-RouteUses -Route $routeAfter -Device $clientBDevice -Source $clientB -Via $serverB
    } else {
        $routeAfter = Get-Route -Container $client -Destination $serverA
        Assert-RouteUses -Route $routeAfter -Device $clientADevice -Source $clientA
    }

    [void](Invoke-CheckedDocker -Arguments @(
        'exec', '--user', '65534:65534', $client, '/usr/bin/touch', $resumePath
    ))
    [void](Wait-LabEvent -Container $client -Role 'client' -Event 'outcome')
    [void](Wait-LabEvent -Container $server -Role 'server' -Event 'outcome')
    $counterAfter = Get-IngressCounter -Server $server -Device $serverBDevice

    $clientResult = Complete-LabContainer -Container $client -Role 'client'
    $serverResult = Complete-LabContainer -Container $server -Role 'server'
    if ($clientResult.adapter_calls -ne 'tcp=0,quic=1' -or
        $clientResult.offer_digest -ne $serverResult.offer_digest -or
        -not [bool]$clientResult.cleanup_joined -or -not [bool]$serverResult.cleanup_joined) {
        throw "$Case failed its single-Adapter, offer, or joined-cleanup oracle"
    }
    if ((Get-RemoteIP -Remote $serverResult.remote_start) -ne $clientA) {
        throw "$Case did not start from client-A"
    }

    if ($ChangePath) {
        if ($counterAfter.Packets -le 0 -or $counterAfter.Bytes -le 0) {
            throw "$Case did not prove post-change client-B/server-B traffic; " +
                "client=$($clientResult.observed), server=$($serverResult.observed), " +
                "route=$routeAfter, remote=$($serverResult.remote_start)->$($serverResult.remote_end), " +
                "counter=$($counterAfter.Bytes)/$($counterAfter.Packets)"
        }
        if ($clientResult.observed -eq 'success') {
            if ($clientResult.payload_bytes -ne 262144 -or $clientResult.payload_sha256 -ne $payloadSHA256 -or
                $serverResult.observed -ne 'success' -or
                (Get-RemoteIP -Remote $serverResult.remote_end) -ne $clientB) {
                throw "$Case reported success without exact transcript and remote-IP migration"
            }
        } elseif ($clientResult.observed -ne 'timeout') {
            throw "$Case returned an unapproved migration result $($clientResult.observed)"
        }
    } else {
        if ($counterAfter.Packets -ne 0 -or $counterAfter.Bytes -ne 0 -or
            $clientResult.observed -ne 'success' -or $serverResult.observed -ne 'success' -or
            $clientResult.payload_bytes -ne 262144 -or $clientResult.payload_sha256 -ne $payloadSHA256 -or
            (Get-RemoteIP -Remote $serverResult.remote_end) -ne $clientA) {
            throw "$Case failed the unchanged-path control oracle"
        }
    }

    $caseResults.Add([pscustomobject]@{
        case = $Case
        path_changed = $ChangePath
        expected = $expected
        client_observed = $clientResult.observed
        server_observed = $serverResult.observed
        client_a = $clientA
        client_b = $clientB
        server_a = $serverA
        server_b = $serverB
        client_a_interface = $clientADevice
        client_b_interface = $clientBDevice
        server_a_interface = $serverADevice
        server_b_interface = $serverBDevice
        route_before = $routeBefore
        route_after = $routeAfter
        server_b_packets_before = $counterAfterOpen.Packets
        server_b_packets_after = $counterAfter.Packets
        server_b_bytes_after = $counterAfter.Bytes
        remote_start = $serverResult.remote_start
        remote_end = $serverResult.remote_end
        adapter_calls = $clientResult.adapter_calls
        payload_bytes = $clientResult.payload_bytes
        payload_sha256 = $clientResult.payload_sha256
        client_fds = ([string]$clientResult.fds_start + '->' + [string]$clientResult.fds_end)
        server_fds = ([string]$serverResult.fds_start + '->' + [string]$serverResult.fds_end)
        client_goroutines = ([string]$clientResult.goroutines_start + '->' + [string]$clientResult.goroutines_end)
        server_goroutines = ([string]$serverResult.goroutines_start + '->' + [string]$serverResult.goroutines_end)
    })

    Remove-OwnedContainer -Container $client
    Remove-OwnedContainer -Container $server
}

try {
    try {
        $actualToolImageID = ([string](Invoke-CheckedDocker -Arguments @(
            'image', 'inspect', $ToolImage, '--format', '{{.Id}}'
        ))).Trim()
        if ($actualToolImageID -ne $ExpectedToolImageID) {
            throw "tool image mismatch: expected $ExpectedToolImageID, observed $actualToolImageID"
        }

        Push-Location $repository
        try {
            $previousGOOS, $previousGOARCH, $previousCGO = $env:GOOS, $env:GOARCH, $env:CGO_ENABLED
            $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = 'linux', 'amd64', '0'
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
                throw 'path-migration binary build failed'
            }
        } finally {
            $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = $previousGOOS, $previousGOARCH, $previousCGO
            Pop-Location
        }
        $binarySHA256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $binary).Hash.ToLowerInvariant()

        [void](Invoke-CheckedDocker -Arguments @('network', 'create', '--internal', '--label', $label, $networkA))
        [void](Invoke-CheckedDocker -Arguments @('network', 'create', '--internal', '--label', $label, $networkB))
        $environment = @(Invoke-CheckedDocker -Arguments @(
            'run', '--rm', '--network', 'none', '--label', $label,
            '--entrypoint', '/bin/sh', $ToolImage, '-lc', 'uname -srmo; ip -Version; tc -Version'
        ))
        $environment | ForEach-Object { Write-Output $_ }

        Invoke-PathCase -Case 'dual-homed-baseline' -ChangePath $false
        Invoke-PathCase -Case 'source-interface-migration' -ChangePath $true
        if ($caseResults.Count -ne 2) {
            throw "incomplete path matrix: cases=$($caseResults.Count)"
        }
    } catch {
        $runError = $_.Exception
    }
} finally {
    foreach ($container in $createdContainers) {
        try {
            Remove-OwnedContainer -Container $container
        } catch {
            $cleanupErrors.Add($_.Exception.Message)
        }
    }
    foreach ($network in @($networkA, $networkB)) {
        try {
            Remove-OwnedNetwork -Network $network
        } catch {
            $cleanupErrors.Add($_.Exception.Message)
        }
    }
    try {
        if (([System.IO.Path]::GetDirectoryName($binary) -ne $temporaryRoot)) {
            throw "refusing to remove binary outside the declared temporary root: $binary"
        }
        if (Test-Path -LiteralPath $binary) {
            Remove-Item -LiteralPath $binary -Force
        }
    } catch {
        $cleanupErrors.Add($_.Exception.Message)
    }
}

$remainingContainers = @(& docker ps -aq --filter ('label=' + $label) 2>$null)
$remainingNetworks = @(& docker network ls -q --filter ('label=' + $label) 2>$null)
if ($remainingContainers.Count -ne 0 -or $remainingNetworks.Count -ne 0 -or (Test-Path -LiteralPath $binary)) {
    $cleanupErrors.Add("residue remains: containers=$($remainingContainers.Count), networks=$($remainingNetworks.Count), binary=$(Test-Path -LiteralPath $binary)")
}
if ($cleanupErrors.Count -ne 0) {
    $original = if ($null -ne $runError) { '; original=' + $runError.Message } else { '' }
    throw "path-lab cleanup failed: $($cleanupErrors -join ' | ')$original"
}
if ($null -ne $runError) {
    throw $runError
}

[pscustomobject]@{
    schema = 'ardents-r094-path-migration-lab-v1'
    passed = $true
    cases = @($caseResults)
    binary_sha256 = $binarySHA256
    tool_image = $actualToolImageID
    environment = @($environment)
    cleanup_joined = $true
} | ConvertTo-Json -Depth 6 -Compress
