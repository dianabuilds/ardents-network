$ErrorActionPreference = 'Stop'

$tempBase = [System.IO.Path]::GetTempPath()
$root = Join-Path $tempBase 'a-r102'
$binary = Join-Path $tempBase 'a-r102.exe'
$ready = Join-Path $tempBase 'a-r102.ready'
$stdout = Join-Path $tempBase 'a-r102.stdout'
$stderr = Join-Path $tempBase 'a-r102.stderr'
$process = $null

foreach ($path in @($root, $binary, $ready, $stdout, $stderr)) {
    if (Test-Path -LiteralPath $path) {
        throw "Refusing to reuse existing experiment path: $path"
    }
}

try {
    go build -o $binary main.go lock_windows.go
    if ($LASTEXITCODE -ne 0) {
        throw "Windows experiment build failed with exit code $LASTEXITCODE"
    }
    $process = Start-Process -FilePath $binary -ArgumentList @('-root', $root, '-hold-ready', $ready) -PassThru -WindowStyle Hidden -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    $deadline = (Get-Date).AddSeconds(10)
    while (-not (Test-Path -LiteralPath $ready)) {
        if ($process.HasExited) {
            $details = if (Test-Path -LiteralPath $stderr) { Get-Content -LiteralPath $stderr -Raw } else { '' }
            throw "First owner exited before readiness marker: $($process.ExitCode) $details"
        }
        if ((Get-Date) -ge $deadline) {
            throw 'Timed out waiting for first owner readiness marker'
        }
        Start-Sleep -Milliseconds 100
    }

    $socket = Join-Path $root 'endpoint-state\runtime\attachment.sock'
    $contender = & $binary -root $root -expect-busy -recover-socket 2>&1
    if ($LASTEXITCODE -ne 0 -or $contender -ne 'lock=busy') {
        throw "Concurrent owner result was not lock=busy: $contender"
    }
    Write-Output $contender
    if (-not (Test-Path -LiteralPath $socket)) {
        throw 'Concurrent recovery request removed the live socket'
    }
    Write-Output 'live_socket_preserved=true'

    Stop-Process -Id $process.Id -Force
    Wait-Process -Id $process.Id -ErrorAction SilentlyContinue
    $process = $null

    $recovered = $false
    $lastRecovery = ''
    for ($attempt = 1; $attempt -le 100; $attempt++) {
        $lastRecovery = & $binary -root $root -recover-socket 2>&1
        if ($LASTEXITCODE -eq 0) {
            $recovered = $true
            break
        }
        Start-Sleep -Milliseconds 100
    }
    if (-not $recovered) {
        throw "Post-kill recovery failed: $lastRecovery"
    }
    $lastRecovery

    Remove-Item -LiteralPath $socket -Force -ErrorAction SilentlyContinue
    Set-Content -LiteralPath $socket -Value 'preserve-regular' -NoNewline
    $regular = & $binary -root $root -recover-socket 2>&1
    $regularCode = $LASTEXITCODE
    $regularText = ([string]::Join([Environment]::NewLine, @($regular))).Trim()
    if ($regularCode -eq 0 -or $regularText -ne 'failure=unexpected_runtime_entry_type') {
        throw "Regular runtime entry was not rejected: $regularText"
    }
    if ((Get-Content -Raw -LiteralPath $socket) -ne 'preserve-regular') {
        throw 'Rejected regular runtime entry was mutated'
    }
    Write-Output 'unexpected_regular_preserved=true'

    Remove-Item -LiteralPath $socket -Force
    New-Item -ItemType Directory -Path $socket | Out-Null
    $directory = & $binary -root $root -recover-socket 2>&1
    $directoryCode = $LASTEXITCODE
    $directoryText = ([string]::Join([Environment]::NewLine, @($directory))).Trim()
    if ($directoryCode -eq 0 -or $directoryText -ne 'failure=unexpected_runtime_entry_type') {
        throw "Directory runtime entry was not rejected: $directoryText"
    }
    if (-not (Test-Path -LiteralPath $socket -PathType Container)) {
        throw 'Rejected directory runtime entry was mutated'
    }
    Write-Output 'unexpected_directory_preserved=true'

    Remove-Item -LiteralPath $socket -Force
    $junctionTarget = Join-Path $root 'unexpected-target'
    New-Item -ItemType Directory -Path $junctionTarget | Out-Null
    Set-Content -LiteralPath (Join-Path $junctionTarget 'keep.txt') -Value 'preserve-junction' -NoNewline
    New-Item -ItemType Junction -Path $socket -Target $junctionTarget | Out-Null
    $junction = & $binary -root $root -recover-socket 2>&1
    $junctionCode = $LASTEXITCODE
    $junctionText = ([string]::Join([Environment]::NewLine, @($junction))).Trim()
    if ($junctionCode -eq 0 -or $junctionText -ne 'failure=unexpected_runtime_entry_type') {
        throw "Junction runtime entry was not rejected: $junctionText"
    }
    if ((Get-Content -Raw -LiteralPath (Join-Path $junctionTarget 'keep.txt')) -ne 'preserve-junction') {
        throw 'Rejected junction runtime entry was mutated'
    }
    $junctionItem = Get-Item -LiteralPath $socket -Force
    if ($junctionItem.LinkType -ne 'Junction' -or
        -not [string]::Equals($junctionItem.FullName, [System.IO.Path]::GetFullPath($socket), [System.StringComparison]::OrdinalIgnoreCase)) {
        throw 'Rejected junction runtime entry changed type or path'
    }
    Remove-Item -LiteralPath $socket -Force
    Write-Output 'unexpected_junction_preserved=true'
}
finally {
    if ($process -and -not $process.HasExited) {
        Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
    }
    foreach ($path in @($root, $binary, $ready, $stdout, $stderr)) {
        if ((Test-Path -LiteralPath $path) -and ((Resolve-Path -LiteralPath $path).Path -eq $path)) {
            Remove-Item -LiteralPath $path -Recurse -Force
        }
    }
}
