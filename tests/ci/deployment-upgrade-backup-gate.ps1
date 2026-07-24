# testkit:scenario SDE-002
# testkit:layer integration
# testkit:domain deployment

param(
    [string]$ReportDir = "tests/.artifacts/deployment/upgrade-backup"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$requestedReportPath = if ([IO.Path]::IsPathRooted($ReportDir)) { $ReportDir } else { Join-Path $root $ReportDir }
$reportPath = [IO.Path]::GetFullPath($requestedReportPath)
$artifactRoot = [IO.Path]::GetFullPath((Join-Path $root "tests/.artifacts"))
if (-not $reportPath.StartsWith($artifactRoot + [IO.Path]::DirectorySeparatorChar)) {
    throw "upgrade backup evidence path must stay under tests/.artifacts: $reportPath"
}
New-Item -ItemType Directory -Force -Path $reportPath | Out-Null

$testRoot = Join-Path ([IO.Path]::GetTempPath()) ("ardents-upgrade-backup-{0}" -f [guid]::NewGuid().ToString("N"))
$stageRoot = Join-Path $testRoot "stage"
$bundleArchive = Join-Path $testRoot "ardents-test-linux-amd64.tar.gz"
$bundleRoot = Join-Path $testRoot "bundle"
$positiveState = Join-Path $testRoot "positive-state"
$negativeState = Join-Path $testRoot "negative-state"
$global:FakeDockerEvents = [Collections.Generic.List[string]]::new()
$global:FakeBackupFailureNode = ""

function Write-ClusterFixture([string]$StatePath) {
    New-Item -ItemType Directory -Force -Path $StatePath | Out-Null
    [ordered]@{
        schema = "ardents.deployment/v1"
        project = "ardents-bundle-test"
        image = "registry.example/ardents@sha256:$("1" * 64)"
        previous_image = ""
        bootstrap_endpoint = "/ip4/127.0.0.1/tcp/60000/p2p/test"
        operator_principal = "p1_operator"
        node_principals = [ordered]@{
            seed = "p1_seed"
            peer2 = "p1_peer2"
            peer3 = "p1_peer3"
        }
        services = @("seed", "peer2", "peer3")
    } | ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 -LiteralPath (Join-Path $StatePath "cluster.json")
}

function global:docker {
    $arguments = @($args | ForEach-Object { [string]$_ })
    $global:FakeDockerEvents.Add(($arguments -join " "))
    $global:LASTEXITCODE = 0
    Set-Variable -Name LASTEXITCODE -Value 0 -Scope 1

    $composeIndex = [Array]::IndexOf($arguments, "-f")
    if ($composeIndex -ge 0) {
        $composePath = $arguments[$composeIndex + 1]
        if (-not (Test-Path -LiteralPath $composePath)) {
            throw "fake docker rejected missing Compose file: $composePath"
        }
    }

    if ($arguments[0] -eq "inspect") {
        $node = $arguments[-1] -replace "-container$", ""
        return @([ordered]@{
            Mounts = @([ordered]@{
                Destination = "/var/lib/ardents"
                Name = "ardents-bundle-test_${node}_data"
            })
        }) | ConvertTo-Json -Depth 4
    }

    if ($arguments[0] -eq "run") {
        $volumeMount = $arguments | Where-Object { $_ -match "^source=ardents-bundle-test_(seed|peer2|peer3)_data," } | Select-Object -First 1
        $node = if ($volumeMount -match "^source=ardents-bundle-test_(seed|peer2|peer3)_data,") { $Matches[1] } else { "" }
        if ($node -eq $global:FakeBackupFailureNode) {
            $global:LASTEXITCODE = 1
            Set-Variable -Name LASTEXITCODE -Value 1 -Scope 1
            return
        }
        $backupMount = $arguments | Where-Object { $_ -match "^type=bind,source=.+,target=/backup(?:,readonly)?$" } | Select-Object -First 1
        $archiveDir = $backupMount -replace "^type=bind,source=", "" -replace ",target=/backup(?:,readonly)?$", ""
        $archiveIndex = [Array]::IndexOf($arguments, "-czf")
        if ($backupMount -and $archiveIndex -ge 0) {
            $archiveName = [IO.Path]::GetFileName($arguments[$archiveIndex + 1])
            $payloadName = "$node-state.txt"
            Set-Content -Encoding utf8 -LiteralPath (Join-Path $archiveDir $payloadName) -Value "verified backup for $node"
            & tar -czf (Join-Path $archiveDir $archiveName) -C $archiveDir $payloadName
            $tarExitCode = $LASTEXITCODE
            Remove-Item -LiteralPath (Join-Path $archiveDir $payloadName) -Force
            $global:LASTEXITCODE = $tarExitCode
            Set-Variable -Name LASTEXITCODE -Value $tarExitCode -Scope 1
            return
        }
        $verifyIndex = [Array]::IndexOf($arguments, "-tzf")
        if ($backupMount -and $verifyIndex -ge 0) {
            $archiveName = [IO.Path]::GetFileName($arguments[$verifyIndex + 1])
            & tar -tzf (Join-Path $archiveDir $archiveName) | Out-Null
            $tarExitCode = $LASTEXITCODE
            $global:LASTEXITCODE = $tarExitCode
            Set-Variable -Name LASTEXITCODE -Value $tarExitCode -Scope 1
            if ($tarExitCode -ne 0) { throw "fake docker could not read backup archive: $(Join-Path $archiveDir $archiveName)" }
        }
        return
    }

    if ($arguments -contains "records" -and $arguments -contains "list") {
        return [ordered]@{
            records = @([ordered]@{
                sourceV1 = "local"
                nodeFacts = [ordered]@{
                    principal = "p1_fixture"
                    endpoints = @("/ip4/127.0.0.1/tcp/60000/p2p/fixture-peer")
                }
            })
        } | ConvertTo-Json -Depth 6
    }
    if ($arguments -contains "network" -and $arguments -contains "status") {
        return [ordered]@{ network = [ordered]@{ state = "ready"; joined = $true } } | ConvertTo-Json -Depth 3
    }
    if ($arguments -contains "ps" -and $arguments -contains "-aq") {
        return "$($arguments[-1])-container"
    }
}

function Save-Evidence([string]$Prefix, [string]$StatePath) {
    $global:FakeDockerEvents | ConvertTo-Json | Set-Content -Encoding utf8 -LiteralPath (Join-Path $reportPath "$Prefix-events.json")
    $backupEvidencePath = Join-Path $reportPath $Prefix
    New-Item -ItemType Directory -Force -Path $backupEvidencePath | Out-Null
    $backups = [Collections.Generic.List[object]]::new()
    $validations = [Collections.Generic.List[object]]::new()
    Get-ChildItem -LiteralPath (Join-Path $StatePath "backups") -Filter "*.tar.gz.json" -ErrorAction SilentlyContinue |
        ForEach-Object {
            $archivePath = $_.FullName.Substring(0, $_.FullName.Length - ".json".Length)
            $entries = @(& tar -tzf $archivePath)
            if ($LASTEXITCODE -ne 0) { throw "backup evidence is not a readable tar archive: $archivePath" }
            $manifest = Get-Content -Raw -LiteralPath $_.FullName | ConvertFrom-Json
            $actualHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $archivePath).Hash.ToLowerInvariant()
            if ($actualHash -ne $manifest.archive_sha256) {
                throw "backup evidence checksum mismatch: $archivePath"
            }
            Copy-Item -LiteralPath $_.FullName -Destination $backupEvidencePath
            $backups.Add($manifest)
            $validations.Add([ordered]@{
                node = $manifest.node
                archive_sha256 = $actualHash
                archive_size = (Get-Item -LiteralPath $archivePath).Length
                readable_tar = $true
                entries = $entries
            })
        }
    $backups | ConvertTo-Json -Depth 6 | Set-Content -Encoding utf8 -LiteralPath (Join-Path $reportPath "$Prefix-backups.json")
    $validations | ConvertTo-Json -Depth 6 | Set-Content -Encoding utf8 -LiteralPath (Join-Path $reportPath "$Prefix-backup-validation.json")
    return @($backups)
}

try {
    New-Item -ItemType Directory -Force -Path $stageRoot, $bundleRoot | Out-Null
    foreach ($entry in Get-Content -LiteralPath (Join-Path $root "scripts/release/distribution-files.txt")) {
        if ([string]::IsNullOrWhiteSpace($entry)) { continue }
        $parts = $entry -split "\|", 2
        if ($parts.Count -ne 2 -or [IO.Path]::IsPathRooted($parts[1]) -or $parts[1] -match "(^|/)\.\.(/|$)") {
            throw "invalid distribution file mapping: $entry"
        }
        $destination = Join-Path $stageRoot $parts[1]
        New-Item -ItemType Directory -Force -Path ([IO.Path]::GetDirectoryName($destination)) | Out-Null
        Copy-Item -LiteralPath (Join-Path $root $parts[0]) -Destination $destination
    }
    & tar -czf $bundleArchive -C $stageRoot .
    if ($LASTEXITCODE -ne 0) { throw "could not create clean distribution fixture" }
    & tar -xzf $bundleArchive -C $bundleRoot
    if ($LASTEXITCODE -ne 0) { throw "could not extract clean distribution fixture" }

    Write-ClusterFixture $positiveState
    $global:FakeDockerEvents.Clear()
    $global:FakeBackupFailureNode = ""
    & (Join-Path $bundleRoot "ardents.ps1") upgrade `
        -StateDir $positiveState `
        -Project "ardents-bundle-test" `
        -NewImage "registry.example/ardents@sha256:$("2" * 64)"

    $positiveBackups = @(Save-Evidence "positive" $positiveState)
    if ($positiveBackups.Count -ne 3) { throw "positive upgrade produced $($positiveBackups.Count) verified backups, want 3" }
    $firstRecreate = -1
    for ($index = 0; $index -lt $global:FakeDockerEvents.Count; $index++) {
        if ($global:FakeDockerEvents[$index] -match "compose .* create .*--force-recreate") { $firstRecreate = $index; break }
    }
    if ($firstRecreate -lt 0) { throw "positive upgrade never reached node recreation" }
    $archivesBeforeMutation = @($global:FakeDockerEvents[0..($firstRecreate - 1)] | Where-Object { $_ -match "^run .* -czf " })
    if ($archivesBeforeMutation.Count -ne 3) {
        throw "upgrade created $($archivesBeforeMutation.Count) backups before first recreation, want 3"
    }

    Write-ClusterFixture $negativeState
    $global:FakeDockerEvents.Clear()
    $global:FakeBackupFailureNode = "peer2"
    $failure = $null
    try {
        & (Join-Path $bundleRoot "ardents.ps1") upgrade `
            -StateDir $negativeState `
            -Project "ardents-bundle-test" `
            -NewImage "registry.example/ardents@sha256:$("2" * 64)"
    } catch {
        $failure = $_.Exception.Message
    }
    $negativeBackups = @(Save-Evidence "negative" $negativeState)
    if ([string]::IsNullOrWhiteSpace($failure) -or $failure -notmatch "backup") {
        throw "injected backup failure did not fail the upgrade with a backup error"
    }
    if ($global:FakeDockerEvents | Where-Object { $_ -match "compose .* create .*--force-recreate" }) {
        throw "upgrade recreated a node after an injected backup failure"
    }
    if ($negativeBackups.Count -ne 1 -or $negativeBackups[0].node -ne "seed") {
        throw "negative upgrade did not preserve the completed seed backup evidence"
    }

    @(
        "deployment-upgrade-backup-gate=passed"
        "positive_backups=$($positiveBackups.Count)"
        "negative_backups=$($negativeBackups.Count)"
        "failure=$failure"
    ) | Set-Content -Encoding utf8 -LiteralPath (Join-Path $reportPath "passed.txt")
    Write-Host "deployment-upgrade-backup-gate=passed"
}
finally {
    Remove-Item Function:\global:docker -ErrorAction SilentlyContinue
    Set-Location $root
    $resolvedTestRoot = [IO.Path]::GetFullPath($testRoot)
    $resolvedTempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
    if ($resolvedTestRoot.StartsWith($resolvedTempRoot, [StringComparison]::OrdinalIgnoreCase) -and
        (Test-Path -LiteralPath $resolvedTestRoot)) {
        Remove-Item -LiteralPath $resolvedTestRoot -Recurse -Force
    }
}
