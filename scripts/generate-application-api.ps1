param([switch]$Check)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$expectedVersions = @{
    "protoc" = "libprotoc 33.4"
    "protoc-gen-go" = "protoc-gen-go v1.36.9"
    "protoc-gen-connect-go" = "1.19.1"
}

foreach ($tool in "protoc", "protoc-gen-go", "protoc-gen-connect-go") {
    if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) {
        throw "$tool is required to generate the Application Interface"
    }
    $actualVersion = ((& $tool --version).Trim() -replace '\.exe', '')
    if ($actualVersion -ne $expectedVersions[$tool]) {
        throw "$tool version mismatch: expected '$($expectedVersions[$tool])', got '$actualVersion'"
    }
}

$outputRoot = $root
$temporaryRoot = $null
try {
    if ($Check) {
        $temporaryRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("ardents-application-api-" + [guid]::NewGuid())
        New-Item -ItemType Directory -Path $temporaryRoot | Out-Null
        $outputRoot = $temporaryRoot
    }

    & protoc --proto_path=$root `
        --go_out=$outputRoot --go_opt=module=ardents `
        --connect-go_out=$outputRoot --connect-go_opt=module=ardents `
        api/ardents/application/v1/content.proto
    if ($LASTEXITCODE -ne 0) { throw "Application Interface generation failed" }

    if ($Check) {
        foreach ($relativePath in @(
            "sdk/go/protocol/applicationv1/content.pb.go",
            "sdk/go/protocol/applicationv1/applicationv1connect/content.connect.go"
        )) {
            $expected = Join-Path $root $relativePath
            $actual = Join-Path $temporaryRoot $relativePath
            if (-not (Test-Path $expected) -or
                (Get-FileHash $expected).Hash -ne (Get-FileHash $actual).Hash) {
                throw "generated Application Interface is stale: $relativePath"
            }
        }
    }
} finally {
    if ($temporaryRoot -and (Test-Path $temporaryRoot)) {
        Remove-Item -LiteralPath $temporaryRoot -Recurse -Force
    }
}
