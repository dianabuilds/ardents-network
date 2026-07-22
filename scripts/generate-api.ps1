param([switch]$Check)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$expectedVersions = @{
    "protoc" = "libprotoc 33.4"
    "protoc-gen-go" = "protoc-gen-go v1.36.9"
    "protoc-gen-connect-go" = "1.19.1"
}
foreach ($tool in $expectedVersions.Keys) {
    if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) { throw "$tool is required to generate Ardents APIs" }
    $actual = ((& $tool --version).Trim() -replace '\.exe', '')
    if ($actual -ne $expectedVersions[$tool]) { throw "$tool version mismatch: expected '$($expectedVersions[$tool])', got '$actual'" }
}

$operatorSources = @(Get-ChildItem api/ardents/v1 -Filter *.proto | ForEach-Object { $_.FullName.Substring($root.Length + 1).Replace('\','/') })
$applicationSources = @(Get-ChildItem api/ardents/application/v1 -Filter *.proto | ForEach-Object { $_.FullName.Substring($root.Length + 1).Replace('\','/') })
$artifactSource = "api/ardents/identity/v1/artifacts.proto"
$applicationIdentitySource = "api/ardents/application/v1/identity.proto"
$applicationContentSources = @($applicationSources | Where-Object { $_ -ne $applicationIdentitySource })
$plannedIdentitySources = @("api/ardents/v1/identity.proto", "api/ardents/application/v1/identity.proto")

$outputRoot = $root
$temporaryRoot = $null
try {
    if ($Check) {
        $temporaryRoot = Join-Path ([IO.Path]::GetTempPath()) ("ardents-api-" + [guid]::NewGuid())
        New-Item -ItemType Directory -Path $temporaryRoot | Out-Null
        $outputRoot = $temporaryRoot
    }
    & protoc --proto_path=$root --go_out=$outputRoot --go_opt=module=ardents --connect-go_out=$outputRoot --connect-go_opt=module=ardents @operatorSources
    if ($LASTEXITCODE -ne 0) { throw "Operator API generation failed" }
    if ($applicationContentSources.Count -gt 0) {
        & protoc --proto_path=$root --go_out=$outputRoot --go_opt=module=ardents --connect-go_out=$outputRoot --connect-go_opt=module=ardents @applicationContentSources
        if ($LASTEXITCODE -ne 0) { throw "Application content API generation failed" }
    }
    if (Test-Path $applicationIdentitySource) {
        $sdkApplicationIdentityMapping = "Mapi/ardents/application/v1/identity.proto=ardents/sdk/go/protocol/applicationidentityv1"
        & protoc --proto_path=$root --go_out=$outputRoot --go_opt=module=ardents --go_opt=$sdkApplicationIdentityMapping --go_opt="Mapi/ardents/identity/v1/artifacts.proto=ardents/sdk/go/protocol/identityv1" --connect-go_out=$outputRoot --connect-go_opt=module=ardents --connect-go_opt=$sdkApplicationIdentityMapping $applicationIdentitySource
        if ($LASTEXITCODE -ne 0) { throw "Application identity SDK generation failed" }
        $serverApplicationMapping = "Mapi/ardents/application/v1/identity.proto=ardents/internal/applicationapi/protocol/applicationv1"
        & protoc --proto_path=$root --go_out=$outputRoot --go_opt=module=ardents --go_opt=$serverApplicationMapping --go_opt="Mapi/ardents/identity/v1/artifacts.proto=ardents/internal/identity/protocol" --connect-go_out=$outputRoot --connect-go_opt=module=ardents --connect-go_opt=$serverApplicationMapping $applicationIdentitySource
        if ($LASTEXITCODE -ne 0) { throw "server Application identity API generation failed" }
    }

    if (Test-Path $artifactSource) {
        & protoc --proto_path=$root --go_out=$outputRoot --go_opt=module=ardents --go_opt="Mapi/ardents/identity/v1/artifacts.proto=ardents/internal/identity/protocol" $artifactSource
        if ($LASTEXITCODE -ne 0) { throw "server artifact generation failed" }
        & protoc --proto_path=$root --go_out=$outputRoot --go_opt=module=ardents --go_opt="Mapi/ardents/identity/v1/artifacts.proto=ardents/sdk/go/protocol/identityv1" $artifactSource
        if ($LASTEXITCODE -ne 0) { throw "SDK artifact generation failed" }
    }

    if ($Check) {
        $generated = @(Get-ChildItem $temporaryRoot -Recurse -File -Include *.pb.go,*.connect.go)
        foreach ($file in $generated) {
            $relative = $file.FullName.Substring($temporaryRoot.Length + 1)
            $expected = Join-Path $root $relative
            if (-not (Test-Path $expected) -or (Get-FileHash $expected).Hash -ne (Get-FileHash $file.FullName).Hash) { throw "generated API is stale: $relative" }
        }
        foreach ($source in $plannedIdentitySources) {
            if (Test-Path $source) { continue }
            Write-Verbose "planned identity source not introduced yet: $source"
        }
    }

    & (Join-Path $PSScriptRoot "generate-identity-artifact-vectors.ps1") -Check:$Check
    if ($LASTEXITCODE -ne 0) { throw "identity artifact vector generation failed" }
} finally {
    if ($temporaryRoot -and (Test-Path $temporaryRoot)) { Remove-Item -LiteralPath $temporaryRoot -Recurse -Force }
}
