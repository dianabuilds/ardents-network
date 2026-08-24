$ErrorActionPreference = 'Stop'

$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..\..')).Path
$labRoot = Join-Path ([System.IO.Path]::GetTempPath()) 'ardents-r105-introduction-local'
$binary = Join-Path $labRoot 'r105-introduction.exe'
$processes = [System.Collections.Generic.List[System.Diagnostics.Process]]::new()

if (Test-Path -LiteralPath $labRoot) { throw "Refusing to reuse lab root: $labRoot" }
New-Item -ItemType Directory -Path $labRoot | Out-Null

function Start-Role {
    param([string]$Name, [string[]]$Arguments)
    $stdout = Join-Path $labRoot "$Name.stdout"
    $stderr = Join-Path $labRoot "$Name.stderr"
    $process = Start-Process -FilePath $binary -ArgumentList $Arguments -PassThru -WindowStyle Hidden `
        -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    $processes.Add($process)
    return [pscustomobject]@{ Name = $Name; Process = $process; Stdout = $stdout; Stderr = $stderr }
}

function Wait-Text {
    param([string]$Path, [string]$Text)
    $until = (Get-Date).AddSeconds(5)
    while ((Get-Date) -lt $until) {
        if ((Test-Path -LiteralPath $Path) -and (Select-String -LiteralPath $Path -SimpleMatch $Text -Quiet)) { return }
        Start-Sleep -Milliseconds 50
    }
    throw "Timed out waiting for $Text in $Path"
}

function Wait-Role {
    param($Role)
    if (-not $Role.Process.WaitForExit(10000)) { throw "$($Role.Name) did not exit" }
    if ($Role.Process.ExitCode -ne 0) {
        $errorText = if (Test-Path -LiteralPath $Role.Stderr) { Get-Content -LiteralPath $Role.Stderr -Raw } else { '' }
        throw "$($Role.Name) failed: $errorText"
    }
}

function Final-Record {
    param($Role, [string]$Schema)
    $records = @(Get-Content -LiteralPath $Role.Stdout | ForEach-Object { $_ | ConvertFrom-Json })
    $found = @($records | Where-Object schema -eq $Schema)
    if ($found.Count -ne 1 -or -not $found[0].passed) { throw "Unexpected $Schema record in $($Role.Name).stdout" }
    return $found[0]
}

function Invoke-Cell {
    param([string]$Mode, [int]$Port)
    $deadline = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds() + 12
    $endpoint = "127.0.0.1:$Port"
    $intro = Start-Role "$Mode-introduction" @('introduction', '-endpoint', $endpoint, '-deadline-unix', $deadline, '-mode', $Mode)
    Wait-Text $intro.Stdout '"event":"ready"'
    $publisher = Start-Role "$Mode-publisher" @('publisher', '-endpoint', $endpoint, '-deadline-unix', $deadline, '-mode', $Mode)
    Wait-Text $publisher.Stdout '"event":"slot-ready"'
    if ($Mode -eq 'withdrawn-slot') { Wait-Role $publisher; Start-Sleep -Milliseconds 150 }
    $user = Start-Role "$Mode-user" @('user', '-endpoint', $endpoint, '-deadline-unix', $deadline, '-mode', $Mode)
    Wait-Role $user
    if ($Mode -ne 'withdrawn-slot') { Wait-Role $publisher }
    Wait-Role $intro
    $i = Final-Record $intro 'ardents-r105-introduction-v1'
    $u = Final-Record $user 'ardents-r105-user-v1'
    if ($Mode -ne 'withdrawn-slot') { $p = Final-Record $publisher 'ardents-r105-publisher-v1' }
    if ($Mode -eq 'exact' -and ($i.delivered -ne 1 -or $p.delivered -ne 1)) { throw 'exact delivery oracle failed' }
    if ($Mode -eq 'replay' -and ($i.delivered -ne 1 -or $i.replay_refused -ne 1 -or $u.replay_refused -ne 1)) { throw 'replay oracle failed' }
    if (($Mode -eq 'header-tamper' -or $Mode -eq 'ciphertext-tamper') -and $p.delivered -ne 0) { throw 'tampered payload was accepted' }
    if ($Mode -eq 'withdrawn-slot' -and $i.delivered -ne 0) { throw 'withdrawn slot delivered' }
    [pscustomobject]@{ case = $Mode; passed = $true; delivered = $i.delivered; replay_refused = $i.replay_refused } | ConvertTo-Json -Compress
}

try {
    Push-Location $repoRoot
    try {
        & go build -trimpath -o $binary `
            experiments/r-105-live-introduction-tracer/main.go `
            experiments/r-105-live-introduction-tracer/material.go `
            experiments/r-105-live-introduction-tracer/roles.go
        if ($LASTEXITCODE -ne 0) { throw 'R-105 tracer build failed' }
    } finally { Pop-Location }
    Invoke-Cell exact 47951
    Invoke-Cell replay 47952
    Invoke-Cell header-tamper 47953
    Invoke-Cell ciphertext-tamper 47954
    Invoke-Cell withdrawn-slot 47955
} finally {
    foreach ($process in $processes) { if ($process -and -not $process.HasExited) { Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue } }
    if (Test-Path -LiteralPath $labRoot) {
        $resolved = (Resolve-Path -LiteralPath $labRoot).Path
        if (-not [string]::Equals($resolved, $labRoot, [System.StringComparison]::OrdinalIgnoreCase)) { throw "Unexpected cleanup path: $resolved" }
        $children = @(Get-ChildItem -LiteralPath $resolved -Force)
        if (@($children | Where-Object PSIsContainer).Count -ne 0) { throw 'Refusing nested cleanup' }
        $children | Remove-Item -Force
        Remove-Item -LiteralPath $resolved -Force
    }
}
