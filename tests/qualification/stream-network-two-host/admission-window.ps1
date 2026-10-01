function Get-QualificationAdmissionWindowDelay {
    param(
        [Parameter(Mandatory = $true)][DateTimeOffset]$Now,
        [Parameter(Mandatory = $true)][TimeSpan]$MinimumRemaining,
        [TimeSpan]$BoundaryGrace = [TimeSpan]::FromSeconds(2)
    )

    if ($MinimumRemaining -le [TimeSpan]::Zero -or $MinimumRemaining -ge [TimeSpan]::FromHours(1) -or
        $BoundaryGrace -lt [TimeSpan]::Zero -or $BoundaryGrace -ge [TimeSpan]::FromMinutes(1)) {
        throw 'Qualification admission-window bounds are invalid.'
    }
    $utc = $Now.ToUniversalTime()
    $hour = [DateTimeOffset]::new($utc.Year, $utc.Month, $utc.Day, $utc.Hour, 0, 0, [TimeSpan]::Zero)
    $remaining = $hour.AddHours(1) - $utc
    if ($remaining -ge $MinimumRemaining) {
        return [TimeSpan]::Zero
    }
    return $remaining + $BoundaryGrace
}
