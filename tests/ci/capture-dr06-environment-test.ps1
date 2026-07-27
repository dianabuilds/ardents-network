$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location $root
$fixtureRoot = Join-Path $root "tests/.artifacts/dr06-environment-test-$([guid]::NewGuid().ToString('N'))"
$outputPath = Join-Path $fixtureRoot "environment.json"
$contractPath = Join-Path $fixtureRoot "gates.json"

try {
    New-Item -ItemType Directory -Force $fixtureRoot | Out-Null
    $relativeOutput = $outputPath.Substring($root.Length + 1).Replace("\", "/")
    [ordered]@{
        schema_version = 1
        gates = @([ordered]@{
            id = "fixture"
            environment = "fixture-environment"
            environment_output_path = $relativeOutput
            material_names = @("go")
        })
    } | ConvertTo-Json -Depth 6 | Set-Content -Encoding utf8 -LiteralPath $contractPath
    ./tests/ci/capture-dr06-environment.ps1 `
        -Gate "fixture" `
        -ContractPath $contractPath
    $manifest = Get-Content -Raw -LiteralPath $outputPath | ConvertFrom-Json
    if ($manifest.schema_version -ne 1 -or
        $manifest.gate -ne "fixture" -or
        $manifest.environment_contract -ne "fixture-environment" -or
        $manifest.source_commit -notmatch '^[0-9a-f]{40}$' -or
        [string]::IsNullOrWhiteSpace($manifest.toolchains.go) -or
        [string]::IsNullOrWhiteSpace($manifest.toolchains.powershell) -or
        @($manifest.declared_material_names).Count -ne 1 -or
        @($manifest.immutable_materials).Count -ne 1) {
        throw "DR-06 environment manifest contract mismatch"
    }
    Write-Host "dr06-environment-test=passed"
} finally {
    if (Test-Path -LiteralPath $fixtureRoot) {
        Remove-Item -LiteralPath $fixtureRoot -Recurse -Force
    }
}
