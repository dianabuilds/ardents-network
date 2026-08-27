[CmdletBinding()]
param(
    [string]$Distribution = 'Ubuntu-24.04'
)

$ErrorActionPreference = 'Stop'
$repository = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '../..')).Path
$temporaryRoot = (Resolve-Path -LiteralPath $env:TEMP).Path
$v1 = Join-Path $temporaryRoot 'ardents-r095-lifecycle-v1-linux'
$v2 = Join-Path $temporaryRoot 'ardents-r095-lifecycle-v2-linux'
$source = Join-Path $PSScriptRoot 'lifecycle_main.go'
$runner = Join-Path $PSScriptRoot 'run-ubuntu-lifecycle.sh'

foreach ($path in @($v1, $v2)) {
    if (Test-Path -LiteralPath $path) {
        throw "refusing existing temporary artifact $path"
    }
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
        & go build -trimpath -ldflags '-buildid= -X main.buildID=v1' -o $v1 $source
        if ($LASTEXITCODE -ne 0) {
            throw 'v1 lifecycle fixture build failed'
        }
        & go build -trimpath -ldflags '-buildid= -X main.buildID=v2' -o $v2 $source
        if ($LASTEXITCODE -ne 0) {
            throw 'v2 lifecycle fixture build failed'
        }
    } finally {
        $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = $previousGOOS, $previousGOARCH, $previousCGO
        Pop-Location
    }

    $runnerLinux = ConvertTo-WslPath -Path $runner
    $v1Linux = ConvertTo-WslPath -Path $v1
    $v2Linux = ConvertTo-WslPath -Path $v2
    & wsl.exe -d $Distribution -- bash $runnerLinux $v1Linux $v2Linux
    if ($LASTEXITCODE -ne 0) {
        throw "Ubuntu lifecycle runner exited $LASTEXITCODE"
    }
} finally {
    foreach ($path in @($v1, $v2)) {
        $full = [System.IO.Path]::GetFullPath($path)
        if ([System.IO.Path]::GetDirectoryName($full) -ne $temporaryRoot) {
            throw "unsafe temporary cleanup target $full"
        }
        if (Test-Path -LiteralPath $full) {
            Remove-Item -LiteralPath $full -Force
        }
    }
}
