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
            $status = & $Probe $Service
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
