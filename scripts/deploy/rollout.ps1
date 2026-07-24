param(
    [Parameter(Position = 0, Mandatory = $true)]
    [ValidateSet("upgrade", "rollback")]
    [string]$Action,
    [string]$NewImage,
    [string]$StateDir = "var/deployment/local-multinode",
    [string]$Project = "ardents-local",
    [ValidateRange(10, 300)]
    [int]$TimeoutSeconds = 120,
    [switch]$AllowMutableImage
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
. (Join-Path $PSScriptRoot "composite-readiness.ps1")
Set-Location $root
$statePath = [IO.Path]::GetFullPath($StateDir)
$manifestPath = Join-Path $statePath "cluster.json"
$journalPath = Join-Path $statePath "rollout-transaction.json"
$repositoryComposeFile = Join-Path $root "deploy/docker/compose/docker-compose.multinode.yml"
$bundleComposeFile = Join-Path $root "docker/docker-compose.multinode.yml"
$composeFile = if (Test-Path -LiteralPath $repositoryComposeFile) {
    $repositoryComposeFile
} elseif (Test-Path -LiteralPath $bundleComposeFile) {
    $bundleComposeFile
} else {
    throw "multinode Compose file is missing from the repository or distribution bundle"
}
$composePrefix = @("compose", "-p", $Project, "-f", $composeFile)
$services = @("seed", "peer2", "peer3")

if (-not ("Ardents.RolloutDurableFile" -as [type])) {
    Add-Type -TypeDefinition @"
using System;
using System.ComponentModel;
using System.IO;
using System.Runtime.InteropServices;

namespace Ardents {
    public static class RolloutDurableFile {
        private const int MoveFileReplaceExisting = 0x1;
        private const int MoveFileWriteThrough = 0x8;
        private const int OpenReadOnly = 0;
        private const int OpenDirectory = 0x10000;

        [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
        private static extern bool MoveFileEx(
            string existingName,
            string newName,
            int flags
        );

        [DllImport("libc", SetLastError = true)]
        private static extern int open(string path, int flags);

        [DllImport("libc", SetLastError = true)]
        private static extern int fsync(int descriptor);

        [DllImport("libc", SetLastError = true)]
        private static extern int close(int descriptor);

        public static void Replace(string source, string destination) {
            if (RuntimeInformation.IsOSPlatform(OSPlatform.Windows)) {
                if (!MoveFileEx(
                    source,
                    destination,
                    MoveFileReplaceExisting | MoveFileWriteThrough
                )) {
                    throw new Win32Exception(Marshal.GetLastWin32Error());
                }
                return;
            }

            if (File.Exists(destination)) {
                File.Replace(source, destination, null);
            } else {
                File.Move(source, destination);
            }
            string directory = Path.GetDirectoryName(destination);
            int descriptor = open(directory, OpenReadOnly | OpenDirectory);
            if (descriptor < 0) {
                throw new Win32Exception(Marshal.GetLastWin32Error());
            }
            try {
                if (fsync(descriptor) != 0) {
                    throw new Win32Exception(Marshal.GetLastWin32Error());
                }
            } finally {
                close(descriptor);
            }
        }
    }
}
"@
}

if (-not (Test-Path -LiteralPath $manifestPath)) { throw "cluster manifest is missing" }
$cluster = Get-Content -Raw -LiteralPath $manifestPath | ConvertFrom-Json
$env:ARDENTS_BOOTSTRAP_PEER = [string]$cluster.bootstrap_endpoint
$operatorDevice = "/operator-identity/device.json"

function Invoke-Compose([string[]]$Arguments) {
    & docker @composePrefix @Arguments
    if ($LASTEXITCODE -ne 0) { throw "docker compose failed: $($Arguments -join ' ')" }
}

function Write-AtomicJson([string]$Path, [object]$Value) {
    $temporaryPath = "$Path.new"
    $json = $Value | ConvertTo-Json -Depth 8
    $encoding = [Text.UTF8Encoding]::new($false)
    $stream = [IO.File]::Open(
        $temporaryPath,
        [IO.FileMode]::Create,
        [IO.FileAccess]::Write,
        [IO.FileShare]::None
    )
    try {
        $writer = [IO.StreamWriter]::new($stream, $encoding)
        try {
            $writer.Write($json)
            $writer.Flush()
            $stream.Flush($true)
        } finally {
            $writer.Dispose()
        }
    } finally {
        $stream.Dispose()
    }
    [Ardents.RolloutDurableFile]::Replace($temporaryPath, $Path)
}

function Write-RolloutJournal([object]$Journal) {
    Write-AtomicJson $journalPath $Journal
}

function Invoke-ArdJson([string]$Service, [string[]]$Arguments) {
    $principalProperty = $cluster.node_principals.PSObject.Properties[$Service]
    if ($null -eq $principalProperty -or [string]::IsNullOrWhiteSpace([string]$principalProperty.Value)) {
        throw "cluster manifest has no node Principal for $Service"
    }
    $raw = & docker @composePrefix --profile tools run --rm --no-deps "$Service-operator" `
        --output json --signer-file $operatorDevice --principal ([string]$principalProperty.Value) @Arguments
    if ($LASTEXITCODE -ne 0) { throw "ardentsctl command failed for $Service" }
    return (($raw -join "`n") | ConvertFrom-Json)
}

function Wait-Service([string]$Service) {
    Wait-ArdentsCompositeReadiness `
        -Service $Service `
        -TimeoutSeconds $TimeoutSeconds `
        -Probe { param($target) Invoke-ArdJson $target @("node", "runtime") }
}

function Set-Image([string]$Image) {
    if ([string]::IsNullOrWhiteSpace($Image)) { throw "image reference is empty" }
    if (-not $AllowMutableImage -and $Image -notmatch "@sha256:[0-9a-fA-F]{64}$") {
        throw "an immutable digest image is required; use -AllowMutableImage only for a local development image"
    }
    $env:ARDENTS_DOCKER_IMAGE = $Image
}

function Create-Service([string]$Service) {
    Invoke-Compose @("create", "--no-deps", "--force-recreate", $Service)
}

function Start-Service([string]$Service) {
    Invoke-Compose @("start", $Service)
}

function Recreate-Service([string]$Service) {
    Create-Service $Service
    Start-Service $Service
    Wait-Service $Service
}

function Write-ClusterManifest([string]$Image, [string]$PreviousImage) {
    $updated = [ordered]@{
        schema = "ardents.deployment/v1"
        project = [string]$cluster.project
        image = $Image
        previous_image = $PreviousImage
        bootstrap_endpoint = [string]$cluster.bootstrap_endpoint
        operator_principal = [string]$cluster.operator_principal
        node_principals = $cluster.node_principals
        services = $services
        updated_at = [DateTime]::UtcNow.ToString("O")
    }
    Write-AtomicJson $manifestPath $updated
}

function New-RolloutJournal([string]$RolloutAction, [string]$TargetImage, [string]$FallbackImage) {
    return [pscustomobject][ordered]@{
        schema = "ardents.rollout-transaction/v1"
        project = $Project
        action = $RolloutAction
        phase = "applying"
        target_image = $TargetImage
        fallback_image = $FallbackImage
        started_at = [DateTime]::UtcNow.ToString("O")
        failure = ""
        nodes = @()
    }
}

function Get-JournalNode([object]$Journal, [string]$Service) {
    return $Journal.nodes | Where-Object { $_.service -eq $Service } | Select-Object -First 1
}

function Invoke-RolloutCompensation([object]$Journal) {
    $Journal.phase = "compensating"
    Write-RolloutJournal $Journal
    Set-Image ([string]$Journal.fallback_image)
    $rollbackErrors = [Collections.Generic.List[string]]::new()
    $compensationNodes = @($Journal.nodes)
    [array]::Reverse($compensationNodes)
    foreach ($node in $compensationNodes) {
        if ($node.status -eq "restored") { continue }
        $node.status = "compensating"
        $node.last_error = ""
        Write-RolloutJournal $Journal
        try {
            Recreate-Service ([string]$node.service)
            $node.status = "restored"
            $node.restored_at = [DateTime]::UtcNow.ToString("O")
            Write-RolloutJournal $Journal
        } catch {
            $message = "$($node.service): $($_.Exception.Message)"
            $node.status = "rollback_failed"
            $node.last_error = $message
            Write-RolloutJournal $Journal
            $rollbackErrors.Add($message)
        }
    }
    if ($rollbackErrors.Count -eq 0) {
        Remove-Item -Force -LiteralPath $journalPath
    }
    return @($rollbackErrors)
}

function Resume-PendingRollout {
    if (-not (Test-Path -LiteralPath $journalPath)) { return $false }
    $journal = Get-Content -Raw -LiteralPath $journalPath | ConvertFrom-Json
    if ($journal.schema -ne "ardents.rollout-transaction/v1" -or $journal.project -ne $Project) {
        throw "rollout transaction journal does not match the supported schema and project: $journalPath"
    }
    if ($journal.phase -eq "ready_to_commit" -and [string]$cluster.image -eq [string]$journal.target_image) {
        Remove-Item -Force -LiteralPath $journalPath
        Write-Host "Completed rollout transaction was already committed; cleared its journal"
        return $true
    }
    $rollbackErrors = @(Invoke-RolloutCompensation $journal)
    if ($rollbackErrors.Count -gt 0) {
        throw "pending rollout compensation remains incomplete: $($rollbackErrors -join '; ')"
    }
    Write-Host "Pending rollout compensation completed at fallback image $($journal.fallback_image)"
    return $true
}

function Invoke-RollingChange(
    [string]$RolloutAction,
    [string]$TargetImage,
    [string]$FallbackImage
) {
    Set-Image $TargetImage
    $journal = New-RolloutJournal $RolloutAction $TargetImage $FallbackImage
    Write-RolloutJournal $journal
    try {
        foreach ($service in $services) {
            $journal.nodes += [pscustomobject][ordered]@{
                service = $service
                status = "mutation_pending"
                last_error = ""
                journaled_at = [DateTime]::UtcNow.ToString("O")
                applied_at = ""
                restored_at = ""
            }
            Write-RolloutJournal $journal
            $node = Get-JournalNode $journal $service
            Create-Service $service
            $node.status = "recreated"
            Write-RolloutJournal $journal
            Start-Service $service
            $node.status = "started"
            Write-RolloutJournal $journal
            Wait-Service $service
            $node.status = "applied"
            $node.applied_at = [DateTime]::UtcNow.ToString("O")
            Write-RolloutJournal $journal
        }
    } catch {
        $changeError = $_.Exception.Message
        $journal.failure = $changeError
        Write-RolloutJournal $journal
        $rollbackErrors = @(Invoke-RolloutCompensation $journal)
        if ($rollbackErrors.Count -gt 0) {
            throw "image change failed: $changeError; automatic rollback also failed: $($rollbackErrors -join '; ')"
        }
        throw "image change failed and changed nodes were rolled back: $changeError"
    }
    $journal.phase = "ready_to_commit"
    Write-RolloutJournal $journal
}

function New-UpgradeBackups {
    foreach ($service in $services) {
        & (Join-Path $PSScriptRoot "data.ps1") backup -Node $service -StateDir $statePath -Project $Project -TimeoutSeconds $TimeoutSeconds
        if ($LASTEXITCODE -ne 0) { throw "pre-upgrade backup failed for $service" }
    }
}

if (Resume-PendingRollout) { return }

if ($Action -eq "upgrade") {
    if ([string]::IsNullOrWhiteSpace($NewImage)) { throw "-NewImage is required for upgrade" }
    $previous = [string]$cluster.image
    New-UpgradeBackups
    Invoke-RollingChange $Action $NewImage $previous
    Write-ClusterManifest $NewImage $previous
    Remove-Item -Force -LiteralPath $journalPath
    Write-Host "Rolling upgrade accepted: $previous -> $NewImage"
} else {
    $previous = [string]$cluster.previous_image
    if ([string]::IsNullOrWhiteSpace($previous)) { throw "cluster manifest has no previous image for rollback" }
    $current = [string]$cluster.image
    Invoke-RollingChange $Action $previous $current
    Write-ClusterManifest $previous $current
    Remove-Item -Force -LiteralPath $journalPath
    Write-Host "Rolling rollback accepted: $current -> $previous"
}
