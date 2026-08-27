$ErrorActionPreference = 'Stop'

$tempBase = [System.IO.Path]::GetTempPath()
$parent = Join-Path $tempBase 'a-r102-sub'
$root = Join-Path $parent 'root'
$state = Join-Path $root 'endpoint-state'
$runtime = Join-Path $state 'runtime'
$outside = Join-Path $parent 'outside'
$sentinel = Join-Path $outside 'keep.txt'
$binary = Join-Path $tempBase 'a-r102-sub.exe'
$ready = Join-Path $tempBase 'a-r102-sub.ready'
$stdout = Join-Path $tempBase 'a-r102-sub.stdout'
$stderr = Join-Path $tempBase 'a-r102-sub.stderr'
$process = $null

foreach ($path in @($parent, $binary, $ready, $stdout, $stderr)) {
    if (Test-Path -LiteralPath $path) {
        throw "Refusing to reuse existing experiment path: $path"
    }
}

try {
    New-Item -ItemType Directory -Path (Join-Path $state 'live') -Force | Out-Null
    New-Item -ItemType Directory -Path (Join-Path $state 'vault') -Force | Out-Null
    New-Item -ItemType Directory -Path $outside -Force | Out-Null
    Set-Content -LiteralPath $sentinel -Value 'preserve' -NoNewline
    New-Item -ItemType Junction -Path $runtime -Target $outside | Out-Null

    go build -o $binary main.go lock_windows.go
    if ($LASTEXITCODE -ne 0) {
        throw "Windows substitution build failed with exit code $LASTEXITCODE"
    }
    $process = Start-Process -FilePath $binary -ArgumentList @(
        '-root', $root, '-hold-ready', $ready
    ) -PassThru -WindowStyle Hidden -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    $deadline = (Get-Date).AddSeconds(10)
    while (-not (Test-Path -LiteralPath $ready) -and -not $process.HasExited) {
        if ((Get-Date) -ge $deadline) {
            throw 'Timed out waiting for substitution result'
        }
        Start-Sleep -Milliseconds 100
        $process.Refresh()
    }

    Write-Output "substituted_runtime_accepted=$([bool](Test-Path -LiteralPath $ready))"
    Write-Output "child_exited=$($process.HasExited)"
    if ($process.HasExited) {
        Write-Output "exit_code=$($process.ExitCode)"
    }
    Write-Output "outside_socket_created=$([bool](Test-Path -LiteralPath (Join-Path $outside 'attachment.sock')))"
    $stderrText = [string](Get-Content -Raw -LiteralPath $stderr -ErrorAction SilentlyContinue)
    if ($null -eq $stderrText) {
        $stderrText = ''
    }
    Write-Output "stderr=$($stderrText.Trim())"
}
finally {
    if ($process -and -not $process.HasExited) {
        $expected = (Resolve-Path -LiteralPath $binary).Path
        if ([string]::Equals($process.Path, $expected, [System.StringComparison]::OrdinalIgnoreCase)) {
            Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
            Wait-Process -Id $process.Id -ErrorAction SilentlyContinue
        }
    }
    if (Test-Path -LiteralPath $runtime) {
        $item = Get-Item -LiteralPath $runtime -Force
        if ($item.LinkType -ne 'Junction' -or
            -not [string]::Equals($item.FullName, [System.IO.Path]::GetFullPath($runtime), [System.StringComparison]::OrdinalIgnoreCase)) {
            throw "Refusing to remove unexpected runtime entry: $runtime"
        }
        Remove-Item -LiteralPath $runtime -Force
    }
    if (Test-Path -LiteralPath $sentinel) {
        Write-Output "sentinel_preserved_after_link_removal=$((Get-Content -Raw -LiteralPath $sentinel) -eq 'preserve')"
    }
    if (Test-Path -LiteralPath $parent) {
        $resolved = (Resolve-Path -LiteralPath $parent).Path
        if ($resolved -ne $parent) {
            throw "Refusing to remove unexpected experiment parent: $resolved"
        }
        Remove-Item -LiteralPath $resolved -Recurse -Force
    }
    foreach ($path in @($binary, $ready, $stdout, $stderr)) {
        if ((Test-Path -LiteralPath $path) -and ((Resolve-Path -LiteralPath $path).Path -eq $path)) {
            Remove-Item -LiteralPath $path -Force
        }
    }
}
