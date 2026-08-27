param([switch]$Race)

$ErrorActionPreference = 'Stop'

$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..\..')).Path
$labRoot = Join-Path ([System.IO.Path]::GetTempPath()) 'ardents-r092-rendezvous-local'
$binary = Join-Path $labRoot 'r092-rendezvous.exe'
$processes = [System.Collections.Generic.List[System.Diagnostics.Process]]::new()

if (Test-Path -LiteralPath $labRoot) {
    throw "Refusing to reuse existing lab root: $labRoot"
}
New-Item -ItemType Directory -Path $labRoot | Out-Null

function Start-LoggedProcess {
    param([string]$Name, [string[]]$Arguments)
    $stdout = Join-Path $labRoot "$Name.stdout"
    $stderr = Join-Path $labRoot "$Name.stderr"
    $process = Start-Process -FilePath $binary -ArgumentList $Arguments -PassThru -WindowStyle Hidden `
        -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    $processes.Add($process)
    return [pscustomobject]@{ Name = $Name; Process = $process; Stdout = $stdout; Stderr = $stderr }
}

function Wait-LoggedProcess {
    param($Run, [int]$Milliseconds)
    if (-not $Run.Process.WaitForExit($Milliseconds)) {
        throw "$($Run.Name) did not exit within $Milliseconds ms"
    }
    return $Run.Process.ExitCode
}

function Wait-LogText {
    param([string]$Path, [string]$Text, [int]$Milliseconds = 5000)
    $deadline = (Get-Date).AddMilliseconds($Milliseconds)
    while ((Get-Date) -lt $deadline) {
        if ((Test-Path -LiteralPath $Path) -and
            (Select-String -LiteralPath $Path -SimpleMatch $Text -Quiet)) {
            return
        }
        Start-Sleep -Milliseconds 50
    }
    throw "Timed out waiting for '$Text' in $Path"
}

function Last-JsonRecord {
    param([string]$Path, [string]$Schema)
    $records = @(Get-Content -LiteralPath $Path | ForEach-Object { $_ | ConvertFrom-Json })
    $selected = @($records | Where-Object schema -eq $Schema)
    if ($selected.Count -ne 1) {
        throw "Expected one $Schema record in $Path, found $($selected.Count)"
    }
    return $selected[0]
}

function Assert-FinalCleanup {
    param($Result)
    if (-not $Result.passed -or -not $Result.cleanup_joined -or
        $Result.final_handshakes -ne 0 -or $Result.final_waiting_legs -ne 0 -or
        $Result.final_active_pairs -ne 0 -or $Result.final_connections -ne 0) {
        throw "Server cleanup oracle failed: $($Result | ConvertTo-Json -Compress)"
    }
}

function New-Deadline { return [DateTimeOffset]::UtcNow.ToUnixTimeSeconds() + 15 }

try {
    Push-Location $repoRoot
    try {
        $sources = @(
            'experiments/r-092-rendezvous-tracer/main.go',
            'experiments/r-092-rendezvous-tracer/identity.go',
            'experiments/r-092-rendezvous-tracer/binding.go',
            'experiments/r-092-rendezvous-tracer/transcript.go',
            'experiments/r-092-rendezvous-tracer/client.go',
            'experiments/r-092-rendezvous-tracer/server.go'
        )
        $buildArguments = @('build', '-trimpath')
        if ($Race) { $buildArguments += '-race' }
        & go @buildArguments -o $binary @sources
        if ($LASTEXITCODE -ne 0) { throw 'R-092 local tracer build failed' }
    } finally {
        Pop-Location
    }

    # Exact pair.
    $deadline = New-Deadline
    $server = Start-LoggedProcess exact-server @('server', '-listen', '127.0.0.1:47921',
        '-deadline-unix', $deadline, '-handshakes', '2', '-waiting', '2', '-pairs', '1', '-expect-pairs', '1')
    Wait-LogText $server.Stdout '"event":"ready"'
    $initiator = Start-LoggedProcess exact-initiator @('client', '-endpoint', '127.0.0.1:47921',
        '-side', 'initiator', '-token', 'exact-pair', '-deadline-unix', $deadline)
    $responder = Start-LoggedProcess exact-responder @('client', '-endpoint', '127.0.0.1:47921',
        '-side', 'responder', '-token', 'exact-pair', '-deadline-unix', $deadline)
    if ((Wait-LoggedProcess $initiator 10000) -ne 0 -or (Wait-LoggedProcess $responder 10000) -ne 0 -or
        (Wait-LoggedProcess $server 10000) -ne 0) { throw 'Exact-pair process failed' }
    $exactServer = Last-JsonRecord $server.Stdout 'ardents-r092-rendezvous-server-v1'
    $exactInitiator = Last-JsonRecord $initiator.Stdout 'ardents-r092-rendezvous-client-v1'
    $exactResponder = Last-JsonRecord $responder.Stdout 'ardents-r092-rendezvous-client-v1'
    Assert-FinalCleanup $exactServer
    if ($exactServer.successful_pairs -ne 1 -or $exactInitiator.payload_bytes -ne 262144 -or
        $exactResponder.payload_bytes -ne 262144 -or
        $exactInitiator.payload_sha256 -ne $exactResponder.payload_sha256) {
        throw 'Exact-pair transcript oracle failed'
    }
    [pscustomobject]@{ case = 'exact-pair'; passed = $true; pairs = 1; digest = $exactInitiator.payload_sha256 } |
        ConvertTo-Json -Compress

    # Duplicate side retains the first waiting leg until expiry.
    $deadline = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds() + 3
    $server = Start-LoggedProcess duplicate-server @('server', '-listen', '127.0.0.1:47922',
        '-deadline-unix', $deadline, '-handshakes', '2', '-waiting', '2', '-pairs', '1', '-expect-pairs', '0')
    Wait-LogText $server.Stdout '"event":"ready"'
    $first = Start-LoggedProcess duplicate-first @('client', '-endpoint', '127.0.0.1:47922',
        '-side', 'initiator', '-token', 'duplicate-side', '-deadline-unix', $deadline)
    Start-Sleep -Milliseconds 150
    $second = Start-LoggedProcess duplicate-second @('client', '-endpoint', '127.0.0.1:47922',
        '-side', 'initiator', '-token', 'duplicate-side', '-deadline-unix', $deadline)
    if ((Wait-LoggedProcess $first 8000) -eq 0 -or (Wait-LoggedProcess $second 8000) -eq 0 -or
        (Wait-LoggedProcess $server 8000) -ne 0) { throw 'Duplicate-side exit oracle failed' }
    $duplicate = Last-JsonRecord $server.Stdout 'ardents-r092-rendezvous-server-v1'
    Assert-FinalCleanup $duplicate
    if ($duplicate.duplicate_side_rejected -ne 1 -or $duplicate.unmatched_expired -ne 1 -or
        $duplicate.successful_pairs -ne 0) { throw 'Duplicate-side state oracle failed' }
    [pscustomobject]@{ case = 'duplicate-side'; passed = $true; rejected = 1; retained_then_expired = 1 } |
        ConvertTo-Json -Compress

    # One unmatched leg expires and releases its reservation.
    $deadline = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds() + 2
    $server = Start-LoggedProcess unmatched-server @('server', '-listen', '127.0.0.1:47923',
        '-deadline-unix', $deadline, '-handshakes', '2', '-waiting', '2', '-pairs', '1', '-expect-pairs', '0')
    Wait-LogText $server.Stdout '"event":"ready"'
    $single = Start-LoggedProcess unmatched-client @('client', '-endpoint', '127.0.0.1:47923',
        '-side', 'initiator', '-token', 'unmatched', '-deadline-unix', $deadline)
    if ((Wait-LoggedProcess $single 7000) -eq 0 -or (Wait-LoggedProcess $server 7000) -ne 0) {
        throw 'Unmatched exit oracle failed'
    }
    $unmatched = Last-JsonRecord $server.Stdout 'ardents-r092-rendezvous-server-v1'
    Assert-FinalCleanup $unmatched
    if ($unmatched.unmatched_expired -ne 1 -or $unmatched.peak_waiting_legs -ne 1) {
        throw 'Unmatched reservation oracle failed'
    }
    [pscustomobject]@{ case = 'unmatched-timeout'; passed = $true; expired = 1 } | ConvertTo-Json -Compress

    # Pair saturation rejects new sockets before TLS while preserving the active pair.
    $deadline = New-Deadline
    $server = Start-LoggedProcess full-server @('server', '-listen', '127.0.0.1:47924',
        '-deadline-unix', $deadline, '-handshakes', '2', '-waiting', '2', '-pairs', '1', '-expect-pairs', '1')
    Wait-LogText $server.Stdout '"event":"ready"'
    $heldInitiator = Start-LoggedProcess full-held-initiator @('client', '-endpoint', '127.0.0.1:47924',
        '-side', 'initiator', '-token', 'held-pair', '-deadline-unix', $deadline, '-hold-ms', '2000')
    $heldResponder = Start-LoggedProcess full-held-responder @('client', '-endpoint', '127.0.0.1:47924',
        '-side', 'responder', '-token', 'held-pair', '-deadline-unix', $deadline, '-hold-ms', '2000')
    Wait-LogText $server.Stdout '"event":"pair-active"'
    $rejectedInitiator = Start-LoggedProcess full-rejected-initiator @('client', '-endpoint', '127.0.0.1:47924',
        '-side', 'initiator', '-token', 'rejected-pair', '-deadline-unix', $deadline)
    $rejectedResponder = Start-LoggedProcess full-rejected-responder @('client', '-endpoint', '127.0.0.1:47924',
        '-side', 'responder', '-token', 'rejected-pair', '-deadline-unix', $deadline)
    if ((Wait-LoggedProcess $rejectedInitiator 5000) -eq 0 -or (Wait-LoggedProcess $rejectedResponder 5000) -eq 0 -or
        (Wait-LoggedProcess $heldInitiator 10000) -ne 0 -or (Wait-LoggedProcess $heldResponder 10000) -ne 0 -or
        (Wait-LoggedProcess $server 10000) -ne 0) { throw 'Pair-full exit oracle failed' }
    $full = Last-JsonRecord $server.Stdout 'ardents-r092-rendezvous-server-v1'
    Assert-FinalCleanup $full
    if ($full.pre_tls_capacity_rejected -lt 2 -or $full.successful_pairs -ne 1 -or
        $full.peak_active_pairs -ne 1) { throw 'Pair-full reservation oracle failed' }
    [pscustomobject]@{ case = 'pair-full'; passed = $true; pre_tls_rejected = $full.pre_tls_capacity_rejected;
        active_pair_preserved = $true } | ConvertTo-Json -Compress

    # Owned drain terminates a held active pair and joins all work.
    $deadline = New-Deadline
    $server = Start-LoggedProcess drain-server @('server', '-listen', '127.0.0.1:47925',
        '-deadline-unix', $deadline, '-handshakes', '2', '-waiting', '2', '-pairs', '1',
        '-expect-pairs', '0', '-drain-after-ms', '2000')
    Wait-LogText $server.Stdout '"event":"ready"'
    $drainInitiator = Start-LoggedProcess drain-initiator @('client', '-endpoint', '127.0.0.1:47925',
        '-side', 'initiator', '-token', 'drain-pair', '-deadline-unix', $deadline, '-hold-ms', '5000')
    $drainResponder = Start-LoggedProcess drain-responder @('client', '-endpoint', '127.0.0.1:47925',
        '-side', 'responder', '-token', 'drain-pair', '-deadline-unix', $deadline, '-hold-ms', '5000')
    Wait-LogText $server.Stdout '"event":"pair-active"'
    if ((Wait-LoggedProcess $drainInitiator 8000) -eq 0 -or (Wait-LoggedProcess $drainResponder 8000) -eq 0 -or
        (Wait-LoggedProcess $server 8000) -ne 0) { throw 'Drain exit oracle failed' }
    $drain = Last-JsonRecord $server.Stdout 'ardents-r092-rendezvous-server-v1'
    Assert-FinalCleanup $drain
    if ($drain.peak_active_pairs -ne 1 -or $drain.successful_pairs -ne 0 -or -not $drain.cleanup_joined) {
        throw 'Drain reservation oracle failed'
    }
    [pscustomobject]@{ case = 'drain'; passed = $true; joined = $true; final_connections = 0 } |
        ConvertTo-Json -Compress
} finally {
    foreach ($process in $processes) {
        if ($process -and -not $process.HasExited) {
            Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
        }
    }
    if (Test-Path -LiteralPath $labRoot) {
        $resolved = (Resolve-Path -LiteralPath $labRoot).Path
        if (-not [string]::Equals($resolved, $labRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
            throw "Unexpected cleanup path: $resolved"
        }
        $children = @(Get-ChildItem -LiteralPath $resolved -Force)
        if (@($children | Where-Object PSIsContainer).Count -ne 0) {
            throw "Refusing nested directory cleanup under $resolved"
        }
        $children | Remove-Item -Force
        Remove-Item -LiteralPath $resolved -Force
    }
}
