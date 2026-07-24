$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$policyPath = Join-Path $root "scripts/release/materials.json"

if (-not (Test-Path -LiteralPath $policyPath -PathType Leaf)) {
    throw "release materials policy is missing"
}

$policy = Get-Content -Raw -LiteralPath $policyPath | ConvertFrom-Json
if ($policy.schema_version -ne 1 -or $policy.platform -ne "linux/amd64") {
    throw "release materials policy contract mismatch"
}
if ($policy.source.uri -cne "git+https://github.com/dianabuilds/ardents-network.git") {
    throw "release source material URI mismatch"
}

$requiredRoles = @(
    "bundle_builder"
    "ingress_builder"
    "metadata_builder"
    "node_builder"
    "node_runtime"
    "smoke_verifier"
    "test_runner_go"
    "test_runner_powershell"
) | Sort-Object
$roles = [Collections.Generic.List[string]]::new()
$allowedReferences = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
foreach ($image in @($policy.images)) {
    if ($image.name -notmatch '^[a-z][a-z0-9_-]+$' -or
        $image.kind -notin @("toolchain", "base") -or
        $image.reference -notmatch '^[a-z0-9./-]+(?:\:[a-z0-9._-]+)?@sha256:([0-9a-f]{64})$' -or
        $image.digest.sha256 -cne $Matches[1]) {
        throw "release image material is not immutable: $($image.name)"
    }
    if (-not $allowedReferences.Add([string]$image.reference)) {
        throw "duplicate release image material reference: $($image.reference)"
    }
    foreach ($role in @($image.roles)) {
        if ($roles.Contains([string]$role)) { throw "duplicate release material role: $role" }
        $roles.Add([string]$role)
    }
}
$roleDifference = @(Compare-Object $requiredRoles @($roles | Sort-Object))
if ($roleDifference.Count -ne 0) {
    throw "release materials do not cover every builder/runtime role: $($roleDifference | Out-String)"
}

function Get-RoleReference([string]$Role) {
    if ($Role -eq "scratch") { return "scratch" }
    $matches = @($policy.images | Where-Object { @($_.roles) -contains $Role })
    if ($matches.Count -ne 1) { throw "release material role is ambiguous: $Role" }
    return [string]$matches[0].reference
}

function Test-DockerfileRoleBindings([string]$Content, [string[]]$ExpectedRoles) {
    $actual = @([regex]::Matches($Content, '(?m)^FROM\s+(\S+)') | ForEach-Object { $_.Groups[1].Value })
    if ($actual.Count -ne $ExpectedRoles.Count) { return $false }
    for ($index = 0; $index -lt $ExpectedRoles.Count; $index++) {
        if ($actual[$index] -cne (Get-RoleReference $ExpectedRoles[$index])) {
            return $false
        }
    }
    return $true
}

$dockerfileRoles = [ordered]@{
    "node.Dockerfile" = @("node_builder", "node_runtime")
    "workload-ingress-proxy.Dockerfile" = @("ingress_builder", "scratch")
    "test-runner.Dockerfile" = @("test_runner_powershell", "test_runner_go")
}
foreach ($entry in $dockerfileRoles.GetEnumerator()) {
    $dockerfile = Join-Path $root "deploy/docker/images/$($entry.Key)"
    $content = Get-Content -Raw -LiteralPath $dockerfile
    if (-not (Test-DockerfileRoleBindings $content $entry.Value)) {
        throw "Dockerfile stages do not match release material roles: $($entry.Key)"
    }
}

$nodeDockerfile = Get-Content -Raw -LiteralPath (Join-Path $root "deploy/docker/images/node.Dockerfile")
$wrongRuntime = $nodeDockerfile.Replace(
    (Get-RoleReference "node_runtime"),
    (Get-RoleReference "node_builder")
)
if (Test-DockerfileRoleBindings $wrongRuntime $dockerfileRoles["node.Dockerfile"]) {
    throw "release materials gate accepted a cross-role Dockerfile substitution"
}

$buildScript = Get-Content -Raw -LiteralPath (Join-Path $root "scripts/release/build.ps1")
if ($buildScript -match '--mount\s+source=ardents-(?:go-build|go-mod)-cache' -or
    $buildScript -notmatch 'docker build\s+`\s*\r?\n\s*--no-cache' -or
    $buildScript -notmatch '\$bundleBuilderImage\s*=\s*Get-ReleaseImage\s+"bundle_builder"' -or
    $buildScript -notmatch '\$metadataBuilderImage\s*=\s*Get-ReleaseImage\s+"metadata_builder"' -or
    $buildScript -notmatch '-w /workspace \$bundleBuilderImage' -or
    $buildScript -notmatch '-w /workspace \$metadataBuilderImage') {
    throw "release builds do not isolate mutable build caches"
}

$workflow = Get-Content -Raw -LiteralPath (Join-Path $root ".github/workflows/ci.yml")
if ($workflow -match 'debian:bookworm-slim(?!@sha256:)') {
    throw "release workflow uses a mutable verifier image"
}
$smokeImage = @($policy.images | Where-Object { @($_.roles) -contains "smoke_verifier" })
if ($smokeImage.Count -ne 1 -or -not $workflow.Contains([string]$smokeImage[0].reference)) {
    throw "release workflow verifier image differs from material policy"
}
if ($workflow -notmatch '(?m)^\s+release-builds:\s*$' -or
    $workflow -notmatch 'rebuild:\s*\[a,\s*b\]' -or
    $workflow -match 'Build twice and compare' -or
    $workflow -notmatch '\./tests/ci/release-materials-policy-gate\.ps1') {
    throw "release workflow does not use independently hosted material-policy builds"
}

Write-Host "release-materials-policy-gate=passed"
