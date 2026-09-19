#Requires -Version 7.0
param(
    [Parameter(Mandatory = $true)][string]$PublisherHost,
    [Parameter(Mandatory = $true)][string]$ReaderHost,
    [Parameter(Mandatory = $true)][string]$SSHKey,
    [Parameter(Mandatory = $true)][string]$RunnerBinary,
    [Parameter(Mandatory = $true)][string]$WorkerBinary,
    [Parameter(Mandatory = $true)][string]$RelayBinary,
    [Parameter(Mandatory = $true)][string]$NodeBinary,
    [Parameter(Mandatory = $true)][string]$SourceCommit,
    [Parameter(Mandatory = $true)][string]$NodeInventory,
    [Parameter(Mandatory = $true)][string]$PublisherPlan,
    [Parameter(Mandatory = $true)][string]$ReaderPlanTemplate,
    [Parameter(Mandatory = $true)][string]$AuthorityInventory,
    [Parameter(Mandatory = $true)][string]$NetworkManifest,
    [Parameter(Mandatory = $true)][string]$EvidenceOutput,
    [ValidateRange(0,120)][int]$SmokeSeconds = 0,
    [string]$User = 'root'
)

$ErrorActionPreference = 'Stop'
$repository = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..\..'))
. (Join-Path $PSScriptRoot 'admission-window.ps1')
$packageRoot = Join-Path $repository 'packaging\stream-qualification-worker'
$recoveryFaultPath = Join-Path $PSScriptRoot 'recovery_faults.py'
$nodeSamplerPath = Join-Path $PSScriptRoot 'node_owner_samples.py'
$relaySamplerPath = Join-Path $PSScriptRoot 'relay_owner_samples.py'
$clockObserverPath = Join-Path $PSScriptRoot 'clock_observer.py'
$relayImage = 'golang@sha256:3c3e25a4da13fd0478eed2df1eb35a0e667094a7124d3993a6a1d30f71c17e79'
$requiredPackage = @(
    'install.py', 'install_endpoint.py', '50-ardents-stream-qualification.rules', 'ardents-endpoint.service',
    'ardents-stream-qualification-reader.socket', 'ardents-stream-qualification-reader@.service',
    'ardents-stream-qualification-publisher.socket', 'ardents-stream-qualification-publisher@.service'
)

function Resolve-InputFile([string]$Path, [string]$Name) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { throw "$Name is absent." }
    return (Resolve-Path -LiteralPath $Path).Path
}
function Assert-Host([string]$Value, [string]$Name) {
    if ($Value -notmatch '^(?:[A-Za-z0-9](?:[A-Za-z0-9.-]{0,251}[A-Za-z0-9])?|(?:\d{1,3}\.){3}\d{1,3})$') { throw "$Name is invalid." }
}
function Assert-RemotePath([string]$Value, [string]$Name) {
    if ($Value -notmatch '^/[A-Za-z0-9._/-]+$' -or $Value.Contains('//') -or $Value.Contains('/../')) { throw "$Name is not a canonical safe remote path." }
}
function Assert-Hex([string]$Value, [string]$Name) {
    if ($Value -cnotmatch '^[0-9a-f]{64}$') { throw "$Name must be lowercase SHA-256-width hex." }
}
function Write-Utf8([string]$Path, [string]$Body) {
    [IO.File]::WriteAllText($Path, $Body, [Text.UTF8Encoding]::new($false))
}
function Convert-Digest([object]$Value, [string]$Name) {
    $bytes = @($Value)
    if ($bytes.Count -ne 32) { throw "$Name is not a 32-byte observation." }
    $hex = -join ($bytes | ForEach-Object {
        $number = [int]$_
        if ($number -lt 0 -or $number -gt 255) { throw "$Name contains an invalid byte." }
        $number.ToString('x2')
    })
    Assert-Hex $hex $Name
    return $hex
}
function Save-NativeFailure([string]$Label, [string]$Detail) {
    if (-not $script:evidence -or -not (Test-Path -LiteralPath $script:evidence -PathType Container)) { return }
    $safe = ($Label -replace '[^A-Za-z0-9._-]', '-').Trim('-')
    if ($safe.Length -gt 80) { $safe = $safe.Substring(0, 80) }
    Write-Utf8 (Join-Path $script:evidence "native-failure-$safe-$([Guid]::NewGuid().ToString('N')).txt") ($Detail + [Environment]::NewLine)
}
function Invoke-Native([string]$Program, [string[]]$Arguments, [string]$Label, [int]$TimeoutSeconds = 180, [string[]]$InputLines = $null) {
    $start = [Diagnostics.ProcessStartInfo]::new()
    $start.FileName = $Program
    $start.UseShellExecute = $false
    $start.RedirectStandardOutput = $true
    $start.RedirectStandardError = $true
    $start.RedirectStandardInput = $null -ne $InputLines
    foreach ($argument in $Arguments) { [void]$start.ArgumentList.Add($argument) }
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = $start
    if (-not $process.Start()) { throw "$Label did not start." }
    $stdout = $process.StandardOutput.ReadToEndAsync()
    $stderr = $process.StandardError.ReadToEndAsync()
    if ($null -ne $InputLines) {
        $process.StandardInput.WriteLine(($InputLines -join "`n"))
        $process.StandardInput.Close()
    }
    if (-not $process.WaitForExit($TimeoutSeconds * 1000)) {
        $process.Kill($true)
        $process.WaitForExit()
        $detail = "$Label exceeded its $TimeoutSeconds-second local deadline."
        $captured = $stdout.Result + $stderr.Result
        if ($captured.Length -gt 65536) { $captured = $captured.Substring($captured.Length - 65536) }
        if (-not [string]::IsNullOrWhiteSpace($captured)) { $detail += [Environment]::NewLine + $captured }
        Save-NativeFailure $Label $detail
        throw $detail
    }
    $output = @((($stdout.Result + $stderr.Result) -split "`r?`n") | Where-Object { $_ -ne '' })
    if ($process.ExitCode -ne 0) {
        $detail = "$Label failed with exit code $($process.ExitCode): $($output -join [Environment]::NewLine)"
        Save-NativeFailure $Label $detail
        throw $detail
    }
    return $output
}

Assert-Host $PublisherHost 'PublisherHost'
Assert-Host $ReaderHost 'ReaderHost'
Assert-Host $User 'User'
$keyPath = Resolve-InputFile $SSHKey 'SSHKey'
$runnerPath = Resolve-InputFile $RunnerBinary 'RunnerBinary'
$workerPath = Resolve-InputFile $WorkerBinary 'WorkerBinary'
$relayPath = Resolve-InputFile $RelayBinary 'RelayBinary'
$nodePath = Resolve-InputFile $NodeBinary 'NodeBinary'
if ($SourceCommit -cnotmatch '^[0-9a-f]{40}$') { throw 'SourceCommit must be a full lowercase Git commit.' }
$actualCommit = ((& git -C $repository rev-parse HEAD 2>$null) -join '').Trim()
if ($LASTEXITCODE -ne 0 -or $actualCommit -cne $SourceCommit) { throw 'SourceCommit must equal the checked-out candidate commit.' }
$sourceStatus = @(& git -C $repository status --porcelain --untracked-files=normal)
if ($LASTEXITCODE -ne 0 -or $sourceStatus.Count -ne 0) { throw 'Qualification requires a clean source commit.' }
$nodeInventoryPath = Resolve-InputFile $NodeInventory 'NodeInventory'
$publisherPlanPath = Resolve-InputFile $PublisherPlan 'PublisherPlan'
$readerTemplatePath = Resolve-InputFile $ReaderPlanTemplate 'ReaderPlanTemplate'
$authorityPath = Resolve-InputFile $AuthorityInventory 'AuthorityInventory'
$secretPath = Resolve-InputFile (Join-Path (Split-Path -Parent $authorityPath) 'admission-secret.dpapi') 'protected admission secret'
$protectedSecret = [IO.File]::ReadAllBytes($secretPath)
$plainSecret = [Security.Cryptography.ProtectedData]::Unprotect(
    $protectedSecret, $null, [Security.Cryptography.DataProtectionScope]::CurrentUser)
try { $admissionSecret = [Text.Encoding]::UTF8.GetString($plainSecret) }
finally { [Array]::Clear($plainSecret, 0, $plainSecret.Length) }
$networkManifestPath = Resolve-InputFile $NetworkManifest 'NetworkManifest'
foreach ($name in $requiredPackage) { [void](Resolve-InputFile (Join-Path $packageRoot $name) "package/$name") }
[void](Resolve-InputFile $recoveryFaultPath 'recovery_faults.py')
[void](Resolve-InputFile $nodeSamplerPath 'node_owner_samples.py')
[void](Resolve-InputFile $relaySamplerPath 'relay_owner_samples.py')
[void](Resolve-InputFile $clockObserverPath 'clock_observer.py')

$evidence = [IO.Path]::GetFullPath($EvidenceOutput)
if ($evidence.StartsWith($repository + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'EvidenceOutput must remain outside the repository.'
}
if (Test-Path -LiteralPath $evidence) { throw 'EvidenceOutput must be a new directory.' }
[IO.Directory]::CreateDirectory($evidence) | Out-Null

$authority = Get-Content -LiteralPath $authorityPath -Raw | ConvertFrom-Json
foreach ($field in @('Host', 'Binary', 'VaultRoot', 'RecordID', 'EnvironmentCommitment', 'NetworkCommitment', 'RootCommitment', 'IDCommitment')) {
    if (-not $authority.PSObject.Properties[$field] -or [string]::IsNullOrWhiteSpace([string]$authority.$field)) { throw "Authority inventory lacks $field." }
}
Assert-Host $authority.Host 'authority Host'
foreach ($field in @('Binary', 'VaultRoot')) { Assert-RemotePath ([string]$authority.$field) "authority $field" }
if ([string]$authority.RecordID -notmatch '^[A-Za-z0-9._-]{1,160}$') { throw 'authority RecordID is invalid.' }
foreach ($field in @('EnvironmentCommitment', 'NetworkCommitment', 'RootCommitment', 'IDCommitment')) { Assert-Hex ([string]$authority.$field) "authority $field" }

$publisherPlanObject = Get-Content -LiteralPath $publisherPlanPath -Raw | ConvertFrom-Json
$readerPlanObject = Get-Content -LiteralPath $readerTemplatePath -Raw | ConvertFrom-Json
if (($publisherPlanObject.Mode -and [string]$publisherPlanObject.Mode -ne 'stream') -or ($readerPlanObject.Mode -and [string]$readerPlanObject.Mode -ne 'stream')) { throw 'Two-host workload plans must use stream mode.' }
if (@($publisherPlanObject.Participants).Count -ne 1 -or [int]$publisherPlanObject.Participants[0].Role -ne 2) { throw 'Publisher plan must contain one Publisher.' }
if (@($readerPlanObject.Participants).Count -ne 4) { throw 'Reader plan must contain four Readers.' }
for ($index = 0; $index -lt 4; $index++) {
    $item = $readerPlanObject.Participants[$index]
    if ([int]$item.Role -ne 1 -or [int]$item.ReaderIndex -ne $index -or -not [string]::IsNullOrEmpty([string]$item.Link)) {
        throw 'Reader plan template must contain ordered Reader indexes and empty Links.'
    }
}
$profile = [int]$publisherPlanObject.Participants[0].Profile
$condition = [int]$publisherPlanObject.Participants[0].Condition
$seed = [string]$publisherPlanObject.Participants[0].Seed
foreach ($item in @($readerPlanObject.Participants)) {
    if ([int]$item.Profile -ne $profile -or [int]$item.Condition -ne $condition -or [string]$item.Seed -cne $seed) { throw 'The two owner plans do not share profile, condition and seed.' }
}
Assert-Hex $seed 'workload seed'
$networkManifestObject = Get-Content -LiteralPath $networkManifestPath -Raw | ConvertFrom-Json
$expectedCells = switch ($condition) { 1 { @('net14ad') } 2 { @('net14s') } 3 { @('net14-recovery') } default { throw 'Plan network condition is invalid.' } }
if ($expectedCells -cnotcontains [string]$networkManifestObject.Cell) { throw 'Network manifest cell does not match the owner plans.' }
if (@($networkManifestObject.SeedSchedule) -cnotcontains $seed) { throw 'Workload seed is absent from the immutable network manifest schedule.' }

$nodeInventoryObject = Get-Content -LiteralPath $nodeInventoryPath -Raw | ConvertFrom-Json
if (@($nodeInventoryObject.PSObject.Properties.Name).Count -ne 3 -or -not $nodeInventoryObject.PSObject.Properties['Schema'] -or
    -not $nodeInventoryObject.PSObject.Properties['Nodes'] -or -not $nodeInventoryObject.PSObject.Properties['Sources'] -or
    [string]$nodeInventoryObject.Schema -cne 'ardents-qualification-node-inventory-v2' -or
    @($nodeInventoryObject.Nodes).Count -ne 16 -or @($nodeInventoryObject.Sources).Count -ne 2) {
    throw 'Node inventory is not the exact sixteen-Node and two-Source qualification inventory.'
}
$manifestNodeIDs = @($networkManifestObject.Paths | ForEach-Object { @($_.Segments) } | ForEach-Object { @([string]$_.From, [string]$_.To) } | Where-Object { $_ -cne 'user' -and $_ -cne 'publisher' } | Sort-Object -Unique)
if ($manifestNodeIDs.Count -ne 5) { throw 'Network manifest does not name five intermediate Nodes.' }
$forwardPath = @($networkManifestObject.Paths | Where-Object { [string]$_.Name -ceq 'user-to-publisher' })
if ($forwardPath.Count -ne 1 -or @($forwardPath[0].Segments).Count -ne 6) { throw 'Network manifest has no unique six-link forward path.' }
$manifestNodeTargets = @{}
$forwardRelays = @{}
foreach ($relay in @($networkManifestObject.Relays)) {
    $segmentIndex = [Array]::IndexOf(@($forwardPath[0].Segments | ForEach-Object { [string]$_.ID }), [string]$relay.UpstreamSegment)
    if ($segmentIndex -lt 0 -or $segmentIndex -gt 5 -or $forwardRelays.ContainsKey($segmentIndex)) {
        throw 'Network relay does not own one forward physical segment.'
    }
    $forwardRelays[$segmentIndex] = $relay
    $segment = $forwardPath[0].Segments[$segmentIndex]
    $targetID = if ($segmentIndex -lt 3) { [string]$segment.To } else { [string]$segment.From }
    $binding = [ordered]@{ Host=[string]$relay.Host; Target=[string]$relay.Target }
    if ($manifestNodeTargets.ContainsKey($targetID)) {
        $current = $manifestNodeTargets[$targetID]
        if ([string]$current.Host -cne [string]$binding.Host -or [string]$current.Target -cne [string]$binding.Target) {
            throw "Manifest Node $targetID has inconsistent physical relay targets."
        }
    } else {
        $manifestNodeTargets[$targetID] = $binding
    }
}
if ($forwardRelays.Count -ne 6 -or $manifestNodeTargets.Count -ne 5) { throw 'Network manifest does not bind all six links and five Route positions.' }
$manifestCarrierRelays = @{}
$serviceInteriorID = [string]$forwardPath[0].Segments[3].To
$manifestCarrierRelays[$serviceInteriorID] = [string]$forwardRelays[3].PublicEndpoint
$manifestNodeDuties = @{}
for ($index = 0; $index -lt 5; $index++) {
    $id = [string]$forwardPath[0].Segments[$index].To
    $manifestNodeDuties[$id] = if ($index -eq 2) { 'closed_data_join' } else { 'closed_forwarding' }
}
$inventorySources = @()
$seenSourceHosts = @{}
$seenSourceIDs = @{}
$sourceDirectory = Split-Path -Parent $nodeInventoryPath
foreach ($item in @($nodeInventoryObject.Sources)) {
    if (@($item.PSObject.Properties.Name).Count -ne 4 -or -not $item.PSObject.Properties['ID'] -or
        -not $item.PSObject.Properties['Host'] -or -not $item.PSObject.Properties['Endpoint'] -or -not $item.PSObject.Properties['Plan']) {
        throw 'State Source inventory entry has unknown or missing fields.'
    }
    $id = [string]$item.ID
    Assert-Hex $id 'State Source ID'
    if ($seenSourceIDs.ContainsKey($id)) { throw 'State Source inventory ID is duplicated.' }
    $seenSourceIDs[$id] = $true
    $role = [string]$item.Host
    if (($role -cne 'reader' -and $role -cne 'publisher') -or $seenSourceHosts.ContainsKey($role)) {
        throw 'State Source inventory must contain one Source on each owner host.'
    }
    $seenSourceHosts[$role] = $true
    $endpoint = [string]$item.Endpoint
    $expectedHost = if ($role -ceq 'reader') { $ReaderHost } else { $PublisherHost }
    if ($endpoint -notmatch '^((?:[0-9]{1,3}.){3}[0-9]{1,3}):([1-9][0-9]{0,4})$' -or
        $Matches[1] -cne $expectedHost -or [int]$Matches[2] -gt 65535) {
        throw "State Source $id endpoint differs from its selected host."
    }
    $port = [int]$Matches[2]
    $declaredPlan = [string]$item.Plan
    $candidatePlan = if ([IO.Path]::IsPathRooted($declaredPlan)) { $declaredPlan } else { Join-Path $sourceDirectory $declaredPlan }
    $resolvedPlan = Resolve-InputFile $candidatePlan "State Source $id plan"
    $planObject = Get-Content -LiteralPath $resolvedPlan -Raw | ConvertFrom-Json
    if ([string]$planObject.schema -cne 'ardents-source-server-v1' -or [string]$planObject.state_profile -cne 'ardents-route-v3' -or
        [string]$planObject.listen -cne $endpoint) {
        throw "State Source $id plan is not the exact closed Route source binding."
    }
    $inventorySources += [ordered]@{ ID=$id; Host=$role; Endpoint=$endpoint; Plan=$resolvedPlan; PlanSHA256=(Get-FileHash -LiteralPath $resolvedPlan -Algorithm SHA256).Hash.ToLowerInvariant() }
}
if ($seenSourceHosts.Count -ne 2) { throw 'State Source inventory does not cover both owner hosts.' }
$inventoryNodes = @()
$seenNodeIDs = @{}
$dutyNames = @('closed_issuer', 'closed_forwarding', 'closed_resolution', 'closed_introduction', 'closed_data_join')
$dutyCounts = @{}
foreach ($name in $dutyNames) { $dutyCounts[$name] = 0 }
$inventoryDirectory = Split-Path -Parent $nodeInventoryPath
foreach ($item in @($nodeInventoryObject.Nodes)) {
    if (@($item.PSObject.Properties.Name).Count -ne 3 -or -not $item.PSObject.Properties['ID'] -or -not $item.PSObject.Properties['Host'] -or -not $item.PSObject.Properties['Plan']) { throw 'Node inventory entry has unknown or missing fields.' }
    $id = [string]$item.ID
    Assert-Hex $id 'inventory Node ID'
    if ($seenNodeIDs.ContainsKey($id) -or $seenSourceIDs.ContainsKey($id)) { throw 'Node inventory ID is duplicated or collides with a State Source.' }
    $seenNodeIDs[$id] = $true
    $role = [string]$item.Host
    if ($role -cne 'reader' -and $role -cne 'publisher') { throw 'Node inventory Host must be reader or publisher.' }
    $declaredPlan = [string]$item.Plan
    $candidatePlan = if ([IO.Path]::IsPathRooted($declaredPlan)) { $declaredPlan } else { Join-Path $inventoryDirectory $declaredPlan }
    $resolvedPlan = Resolve-InputFile $candidatePlan "Node $id plan"
    $planObject = Get-Content -LiteralPath $resolvedPlan -Raw | ConvertFrom-Json
    $boundSources = @($planObject.sources)
    if ($boundSources.Count -ne 2) { throw "Node $id does not bind the two retained State Sources." }
    foreach ($source in $inventorySources) {
        $matches = @($boundSources | Where-Object { [string]$_.identity -ceq [string]$source.ID -and [string]$_.address -ceq [string]$source.Endpoint })
        if ($matches.Count -ne 1) { throw "Node $id does not bind State Source $($source.ID)." }
    }
    $selectedDuties = @($dutyNames | Where-Object { $planObject.PSObject.Properties[$_] -and $null -ne $planObject.$_ })
    if ($selectedDuties.Count -ne 1) { throw "Node $id plan must reserve exactly one closed duty." }
    $dutyCounts[$selectedDuties[0]]++
    $declaredCarrierRelay = ''
    if ($selectedDuties[0] -ceq 'closed_forwarding') {
        $declaredCarrierRelay = [string]$planObject.closed_forwarding.carrier_relay_endpoint
    }
    $expectedCarrierRelay = if ($manifestCarrierRelays.ContainsKey($id)) { [string]$manifestCarrierRelays[$id] } else { '' }
    if ($declaredCarrierRelay -cne $expectedCarrierRelay) {
        throw "Node $id does not bind its exact outgoing physical relay."
    }
    if ($manifestNodeDuties.ContainsKey($id) -and [string]$selectedDuties[0] -cne [string]$manifestNodeDuties[$id]) {
        throw "Manifest Node $id reserves the wrong Route duty."
    }
    $ownerHostingRoot = if ($role -ceq 'reader') { [string]$readerPlanObject.Participants[0].HostingRoot } else { [string]$publisherPlanObject.Participants[0].HostingRoot }
    if ([string]$planObject.schema -cne 'ardents-node-plan-v1' -or [string]$planObject.node_id -cne $id -or [string]$planObject.hosting_root -cne $ownerHostingRoot) { throw "Node $id plan identity or shared hosting owner differs from inventory." }
    if ($manifestNodeTargets.ContainsKey($id)) {
        $target = $manifestNodeTargets[$id]
        if ($role -cne [string]$target.Host -or [string]$planObject.closed_listen_private_override -cne [string]$target.Target) {
            throw "Manifest Node $id does not bind its exact local relay target and host."
        }
    } elseif (-not [string]::IsNullOrEmpty([string]$planObject.closed_listen_private_override)) {
        throw "Non-path Node $id cannot select a relay listen override."
    }
    if ($selectedDuties[0] -ceq 'closed_forwarding' -or $selectedDuties[0] -ceq 'closed_data_join') {
        if ([string]$planObject.($selectedDuties[0]).hosting_root -cne $ownerHostingRoot) { throw "Node $id duty does not use its Endpoint owner hosting period." }
    }
    $inventoryNodes += [ordered]@{ ID=$id; Host=$role; Plan=$resolvedPlan; PlanSHA256=(Get-FileHash -LiteralPath $resolvedPlan -Algorithm SHA256).Hash.ToLowerInvariant() }
}
foreach ($id in $manifestNodeIDs) { if (-not $seenNodeIDs.ContainsKey($id)) { throw "Manifest Node $id is absent from inventory." } }
$expectedDuties = @{ closed_issuer=1; closed_forwarding=12; closed_resolution=1; closed_introduction=1; closed_data_join=1 }
foreach ($name in $dutyNames) { if ($dutyCounts[$name] -ne $expectedDuties[$name]) { throw "Qualification inventory has $($dutyCounts[$name]) $name duties; expected $($expectedDuties[$name])." } }
$ssh = (Get-Command ssh.exe -CommandType Application).Source
$scp = (Get-Command scp.exe -CommandType Application).Source
$knownHosts = Join-Path $evidence 'ssh-known-hosts'
$sshOptions = @('-i', $keyPath, '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=15', '-o', 'ConnectionAttempts=1', '-o', "UserKnownHostsFile=$knownHosts", '-o', 'StrictHostKeyChecking=accept-new')
$scpOptions = @('-i', $keyPath, '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=15', '-o', "UserKnownHostsFile=$knownHosts", '-o', 'StrictHostKeyChecking=accept-new')
$attempt = [Guid]::NewGuid().ToString('N')
$remoteRoot = "/var/tmp/ardents-qualification-$attempt"
Assert-RemotePath $remoteRoot 'remote attempt root'
[IO.File]::Copy($publisherPlanPath, (Join-Path $evidence 'publisher-plan.installed.json'))
[IO.File]::Copy($authorityPath, (Join-Path $evidence 'authority-public-inventory.json'))
[IO.File]::Copy($networkManifestPath, (Join-Path $evidence 'network-manifest.json'))
[IO.File]::Copy($nodeInventoryPath, (Join-Path $evidence 'node-inventory.json'))
$inputFiles = [ordered]@{}
foreach ($entry in @(@{Name='runner'; Path=$runnerPath}, @{Name='worker'; Path=$workerPath}, @{Name='relay'; Path=$relayPath}, @{Name='node'; Path=$nodePath}, @{Name='node_inventory'; Path=$nodeInventoryPath}, @{Name='publisher_plan'; Path=$publisherPlanPath}, @{Name='reader_plan_template'; Path=$readerTemplatePath}, @{Name='network_manifest'; Path=$networkManifestPath})) {
    $inputFiles[$entry.Name] = (Get-FileHash -LiteralPath $entry.Path -Algorithm SHA256).Hash.ToLowerInvariant()
}
foreach ($nodeInput in $inventoryNodes) { $inputFiles["node_plan/$($nodeInput.ID)"] = $nodeInput.PlanSHA256 }
foreach ($sourceInput in $inventorySources) { $inputFiles["source_plan/$($sourceInput.ID)"] = $sourceInput.PlanSHA256 }
$inputFiles.relay_owner_samples = (Get-FileHash -LiteralPath $relaySamplerPath -Algorithm SHA256).Hash.ToLowerInvariant()
$inputFiles.node_owner_samples = (Get-FileHash -LiteralPath $nodeSamplerPath -Algorithm SHA256).Hash.ToLowerInvariant()
$inputFiles.clock_observer = (Get-FileHash -LiteralPath $clockObserverPath -Algorithm SHA256).Hash.ToLowerInvariant()
$inputFiles.recovery_faults = (Get-FileHash -LiteralPath $recoveryFaultPath -Algorithm SHA256).Hash.ToLowerInvariant()
foreach ($name in $requiredPackage) { $inputFiles["package/$name"] = (Get-FileHash -LiteralPath (Join-Path $packageRoot $name) -Algorithm SHA256).Hash.ToLowerInvariant() }
$attemptMode = if ($SmokeSeconds -gt 0) { 'smoke' } else { 'acceptance' }
Write-Utf8 (Join-Path $evidence 'input-digests.json') (([ordered]@{ Schema='ardents-qualification-two-host-inputs-v1'; SourceCommit=$SourceCommit; Mode=$attemptMode; SmokeSeconds=$SmokeSeconds; Files=$inputFiles } | ConvertTo-Json -Depth 5 -Compress) + "`n")

function Remote([string]$HostName) { return "$User@$HostName" }
function Invoke-SSH([string]$HostName, [string]$Command, [string]$Label) {
    return @(Invoke-Native $ssh ($sshOptions + @('-n', (Remote $HostName), $Command)) $Label)
}
function Send-File([string]$Source, [string]$HostName, [string]$Destination, [string]$Label) {
    Invoke-Native $scp ($scpOptions + @($Source, "$(Remote $HostName):$Destination")) $Label
}
function Receive-File([string]$HostName, [string]$Source, [string]$Destination, [string]$Label) {
    Invoke-Native $scp ($scpOptions + @("$(Remote $HostName):$Source", $Destination)) $Label
}
function Deploy-Owner([string]$HostName, [string]$PlanPath) {
    [void](Invoke-SSH $HostName "install -d -m 700 '$remoteRoot' '$remoteRoot/package'" 'create remote staging')
    Send-File $runnerPath $HostName "$remoteRoot/runner" 'upload runner'
    Send-File $workerPath $HostName "$remoteRoot/worker" 'upload worker'
    Send-File $PlanPath $HostName "$remoteRoot/plan.json" 'upload plan'
    Send-File $networkManifestPath $HostName "$remoteRoot/network-manifest.json" 'upload network manifest'
    foreach ($name in $requiredPackage) { Send-File (Join-Path $packageRoot $name) $HostName "$remoteRoot/package/$name" "upload $name" }
    $command = "set -eu; chmod 700 '$remoteRoot/package/install.py' '$remoteRoot/package/install_endpoint.py'; systemctl stop ardents-endpoint.service 2>/dev/null || :; python3 '$remoteRoot/package/install_endpoint.py' '$remoteRoot/runner' '$remoteRoot/plan.json'; python3 '$remoteRoot/package/install.py' '$remoteRoot/worker'; systemctl daemon-reload; state=`$(systemctl show ardents-endpoint.service -p ActiveState --value); if test `"`$state`" = failed; then systemctl reset-failed ardents-endpoint.service; else test `"`$state`" = inactive; fi"
    [void](Invoke-SSH $HostName $command 'install owner artifacts')
}
function Assert-TransientOwner([string]$HostName, [string]$Unit, [string]$Label) {
    $properties = @{}
    foreach ($line in @(Invoke-SSH $HostName "systemctl show '$Unit' -p User -p Group" "verify $Label owner")) {
        if ($line -match '^([^=]+)=(.*)$') { $properties[$Matches[1]] = $Matches[2] }
    }
    if ([string]$properties.User -cne 'ardents-endpoint' -or [string]$properties.Group -cne 'ardents-endpoint') {
        throw "$Label does not run as the shared qualification owner."
    }
}
function Verify-NetworkManifest([string]$HostName, [string]$EvidenceName) {
    $lines = @(Invoke-SSH $HostName "chmod 700 '$remoteRoot/runner'; '$remoteRoot/runner' verify-network-manifest '$remoteRoot/network-manifest.json'" 'verify immutable network manifest')
    if ($lines.Count -ne 1) { throw 'Network manifest verifier returned an invalid evidence record.' }
    $record = $lines[0] | ConvertFrom-Json
    $expected = $inputFiles.network_manifest
    if ([string]$record.Kind -cne 'network-manifest' -or [string]$record.SHA256 -cne $expected -or [int]$record.Segments -ne 12 -or [int]$record.Relays -ne 6) {
        throw 'Network manifest verifier did not bind the exact six-link input.'
    }
    Write-Utf8 (Join-Path $evidence $EvidenceName) ($lines[0] + "`n")
}
function Invoke-OwnerPreflight([string]$HostName, [string]$ExpectedPlanSHA256, [int]$ExpectedParticipants, [string]$EvidenceName) {
    $lines = @(Invoke-SSH $HostName "runuser -u ardents-endpoint -- /usr/lib/ardents/qualification/ardents-qualification preflight /etc/ardents/qualification-plan.json" 'preflight installed qualification owner')
    if ($lines.Count -ne 1) { throw "Qualification preflight on $HostName returned an invalid record." }
    $record = $lines[0] | ConvertFrom-Json
    if ([string]$record.Kind -cne 'qualification-preflight' -or [string]$record.PlanSHA256 -cne $ExpectedPlanSHA256 -or [int]$record.Participants -ne $ExpectedParticipants -or @($record.Selections).Count -ne $ExpectedParticipants) {
        throw "Qualification preflight on $HostName did not bind the installed owner plan and retained Route."
    }
    Write-Utf8 (Join-Path $evidence $EvidenceName) ($lines[0] + "`n")
    return $record
}
function Assert-RetainedManifestPath([object]$PublisherPreflight, [object]$ReaderPreflight) {
    $segments = @($forwardPath[0].Segments)
    $readerEntry = [string]$segments[0].To
    $readerInterior = [string]$segments[1].To
    $publisherInterior = [string]$segments[3].To
    $publisherEntry = [string]$segments[4].To
    foreach ($selection in @($ReaderPreflight.Selections)) {
        if ([int]$selection.Role -ne 1 -or
            (Convert-Digest $selection.EntryNodeID 'reader retained Entry Node') -cne $readerEntry -or
            (Convert-Digest $selection.InteriorNodeID 'reader retained Interior Node') -cne $readerInterior) {
            throw 'Reader preflight retained a Route outside the measured network manifest.'
        }
    }
    foreach ($selection in @($PublisherPreflight.Selections)) {
        if ([int]$selection.Role -ne 2 -or
            (Convert-Digest $selection.EntryNodeID 'publisher retained Entry Node') -cne $publisherEntry -or
            (Convert-Digest $selection.InteriorNodeID 'publisher retained Interior Node') -cne $publisherInterior) {
            throw 'Publisher preflight retained a Route outside the measured network manifest.'
        }
    }
}
function Relay-Host([string]$Role) {
    if ($Role -ceq 'reader') { return $ReaderHost }
    if ($Role -ceq 'publisher') { return $PublisherHost }
    throw 'Verified relay has an unknown host role.'
}
function Configure-OwnerSlices {
    foreach ($role in @('reader', 'publisher')) {
        $hostName = Relay-Host $role
        $quota = if ($role -ceq 'reader') { '50%' } else { '100%' }
        $cpuMax = if ($role -ceq 'reader') { '50000 100000' } else { '100000 100000' }
        $memoryMax = if ($role -ceq 'reader') { [uint64](512MB) } else { [uint64](1GB) }
        $unit = 'ardents-qualification-owner.slice'
        $samplerUnit = "ardents-qualification-owner-sample-$attempt-$role"
        $command = "systemctl stop '$unit' 2>/dev/null || :; systemctl revert '$unit' 2>/dev/null || :; systemctl set-property --runtime '$unit' 'CPUQuota=$quota' 'MemoryMax=$memoryMax' IPAccounting=yes; systemctl start '$unit'; group=`$(systemctl show '$unit' -p ControlGroup --value); systemctl show '$unit' -p ActiveState -p ControlGroup -p CPUQuotaPerSecUSec -p MemoryMax -p IPAccounting -p DropInPaths; printf 'CPU_MAX='; cat `"/sys/fs/cgroup`$group/cpu.max`"; printf 'MEMORY_MAX='; cat `"/sys/fs/cgroup`$group/memory.max`""
        $owner = [ordered]@{ Host=$role; Machine=$hostName; Unit=$unit; SamplerUnit=$samplerUnit; CPUQuota=$quota; CPUMax=$cpuMax; MemoryMax=$memoryMax; Receipt=@() }
        $script:ownerSlices += $owner
        $receipt = @((Invoke-SSH $hostName $command "configure $role whole-owner slice"))
        $owner.Receipt = @($receipt)
        $values = @{}
        foreach ($line in $receipt) { if ($line -match '^([^=]+)=(.*)$') { $values[$Matches[1]] = $Matches[2] } }
        if ([string]$values.ActiveState -cne 'active' -or [string]$values.ControlGroup -cne '/ardents.slice/ardents-qualification.slice/ardents-qualification-owner.slice' -or
            [string]$values.CPU_MAX -cne $cpuMax -or [string]$values.MEMORY_MAX -cne [string]$memoryMax -or
            [string]$values.IPAccounting -cne 'yes' -or [string]::IsNullOrWhiteSpace([string]$values.DropInPaths)) {
            throw "$role whole-owner slice does not expose the fixed effective limits."
        }
        [void](Invoke-SSH $hostName "chmod 700 '$remoteRoot/node_owner_samples.py'; systemd-run --unit '$samplerUnit' --property Type=exec --property NoNewPrivileges=yes --property MemoryMax=32M --property TasksMax=16 --property RuntimeMaxSec=22min python3 '$remoteRoot/node_owner_samples.py' '$unit'" "start $role whole-owner sampler")
    }
}
function Stop-OwnerSlices {
    $records = @()
    foreach ($owner in $ownerSlices) {
        [void](Invoke-SSH ([string]$owner.Machine) "timeout 5s systemctl stop '$($owner.SamplerUnit)'" "stop $($owner.Host) whole-owner sampler")
        $sampleLines = @(Invoke-SSH ([string]$owner.Machine) "journalctl -u '$($owner.SamplerUnit)' --no-pager -o cat" "collect $($owner.Host) whole-owner samples")
        $samples = @()
        foreach ($line in $sampleLines) {
            try {
                $sample = $line | ConvertFrom-Json -ErrorAction Stop
                if ($sample.PSObject.Properties['At'] -and $sample.PSObject.Properties['MemoryCurrent'] -and
                    $sample.PSObject.Properties['CPUUsageNSec'] -and $sample.PSObject.Properties['IPIngressBytes'] -and
                    $sample.PSObject.Properties['IPEgressBytes']) { $samples += $sample }
            } catch {}
        }
        $records += [ordered]@{ Host=[string]$owner.Host; Unit=[string]$owner.Unit; CPUQuota=[string]$owner.CPUQuota; CPUMax=[string]$owner.CPUMax; MemoryMax=[uint64]$owner.MemoryMax; Receipt=@($owner.Receipt); Samples=@($samples) }
    }
    $script:ownerSliceRecords = @($records)
}
function Start-ClockObservers {
    $declared = @{
        reader = [string]$readerPlanObject.Participants[0].Participant.Network.ClockObservationFile
        publisher = [string]$publisherPlanObject.Participants[0].Participant.Network.ClockObservationFile
    }
    foreach ($role in @('reader', 'publisher')) {
        $clock = [string]$declared[$role]
        Assert-RemotePath $clock "$role clock observation"
        $ownerPlan = if ($role -ceq 'reader') { $readerPlanObject } else { $publisherPlanObject }
        foreach ($participant in @($ownerPlan.Participants)) {
            if ([string]$participant.Participant.Network.ClockObservationFile -cne $clock) {
                throw "All $role participants must use their one runner-owned clock observation."
            }
        }
        foreach ($node in @($inventoryNodes | Where-Object { [string]$_.Host -ceq $role })) {
            $plan = Get-Content -LiteralPath ([string]$node.Plan) -Raw | ConvertFrom-Json
            if ([string]$plan.clock_observation_file -cne $clock) {
                throw "Route Node $($node.ID) does not use the $role clock observation."
            }
        }
        $hostName = Relay-Host $role
        $unit = "ardents-qualification-clock-$attempt-$role"
        [void](Invoke-SSH $hostName "chmod 700 '$remoteRoot/clock_observer.py'; systemd-run --unit '$unit' --property Type=exec --property NoNewPrivileges=yes --property MemoryMax=32M --property 'CPUQuota=5%' --property TasksMax=16 --property RuntimeMaxSec=22min python3 '$remoteRoot/clock_observer.py' '$clock'" "start $role clock observer")
        $ready = $false
        for ($poll = 0; $poll -lt 50; $poll++) {
            $fresh = ((Invoke-SSH $hostName "test -s '$clock' && stat -c %Y '$clock' || echo 0" "read $role clock observation") -join '').Trim()
            if ($fresh -match '^[0-9]+$' -and [DateTimeOffset]::UtcNow.ToUnixTimeSeconds() - [int64]$fresh -le 2) { $ready = $true; break }
            Start-Sleep -Milliseconds 200
        }
        if (-not $ready) { throw "$role clock observation did not become current." }
        $script:clockUnits += [ordered]@{ Host=$hostName; Role=$role; Unit=$unit; Path=$clock }
    }
}
function Stop-ClockObservers {
    foreach ($clock in $clockUnits) {
        [void](Invoke-SSH ([string]$clock.Host) "timeout 5s systemctl stop '$($clock.Unit)'; systemctl show '$($clock.Unit)' -p ActiveState -p Result -p ExecMainStatus" "stop $($clock.Role) clock observer")
    }
}
function Start-StateSources {
    for ($index = 0; $index -lt $inventorySources.Count; $index++) {
        $item = $inventorySources[$index]
        $hostName = Relay-Host ([string]$item.Host)
        $planRemote = "$remoteRoot/source-$index.json"
        Send-File ([string]$item.Plan) $hostName $planRemote "upload State Source $index plan"
        $unit = "ardents-qualification-source-$attempt-$index"
        $samplerUnit = "ardents-qualification-source-sample-$attempt-$index"
        $command = "chmod 755 '$remoteRoot/ardents-node'; chmod 700 '$remoteRoot/node_owner_samples.py'; systemd-run --unit '$unit' --slice ardents-qualification-owner.slice --property Type=exec --property User=ardents-endpoint --property Group=ardents-endpoint --property NoNewPrivileges=yes --property MemoryMax=128M --property IPAccounting=yes --property TasksMax=64 --property RuntimeMaxSec=22min '$remoteRoot/ardents-node' source --config '$planRemote'"
        [void](Invoke-SSH $hostName $command "start State Source $index")
        $invocation = ''
        for ($poll = 0; $poll -lt 50; $poll++) {
            $invocation = ((Invoke-SSH $hostName "systemctl show '$unit' -p InvocationID --value" "read State Source $index invocation") -join '').Trim()
            if ($invocation -match '^[0-9a-f]{32}$') { break }
            Start-Sleep -Milliseconds 200
        }
        if ($invocation -notmatch '^[0-9a-f]{32}$') { throw "State Source $index published no invocation identity." }
        Assert-TransientOwner $hostName $unit "State Source $index"
        $script:startedSources += [ordered]@{ ID=[string]$item.ID; Host=[string]$item.Host; Machine=$hostName; PlanSHA256=[string]$item.PlanSHA256; Unit=$unit; SamplerUnit=$samplerUnit; InvocationID=$invocation }
        $ready = $false
        for ($poll = 0; $poll -lt 75; $poll++) {
            $journal = (Invoke-SSH $hostName "journalctl _SYSTEMD_INVOCATION_ID=$invocation --no-pager -o cat" "read State Source $index journal") -join [Environment]::NewLine
            foreach ($line in $journal -split [Environment]::NewLine) {
                try { $event = $line | ConvertFrom-Json -ErrorAction Stop } catch { continue }
                if ([string]$event.schema -ceq 'ardents-source-event-v1' -and [string]$event.kind -ceq 'source-ready') { $ready = $true; break }
            }
            if ($ready) { break }
            $active = ((Invoke-SSH $hostName "systemctl show '$unit' -p ActiveState --value" "read State Source $index state") -join '').Trim()
            if ($active -eq 'inactive' -or $active -eq 'failed') {
                $detail = @(($journal -split [Environment]::NewLine) | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Select-Object -Last 8) -join ' | '
                throw "State Source $index stopped before readiness (state=$active): $detail"
            }
            Start-Sleep -Milliseconds 200
        }
        if (-not $ready) { throw "State Source $index readiness exceeded 15 seconds." }
        [void](Invoke-SSH $hostName "systemd-run --unit '$samplerUnit' --property Type=exec --property NoNewPrivileges=yes --property MemoryMax=32M --property TasksMax=16 --property RuntimeMaxSec=22min python3 '$remoteRoot/node_owner_samples.py' '$unit.service'" "start State Source $index owner sampler")
    }
}
function Stop-StateSources {
    $records = @()
    foreach ($started in $startedSources) {
        [void](Invoke-SSH ([string]$started.Machine) "timeout 30s systemctl stop '$($started.Unit)'" "stop State Source $($started.ID)")
        for ($poll = 0; $poll -lt 40; $poll++) {
            $samplerState = ((Invoke-SSH ([string]$started.Machine) "systemctl show '$($started.SamplerUnit)' -p ActiveState --value" 'read State Source sampler state') -join '').Trim()
            if ($samplerState -eq 'inactive' -or $samplerState -eq 'failed') { break }
            Start-Sleep -Milliseconds 250
        }
        $statusLines = @(Invoke-SSH ([string]$started.Machine) "systemctl show '$($started.Unit)' -p ActiveState -p Result -p ExecMainStatus -p Slice" 'read terminal State Source state')
        $status = @{}
        foreach ($line in $statusLines) { if ($line -match '^([^=]+)=(.*)$') { $status[$Matches[1]] = $Matches[2] } }
        $journal = @(Invoke-SSH ([string]$started.Machine) "journalctl _SYSTEMD_INVOCATION_ID=$($started.InvocationID) --no-pager -o cat" 'collect State Source journal')
        $sampleLines = @(Invoke-SSH ([string]$started.Machine) "journalctl -u '$($started.SamplerUnit)' --no-pager -o cat" 'collect State Source owner samples')
        $samples = @()
        foreach ($line in $sampleLines) { try { $sample = $line | ConvertFrom-Json -ErrorAction Stop; if ($sample.PSObject.Properties['At'] -and $sample.PSObject.Properties['MemoryCurrent'] -and $sample.PSObject.Properties['CPUUsageNSec'] -and $sample.PSObject.Properties['IPIngressBytes'] -and $sample.PSObject.Properties['IPEgressBytes']) { $samples += $sample } } catch {} }
        $records += [ordered]@{ ID=[string]$started.ID; Host=[string]$started.Host; PlanSHA256=[string]$started.PlanSHA256; InvocationID=[string]$started.InvocationID; BinarySHA256=[string]$inputFiles.node; ActiveState=[string]$status.ActiveState; Result=[string]$status.Result; ExecMainStatus=[int]$status.ExecMainStatus; Slice=[string]$status.Slice; Journal=@($journal); Samples=@($samples) }
    }
    $script:sourceRecords = @($records)
}
function Write-NodeAndSourceResults {
    Write-Utf8 (Join-Path $evidence 'node-results.json') (([ordered]@{ InventorySHA256=[string]$inputFiles.node_inventory; OwnerSlices=@($script:ownerSliceRecords); Nodes=@($script:nodeRecords); Sources=@($script:sourceRecords) } | ConvertTo-Json -Depth 15 -Compress) + [Environment]::NewLine)
}
function Start-RouteNodes {
    $preparedNodes = @()
    for ($index = 0; $index -lt $inventoryNodes.Count; $index++) {
        $item = $inventoryNodes[$index]
        $hostName = Relay-Host ([string]$item.Host)
        $planRemote = "$remoteRoot/node-$index.json"
        Send-File ([string]$item.Plan) $hostName $planRemote "upload Node $index plan"
        $unit = "ardents-qualification-node-$attempt-$index"
        $samplerUnit = "ardents-qualification-sample-$attempt-$index"
        $preparedNodes += [ordered]@{ Index=$index; ID=[string]$item.ID; Host=[string]$item.Host; Machine=$hostName; PlanRemote=$planRemote; PlanSHA256=[string]$item.PlanSHA256; Unit=$unit; SamplerUnit=$samplerUnit }
    }
    foreach ($group in @($preparedNodes | Group-Object { [string]$_.Machine })) {
        $commands = @("set -eu", "chmod 755 '$remoteRoot/ardents-node'", "chmod 700 '$remoteRoot/node_owner_samples.py'")
        foreach ($prepared in @($group.Group)) {
            $unit = [string]$prepared.Unit
            $planRemote = [string]$prepared.PlanRemote
            $commands += "systemd-run --no-block --unit '$unit' --slice ardents-qualification-owner.slice --property Type=exec --property User=ardents-endpoint --property Group=ardents-endpoint --property NoNewPrivileges=yes --property IPAccounting=yes --property TasksMax=256 --property RuntimeMaxSec=22min '$remoteRoot/ardents-node' node --config '$planRemote'"
        }
        [void](Invoke-SSH ([string]$group.Name) ($commands -join '; ') "start Route Node group on $($group.Name)")
    }
    foreach ($prepared in $preparedNodes) {
        $index = [int]$prepared.Index
        $hostName = [string]$prepared.Machine
        $unit = [string]$prepared.Unit
        $invocation = ''
        for ($poll = 0; $poll -lt 50; $poll++) {
            $invocation = ((Invoke-SSH $hostName "systemctl show '$unit' -p InvocationID --value" "read Route Node $index invocation") -join '').Trim()
            if ($invocation -match '^[0-9a-f]{32}$') { break }
            Start-Sleep -Milliseconds 200
        }
        if ($invocation -notmatch '^[0-9a-f]{32}$') { throw "Route Node $index published no invocation identity." }
        Assert-TransientOwner $hostName $unit "Route Node $index"
        $script:startedNodes += [ordered]@{ Index=$index; ID=[string]$prepared.ID; Host=[string]$prepared.Host; Machine=$hostName; PlanSHA256=[string]$prepared.PlanSHA256; Unit=$unit; SamplerUnit=[string]$prepared.SamplerUnit; InvocationID=$invocation }
    }
    foreach ($started in $script:startedNodes) {
        $index = [int]$started.Index
        $hostName = [string]$started.Machine
        $unit = [string]$started.Unit
        $invocation = [string]$started.InvocationID
        $ready = $false
        for ($poll = 0; $poll -lt 150; $poll++) {
            $journal = (Invoke-SSH $hostName "journalctl _SYSTEMD_INVOCATION_ID=$invocation --no-pager -o cat" "read Route Node $index journal") -join [Environment]::NewLine
            $lifecycle = @()
            foreach ($line in $journal -split [Environment]::NewLine) {
                try { $event = $line | ConvertFrom-Json -ErrorAction Stop } catch { continue }
                if ([string]$event.schema -ceq 'ardents-node-event-v1' -and [string]$event.kind -ceq 'lifecycle') { $lifecycle += $event }
            }
            $latest = @($lifecycle | Select-Object -Last 1)
            if ($latest.Count -eq 1 -and [string]$latest[0].state -ceq 'READY') { $ready = $true; break }
            if ($latest.Count -eq 1 -and @('FAILED', 'DRAINING', 'WITHDRAWN') -ccontains [string]$latest[0].state) {
                $detail = @(($journal -split [Environment]::NewLine) | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Select-Object -Last 8) -join ' | '
                throw "Route Node $index terminated before group readiness: $detail"
            }
            $active = ((Invoke-SSH $hostName "systemctl show '$unit' -p ActiveState --value" "read Route Node $index state") -join '').Trim()
            if ($active -eq 'inactive' -or $active -eq 'failed') {
                $detail = @(($journal -split [Environment]::NewLine) | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Select-Object -Last 8) -join ' | '
                throw "Route Node $index stopped before readiness (state=$active): $detail"
            }
            Start-Sleep -Milliseconds 200
        }
        if (-not $ready) { throw "Route Node $index readiness exceeded 30 seconds." }
    }
    foreach ($group in @($script:startedNodes | Group-Object { [string]$_.Machine })) {
        $commands = @("set -eu")
        $activeUnits = @()
        foreach ($started in @($group.Group)) {
            $unit = [string]$started.Unit
            $samplerUnit = [string]$started.SamplerUnit
            $commands += "systemd-run --unit '$samplerUnit' --property Type=exec --property NoNewPrivileges=yes --property MemoryMax=32M --property TasksMax=16 --property RuntimeMaxSec=22min python3 '$remoteRoot/node_owner_samples.py' '$unit.service'"
            $activeUnits += "'$unit'"
        }
        $commands += "systemctl is-active $($activeUnits -join ' ')"
        [void](Invoke-SSH ([string]$group.Name) ($commands -join '; ') "start samplers and verify Route Node group on $($group.Name)")
    }
}
function Stop-RouteNodes {
    $records = @()
    foreach ($started in $startedNodes) {
        [void](Invoke-SSH ([string]$started.Machine) "timeout 30s systemctl stop '$($started.Unit)'" "stop Route Node $($started.ID)")
        for ($poll = 0; $poll -lt 40; $poll++) {
            $samplerState = ((Invoke-SSH ([string]$started.Machine) "systemctl show '$($started.SamplerUnit)' -p ActiveState --value" 'read Node sampler state') -join '').Trim()
            if ($samplerState -eq 'inactive' -or $samplerState -eq 'failed') { break }
            Start-Sleep -Milliseconds 250
        }
        $statusLines = @(Invoke-SSH ([string]$started.Machine) "systemctl show '$($started.Unit)' -p ActiveState -p Result -p ExecMainStatus -p Slice" 'read terminal Route Node state')
        $status = @{}
        foreach ($line in $statusLines) { if ($line -match '^([^=]+)=(.*)$') { $status[$Matches[1]] = $Matches[2] } }
        $journal = @(Invoke-SSH ([string]$started.Machine) "journalctl _SYSTEMD_INVOCATION_ID=$($started.InvocationID) --no-pager -o cat" 'collect Route Node journal')
        $sampleLines = @(Invoke-SSH ([string]$started.Machine) "journalctl -u '$($started.SamplerUnit)' --no-pager -o cat" 'collect Route Node owner samples')
        $samples = @()
        foreach ($line in $sampleLines) { try { $sample = $line | ConvertFrom-Json -ErrorAction Stop; if ($sample.PSObject.Properties['At'] -and $sample.PSObject.Properties['MemoryCurrent'] -and $sample.PSObject.Properties['CPUUsageNSec'] -and $sample.PSObject.Properties['IPIngressBytes'] -and $sample.PSObject.Properties['IPEgressBytes']) { $samples += $sample } } catch {} }
        $records += [ordered]@{ ID=[string]$started.ID; Host=[string]$started.Host; PlanSHA256=[string]$started.PlanSHA256; InvocationID=[string]$started.InvocationID; BinarySHA256=[string]$inputFiles.node; ActiveState=[string]$status.ActiveState; Result=[string]$status.Result; ExecMainStatus=[int]$status.ExecMainStatus; Slice=[string]$status.Slice; Journal=@($journal); Samples=@($samples) }
    }
    $script:nodeRecords = @($records)
}
function Start-NetworkRelays {
    $records = @()
    $relays = @($networkManifestObject.Relays)
    $dockerGateways = @{}
    $dockerImages = @{}
    foreach ($role in @($relays | ForEach-Object { [string]$_.Host } | Sort-Object -Unique)) {
        $hostName = Relay-Host $role
        $gateway = ((Invoke-SSH $hostName "docker network inspect bridge --format '{{(index .IPAM.Config 0).Gateway}}'" "read Docker bridge gateway for $role relays") -join '').Trim()
        if ($gateway -notmatch '^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$') { throw "Docker bridge gateway for $role relays is invalid." }
        $dockerGateways[$role] = $gateway
        $image = ((Invoke-SSH $hostName "docker image inspect '$relayImage' --format '{{.Id}}|{{.Created}}|{{.Os}}|{{.Architecture}}'" "inspect immutable relay image for $role") -join '').Trim()
        if ($image -notmatch '^(sha256:[0-9a-f]{64})\|([^|]+)\|linux\|amd64$') { throw "Immutable relay image for $role is unavailable or invalid." }
        $dockerImages[$role] = [ordered]@{ Digest=$relayImage; ID=$Matches[1]; Created=$Matches[2]; OS='linux'; Architecture='amd64' }
    }
    foreach ($relay in $relays) {
        $target = [string]$relay.Target
        if ($target -notmatch '^((?:[0-9]{1,3}\.){3}[0-9]{1,3}):([1-9][0-9]{0,4})$' -or $Matches[1] -cne [string]$dockerGateways[[string]$relay.Host] -or [int]$Matches[2] -gt 65535) {
            throw "Relay $($relay.ID) target is not the exact local Docker bridge gateway."
        }
        $publicEndpoint = [string]$relay.PublicEndpoint
        $expectedPublicHost = Relay-Host ([string]$relay.Host)
        if ($publicEndpoint -notmatch '^((?:[0-9]{1,3}\.){3}[0-9]{1,3}):([1-9][0-9]{0,4})$' -or $Matches[1] -cne [string]$expectedPublicHost -or [int]$Matches[2] -gt 65535) {
            throw "Relay $($relay.ID) public endpoint differs from its selected host."
        }
        $listenPort = [int](([string]$relay.Listen).TrimStart(':'))
        if ($listenPort -ne [int]$Matches[2]) { throw "Relay $($relay.ID) public endpoint differs from its listener." }
    }
    for ($index = $relays.Count - 1; $index -ge 0; $index--) {
        $relay = $relays[$index]
        $hostName = Relay-Host ([string]$relay.Host)
        $listen = [string]$relay.Listen
        if ($listen -notmatch '^:([1-9][0-9]{0,4})$') { throw 'Verified relay listen endpoint is not externally publishable.' }
        $port = [int]$Matches[1]
        if ($port -gt 65535) { throw 'Verified relay listen port is invalid.' }
        $container = [string]$relay.Container
        $samplerUnit = "ardents-qualification-relay-sample-$attempt-$index"
        $network = [string]$relay.Network
        $segment = @($networkManifestObject.Paths | ForEach-Object { @($_.Segments) } | Where-Object { [string]$_.ID -ceq [string]$relay.UpstreamSegment })
        if ($segment.Count -ne 1) { throw "Relay $container has no unique upstream segment." }
        $command = "set -eu; docker image inspect '$relayImage' >/dev/null; ! docker container inspect '$container' >/dev/null 2>&1; chmod 755 '$remoteRoot/netem-relay'; docker run --detach --name '$container' --label 'ardents.qualification.manifest=$($inputFiles.network_manifest)' --cap-drop ALL --cap-add NET_ADMIN --pids-limit 64 --memory 128m --cpus 0.5 --read-only -p '$port`:$port/$network' -v '$remoteRoot/netem-relay:/work/netem-relay:ro' -v /usr/sbin/tc:/usr/sbin/tc:ro -v /lib/x86_64-linux-gnu:/lib/x86_64-linux-gnu:ro -v /lib64:/lib64:ro --workdir /work --entrypoint /work/netem-relay '$relayImage' -listen '$listen' -target '$($relay.Target)' -network '$network' -mode '$($relay.Mode)' -upstream-rate '$($relay.UpstreamRate)' -client-rate '$($relay.ClientRate)' -segment-delay '$($segment[0].DelayMicros)us' -segment-jitter '$($segment[0].JitterP95Micros)us' -segment-loss-ppm '$($segment[0].LossPartsPerMillion)'"
        $containerID = ((Invoke-SSH $hostName $command "start relay $container") -join '').Trim()
        if ($containerID -notmatch '^[0-9a-f]{64}$') { throw "Relay $container returned no container identity." }
        $script:startedRelays += [ordered]@{ Host=$hostName; Container=$container; SamplerUnit=$samplerUnit }
        $ready = $false
        for ($poll = 0; $poll -lt 75; $poll++) {
            $logs = (Invoke-SSH $hostName "docker logs '$container' 2>&1 || :" "read relay $container logs") -join "`n"
            if ($logs.Contains('netem-relay-ready')) { $ready = $true; break }
            Start-Sleep -Milliseconds 200
        }
        if (-not $ready) { throw "Relay $container did not become ready." }
        [void](Invoke-SSH $hostName "chmod 700 '$remoteRoot/relay_owner_samples.py'; systemd-run --unit '$samplerUnit' --property Type=exec --property NoNewPrivileges=yes --property MemoryMax=32M --property TasksMax=16 --property RuntimeMaxSec=22min python3 '$remoteRoot/relay_owner_samples.py' '$container'" "start relay $container sampler")
        $receipt = Invoke-SSH $hostName "test `"`$(docker inspect --format '{{index .Config.Labels `"ardents.qualification.manifest`"}}' '$container')`" = '$($inputFiles.network_manifest)'; docker exec '$container' /usr/sbin/tc -s qdisc show dev eth0; docker exec '$container' /usr/sbin/tc class show dev eth0; docker exec '$container' /usr/sbin/tc filter show dev eth0" "verify relay $container"
        $records += [ordered]@{ ID=[string]$relay.ID; Host=$hostName; Container=$container; ContainerID=$containerID; Image=$dockerImages[[string]$relay.Host]; Receipt=@($receipt) }
    }
    Write-Utf8 (Join-Path $evidence 'relay-receipts.json') (($records | ConvertTo-Json -Depth 8 -Compress) + "`n")
}
function Stop-NetworkRelays {
    $records = @()
    foreach ($started in $startedRelays) {
        $relay = @($networkManifestObject.Relays | Where-Object { [string]$_.Container -ceq [string]$started.Container })
        if ($relay.Count -ne 1) { throw 'Started relay no longer has one manifest owner.' }
        [void](Invoke-SSH $started.Host "timeout 5s systemctl stop '$($started.SamplerUnit)'" "stop relay sampler $($started.Container)")
        $sampleLines = @(Invoke-SSH $started.Host "journalctl -u '$($started.SamplerUnit)' --no-pager -o cat" "collect relay sampler $($started.Container)")
        $samples = @()
        foreach ($line in $sampleLines) { try { $sample = $line | ConvertFrom-Json -ErrorAction Stop; if ($sample.PSObject.Properties['At'] -and $sample.PSObject.Properties['UpstreamBytes'] -and $sample.PSObject.Properties['ClientBytes']) { $samples += $sample } } catch {} }
        $trafficControl = Invoke-SSH $started.Host "docker exec '$($started.Container)' /usr/sbin/tc -s -j qdisc show dev eth0; docker exec '$($started.Container)' /usr/sbin/tc -s -j class show dev eth0; docker exec '$($started.Container)' /usr/sbin/tc -s -j filter show dev eth0" "capture final relay counters $($started.Container)"
        [void](Invoke-SSH $started.Host "docker stop --time 5 '$($started.Container)'" "stop relay $($started.Container)")
        $logs = @(Invoke-SSH $started.Host "docker logs '$($started.Container)' 2>&1" "capture relay $($started.Container) result")
        $state = ((Invoke-SSH $started.Host "docker inspect --format '{{.State.ExitCode}} {{.State.OOMKilled}}' '$($started.Container)'" "verify relay $($started.Container) exit") -join '').Trim()
        $result = @()
        foreach ($line in $logs) { try { $record = $line | ConvertFrom-Json -ErrorAction Stop; if ([string]$record.kind -ceq 'netem-relay-result') { $result += $record } } catch {} }
        if ($state -cne '0 false' -or $result.Count -ne 1) { throw "Relay $($started.Container) did not emit one clean terminal traffic receipt." }
        $records += [ordered]@{ ID=[string]$relay[0].ID; Host=[string]$started.Host; Container=[string]$started.Container; UpstreamSegment=[string]$relay[0].UpstreamSegment; ClientSegment=[string]$relay[0].ClientSegment; BinarySHA256=[string]$inputFiles.relay; UpstreamBytes=[uint64]$result[0].upstream_bytes; ClientBytes=[uint64]$result[0].client_bytes; TrafficControl=@($trafficControl); Samples=@($samples); Logs=@($logs) }
    }
    Write-Utf8 (Join-Path $evidence 'relay-results.json') (($records | ConvertTo-Json -Depth 12 -Compress) + "`n")
}
function Capture-FailedNetworkRelays {
    $records = @()
    foreach ($started in $startedRelays) {
        $relay = @($networkManifestObject.Relays | Where-Object { [string]$_.Container -ceq [string]$started.Container })
        if ($relay.Count -ne 1) { continue }
        $sampleLines = @(Invoke-SSH $started.Host "journalctl -u '$($started.SamplerUnit)' --no-pager -o cat" "retain failed relay samples $($started.Container)")
        $samples = @()
        foreach ($line in $sampleLines) {
            try {
                $sample = $line | ConvertFrom-Json -ErrorAction Stop
                if ($sample.PSObject.Properties['At'] -and $sample.PSObject.Properties['UpstreamBytes'] -and $sample.PSObject.Properties['ClientBytes']) { $samples += $sample }
            } catch { continue }
        }
        $trafficControl = @(Invoke-SSH $started.Host "docker exec '$($started.Container)' /usr/sbin/tc -s -j qdisc show dev eth0; docker exec '$($started.Container)' /usr/sbin/tc -s -j class show dev eth0; docker exec '$($started.Container)' /usr/sbin/tc -s -j filter show dev eth0" "retain failed relay counters $($started.Container)")
        $logs = @(Invoke-SSH $started.Host "docker logs '$($started.Container)' 2>&1 || :" "retain failed relay logs $($started.Container)")
        $records += [ordered]@{ ID=[string]$relay[0].ID; Host=[string]$started.Host; Container=[string]$started.Container; UpstreamSegment=[string]$relay[0].UpstreamSegment; ClientSegment=[string]$relay[0].ClientSegment; BinarySHA256=[string]$inputFiles.relay; TrafficControl=@($trafficControl); Samples=@($samples); Logs=@($logs); Complete=$false }
    }
    Write-Utf8 (Join-Path $evidence 'relay-results.failed.json') (($records | ConvertTo-Json -Depth 12 -Compress) + "`n")
    foreach ($scheduled in $recoveryUnits) {
        $status = @(Invoke-SSH $scheduled.Host "systemctl show '$($scheduled.Unit)' -p ActiveState -p Result -p ExecMainStatus; journalctl -u '$($scheduled.Unit)' --no-pager -o cat" "retain failed recovery scheduler $($scheduled.Role)")
        Write-Utf8 (Join-Path $evidence "recovery-$($scheduled.Role).failed.jsonl") (($status -join "`n") + "`n")
    }
}
function Start-Owner([string]$HostName) {
    [void](Invoke-SSH $HostName 'systemctl start --no-block ardents-endpoint.service' 'start Endpoint')
    for ($attemptIndex = 0; $attemptIndex -lt 30; $attemptIndex++) {
        $id = ((Invoke-SSH $HostName 'systemctl show ardents-endpoint.service -p InvocationID --value' 'read invocation') -join '').Trim()
        if ($id -match '^[0-9a-f]{32}$') { return $id }
        Start-Sleep -Milliseconds 200
    }
    throw 'Endpoint InvocationID was not published.'
}
function Start-RecoveryFaults([DateTimeOffset]$Started) {
    $startedMillis = $Started.ToUnixTimeMilliseconds()
    foreach ($role in @('reader', 'publisher')) {
        $episodes = @($networkManifestObject.Failures | Where-Object { [string]$segment = [string]$_.SegmentID; @($networkManifestObject.Relays | Where-Object { [string]$_.Host -ceq $role -and ([string]$_.UpstreamSegment -ceq $segment -or [string]$_.ClientSegment -ceq $segment) }).Count -eq 1 })
        if ($episodes.Count -eq 0) { continue }
        $hostName = Relay-Host $role
        $unit = "ardents-qualification-recovery-$attempt-$role"
        $command = "chmod 700 '$remoteRoot/recovery_faults.py'; systemd-run --unit '$unit' --property Type=exec --property NoNewPrivileges=yes --property MemoryMax=64M --property TasksMax=32 --property RuntimeMaxSec=11min python3 '$remoteRoot/recovery_faults.py' '$remoteRoot/network-manifest.json' '$role' '$startedMillis'"
        [void](Invoke-SSH $hostName $command "start recovery scheduler $role")
        $script:recoveryUnits += [ordered]@{ Host=$hostName; Role=$role; Unit=$unit; Expected=$episodes.Count }
    }
}
function Wait-RecoveryFaults([DateTime]$Deadline) {
    foreach ($scheduled in $recoveryUnits) {
        while ([DateTime]::UtcNow -lt $Deadline) {
            $state = ((Invoke-SSH $scheduled.Host "systemctl show '$($scheduled.Unit)' -p ActiveState --value" 'read recovery scheduler state') -join '').Trim()
            if ($state -eq 'inactive' -or $state -eq 'failed') { break }
            Start-Sleep -Milliseconds 500
        }
        $status = (Invoke-SSH $scheduled.Host "systemctl show '$($scheduled.Unit)' -p ActiveState -p Result -p ExecMainStatus; journalctl -u '$($scheduled.Unit)' --no-pager -o cat" 'collect recovery scheduler') -join "`n"
        Write-Utf8 (Join-Path $evidence "recovery-$($scheduled.Role).jsonl") ($status + "`n")
        if ($status -notmatch 'ActiveState=inactive' -or $status -notmatch 'Result=success' -or $status -notmatch 'ExecMainStatus=0') { throw "Recovery scheduler for $($scheduled.Role) failed." }
        $starts = @(); $stops = @(); $complete = $false
        foreach ($line in $status -split "`n") {
            try { $record = $line | ConvertFrom-Json -ErrorAction Stop } catch { continue }
            if ([string]$record.kind -ceq 'recovery-fault-start') { $starts += $record }
            if ([string]$record.kind -ceq 'recovery-fault-stop') { $stops += $record }
            if ([string]$record.kind -ceq 'recovery-faults-complete') { $complete = $true }
        }
        if (-not $complete -or $starts.Count -ne $scheduled.Expected -or $stops.Count -ne $scheduled.Expected) { throw "Recovery scheduler for $($scheduled.Role) produced incomplete evidence." }
        foreach ($record in @($starts + $stops)) {
            if ([Math]::Abs([int64]$record.actual_millis - [int64]$record.scheduled_millis) -gt 1500) { throw "Recovery scheduler for $($scheduled.Role) missed its declared time." }
        }
    }
}
function Journal([string]$HostName, [string]$Invocation) {
    return @(Invoke-SSH $HostName "journalctl _SYSTEMD_INVOCATION_ID=$Invocation --no-pager -o cat" 'read Endpoint journal')
}
function Wait-Event([string]$HostName, [string]$Invocation, [string]$Kind, [int]$Participant, [DateTime]$Deadline) {
    while ([DateTime]::UtcNow -lt $Deadline) {
        foreach ($line in (Journal $HostName $Invocation)) {
            try { $record = $line | ConvertFrom-Json -ErrorAction Stop } catch { continue }
            if ([int]$record.Participant -eq $Participant -and [string]$record.Event.Kind -ceq $Kind) { return $record.Event }
        }
        $state = ((Invoke-SSH $HostName 'systemctl show ardents-endpoint.service -p ActiveState --value' 'read Endpoint state while awaiting event') -join '').Trim()
        if ($state -eq 'inactive' -or $state -eq 'failed') { throw "Endpoint stopped before $Kind; inspect its retained invocation journal." }
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out waiting for $Kind from participant $Participant."
}
function Wait-SmokeProgress([string]$HostName, [string]$Invocation, [int]$Participant, [int]$ExpectedStreams, [int]$ExpectedActive, [DateTime]$Deadline) {
    while ([DateTime]::UtcNow -lt $Deadline) {
        $latest = $null
        foreach ($line in (Journal $HostName $Invocation)) {
            try { $record = $line | ConvertFrom-Json -ErrorAction Stop } catch { continue }
            if ([int]$record.Participant -eq $Participant -and [string]$record.Event.Kind -ceq 'stream-progress') { $latest = $record.Event.Report }
        }
        if ($null -ne $latest -and [string]::IsNullOrEmpty([string]$latest.Failure)) {
            $streams = @($latest.Streams)
            $active = @($streams | Where-Object { [uint32]$_.ID -le 127 -and ([uint64]$_.Tx + [uint64]$_.Rx) -gt 0 })
            $canaries = @($streams | Where-Object { [uint32]$_.ID -gt 127 -and [uint64]$_.Tx -ge 32 -and [uint64]$_.Rx -ge 32 })
            if ($streams.Count -eq $ExpectedStreams -and $active.Count -eq $ExpectedActive -and $canaries.Count -eq ($ExpectedStreams - $ExpectedActive)) {
                $activeBytes = [uint64]0
                foreach ($stream in $active) { $activeBytes += [uint64]$stream.Tx + [uint64]$stream.Rx }
                return [ordered]@{ Participant=$Participant; Streams=$streams.Count; Active=$active.Count; Canaries=$canaries.Count; ActiveBytes=$activeBytes }
            }
        }
        $state = ((Invoke-SSH $HostName 'systemctl show ardents-endpoint.service -p ActiveState --value' 'read Endpoint state during smoke') -join '').Trim()
        if ($state -eq 'inactive' -or $state -eq 'failed') { throw "Endpoint stopped before complete smoke progress for participant $Participant." }
        Start-Sleep -Milliseconds 500
    }
    throw "Timed out waiting for complete smoke progress from participant $Participant."
}
function Issue-Permission([string]$EndpointHost, [object]$Files, [string]$Digest, [string]$Label) {
    Assert-Hex $Digest "$Label request digest"
    Assert-RemotePath ([string]$Files.RequestPath) "$Label request path"
    Assert-RemotePath ([string]$Files.ResponsePath) "$Label response path"
    $requestLocal = Join-Path ([IO.Path]::GetTempPath()) "$attempt-$Label.request"
    $permissionLocal = Join-Path ([IO.Path]::GetTempPath()) "$attempt-$Label.permission"
    $authorityRequest = "$remoteRoot/$Label.request"
    $authorityPermission = "$remoteRoot/$Label.permission"
    try {
        Receive-File $EndpointHost ([string]$Files.RequestPath) $requestLocal "download $Label request"
        $actual = (Get-FileHash -LiteralPath $requestLocal -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($actual -cne $Digest) { throw "$Label request differs from independently observed commitment." }
        [void](Invoke-SSH ([string]$authority.Host) "install -d -m 700 '$remoteRoot'" 'create authority staging')
        Send-File $requestLocal ([string]$authority.Host) $authorityRequest "upload $Label request to custody"
        $args = @('issue-admission-permission', '--vault-root', $authority.VaultRoot, '--record', $authority.RecordID,
            '--request', $authorityRequest, '--permission-output', $authorityPermission,
            '--environment-commitment', $authority.EnvironmentCommitment, '--network-commitment', $authority.NetworkCommitment,
            '--root-commitment', $authority.RootCommitment, '--kind', 'admission', '--id-commitment', $authority.IDCommitment)
        $quoted = ($args | ForEach-Object { "'$($_)'" }) -join ' '
        $custodyCommand = "set +e; stty -echo; '$($authority.Binary)' $quoted; status=`$?; stty echo; exit `$status"
        [void](Invoke-Native $ssh ($sshOptions + @('-tt', (Remote ([string]$authority.Host)), $custodyCommand)) "$Label custody issuance" 900 @($Digest, $admissionSecret))
        Receive-File ([string]$authority.Host) $authorityPermission $permissionLocal "download $Label permission"
        if ((Get-Item -LiteralPath $permissionLocal).Length -ne 228) { throw "$Label permission has the wrong size." }
        Send-File $permissionLocal $EndpointHost "$remoteRoot/$Label.permission" "upload $Label permission"
        $destination = [string]$Files.ResponsePath
        [void](Invoke-SSH $EndpointHost "install -o ardents-endpoint -g ardents-endpoint -m 600 '$remoteRoot/$Label.permission' '$destination'" "publish $Label permission")
    } finally {
        if (Test-Path -LiteralPath $requestLocal) { [IO.File]::Delete($requestLocal) }
        if (Test-Path -LiteralPath $permissionLocal) { [IO.File]::Delete($permissionLocal) }
        try { [void](Invoke-SSH ([string]$authority.Host) "rm -f '$authorityRequest' '$authorityPermission'" 'remove custody staging') } catch {}
    }
}
function Wait-Owner([string]$HostName, [DateTime]$Deadline) {
    while ([DateTime]::UtcNow -lt $Deadline) {
        $state = ((Invoke-SSH $HostName 'systemctl show ardents-endpoint.service -p ActiveState --value' 'read Endpoint state') -join '').Trim()
        if ($state -eq 'inactive' -or $state -eq 'failed') { return }
        Start-Sleep -Seconds 1
    }
    [void](Invoke-SSH $HostName 'systemctl stop ardents-endpoint.service' 'stop timed-out Endpoint')
    throw "Endpoint on $HostName exceeded the 22-minute outer bound."
}

$attemptSchema = if ($SmokeSeconds -gt 0) { 'ardents-qualification-two-host-smoke-attempt-v1' } else { 'ardents-qualification-two-host-attempt-v1' }
$publisherInvocation = $null
$readerInvocation = $null
$startedRelays = @()
$startedSources = @()
$startedNodes = @()
$nodeRecords = @()
$sourceRecords = @()
$recoveryUnits = @()
$clockUnits = @()
$ownerSlices = @()
$ownerSliceRecords = @()
$readerPlanPath = Join-Path $evidence 'reader-plan.installed.json'
$deployedOwners = @()
$runFailure = $null
$cleanupFailures = [Collections.Generic.List[string]]::new()
$admissionDelay = Get-QualificationAdmissionWindowDelay -Now ([DateTimeOffset]::UtcNow) -MinimumRemaining ([TimeSpan]::FromMinutes(15))
if ($admissionDelay -gt [TimeSpan]::Zero) {
    $delaySeconds = [int][Math]::Ceiling($admissionDelay.TotalSeconds)
    Write-Host "Waiting $delaySeconds seconds for a complete admission handover window."
    Start-Sleep -Seconds $delaySeconds
}
try {
    foreach ($hostName in @($PublisherHost, $ReaderHost) | Select-Object -Unique) {
        $hostEnvelope = Invoke-SSH $hostName "uname -a; cat /etc/os-release; systemctl --version; stat -fc %T /sys/fs/cgroup; docker version --format 'docker-client={{.Client.Version}} docker-server={{.Server.Version}}'; /usr/sbin/tc -Version" 'capture host envelope'
        Write-Utf8 (Join-Path $evidence "$hostName-environment.txt") (($hostEnvelope -join "`n") + "`n")
        [void](Invoke-SSH $hostName "install -d -m 700 '$remoteRoot'" 'create relay staging')
        Send-File $relayPath $hostName "$remoteRoot/netem-relay" 'upload relay candidate'
        Send-File $nodePath $hostName "$remoteRoot/ardents-node" 'upload Node candidate'
        Send-File $nodeSamplerPath $hostName "$remoteRoot/node_owner_samples.py" 'upload Node owner sampler'
        Send-File $relaySamplerPath $hostName "$remoteRoot/relay_owner_samples.py" 'upload relay owner sampler'
        Send-File $clockObserverPath $hostName "$remoteRoot/clock_observer.py" 'upload clock observer'
        Send-File $networkManifestPath $hostName "$remoteRoot/network-manifest.json" 'upload network manifest'
        Send-File $recoveryFaultPath $hostName "$remoteRoot/recovery_faults.py" 'upload recovery scheduler'
        [void](Invoke-SSH $hostName "chown ardents-endpoint:ardents-endpoint '$remoteRoot'; chmod 700 '$remoteRoot'" 'bind staging to qualification owner')
    }
    Configure-OwnerSlices
    Start-ClockObservers
    Deploy-Owner $PublisherHost $publisherPlanPath
    $deployedOwners += $PublisherHost
    $publisherPreflight = Invoke-OwnerPreflight $PublisherHost ([string]$inputFiles.publisher_plan) 1 'publisher-preflight.json'
    Deploy-Owner $ReaderHost $readerTemplatePath
    $deployedOwners += $ReaderHost
    $readerPreflight = Invoke-OwnerPreflight $ReaderHost ([string]$inputFiles.reader_plan_template) 4 'reader-preflight.json'
    Assert-RetainedManifestPath $publisherPreflight $readerPreflight
    Verify-NetworkManifest $PublisherHost 'publisher-network-manifest-verdict.json'
    Start-StateSources
    Start-NetworkRelays
    Start-RouteNodes
    $publisherInvocation = Start-Owner $PublisherHost
    $deadline = [DateTime]::UtcNow.AddMinutes($(if ($SmokeSeconds -gt 0) { 10 } else { 22 }))
    $publisherPermission = $publisherPlanObject.Participants[0].Participant.PublisherPermission
    $event = Wait-Event $PublisherHost $publisherInvocation 'permission-required' 0 $deadline
    Issue-Permission $PublisherHost $publisherPermission (Convert-Digest $event.RequestDigest 'publisher request digest') 'publisher'
    $ready = Wait-Event $PublisherHost $publisherInvocation 'publisher-ready' 0 $deadline
    if ([string]::IsNullOrWhiteSpace([string]$ready.Link)) { throw 'Publisher emitted no Target Link.' }
    foreach ($item in @($readerPlanObject.Participants)) { $item.Link = [string]$ready.Link }
    Write-Utf8 $readerPlanPath (($readerPlanObject | ConvertTo-Json -Depth 100 -Compress) + "`n")
    Deploy-Owner $ReaderHost $readerPlanPath
    Verify-NetworkManifest $ReaderHost 'reader-network-manifest-verdict.json'
    $readerInvocation = Start-Owner $ReaderHost
    for ($index = 0; $index -lt 4; $index++) {
        $event = Wait-Event $ReaderHost $readerInvocation 'permission-required' $index $deadline
        Issue-Permission $ReaderHost $readerPlanObject.Participants[$index].Participant.ReaderPermission (Convert-Digest $event.RequestDigest "reader-$index request digest") "reader-$index"
    }
    $smokeSummaries = @()
    if ($SmokeSeconds -gt 0) {
        $initial = @()
        # Each Reader deliberately opens 64 retained connections at one per
        # 1.25 seconds. The full four-Reader setup has measured 225 seconds on
        # the selected one-CPU host, so retain a bounded margin
        # before applying the shorter useful-progress observation window.
        $smokeDeadline = [DateTime]::UtcNow.AddMinutes(6)
        for ($index = 0; $index -lt 4; $index++) {
            $initial += Wait-SmokeProgress $ReaderHost $readerInvocation $index 64 16 $smokeDeadline
        }
        $initial += Wait-SmokeProgress $PublisherHost $publisherInvocation 0 256 64 $smokeDeadline
        Start-Sleep -Seconds $SmokeSeconds
        $progressDeadline = [DateTime]::UtcNow.AddSeconds(30)
        for ($index = 0; $index -lt 4; $index++) {
            $smokeSummaries += Wait-SmokeProgress $ReaderHost $readerInvocation $index 64 16 $progressDeadline
        }
        $smokeSummaries += Wait-SmokeProgress $PublisherHost $publisherInvocation 0 256 64 $progressDeadline
        for ($index = 0; $index -lt $smokeSummaries.Count; $index++) {
            if ([uint64]$smokeSummaries[$index].ActiveBytes -le [uint64]$initial[$index].ActiveBytes) {
                throw "Smoke participant $index stopped useful progress."
            }
        }
        foreach ($hostName in @($ReaderHost, $PublisherHost)) {
            [void](Invoke-SSH $hostName 'timeout 30s systemctl stop ardents-endpoint.service' 'stop bounded smoke Endpoint')
        }
        Wait-Owner $ReaderHost $deadline
        Wait-Owner $PublisherHost $deadline
    } else {
        if ([string]$networkManifestObject.Cell -ceq 'net14-recovery') {
            $progress = Wait-Event $ReaderHost $readerInvocation 'stream-progress' 0 $deadline
            if (-not $progress.Report.Started) { throw 'Recovery workload start was not observed.' }
            Start-RecoveryFaults ([DateTimeOffset]::Parse([string]$progress.Report.Started, [Globalization.CultureInfo]::InvariantCulture))
        }
        Wait-Owner $ReaderHost $deadline
        Wait-Owner $PublisherHost $deadline
        Wait-RecoveryFaults $deadline
    }
    Stop-RouteNodes
    Stop-StateSources
    Stop-OwnerSlices
    Write-NodeAndSourceResults
    Stop-NetworkRelays
    $readerJournal = (Journal $ReaderHost $readerInvocation) -join "`n"
    $publisherJournal = (Journal $PublisherHost $publisherInvocation) -join "`n"
    Write-Utf8 (Join-Path $evidence 'reader.jsonl') ($readerJournal + "`n")
    Write-Utf8 (Join-Path $evidence 'publisher.jsonl') ($publisherJournal + "`n")
    $cleanupRecords = @()
    foreach ($entry in @(@{ Role='publisher'; Host=$PublisherHost }, @{ Role='reader'; Host=$ReaderHost })) {
        $cleanup = @(Invoke-SSH ([string]$entry.Host) "systemctl show ardents-endpoint.service -p ActiveState -p MainPID -p Result -p ExecMainStatus; systemctl list-units 'ardents-stream-qualification-*@*.service' --state=active,activating,deactivating --plain --no-legend" 'verify installed cleanup')
        Write-Utf8 (Join-Path $evidence "$($entry.Host)-cleanup.txt") (($cleanup -join "`n") + "`n")
        $values = @{}
        $activeWorkers = @()
        foreach ($line in $cleanup) {
            if ($line -match '^([^=]+)=(.*)$') { $values[$Matches[1]] = $Matches[2] }
            elseif (-not [string]::IsNullOrWhiteSpace([string]$line)) { $activeWorkers += [string]$line }
        }
        $clean = [string]$values.ActiveState -ceq 'inactive' -and [string]$values.MainPID -ceq '0' -and $activeWorkers.Count -eq 0
        if ($SmokeSeconds -eq 0) {
            $clean = $clean -and [string]$values.Result -ceq 'success' -and [string]$values.ExecMainStatus -ceq '0'
        }
        $cleanupRecords += [ordered]@{ Role=[string]$entry.Role; ActiveState=[string]$values.ActiveState; MainPID=[uint64]$values.MainPID; Result=[string]$values.Result; ExecMainStatus=[int]$values.ExecMainStatus; ActiveWorkers=@($activeWorkers); Passed=$clean }
        if (-not $clean) { throw "Installed cleanup failed on $($entry.Host)." }
    }
    Write-Utf8 (Join-Path $evidence 'cleanup-results.json') (([ordered]@{ Schema='ardents-qualification-cleanup-v1'; Owners=@($cleanupRecords) } | ConvertTo-Json -Depth 5 -Compress) + "`n")
    if ($SmokeSeconds -gt 0) {
        Write-Utf8 (Join-Path $evidence 'smoke-verdict.json') (([ordered]@{ Schema='ardents-qualification-two-host-smoke-verdict-v1'; Seconds=$SmokeSeconds; Participants=$smokeSummaries; Passed=$true } | ConvertTo-Json -Depth 6 -Compress) + "`n")
    } else {
        Send-File (Join-Path $evidence 'reader.jsonl') $PublisherHost "$remoteRoot/reader.jsonl" 'upload Reader evidence for pair verification'
        Send-File (Join-Path $evidence 'publisher.jsonl') $PublisherHost "$remoteRoot/publisher.jsonl" 'upload Publisher evidence for pair verification'
        Send-File (Join-Path $evidence 'relay-results.json') $PublisherHost "$remoteRoot/relay-results.json" 'upload relay counters for pair verification'
        Send-File (Join-Path $evidence 'node-results.json') $PublisherHost "$remoteRoot/node-results.json" 'upload Node owner evidence for pair verification'
        Send-File (Join-Path $evidence 'cleanup-results.json') $PublisherHost "$remoteRoot/cleanup-results.json" 'upload joined cleanup evidence for pair verification'
        $paired = Invoke-SSH $PublisherHost "/usr/lib/ardents/qualification/ardents-qualification verify-pair '$remoteRoot/reader.jsonl' '$remoteRoot/publisher.jsonl' '$remoteRoot/network-manifest.json' '$remoteRoot/relay-results.json' '$remoteRoot/node-results.json' '$($inputFiles.node_inventory)' '$remoteRoot/cleanup-results.json'" 'verify paired evidence'
        Write-Utf8 (Join-Path $evidence 'paired-verdict.json') (($paired -join "`n") + "`n")
    }
} catch {
    $runFailure = $_.Exception
    foreach ($entry in @(@{ Host=$PublisherHost; Invocation=$publisherInvocation; Name='publisher' }, @{ Host=$ReaderHost; Invocation=$readerInvocation; Name='reader' })) {
        if ($entry.Invocation -match '^[0-9a-f]{32}$') {
            try { Write-Utf8 (Join-Path $evidence "$($entry.Name).failed.jsonl") (((Journal $entry.Host $entry.Invocation) -join "`n") + "`n") }
            catch { $cleanupFailures.Add("retain $($entry.Name) failed journal: $($_.Exception.Message)") }
            try { Write-Utf8 (Join-Path $evidence "$($entry.Name).failed-status.txt") (((Invoke-SSH $entry.Host 'systemctl show ardents-endpoint.service -p ActiveState -p MainPID -p Result -p ExecMainStatus' 'read failed Endpoint status') -join "`n") + "`n") }
            catch { $cleanupFailures.Add("retain $($entry.Name) failed status: $($_.Exception.Message)") }
        }
    }
    if ($startedRelays.Count -ne 0) {
        try { Capture-FailedNetworkRelays }
        catch { $cleanupFailures.Add("retain failed relay/recovery evidence: $($_.Exception.Message)") }
    }
} finally {
    foreach ($hostName in @($deployedOwners) | Select-Object -Unique) {
        try { [void](Invoke-SSH $hostName "timeout 30s systemctl stop ardents-endpoint.service; systemctl reset-failed ardents-endpoint.service; test `"`$(systemctl show ardents-endpoint.service -p ActiveState --value)`" = inactive; test -z `"`$(systemctl list-units 'ardents-stream-qualification-*@*.service' --state=active,activating,deactivating --plain --no-legend)`"" 'stop and verify Endpoint attempt') }
        catch { $cleanupFailures.Add("Endpoint cleanup on ${hostName}: $($_.Exception.Message)") }
    }
    for ($clockIndex = $clockUnits.Count - 1; $clockIndex -ge 0; $clockIndex--) {
        $clock = $clockUnits[$clockIndex]
        try { [void](Invoke-SSH ([string]$clock.Host) "timeout 5s systemctl stop '$($clock.Unit)'; systemctl reset-failed '$($clock.Unit)'; test `"`$(systemctl show '$($clock.Unit)' -p ActiveState --value)`" = inactive" 'stop and verify clock observer attempt') }
        catch { $cleanupFailures.Add("clock observer $($clock.Unit): $($_.Exception.Message)") }
    }
    for ($nodeIndex = $startedNodes.Count - 1; $nodeIndex -ge 0; $nodeIndex--) {
        $started = $startedNodes[$nodeIndex]
        try { [void](Invoke-SSH ([string]$started.Machine) "timeout 30s systemctl stop '$($started.SamplerUnit)' '$($started.Unit)'; systemctl reset-failed '$($started.SamplerUnit)' '$($started.Unit)'; test `"`$(systemctl show '$($started.SamplerUnit)' -p ActiveState --value)`" = inactive; test `"`$(systemctl show '$($started.Unit)' -p ActiveState --value)`" = inactive" 'stop and verify Route Node attempt') }
        catch { $cleanupFailures.Add("Route Node $($started.ID): $($_.Exception.Message)") }
    }
    for ($sourceIndex = $startedSources.Count - 1; $sourceIndex -ge 0; $sourceIndex--) {
        $started = $startedSources[$sourceIndex]
        try { [void](Invoke-SSH ([string]$started.Machine) "timeout 30s systemctl stop '$($started.SamplerUnit)' '$($started.Unit)'; systemctl reset-failed '$($started.SamplerUnit)' '$($started.Unit)'; test `"`$(systemctl show '$($started.SamplerUnit)' -p ActiveState --value)`" = inactive; test `"`$(systemctl show '$($started.Unit)' -p ActiveState --value)`" = inactive" 'stop and verify State Source attempt') }
        catch { $cleanupFailures.Add("State Source $($started.ID): $($_.Exception.Message)") }
    }
    for ($ownerIndex = $ownerSlices.Count - 1; $ownerIndex -ge 0; $ownerIndex--) {
        $owner = $ownerSlices[$ownerIndex]
        try {
            [void](Invoke-SSH ([string]$owner.Machine) "timeout 5s systemctl stop '$($owner.SamplerUnit)'; timeout 30s systemctl stop '$($owner.Unit)'; systemctl revert '$($owner.Unit)'; systemctl daemon-reload; test `"`$(systemctl show '$($owner.Unit)' -p ActiveState --value)`" = inactive; test -z `"`$(systemctl show '$($owner.Unit)' -p DropInPaths --value)`"" 'remove and verify whole-owner slice')
        } catch {
            $cleanupFailures.Add("whole-owner slice on $($owner.Host): $($_.Exception.Message)")
        }
    }
    foreach ($scheduled in $recoveryUnits) {
        try { [void](Invoke-SSH ([string]$scheduled.Host) "timeout 15s systemctl stop '$($scheduled.Unit)'; systemctl reset-failed '$($scheduled.Unit)'; test `"`$(systemctl show '$($scheduled.Unit)' -p ActiveState --value)`" = inactive" 'stop and verify recovery scheduler') }
        catch { $cleanupFailures.Add("recovery scheduler $($scheduled.Unit): $($_.Exception.Message)") }
    }
    for ($relayIndex = $startedRelays.Count - 1; $relayIndex -ge 0; $relayIndex--) {
        $relay = $startedRelays[$relayIndex]
        try { [void](Invoke-SSH ([string]$relay.Host) "timeout 5s systemctl stop '$($relay.SamplerUnit)'; systemctl reset-failed '$($relay.SamplerUnit)'; test `"`$(systemctl show '$($relay.SamplerUnit)' -p ActiveState --value)`" = inactive" 'stop and verify relay sampler') }
        catch { $cleanupFailures.Add("relay sampler $($relay.SamplerUnit): $($_.Exception.Message)") }
        try { [void](Invoke-SSH ([string]$relay.Host) "docker rm -f '$($relay.Container)' >/dev/null; ! docker container inspect '$($relay.Container)' >/dev/null 2>&1" 'remove and verify isolated relay') }
        catch { $cleanupFailures.Add("relay $($relay.Container): $($_.Exception.Message)") }
    }
    foreach ($hostName in @($PublisherHost, $ReaderHost, [string]$authority.Host) | Select-Object -Unique) {
        try { [void](Invoke-SSH $hostName "case '$remoteRoot' in /var/tmp/ardents-qualification-[0-9a-f]*) rm -rf -- '$remoteRoot' && test ! -e '$remoteRoot';; *) exit 64;; esac" 'remove and verify bounded remote staging') }
        catch { $cleanupFailures.Add("remote staging on ${hostName}: $($_.Exception.Message)") }
    }
}
$failureMessages = [Collections.Generic.List[string]]::new()
if ($null -ne $runFailure) { $failureMessages.Add($runFailure.Message) }
foreach ($cleanupFailure in $cleanupFailures) { $failureMessages.Add("cleanup/evidence: $cleanupFailure") }
$passed = $failureMessages.Count -eq 0
$failure = if ($passed) { '' } else { $failureMessages -join [Environment]::NewLine }
if (-not $passed) { Write-Utf8 (Join-Path $evidence 'failure.txt') ($failure + "`n") }
Write-Utf8 (Join-Path $evidence 'attempt.json') (([ordered]@{ Schema=$attemptSchema; Mode=$attemptMode; SmokeSeconds=$SmokeSeconds; PublisherHost=$PublisherHost; ReaderHost=$ReaderHost; Profile=$profile; Condition=$condition; Seed=$seed; SourceCommit=$SourceCommit; PublisherInvocation=$publisherInvocation; ReaderInvocation=$readerInvocation; Passed=$passed; Failure=$failure; CompletedAt=[DateTime]::UtcNow.ToString('o') } | ConvertTo-Json -Compress) + "`n")
if (-not $passed) { throw $failure }
