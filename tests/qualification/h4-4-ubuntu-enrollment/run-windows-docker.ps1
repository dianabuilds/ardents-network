[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$SignedXPI
)

$ErrorActionPreference = 'Stop'

if ([string]::IsNullOrWhiteSpace($SignedXPI) -or -not [System.IO.Path]::IsPathRooted($SignedXPI)) {
    throw 'SignedXPI must be one absolute Mozilla-signed XPI path'
}
$signedPath = (Resolve-Path -LiteralPath $SignedXPI -ErrorAction Stop).Path
if (-not (Test-Path -LiteralPath $signedPath -PathType Leaf)) {
    throw "Mozilla-signed XPI is unavailable: $signedPath"
}
$repository = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..\..\..')).Path
$artifacts = Join-Path ([System.IO.Path]::GetTempPath()) ("ardents-h4-4-linux-artifacts-" + [Guid]::NewGuid())
try {
    [System.IO.Directory]::CreateDirectory($artifacts) | Out-Null
    Push-Location $repository
    try {
        $env:GOOS = 'linux'
        $env:GOARCH = 'amd64'
        $env:CGO_ENABLED = '0'
        foreach ($entry in @(
            @{ Output = 'ardents-linux-amd64'; Package = './cmd/ardents' },
            @{ Output = 'ardents-browser-entry-linux-amd64'; Package = './cmd/ardents-browser-entry' },
            @{ Output = 'ardents-control-linux-amd64'; Package = './cmd/ardents-control' }
        )) {
            & go build -o (Join-Path $artifacts $entry.Output) $entry.Package
            if ($LASTEXITCODE -ne 0) {
                throw "build failed for $($entry.Package)"
            }
        }
    }
    finally {
        Pop-Location
    }
    $arguments = @(
        'run', '--rm', '--user', '1000:1000',
        '--mount', "type=bind,src=$repository,dst=/workspace,readonly",
        '--mount', "type=bind,src=$signedPath,dst=/input/ardents-alpha-browser-entry.xpi,readonly",
        '--mount', "type=bind,src=$artifacts,dst=/artifacts,readonly",
        '-w', '/workspace', '-e', 'HOME=/tmp/ardents-h4-4-home',
        'ubuntu:24.04', 'sh', './tests/qualification/h4-4-ubuntu-enrollment/run-ubuntu.sh', '/input/ardents-alpha-browser-entry.xpi', '/artifacts'
    )
    & docker @arguments
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
}
finally {
    if (Test-Path -LiteralPath $artifacts -PathType Container) {
        Remove-Item -LiteralPath $artifacts -Recurse -Force
    }
}
