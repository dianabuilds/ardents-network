param(
    [switch]$Verify
)

$ErrorActionPreference = "Stop"

if (-not ($IsWindows -or $env:OS -eq "Windows_NT")) {
    Write-Host "windows toolchain setup is only required on Windows"
    exit 0
}

$gcc = "C:\msys64\ucrt64\bin\gcc.exe"
$ar = "C:\msys64\ucrt64\bin\ar.exe"

if (-not (Test-Path $gcc)) {
    throw "gcc not found at $gcc"
}
if (-not (Test-Path $ar)) {
    throw "ar not found at $ar"
}

$rlnModuleDir = (& go list -m -f "{{.Dir}}" github.com/waku-org/go-zerokit-rln-x86_64).Trim()
if (-not $rlnModuleDir) {
    throw "could not resolve github.com/waku-org/go-zerokit-rln-x86_64 module dir"
}

$rlnLibDir = Join-Path $rlnModuleDir "libs\x86_64-pc-windows-gnu"
if (-not (Test-Path $rlnLibDir)) {
    throw "rln windows library dir not found at $rlnLibDir"
}

$toolchainDir = Join-Path $PSScriptRoot ".artifacts\toolchain\windows"
New-Item -ItemType Directory -Force -Path $toolchainDir | Out-Null

$stubSource = Join-Path $toolchainDir "stub_dl.c"
$stubObject = Join-Path $toolchainDir "stub_dl.o"
$stubLibrary = Join-Path $toolchainDir "libdl.a"

Set-Content -Path $stubSource -Value "void ardents_stub_dl(void) {}"
& $gcc -c $stubSource -o $stubObject
if ($LASTEXITCODE -ne 0) {
    throw "gcc failed while building libdl stub"
}

& $ar rcs $stubLibrary $stubObject
if ($LASTEXITCODE -ne 0) {
    throw "ar failed while archiving libdl stub"
}

$requiredFlags = "-L$toolchainDir -L$rlnLibDir -Wl,--start-group -lrln -ldl -lbcrypt -Wl,--end-group"
$currentFlags = (go env CGO_LDFLAGS).Trim()
$nextFlags = $requiredFlags
if ($currentFlags -and -not $currentFlags.Contains($requiredFlags)) {
    $nextFlags = "$requiredFlags $currentFlags"
}

& go env -w "CGO_LDFLAGS=$nextFlags"

Write-Host "configured CGO_LDFLAGS:"
Write-Host (go env CGO_LDFLAGS)

if ($Verify) {
    $verifyBinary = Join-Path $env:TEMP "ardents-workload.verify.test.exe"
    & go test -c -tags integration -o $verifyBinary ardents/tests/integration/workload
    if ($LASTEXITCODE -ne 0) {
        throw "verification build failed for ardents/tests/integration/workload"
    }
    Write-Host "verification build passed:"
    Write-Host $verifyBinary
}
