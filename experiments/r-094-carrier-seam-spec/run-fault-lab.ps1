[CmdletBinding()]
param(
    [string]$ToolImage = 'ardents-carrier-tooling:s43-85ffe2d',
    [string]$ExpectedToolImageID = 'sha256:85074e6550c563477d7a1239bab07de3a18986472c08da97058f3264076c2e16'
)

$ErrorActionPreference = 'Stop'
$runID = 'r094-' + [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
$label = 'io.ardents.r094.run=' + $runID
$network = 'ardents-' + $runID
$createdServers = [System.Collections.Generic.List[string]]::new()
$clientResults = [System.Collections.Generic.List[object]]::new()
$serverResults = [System.Collections.Generic.List[object]]::new()
$repository = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '../..')).Path
$temporaryRoot = (Resolve-Path -LiteralPath $env:TEMP).Path
$binary = Join-Path $temporaryRoot ('ardents-' + $runID + '-fault-linux')

function Invoke-CheckedDocker {
    param([string[]]$Arguments)

    $output = @(& docker @Arguments 2>&1)
    if ($LASTEXITCODE -ne 0) {
        throw "docker $($Arguments -join ' ') failed:`n$($output -join [Environment]::NewLine)"
    }
    return $output
}

function Get-JSONRecords {
    param([object[]]$Lines)

    $records = [System.Collections.Generic.List[object]]::new()
    foreach ($line in $Lines) {
        $text = [string]$line
        if ($text.TrimStart().StartsWith('{')) {
            try {
                $record = $text | ConvertFrom-Json
                if ($record.schema -eq 'ardents-r094-fault-case-v1') {
                    $records.Add($record)
                }
            } catch {
                throw "invalid fault-lab JSON record: $text"
            }
        }
    }
    return $records
}

function Assert-NonzeroDrops {
    param([object[]]$Lines, [string]$Case)

    $joined = ($Lines | ForEach-Object { [string]$_ }) -join [Environment]::NewLine
    if ($joined -notmatch 'dropped ([1-9][0-9]*)') {
        throw "$Case did not observe a nonzero tc drop counter"
    }
}

function New-Deadline {
    param([int]$AfterSeconds)

    return [DateTimeOffset]::UtcNow.AddSeconds($AfterSeconds).ToUnixTimeSeconds()
}

function Start-FaultServer {
    param(
        [string]$Profile,
        [string]$Case,
        [string]$Token,
        [long]$Deadline,
        [string]$Expected,
        [string]$Setup = '',
        [switch]$CaptureTC
    )

    $name = 'ardents-' + $runID + '-' + $Case
    $command = ''
    if ($Setup -ne '') {
        $command += $Setup + '; '
    }
    $command += "/lab/r094 fault-server -profile $Profile -endpoint 0.0.0.0:19443 -case $Case " +
        "-token $Token -deadline-unix $Deadline -expect $Expected"
    if ($CaptureTC) {
        $command += '; status=$?; tc -s qdisc show dev eth0; exit $status'
    }

    $arguments = @(
        'run', '-d', '--name', $name, '--label', $label, '--network', $network,
        '--user', '0:0', '-v', "${binary}:/lab/r094:ro"
    )
    if ($Setup -ne '') {
        $arguments += @('--cap-add', 'NET_ADMIN')
    }
    $arguments += @('--entrypoint', '/bin/sh', $ToolImage, '-lc', $command)
    [void](Invoke-CheckedDocker -Arguments $arguments)
    $createdServers.Add($name)

    $ready = $false
    for ($attempt = 0; $attempt -lt 100 -and -not $ready; $attempt++) {
        $logs = @(& docker logs $name 2>&1)
        $ready = (($logs | ForEach-Object { [string]$_ }) -join "`n") -match '"event":"ready"'
        if (-not $ready) {
            Start-Sleep -Milliseconds 50
        }
    }
    if (-not $ready) {
        throw "$name did not report readiness"
    }
    $address = ([string](Invoke-CheckedDocker -Arguments @(
        'inspect', '-f', '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}', $name
    ))).Trim()
    return [pscustomobject]@{ Name = $name; Address = $address; Case = $Case; CaptureTC = [bool]$CaptureTC }
}

function Invoke-FaultClient {
    param(
        [string]$Name,
        [string]$Commands,
        [int]$ExpectedOutcomes = 1,
        [switch]$NeedsNetworkAdmin,
        [switch]$RequireDrops
    )

    $arguments = @(
        'run', '--rm', '--name', ('ardents-' + $runID + '-' + $Name), '--label', $label,
        '--network', $network, '--user', '0:0', '-v', "${binary}:/lab/r094:ro"
    )
    if ($NeedsNetworkAdmin) {
        $arguments += @('--cap-add', 'NET_ADMIN')
    }
    $arguments += @('--entrypoint', '/bin/sh', $ToolImage, '-lc', $Commands)
    $output = Invoke-CheckedDocker -Arguments $arguments
    $output | ForEach-Object { Write-Output $_ }
    $records = @(Get-JSONRecords -Lines $output | Where-Object { $_.role -eq 'client' -and $_.event -eq 'outcome' })
    if ($records.Count -ne $ExpectedOutcomes) {
        throw "$Name returned $($records.Count) client outcomes, expected $ExpectedOutcomes"
    }
    foreach ($record in $records) {
        if (-not [bool]$record.passed) {
            throw "$($record.case) failed its client oracle"
        }
        $clientResults.Add($record)
    }
    if ($RequireDrops) {
        Assert-NonzeroDrops -Lines $output -Case $Name
    }
}

function Complete-FaultServer {
    param([object]$Server, [switch]$RequireDrops)

    $exitCode = ([string](Invoke-CheckedDocker -Arguments @('wait', $Server.Name))).Trim()
    $logs = @(& docker logs $Server.Name 2>&1)
    $logs | ForEach-Object { Write-Output $_ }
    if ($exitCode -ne '0') {
        throw "$($Server.Name) exited $exitCode"
    }
    $records = @(Get-JSONRecords -Lines $logs | Where-Object { $_.role -eq 'server' -and $_.event -eq 'outcome' })
    if ($records.Count -ne 1 -or -not [bool]$records[0].passed) {
        throw "$($Server.Case) failed its server oracle"
    }
    $serverResults.Add($records[0])
    if ($RequireDrops) {
        Assert-NonzeroDrops -Lines $logs -Case $Server.Case
    }
}

function Invoke-SuccessPair {
    param(
        [string]$Profile,
        [string]$Case,
        [string]$Setup = '',
        [string]$Expected = 'success',
        [switch]$CaptureTC,
        [switch]$RequireDrops
    )

    $deadline = New-Deadline -AfterSeconds 15
    $server = Start-FaultServer -Profile $Profile -Case $Case -Token $Case -Deadline $deadline `
        -Expected $Expected -Setup $Setup -CaptureTC:$CaptureTC
    $command = ''
    if ($Setup -ne '') {
        $command += $Setup + '; '
    }
    $command += "/lab/r094 fault-client -profile $Profile -endpoint $($server.Address):19443 " +
        "-case $Case -token $Case -deadline-unix $deadline -expect $Expected"
    if ($CaptureTC) {
        $command += '; status=$?; tc -s qdisc show dev eth0; exit $status'
    }
    Invoke-FaultClient -Name ($Case + '-client') -Commands $command -NeedsNetworkAdmin:($Setup -ne '') `
        -RequireDrops:$RequireDrops
    Complete-FaultServer -Server $server -RequireDrops:$RequireDrops
}

function Invoke-SelectiveRefusal {
    param(
        [string]$BlockedProfile,
        [string]$ControlProfile,
        [int]$IPProtocol,
        [string]$Prefix,
        [int]$Seed
    )

    $blockedCase = $Prefix + '-blocked'
    $controlCase = $Prefix + '-control'
    $blockedDeadline = New-Deadline -AfterSeconds 6
    $controlDeadline = New-Deadline -AfterSeconds 15
    $blocked = Start-FaultServer -Profile $BlockedProfile -Case $blockedCase -Token $blockedCase `
        -Deadline $blockedDeadline -Expected 'timeout'
    $control = Start-FaultServer -Profile $ControlProfile -Case $controlCase -Token $controlCase `
        -Deadline $controlDeadline -Expected 'success'
    $setup = 'tc qdisc add dev eth0 root handle 1: prio bands 3; ' +
        "tc qdisc add dev eth0 parent 1:3 handle 30: netem loss 100% seed $Seed; " +
        "tc filter add dev eth0 protocol ip parent 1:0 prio 3 u32 match ip protocol $IPProtocol 0xff flowid 1:3"
    $commands = $setup + '; ' +
        "/lab/r094 fault-client -profile $BlockedProfile -endpoint $($blocked.Address):19443 " +
        "-case $blockedCase -token $blockedCase -deadline-unix $blockedDeadline -expect timeout; " +
        "/lab/r094 fault-client -profile $ControlProfile -endpoint $($control.Address):19443 " +
        "-case $controlCase -token $controlCase -deadline-unix $controlDeadline -expect success; " +
        'tc -s qdisc show dev eth0; tc filter show dev eth0 parent 1:0'
    Invoke-FaultClient -Name ($Prefix + '-client') -Commands $commands -ExpectedOutcomes 2 `
        -NeedsNetworkAdmin -RequireDrops
    Complete-FaultServer -Server $blocked
    Complete-FaultServer -Server $control
}

try {
    $actualToolImageID = ([string](Invoke-CheckedDocker -Arguments @('image', 'inspect', $ToolImage, '--format', '{{.Id}}'))).Trim()
    if ($actualToolImageID -ne $ExpectedToolImageID) {
        throw "tool image mismatch: expected $ExpectedToolImageID, observed $actualToolImageID"
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
            throw 'fault-lab binary build failed'
        }
    } finally {
        $env:GOOS, $env:GOARCH = $previousGOOS, $previousGOARCH
        Pop-Location
    }

    $binarySHA256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $binary).Hash.ToLowerInvariant()
    [void](Invoke-CheckedDocker -Arguments @('network', 'create', '--label', $label, $network))

    $environment = Invoke-CheckedDocker -Arguments @(
        'run', '--rm', '--network', 'none', '--label', $label, '--entrypoint', '/bin/sh', $ToolImage,
        '-lc', 'uname -srmo; if test -r /proc/sys/net/core/rmem_max; then printf "rmem_max="; cat /proc/sys/net/core/rmem_max; else echo "rmem_max=unavailable"; fi; if test -r /proc/sys/net/core/wmem_max; then printf "wmem_max="; cat /proc/sys/net/core/wmem_max; else echo "wmem_max=unavailable"; fi; tc -Version'
    )
    $environment | ForEach-Object { Write-Output $_ }

    Invoke-SuccessPair -Profile 'r094-tcp-tls-v1' -Case 'baseline-tcp'
    Invoke-SuccessPair -Profile 'r094-quic-v1' -Case 'baseline-quic'
    Invoke-SelectiveRefusal -BlockedProfile 'r094-quic-v1' -ControlProfile 'r094-tcp-tls-v1' `
        -IPProtocol 17 -Prefix 'udp-refusal' -Seed 94017
    Invoke-SelectiveRefusal -BlockedProfile 'r094-tcp-tls-v1' -ControlProfile 'r094-quic-v1' `
        -IPProtocol 6 -Prefix 'tcp-refusal' -Seed 94006

    $netem = 'tc qdisc add dev eth0 root netem limit 256 delay 20ms 5ms 25% distribution normal loss random 5% seed 94095 reorder 10% 25%'
    Invoke-SuccessPair -Profile 'r094-tcp-tls-v1' -Case 'loss-reorder-tcp' -Setup $netem `
        -Expected 'success-or-timeout' -CaptureTC -RequireDrops
    Invoke-SuccessPair -Profile 'r094-quic-v1' -Case 'loss-reorder-quic' -Setup $netem `
        -Expected 'success-or-timeout' -CaptureTC -RequireDrops

    $mtu = 'ip link set dev eth0 mtu 1280; ip -o link show dev eth0'
    Invoke-SuccessPair -Profile 'r094-tcp-tls-v1' -Case 'mtu1280-tcp' -Setup $mtu
    Invoke-SuccessPair -Profile 'r094-quic-v1' -Case 'mtu1280-quic' -Setup $mtu

    if ($clientResults.Count -ne 10 -or $serverResults.Count -ne 10) {
        throw "incomplete fault matrix: client=$($clientResults.Count), server=$($serverResults.Count)"
    }
    $observed = @($clientResults | Group-Object -Property observed | Sort-Object -Property Name |
        ForEach-Object { $_.Name + '=' + $_.Count }) -join ','
    [pscustomobject]@{
        schema = 'ardents-r094-fault-lab-v1'
        passed = $true
        client_cases = $clientResults.Count
        server_cases = $serverResults.Count
        client_outcomes = $observed
        binary_sha256 = $binarySHA256
        tool_image = $actualToolImageID
    } | ConvertTo-Json -Compress
} finally {
    foreach ($server in $createdServers) {
        $existing = @(& docker ps -aq --filter ('name=^/' + $server + '$') 2>$null)
        if ($existing.Count -ne 0) {
            $serverLabel = ([string](& docker inspect -f '{{index .Config.Labels "io.ardents.r094.run"}}' $server 2>$null)).Trim()
            if ($serverLabel -eq $runID) {
                & docker rm -f $server 1>$null
            }
        }
    }
    $networkID = @(& docker network ls -q --filter ('name=^' + $network + '$') 2>$null)
    if ($networkID.Count -ne 0) {
        $networkLabel = ([string](& docker network inspect -f '{{index .Labels "io.ardents.r094.run"}}' $network 2>$null)).Trim()
        if ($networkLabel -eq $runID) {
            & docker network rm $network 1>$null
        }
    }
    if (([System.IO.Path]::GetDirectoryName($binary) -eq $temporaryRoot) -and (Test-Path -LiteralPath $binary)) {
        Remove-Item -LiteralPath $binary -Force
    }
}
