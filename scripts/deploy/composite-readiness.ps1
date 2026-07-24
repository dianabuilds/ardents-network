function ConvertTo-ArdentsProcessArgument([string]$Value) {
    if ($Value -ne "" -and $Value -notmatch '[\s"]') { return $Value }
    $builder = [Text.StringBuilder]::new()
    [void]$builder.Append('"')
    $backslashes = 0
    foreach ($character in $Value.ToCharArray()) {
        if ($character -eq '\') {
            $backslashes++
            continue
        }
        if ($character -eq '"') {
            [void]$builder.Append(('\' * (($backslashes * 2) + 1)))
            [void]$builder.Append('"')
            $backslashes = 0
            continue
        }
        if ($backslashes -gt 0) {
            [void]$builder.Append(('\' * $backslashes))
            $backslashes = 0
        }
        [void]$builder.Append($character)
    }
    if ($backslashes -gt 0) {
        [void]$builder.Append(('\' * ($backslashes * 2)))
    }
    [void]$builder.Append('"')
    return $builder.ToString()
}

function Invoke-ArdentsBoundedProcess {
    param(
        [Parameter(Mandatory = $true)]
        [string]$FilePath,
        [Parameter(Mandatory = $true)]
        [string[]]$ArgumentList,
        [Parameter(Mandatory = $true)]
        [DateTime]$Deadline
    )

    $remaining = [int][Math]::Floor(($Deadline - [DateTime]::UtcNow).TotalMilliseconds)
    if ($remaining -le 0) { throw "composite readiness probe deadline exceeded before process start" }

    $startInfo = [Diagnostics.ProcessStartInfo]::new()
    $startInfo.FileName = $FilePath
    $startInfo.Arguments = (($ArgumentList | ForEach-Object { ConvertTo-ArdentsProcessArgument ([string]$_) }) -join " ")
    $startInfo.UseShellExecute = $false
    $startInfo.CreateNoWindow = $true
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true

    $process = [Diagnostics.Process]::new()
    $process.StartInfo = $startInfo
    try {
        if (-not $process.Start()) { throw "failed to start composite readiness probe process" }
        $stdout = $process.StandardOutput.ReadToEndAsync()
        $stderr = $process.StandardError.ReadToEndAsync()
        if (-not $process.WaitForExit($remaining)) {
            try { $process.Kill() } catch {}
            $process.WaitForExit()
            throw "composite readiness probe deadline exceeded"
        }
        $output = $stdout.GetAwaiter().GetResult()
        $errorOutput = $stderr.GetAwaiter().GetResult().Trim()
        if ($process.ExitCode -ne 0) {
            $detail = if ($errorOutput) { ": $errorOutput" } else { "" }
            throw "composite readiness probe process exited with code $($process.ExitCode)$detail"
        }
        return $output
    } finally {
        $process.Dispose()
    }
}

function Wait-ArdentsCompositeReadiness {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Service,
        [Parameter(Mandatory = $true)]
        [ValidateRange(1, 300)]
        [int]$TimeoutSeconds,
        [Parameter(Mandatory = $true)]
        [scriptblock]$Probe,
        [scriptblock]$UtcNow = { [DateTime]::UtcNow },
        [scriptblock]$Pause = { Start-Sleep -Seconds 2 }
    )

    $deadline = (& $UtcNow).AddSeconds($TimeoutSeconds)
    $lastError = ""
    while ((& $UtcNow) -lt $deadline) {
        try {
            $status = & $Probe $Service $deadline
            $readiness = $status.runtime.readiness
            if ($null -ne $readiness -and $readiness.ready -eq $true) { return }
            if ($null -eq $readiness) {
                $lastError = "node runtime response has no composite readiness"
            } elseif (-not [string]::IsNullOrWhiteSpace([string]$readiness.reason)) {
                $lastError = [string]$readiness.reason
            } else {
                $failedChecks = @(
                    $readiness.checks |
                        Where-Object { $_.ready -ne $true } |
                        ForEach-Object {
                            if ([string]::IsNullOrWhiteSpace([string]$_.reason)) {
                                [string]$_.name
                            } else {
                                "{0}: {1}" -f $_.name, $_.reason
                            }
                        }
                )
                $lastError = if ($failedChecks.Count -gt 0) {
                    $failedChecks -join "; "
                } else {
                    "composite readiness is false without a component reason"
                }
            }
        } catch {
            $lastError = $_.Exception.Message
        }
        & $Pause
    }
    throw "timed out waiting for composite readiness from $Service after image change; $lastError"
}
