param(
    [Parameter(Mandatory = $true)][string]$Output,
    [Parameter(Mandatory = $true)][string]$ReaderHost,
    [Parameter(Mandatory = $true)][string]$PublisherHost,
    [Parameter(Mandatory = $true)][string]$SSHKey,
    [Parameter(Mandatory = $true)][ValidateSet('tcp-tls','quic')][string]$Carrier,
    [Parameter(Mandatory = $true)][ValidateSet('net14ad','net14s','net14-recovery')][string]$Cell,
    [Parameter(Mandatory = $true)][ValidateSet('client-to-publisher','publisher-to-client')][string]$Profile,
    [Parameter(Mandatory = $true)][string]$Seed,
    [Parameter(Mandatory = $true)][string]$At,
    [string]$User = 'root'
)

$ErrorActionPreference = 'Stop'
$repository = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..\..'))
$outputRoot = [IO.Path]::GetFullPath($Output)

function Assert-Host([string]$Value, [string]$Label) {
    if ($Value -notmatch '^(?:[A-Za-z0-9](?:[A-Za-z0-9.-]{0,251}[A-Za-z0-9])?|(?:\d{1,3}\.){3}\d{1,3})$') { throw "$Label is invalid." }
}
Assert-Host $ReaderHost 'ReaderHost'
Assert-Host $PublisherHost 'PublisherHost'
Assert-Host $User 'User'
if ($ReaderHost -ceq $PublisherHost) { throw 'Fixture generation requires two distinct hosts.' }
if ($Seed -cnotmatch '^[0-9a-f]{64}$') { throw 'Seed must be lowercase SHA-256-width hex.' }
$parsedAt = [DateTimeOffset]::ParseExact($At, 'yyyy-MM-ddTHH:mm:ssZ', [Globalization.CultureInfo]::InvariantCulture)
if ($parsedAt.Millisecond -ne 0) { throw 'At must have whole-second precision.' }
if (-not (Test-Path -LiteralPath $SSHKey -PathType Leaf)) { throw 'SSHKey is absent.' }
$key = (Resolve-Path -LiteralPath $SSHKey).Path
if ($outputRoot.StartsWith($repository + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'Private fixture output must remain outside the repository.'
}
if (Test-Path -LiteralPath $outputRoot) { throw 'Output must be a new directory.' }

$temporary = Join-Path ([IO.Path]::GetTempPath()) ("ardents-issue60-generator-" + [guid]::NewGuid().ToString('N'))
[IO.Directory]::CreateDirectory($temporary) | Out-Null
$binary = Join-Path $temporary 'qualification-network'
$remoteRoot = "/var/tmp/ardents-issue60-generate-$($Seed.Substring(0,12))-$([guid]::NewGuid().ToString('N'))"
$sshOptions = @('-i', $key, '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=15', '-o', 'StrictHostKeyChecking=accept-new')
$scpOptions = @('-i', $key, '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=15', '-o', 'ConnectionAttempts=3', '-o', 'StrictHostKeyChecking=accept-new')
$remote = "$User@$ReaderHost"

function Invoke-SCP([string[]]$Arguments, [string]$Label) {
    foreach ($attempt in 1..3) {
        & scp @scpOptions @Arguments
        if ($LASTEXITCODE -eq 0) { return }
        if ($attempt -lt 3) { Start-Sleep -Seconds $attempt }
    }
    throw "$Label failed after three bounded attempts."
}

try {
    $priorGOOS, $priorGOARCH, $priorCGO = $env:GOOS, $env:GOARCH, $env:CGO_ENABLED
    try {
        $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = 'linux', 'amd64', '0'
        & go build -trimpath -o $binary ./tests/qualification/stream-network-two-host/fixturecommand/qualification-network
        if ($LASTEXITCODE -ne 0) { throw 'Cross-build qualification fixture generator failed.' }
    } finally {
        $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = $priorGOOS, $priorGOARCH, $priorCGO
    }
    & ssh @sshOptions $remote "set -eu; test ! -e '$remoteRoot'; install -d -m 700 '$remoteRoot'"
    if ($LASTEXITCODE -ne 0) { throw 'Create remote generator root failed.' }
    Invoke-SCP @($binary, "$($remote):$remoteRoot/qualification-network") 'Upload fixture generator'
    $command = "chmod 700 '$remoteRoot/qualification-network'; '$remoteRoot/qualification-network' -output '$remoteRoot/output' -reader-host '$ReaderHost' -publisher-host '$PublisherHost' -carrier '$Carrier' -cell '$Cell' -profile '$Profile' -seed '$Seed' -at '$At'"
    & ssh @sshOptions $remote $command
    if ($LASTEXITCODE -ne 0) { throw 'Remote fixture generation failed.' }
    [IO.Directory]::CreateDirectory($outputRoot) | Out-Null
    Invoke-SCP @('-r', "$($remote):$remoteRoot/output/.", $outputRoot) 'Download generated private fixture'
    $bundlePath = Join-Path $outputRoot 'fixture.json'
    if (-not (Test-Path -LiteralPath $bundlePath -PathType Leaf)) { throw 'Downloaded fixture is incomplete.' }
    $bundle = Get-Content -LiteralPath $bundlePath -Raw | ConvertFrom-Json
    if ([string]$bundle.Schema -cne 'ardents-qualification-network-fixture-v1' -or
        [string]$bundle.Seed -cne $Seed -or [string]$bundle.Carrier -cne $Carrier -or
        [string]$bundle.Cell -cne $Cell -or [string]$bundle.Profile -cne $Profile) {
        throw 'Downloaded fixture identity differs from the request.'
    }
    Write-Output (Get-Content -LiteralPath $bundlePath -Raw)
} finally {
    if (Test-Path -LiteralPath $temporary) {
        $resolved = (Resolve-Path -LiteralPath $temporary).Path
        $tempPrefix = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar
        if ($resolved.StartsWith($tempPrefix, [StringComparison]::OrdinalIgnoreCase)) {
            Remove-Item -LiteralPath $resolved -Recurse -Force
        }
    }
    & ssh @sshOptions $remote "case '$remoteRoot' in /var/tmp/ardents-issue60-generate-*) rm -rf -- '$remoteRoot';; *) exit 64;; esac" 2>$null
}
