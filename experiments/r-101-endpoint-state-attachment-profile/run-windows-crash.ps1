$ErrorActionPreference = 'Stop'

$tempBase = [System.IO.Path]::GetTempPath()
$root = Join-Path $tempBase 'a-r101c'
$binary = Join-Path $tempBase 'a-r101c.exe'
$ready = Join-Path $tempBase 'a-r101c.ready'
$stdout = Join-Path $tempBase 'a-r101c.stdout'
$stderr = Join-Path $tempBase 'a-r101c.stderr'
$process = $null

foreach ($path in @($root, $binary, $ready, $stdout, $stderr)) {
    if (Test-Path -LiteralPath $path) {
        throw "Refusing to reuse existing experiment path: $path"
    }
}

try {
    go build -o $binary main.go
    if ($LASTEXITCODE -ne 0) {
        throw "Windows experiment build failed with exit code $LASTEXITCODE"
    }

    $process = Start-Process -FilePath $binary -ArgumentList @('-root', $root, '-hold-ready', $ready) -PassThru -WindowStyle Hidden -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    $deadline = (Get-Date).AddSeconds(10)
    while (-not (Test-Path -LiteralPath $ready)) {
        if ($process.HasExited) {
            $details = if (Test-Path -LiteralPath $stderr) { Get-Content -LiteralPath $stderr -Raw } else { '' }
            throw "Harness exited before readiness marker: $($process.ExitCode) $details"
        }
        if ((Get-Date) -ge $deadline) {
            throw 'Timed out waiting for harness readiness marker'
        }
        Start-Sleep -Milliseconds 100
    }

    Stop-Process -Id $process.Id -Force
    Wait-Process -Id $process.Id -ErrorAction SilentlyContinue
    $process = $null

    $ownerPresent = Test-Path -LiteralPath (Join-Path $root 'endpoint-state\live\owner.lock')
    $socketPresent = Test-Path -LiteralPath (Join-Path $root 'endpoint-state\runtime\attachment.sock')
    $vaultPresent = Test-Path -LiteralPath (Join-Path $root 'endpoint-state\vault')
    Write-Output "owner_file_after_kill=$($ownerPresent.ToString().ToLowerInvariant())"
    Write-Output "socket_after_kill=$($socketPresent.ToString().ToLowerInvariant())"
    Write-Output "vault_after_kill=$($vaultPresent.ToString().ToLowerInvariant())"
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
