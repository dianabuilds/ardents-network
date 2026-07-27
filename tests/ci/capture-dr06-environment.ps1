param(
    [Parameter(Mandatory = $true)]
    [string]$Gate,
    [string]$ContractPath = "tests/ci/dr06-gates.json"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location $root
$rootPath = [IO.Path]::GetFullPath($root).TrimEnd(
    [IO.Path]::DirectorySeparatorChar,
    [IO.Path]::AltDirectorySeparatorChar
)
$contractFullPath = [IO.Path]::GetFullPath($ContractPath)
if (-not $contractFullPath.StartsWith(
    $rootPath + [IO.Path]::DirectorySeparatorChar,
    [StringComparison]::OrdinalIgnoreCase
)) {
    throw "DR-06 gate contract must stay inside the repository"
}
$contractDocument = Get-Content -Raw -LiteralPath $contractFullPath | ConvertFrom-Json
$gateContracts = @($contractDocument.gates | Where-Object id -ceq $Gate)
if ($contractDocument.schema_version -ne 1 -or $gateContracts.Count -ne 1) {
    throw "DR-06 gate is absent or ambiguous: $Gate"
}
$gateContract = $gateContracts[0]
$environmentContract = [string]$gateContract.environment
$outputPath = [string]$gateContract.environment_output_path
if ([string]::IsNullOrWhiteSpace($environmentContract) -or
    [string]::IsNullOrWhiteSpace($outputPath)) {
    throw "DR-06 gate environment mapping is incomplete: $Gate"
}

function Invoke-Version([scriptblock]$Command) {
    try {
        $value = (& $Command 2>$null | Out-String).Trim()
        if ($LASTEXITCODE -ne 0) { return "" }
        return $value
    } catch {
        return ""
    }
}

$sourceCommit = [string]$env:GITHUB_SHA
if ([string]::IsNullOrWhiteSpace($sourceCommit)) {
    $sourceCommit = (git rev-parse HEAD).Trim()
    if ($LASTEXITCODE -ne 0) { throw "source commit is unavailable" }
}
if ($sourceCommit -notmatch '^[0-9a-f]{40}$') {
    throw "source commit is not an exact Git identity: $sourceCommit"
}

$materialsPath = Join-Path $root "scripts/release/materials.json"
$goModPath = Join-Path $root "go.mod"
$workflowPath = Join-Path $root ".github/workflows/ci.yml"
$materials = Get-Content -Raw -LiteralPath $materialsPath | ConvertFrom-Json
$materialNames = @($gateContract.material_names)
$selectedMaterials = @($materials.images | Where-Object {
    $materialNames -contains [string]$_.name
})
if ($selectedMaterials.Count -ne $materialNames.Count) {
    throw "DR-06 gate refers to an unknown immutable material: $Gate"
}
$manifest = [ordered]@{
    schema_version = 1
    gate = $Gate
    environment_contract = $EnvironmentContract
    source_commit = $sourceCommit
    workflow_run = [ordered]@{
        run_id = [string]$env:GITHUB_RUN_ID
        run_attempt = [string]$env:GITHUB_RUN_ATTEMPT
        runner_name = [string]$env:RUNNER_NAME
        runner_os = [string]$env:RUNNER_OS
        runner_arch = [string]$env:RUNNER_ARCH
        runner_image_os = [string]$env:ImageOS
        runner_image_version = [string]$env:ImageVersion
    }
    toolchains = [ordered]@{
        go = Invoke-Version { go version }
        powershell = $PSVersionTable.PSVersion.ToString()
        docker_client = Invoke-Version { docker version --format '{{.Client.Version}}' }
        docker_server = Invoke-Version { docker version --format '{{.Server.Version}}' }
    }
    contracts = [ordered]@{
        go_mod_sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $goModPath).Hash.ToLowerInvariant()
        workflow_sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $workflowPath).Hash.ToLowerInvariant()
        gate_contract_sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $contractFullPath).Hash.ToLowerInvariant()
        release_materials_sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $materialsPath).Hash.ToLowerInvariant()
    }
    declared_material_names = $materialNames
    immutable_materials = $selectedMaterials
    captured_at = (Get-Date).ToUniversalTime().ToString("o")
}

$fullPath = [IO.Path]::GetFullPath($OutputPath)
$rootPrefix = $rootPath + [IO.Path]::DirectorySeparatorChar
if (-not $fullPath.StartsWith($rootPrefix, [StringComparison]::OrdinalIgnoreCase)) {
    throw "DR-06 environment output must stay inside the repository"
}
$parent = Split-Path -Parent $fullPath
if ($parent) { [IO.Directory]::CreateDirectory($parent) | Out-Null }
$manifest | ConvertTo-Json -Depth 12 | Set-Content -Encoding utf8 -LiteralPath $fullPath
Write-Host "dr06-environment=$Gate"
