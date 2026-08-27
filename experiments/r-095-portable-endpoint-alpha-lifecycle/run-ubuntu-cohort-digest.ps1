[CmdletBinding()]
param(
    [string]$Distribution = 'Ubuntu-24.04'
)

$ErrorActionPreference = 'Stop'
$repository = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '../..')).Path
$temporaryRoot = (Resolve-Path -LiteralPath $env:TEMP).Path
$artifact = Join-Path $temporaryRoot 'ardents-r095-cohort-digest-linux'
$source = Join-Path $PSScriptRoot 'lifecycle_main.go'
$runner = Join-Path $PSScriptRoot 'run-ubuntu-cohort-digest.sh'
$tufRoot = Join-Path $repository 'internal/release/testdata/r049-public-vector-v1/root.json'

if (Test-Path -LiteralPath $artifact) {
    throw "refusing existing temporary artifact $artifact"
}

function ConvertTo-WslPath {
    param([string]$Path)

    $slashPath = $Path.Replace('\', '/')
    $translated = ([string](& wsl.exe -d $Distribution -- wslpath -a $slashPath)).Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($translated)) {
        throw "failed to translate path for WSL: $Path"
    }
    return $translated
}

try {
    Push-Location $repository
    try {
        $previousGOOS, $previousGOARCH, $previousCGO = $env:GOOS, $env:GOARCH, $env:CGO_ENABLED
        $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = 'linux', 'amd64', '0'
        & go build -trimpath -ldflags '-buildid= -X main.buildID=v1' -o $artifact $source
        if ($LASTEXITCODE -ne 0) {
            throw 'cohort-digest fixture build failed'
        }
    } finally {
        $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = $previousGOOS, $previousGOARCH, $previousCGO
        Pop-Location
    }

    $runnerLinux = ConvertTo-WslPath -Path $runner
    $artifactLinux = ConvertTo-WslPath -Path $artifact
    $tufRootLinux = ConvertTo-WslPath -Path $tufRoot
    & wsl.exe -d $Distribution -- bash $runnerLinux $artifactLinux $tufRootLinux
    if ($LASTEXITCODE -ne 0) {
        throw "Ubuntu cohort-digest runner exited $LASTEXITCODE"
    }
} finally {
    $full = [System.IO.Path]::GetFullPath($artifact)
    if ([System.IO.Path]::GetDirectoryName($full) -ne $temporaryRoot) {
        throw "unsafe temporary cleanup target $full"
    }
    if (Test-Path -LiteralPath $full) {
        Remove-Item -LiteralPath $full -Force
    }
}
