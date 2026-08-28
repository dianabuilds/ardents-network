[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$SourceRevision,
    [Parameter(Mandatory = $true)][string]$ReleaseTag,
    [Parameter(Mandatory = $true)][string]$CandidateArchive,
    [Parameter(Mandatory = $true)][string]$ArchiveSHA256,
    [Parameter(Mandatory = $true)][string]$ManifestPin,
    [Parameter(Mandatory = $true)][string]$EndpointSHA256,
    [Parameter(Mandatory = $true)][string]$ControlSHA256,
    [Parameter(Mandatory = $true)][string]$Cohort,
    [Parameter(Mandatory = $true)][string]$At,
    [Parameter(Mandatory = $true)][string]$VPS,
    [Parameter(Mandatory = $true)][string]$SSHKey,
    [Parameter(Mandatory = $true)][string]$User,
    [Parameter(Mandatory = $true)][int]$BasePort,
    [Parameter(Mandatory = $true)][string]$RemoteImageID,
    [Parameter(Mandatory = $true)][string]$EvidenceOutput,
    [Parameter(Mandatory = $true)][ValidateSet('-timeout=125m')][string]$CampaignTimeout
)

$ErrorActionPreference = 'Stop'
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)

function Write-Utf8([string]$Path, [string]$Value) {
    [IO.File]::WriteAllText($Path, $Value, $utf8NoBom)
}

function Write-Json([string]$Path, [object]$Value) {
    Write-Utf8 $Path (($Value | ConvertTo-Json -Depth 8) + "`n")
}

function Join-NativeArguments([string[]]$Arguments) {
    return (($Arguments | ForEach-Object {
        if ($_ -notmatch '[\s"]') { $_ }
        else { '"' + ($_ -replace '(\\*)"', '$1$1\\"' -replace '(\\*)$', '$1$1') + '"' }
    }) -join ' ')
}

function Write-EvidenceInventory([string]$Root) {
    $inventoryPath = Join-Path $Root 'evidence-sha256.txt'
    $records = foreach ($file in Get-ChildItem -LiteralPath $Root -Recurse -File |
        Where-Object FullName -ne $inventoryPath | Sort-Object FullName) {
        $relative = $file.FullName.Substring($Root.Length).TrimStart('\', '/').Replace('\', '/')
        '{0}  {1}' -f (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant(), $relative
    }
    Write-Utf8 $inventoryPath (($records -join "`n") + "`n")
}

if (-not [IO.Path]::IsPathRooted($EvidenceOutput)) { throw 'EvidenceOutput must be an absolute path.' }
$evidencePath = [IO.Path]::GetFullPath($EvidenceOutput)
$evidenceParent = Split-Path -Parent $evidencePath
if (Test-Path -LiteralPath $evidencePath) { throw 'EvidenceOutput must be previously absent; the entrypoint never overwrites evidence.' }
if (-not (Test-Path -LiteralPath $evidenceParent -PathType Container)) { throw 'EvidenceOutput parent directory is unavailable.' }

$nonce = [Guid]::NewGuid().ToString('N')
$captureStdout = "$evidencePath.entrypoint-$nonce.stdout.log"
$captureStderr = "$evidencePath.entrypoint-$nonce.stderr.log"
if ((Test-Path -LiteralPath $captureStdout) -or (Test-Path -LiteralPath $captureStderr)) {
    throw 'entrypoint capture path unexpectedly exists.'
}

$runner = Join-Path $PSScriptRoot 'run-windows.ps1'
$powershell = (Get-Process -Id $PID).Path
$arguments = @(
    '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $runner,
    '-SourceRevision', $SourceRevision, '-ReleaseTag', $ReleaseTag,
    '-CandidateArchive', $CandidateArchive, '-ArchiveSHA256', $ArchiveSHA256,
    '-ManifestPin', $ManifestPin, '-EndpointSHA256', $EndpointSHA256,
    '-ControlSHA256', $ControlSHA256, '-Cohort', $Cohort, '-At', $At,
    '-VPS', $VPS, '-SSHKey', $SSHKey, '-User', $User,
    '-BasePort', [string]$BasePort, '-RemoteImageID', $RemoteImageID,
    '-EvidenceOutput', $evidencePath, '-CampaignTimeout', $CampaignTimeout
)
$startedAt = [DateTimeOffset]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
$child = $null
$childExited = $false
$childExit = $null
$wrapperError = $null
$acceptedReceipt = $false
try {
    $child = Start-Process -FilePath $powershell -ArgumentList (Join-NativeArguments $arguments) `
        -RedirectStandardOutput $captureStdout -RedirectStandardError $captureStderr -WindowStyle Hidden -PassThru
    # The child owns the exact 125-minute campaign clock.  This outer receipt
    # allows only the matching deadline plus a 30-second receipt/cleanup margin.
    $childExited = $child.WaitForExit(7530000)
    if (-not $childExited) {
        & "$env:WINDIR\System32\taskkill.exe" /PID $child.Id /T /F *>> $captureStderr
        $childExited = $child.WaitForExit(30000)
        if (-not $childExited) { throw 'campaign entrypoint did not exit within its bounded receipt/cleanup margin.' }
    }
    $childExit = [int]$child.ExitCode
}
catch {
    $wrapperError = $_.Exception.Message
    if ($null -ne $child) {
        try {
            $child.Refresh()
            if (-not $child.HasExited) { & "$env:WINDIR\System32\taskkill.exe" /PID $child.Id /T /F *>> $captureStderr }
            $childExited = $child.WaitForExit(30000)
            if ($childExited) { $childExit = [int]$child.ExitCode }
        }
        catch { $wrapperError += " | child cleanup: $($_.Exception.Message)" }
    }
}
finally {
    if ($null -eq $wrapperError -and $childExit -eq 0) {
        $receiptPath = Join-Path $evidencePath 'campaign-receipt.json'
        if (Test-Path -LiteralPath $receiptPath -PathType Leaf) {
            try {
                $receipt = Get-Content -LiteralPath $receiptPath -Raw | ConvertFrom-Json
                $acceptedReceipt = [bool]$receipt.accepted
            }
            catch { $wrapperError = "campaign receipt could not be parsed: $($_.Exception.Message)" }
        }
        if ($null -eq $wrapperError -and -not $acceptedReceipt) {
            $wrapperError = 'runner exited zero without an accepted A11 campaign receipt.'
        }
    }
    foreach ($capture in @($captureStdout, $captureStderr)) {
        if (-not (Test-Path -LiteralPath $capture -PathType Leaf)) { Write-Utf8 $capture '' }
    }
    if (-not (Test-Path -LiteralPath $evidencePath)) { [IO.Directory]::CreateDirectory($evidencePath) | Out-Null }
    Move-Item -LiteralPath $captureStdout -Destination (Join-Path $evidencePath 'entrypoint.stdout.log')
    Move-Item -LiteralPath $captureStderr -Destination (Join-Path $evidencePath 'entrypoint.stderr.log')
    Write-Utf8 (Join-Path $evidencePath 'entrypoint.exitcode') "$(if ($null -eq $childExit) { '' } else { $childExit })`n"
    Write-Json (Join-Path $evidencePath 'entrypoint-status.json') ([ordered]@{
        schema = 'ardents-h4-8-a11-entrypoint-status-v1'
        started_at_utc = $startedAt
        finished_at_utc = [DateTimeOffset]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
        command = [IO.Path]::GetFileName($runner)
        runner_sha256 = (Get-FileHash -LiteralPath $runner -Algorithm SHA256).Hash.ToLowerInvariant()
        entrypoint_sha256 = (Get-FileHash -LiteralPath $PSCommandPath -Algorithm SHA256).Hash.ToLowerInvariant()
        stdout = 'entrypoint.stdout.log'
        stderr = 'entrypoint.stderr.log'
        exit_status = $childExit
        exited_within_bounded_margin = $childExited
        accepted_campaign_receipt = $acceptedReceipt
        wrapper_error = $wrapperError
    })
    Write-EvidenceInventory $evidencePath
}

if ($null -ne $wrapperError) { Write-Error "A11 campaign entrypoint failed: $wrapperError"; exit 1 }
if ($childExit -ne 0) { exit $childExit }
