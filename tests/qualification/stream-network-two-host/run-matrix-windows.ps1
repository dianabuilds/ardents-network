param(
    [Parameter(Mandatory = $true)][string]$Matrix,
    [Parameter(Mandatory = $true)][string]$SSHKey,
    [Parameter(Mandatory = $true)][string]$RunnerBinary,
    [Parameter(Mandatory = $true)][string]$WorkerBinary,
    [Parameter(Mandatory = $true)][string]$RelayBinary,
    [Parameter(Mandatory = $true)][string]$NodeBinary,
    [Parameter(Mandatory = $true)][string]$EvidenceRoot,
    [ValidateSet('smoke','acceptance')][string]$Mode = 'smoke',
    [ValidateRange(1,120)][int]$SmokeSeconds = 10,
    [string]$User = 'root'
)

$ErrorActionPreference = 'Stop'
$repository = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..\..'))
$matrixPath = [IO.Path]::GetFullPath($Matrix)
$evidence = [IO.Path]::GetFullPath($EvidenceRoot)
$runScript = Join-Path $PSScriptRoot 'run-windows.ps1'
$net14vScript = Join-Path $PSScriptRoot 'verify-net14v-windows.ps1'
$net32Script = Join-Path $PSScriptRoot '..\net32-idle-one-host\run-windows.ps1'

function Resolve-File([string]$Path, [string]$Label) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { throw "$Label is absent." }
    return (Resolve-Path -LiteralPath $Path).Path
}
function Resolve-ReferencedFile([string]$Path, [string]$Label) {
    if (-not [IO.Path]::IsPathRooted($Path)) { $Path = Join-Path (Split-Path -Parent $matrixPath) $Path }
    return Resolve-File $Path $Label
}
function Write-Utf8([string]$Path, [string]$Body) {
    [IO.File]::WriteAllText($Path, $Body, [Text.UTF8Encoding]::new($false))
}
function Emit([hashtable]$Record) {
    $line = ($Record | ConvertTo-Json -Depth 8 -Compress) + [Environment]::NewLine
    [IO.File]::AppendAllText($reportPath, $line, [Text.UTF8Encoding]::new($false))
}
function Cell-Key([string]$Carrier, [string]$Cell, [string]$Profile) {
    return "$Carrier|$Cell|$Profile"
}

$matrixPath = Resolve-File $matrixPath 'Matrix'
$keyPath = Resolve-File $SSHKey 'SSHKey'
$runnerPath = Resolve-File $RunnerBinary 'RunnerBinary'
$workerPath = Resolve-File $WorkerBinary 'WorkerBinary'
$relayPath = Resolve-File $RelayBinary 'RelayBinary'
$nodePath = Resolve-File $NodeBinary 'NodeBinary'
foreach ($script in @($runScript,$net14vScript,$net32Script)) { [void](Resolve-File $script 'qualification script') }
if ($evidence.StartsWith($repository + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) { throw 'Matrix evidence must remain outside the repository.' }
if (Test-Path -LiteralPath $evidence) { throw 'EvidenceRoot must be a new directory.' }
[IO.Directory]::CreateDirectory($evidence) | Out-Null
$reportPath = Join-Path $evidence 'matrix-report.jsonl'
Write-Utf8 $reportPath ''

$spec = Get-Content -LiteralPath $matrixPath -Raw | ConvertFrom-Json
$validation = [Collections.Generic.List[string]]::new()
if ([string]$spec.Schema -cne 'ardents-qualification-matrix-v1') { $validation.Add('matrix schema is invalid') }
$sourceCommit = [string]$spec.SourceCommit
if ($sourceCommit -cnotmatch '^[0-9a-f]{40}$') { $validation.Add('matrix SourceCommit must be full lowercase Git hex') }
$actualCommit = ((& git -C $repository rev-parse HEAD 2>$null) -join '').Trim()
if ($LASTEXITCODE -ne 0 -or $actualCommit -cne $sourceCommit) { $validation.Add('matrix SourceCommit differs from checked-out HEAD') }
$sourceStatus = @(& git -C $repository status --porcelain --untracked-files=normal)
if ($LASTEXITCODE -ne 0 -or $sourceStatus.Count -ne 0) { $validation.Add('matrix requires a clean source commit') }

$expected = @(
    'tcp-tls|net14ad|client-to-publisher',
    'tcp-tls|net14-recovery|client-to-publisher',
    'tcp-tls|net14ad|publisher-to-client',
    'tcp-tls|net14-recovery|publisher-to-client',
    'quic|net14ad|client-to-publisher',
    'quic|net14-recovery|client-to-publisher',
    'quic|net14ad|publisher-to-client',
    'quic|net14-recovery|publisher-to-client'
)
$cells = @($spec.Cells)
if ($cells.Count -ne $expected.Count) { $validation.Add("matrix has $($cells.Count) cells; expected $($expected.Count)") }
$seen = @{}
$validated = @{}
foreach ($cell in $cells) {
    $key = Cell-Key ([string]$cell.Carrier) ([string]$cell.Cell) ([string]$cell.Profile)
    $errors = [Collections.Generic.List[string]]::new()
    if ($expected -cnotcontains $key) { $errors.Add('cell identity is outside the fixed matrix') }
    if ($seen.ContainsKey($key)) { $errors.Add('cell identity is duplicated') } else { $seen[$key] = $true }
    try {
        $inputsPath = Resolve-ReferencedFile ([string]$cell.Inputs) "$key prepared inputs"
        $inputs = Get-Content -LiteralPath $inputsPath -Raw | ConvertFrom-Json
        if ([string]$inputs.Schema -cne 'ardents-qualification-prepared-inputs-v1') { $errors.Add('prepared input schema is invalid') }
        if ([string]$inputs.SourceCommit -cne $sourceCommit) { $errors.Add('prepared input SourceCommit differs from matrix') }
        $manifestPath = Resolve-File ([string]$inputs.NetworkManifest) "$key NetworkManifest"
        $manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
        if ([string]$manifest.Carrier -cne [string]$cell.Carrier -or [string]$manifest.Cell -cne [string]$cell.Cell) { $errors.Add('network manifest identity differs from cell') }
        foreach ($field in @('NodeInventory','PublisherPlan','ReaderPlanTemplate','AuthorityInventory','Net32Plan')) {
            try { [void](Resolve-File ([string]$inputs.$field) "$key $field") } catch { $errors.Add($_.Exception.Message) }
        }
        $profileNumber = if ([string]$cell.Profile -ceq 'client-to-publisher') { 1 } else { 2 }
        $conditionNumber = if ([string]$cell.Cell -ceq 'net14ad') { 1 } else { 3 }
        foreach ($planField in @('PublisherPlan','ReaderPlanTemplate')) {
            try {
                $plan = Get-Content -LiteralPath ([string]$inputs.$planField) -Raw | ConvertFrom-Json
                foreach ($participant in @($plan.Participants)) {
                    if ([int]$participant.Profile -ne $profileNumber -or [int]$participant.Condition -ne $conditionNumber) { $errors.Add("$planField profile or condition differs from cell"); break }
                }
            } catch { $errors.Add("$planField cannot be decoded: $($_.Exception.Message)") }
        }
        if ($errors.Count -eq 0) { $validated[$key] = [ordered]@{ Inputs=$inputs; InputsPath=$inputsPath; Manifest=$manifestPath } }
    } catch { $errors.Add($_.Exception.Message) }
    foreach ($errorText in $errors) { $validation.Add("${key}: $errorText") }
}
foreach ($key in $expected) { if (-not $seen.ContainsKey($key)) { $validation.Add("missing matrix cell $key") } }
try {
    $net32InputsPath = Resolve-ReferencedFile ([string]$spec.Net32Inputs) 'NET-32 prepared inputs'
    $net32Inputs = Get-Content -LiteralPath $net32InputsPath -Raw | ConvertFrom-Json
    if ([string]$net32Inputs.Schema -cne 'ardents-qualification-prepared-inputs-v1' -or [string]$net32Inputs.SourceCommit -cne $sourceCommit) { $validation.Add('NET-32 prepared inputs differ from matrix candidate') }
    [void](Resolve-File ([string]$net32Inputs.Net32Plan) 'NET-32 plan')
    [void](Resolve-File ([string]$net32Inputs.AuthorityInventory) 'NET-32 authority inventory')
} catch { $validation.Add($_.Exception.Message) }

foreach ($errorText in $validation) { Emit ([ordered]@{ Kind='validation'; Passed=$false; Failure=$errorText }) }
if ($validation.Count -ne 0) { throw "Matrix validation found $($validation.Count) independent failures; see $reportPath" }
Emit ([ordered]@{ Kind='validation'; Passed=$true; Cells=$expected.Count; SourceCommit=$sourceCommit; Mode=$Mode })

$results = @{}
$failures = 0
foreach ($key in $expected) {
    $entry = $validated[$key]
    $parts = $key.Split('|')
    $cellEvidence = Join-Path $evidence (($parts -join '-'))
    $arguments = @{
        PublisherHost=[string]$entry.Inputs.PublisherHost; ReaderHost=[string]$entry.Inputs.ReaderHost; SSHKey=$keyPath
        RunnerBinary=$runnerPath; WorkerBinary=$workerPath; RelayBinary=$relayPath; NodeBinary=$nodePath; SourceCommit=$sourceCommit
        NodeInventory=[string]$entry.Inputs.NodeInventory; PublisherPlan=[string]$entry.Inputs.PublisherPlan
        ReaderPlanTemplate=[string]$entry.Inputs.ReaderPlanTemplate; AuthorityInventory=[string]$entry.Inputs.AuthorityInventory
        NetworkManifest=[string]$entry.Inputs.NetworkManifest; EvidenceOutput=$cellEvidence; User=$User
    }
    if ($Mode -ceq 'smoke') { $arguments.SmokeSeconds = $SmokeSeconds }
    try {
        & $runScript @arguments
        $results[$key] = [ordered]@{ Passed=$true; Evidence=$cellEvidence }
        Emit ([ordered]@{ Kind='cell'; Cell=$key; Mode=$Mode; Passed=$true; Evidence=$cellEvidence })
    } catch {
        $failures++
        $results[$key] = [ordered]@{ Passed=$false; Evidence=$cellEvidence; Failure=$_.Exception.Message }
        Emit ([ordered]@{ Kind='cell'; Cell=$key; Mode=$Mode; Passed=$false; Evidence=$cellEvidence; Failure=$_.Exception.Message })
    }
}

if ($Mode -ceq 'acceptance') {
    foreach ($carrier in @('tcp-tls','quic')) {
        foreach ($profile in @('client-to-publisher','publisher-to-client')) {
            $baseKey = Cell-Key $carrier 'net14ad' $profile
            $episodeKey = Cell-Key $carrier 'net14-recovery' $profile
            $name = "$carrier|net14v|$profile"
            $net14vEvidence = Join-Path $evidence "$carrier-net14v-$profile"
            if (-not $results[$baseKey].Passed -or -not $results[$episodeKey].Passed) {
                $failures++
                Emit ([ordered]@{ Kind='net14v'; Cell=$name; Passed=$false; Failure='normal or recovery source cell failed' })
                continue
            }
            try {
                & $net14vScript -PublisherHost ([string]$validated[$episodeKey].Inputs.PublisherHost) -SSHKey $keyPath -RunnerBinary $runnerPath -BaselineEvidence $results[$baseKey].Evidence -EpisodeEvidence $results[$episodeKey].Evidence -EvidenceOutput $net14vEvidence -User $User
                Emit ([ordered]@{ Kind='net14v'; Cell=$name; Passed=$true; Evidence=$net14vEvidence })
            } catch {
                $failures++
                Emit ([ordered]@{ Kind='net14v'; Cell=$name; Passed=$false; Evidence=$net14vEvidence; Failure=$_.Exception.Message })
            }
        }
    }
    $net32Evidence = Join-Path $evidence 'net32-idle'
    try {
        & $net32Script -EndpointHost ([string]$net32Inputs.ReaderHost) -SSHKey $keyPath -RunnerBinary $runnerPath -SourceCommit $sourceCommit -Plan ([string]$net32Inputs.Net32Plan) -AuthorityInventory ([string]$net32Inputs.AuthorityInventory) -EvidenceOutput $net32Evidence -User $User
        Emit ([ordered]@{ Kind='net32'; Passed=$true; Evidence=$net32Evidence })
    } catch {
        $failures++
        Emit ([ordered]@{ Kind='net32'; Passed=$false; Evidence=$net32Evidence; Failure=$_.Exception.Message })
    }
}

Emit ([ordered]@{ Kind='matrix-result'; Mode=$Mode; SourceCommit=$sourceCommit; Passed=($failures -eq 0); Failures=$failures; CompletedAt=[DateTimeOffset]::UtcNow.ToString('o') })
if ($failures -ne 0) { throw "Matrix completed with $failures failures; see $reportPath" }