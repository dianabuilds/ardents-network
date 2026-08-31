[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$SignedXPI
)

$ErrorActionPreference = 'Stop'

function Require-Success([string]$Description, [scriptblock]$Action) {
    & $Action
    if ($LASTEXITCODE -ne 0) {
        throw "$Description failed with exit code $LASTEXITCODE"
    }
}

function Write-QualificationFile([string]$Path, [byte[]]$Contents) {
    [System.IO.File]::WriteAllBytes($Path, $Contents)
}

function Get-QualificationSHA256([string]$Path) {
    $stream = [System.IO.File]::OpenRead($Path)
    $hasher = [System.Security.Cryptography.SHA256]::Create()
    try {
        return [System.BitConverter]::ToString($hasher.ComputeHash($stream)).Replace('-', '').ToLowerInvariant()
    }
    finally {
        $hasher.Dispose()
        $stream.Dispose()
    }
}

if ([string]::IsNullOrWhiteSpace($SignedXPI) -or -not [System.IO.Path]::IsPathRooted($SignedXPI)) {
    throw 'SignedXPI must be one absolute Mozilla-signed XPI path'
}
$signedPath = (Resolve-Path -LiteralPath $SignedXPI -ErrorAction Stop).Path
if (-not (Test-Path -LiteralPath $signedPath -PathType Leaf)) {
    throw "Mozilla-signed XPI is unavailable: $signedPath"
}
$signedHash = Get-QualificationSHA256 $signedPath
if ($signedHash -ne 'd88e8ecba84cda82a7b2354d1f445e19b9d092f3f3d068868d1173ef29eaa2a2') {
    throw "SignedXPI digest is not the approved Browser Entry alpha artifact: $signedHash"
}

$repository = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..\..\..')).Path
$platform = 'windows-amd64'
$hostName = "ardents-browser-entry-$platform.exe"
$extensionName = 'ardents-alpha-browser-entry.xpi'
$endpointName = "ardents-$platform"
$adapterName = "ardents-browser-$platform.exe"
$controlName = "ardents-control-$platform"
$nativeHostName = 'org.ardents.alpha_browser_entry'
$registryPath = "HKCU:\Software\Mozilla\NativeMessagingHosts\$nativeHostName"
$manifestPath = Join-Path $env:APPDATA "Ardents\browser-entry\native-host\$nativeHostName.json"
if ((Test-Path -LiteralPath $registryPath) -or (Test-Path -LiteralPath $manifestPath)) {
    throw 'refusing to touch an existing Browser Entry registration or native manifest'
}

$bundle = Join-Path ([System.IO.Path]::GetTempPath()) ("ardents-browser-entry-v4-" + [Guid]::NewGuid())
$inputPath = $null
$installed = $false
try {
    [System.IO.Directory]::CreateDirectory($bundle) | Out-Null
    Push-Location $repository
    try {
        Require-Success 'build enrolled Endpoint artifact' { & go build -o (Join-Path $bundle $endpointName) ./cmd/ardents }
        Require-Success 'build enrolled Browser Adapter artifact' { & go build -o (Join-Path $bundle $adapterName) ./cmd/ardents-browser }
        Require-Success 'build enrolled Browser Entry host' { & go build -o (Join-Path $bundle $hostName) ./cmd/ardents-browser-entry }
        Require-Success 'build enrolled control companion' { & go build -o (Join-Path $bundle $controlName) ./cmd/ardents-control }
    }
    finally {
        Pop-Location
    }
    Copy-Item -LiteralPath $signedPath -Destination (Join-Path $bundle $extensionName)

    $descriptorLines = @(
        'schema=ardents-closed-alpha-enrollment-v4',
        'cohort=browser-entry-windows-qualification',
        'release=mozilla-signed-0.1.0',
        "platform=$platform",
        'environment=alpha',
        'network=browser-entry-windows-qualification',
        "target_path=ardents/$platform/endpoint",
        "artifact=$endpointName",
        'trusted_root=1.root.json',
        'control_catalog=catalog.ac1',
        'disclosure_root=catalog.pub',
        'control_release=release.ac1',
        'control_network=network.ac1',
        'control_compatibility=compatibility.ac1',
        'control_release_root=release.pub',
        'control_network_root=network.pub',
        'control_compatibility_root=compatibility.pub',
        'corpus_authority=corpus.pub',
        "control_artifact=$controlName",
        "browser_adapter_artifact=$adapterName",
        "browser_entry_artifact=$hostName",
        "browser_entry_extension=$extensionName"
    )
    $static = @{
        '1.root.json' = 'qualification trusted root'
        'RELEASE' = (($descriptorLines -join "`n") + "`n")
        'catalog.ac1' = 'qualification control catalog'
        'catalog.pub' = 'qualification disclosure root'
        'compatibility.ac1' = 'qualification compatibility decision'
        'compatibility.pub' = 'qualification compatibility root'
        'corpus.pub' = 'qualification corpus authority'
        'network.ac1' = 'qualification network decision'
        'network.pub' = 'qualification network root'
        'release.ac1' = 'qualification release decision'
        'release.pub' = 'qualification release root'
        'timestamp.json' = 'qualification timestamp'
    }
    foreach ($name in $static.Keys) {
        Write-QualificationFile (Join-Path $bundle $name) ([System.Text.Encoding]::UTF8.GetBytes($static[$name]))
    }
    $bundleFiles = [System.IO.Directory]::GetFiles($bundle)
    [System.Array]::Sort($bundleFiles, [System.StringComparer]::Ordinal)
    $manifestLines = foreach ($filePath in $bundleFiles) {
        $name = [System.IO.Path]::GetFileName($filePath)
        "{0}  {1}" -f (Get-QualificationSHA256 $filePath), $name
    }
    $manifest = (($manifestLines -join "`n") + "`n")
    $manifestPathInBundle = Join-Path $bundle 'SHA256SUMS'
    Write-QualificationFile $manifestPathInBundle ([System.Text.Encoding]::UTF8.GetBytes($manifest))
    $manifestHash = Get-QualificationSHA256 $manifestPathInBundle
    $inputPath = $bundle + '.enrollment.json'
    $input = [ordered]@{
        schema = 'ardents-alpha-enrollment-input-v1'
        bundle_root = $bundle
        cohort = 'browser-entry-windows-qualification'
        release = 'mozilla-signed-0.1.0'
        platform = $platform
        manifest_sha256 = $manifestHash
        environment = 'alpha'
        network = 'browser-entry-windows-qualification'
        target_path = "ardents/$platform/endpoint"
    } | ConvertTo-Json -Compress
    Write-QualificationFile $inputPath ([System.Text.Encoding]::UTF8.GetBytes($input))

    $hostArtifact = Join-Path $bundle $hostName
    $endpoint = Join-Path $bundle $endpointName
    $installResult = & $hostArtifact install --enrollment $inputPath --endpoint-artifact $endpoint --at '2026-08-26T00:00:00Z'
    if ($LASTEXITCODE -ne 0) {
        throw "native manifest installation failed with exit code $LASTEXITCODE"
    }
    $installed = $true
    $result = $installResult | ConvertFrom-Json
    if ($result.schema -ne 'ardents-browser-entry-installation-v1' -or $result.native_manifest -ne $manifestPath -or
        $result.extension -ne (Join-Path $bundle $extensionName) -or $result.extension_installation -ne 'manual-required') {
        throw "unexpected Browser Entry installation result: $installResult"
    }
    if (-not (Test-Path -LiteralPath $registryPath -PathType Container) -or -not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) {
        throw 'native manifest installation did not create the exact current-user registration'
    }
    $nativeManifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
    if ($nativeManifest.name -ne $nativeHostName -or $nativeManifest.path -ne $hostArtifact -or $nativeManifest.allowed_extensions.Count -ne 1 -or
        $nativeManifest.allowed_extensions[0] -ne 'alpha-browser-entry@ardents.network') {
        throw 'native manifest contents are not the exact qualified binding'
    }

    $removeResult = & $hostArtifact remove
    if ($LASTEXITCODE -ne 0) {
        throw "native manifest removal failed with exit code $LASTEXITCODE"
    }
    $installed = $false
    $removed = $removeResult | ConvertFrom-Json
    if ($removed.removal -ne 'native-manifest-withdrawn' -or (Test-Path -LiteralPath $registryPath) -or (Test-Path -LiteralPath $manifestPath)) {
        throw 'native manifest removal left Browser Entry registration behind'
    }
    [ordered]@{
        schema = 'ardents-browser-entry-windows-qualification-v1'
        signed_xpi_sha256 = $signedHash
        native_manifest = 'installed-and-withdrawn'
        extension_installation = 'manual-required'
    } | ConvertTo-Json -Compress
}
finally {
    if ($installed -and (Test-Path -LiteralPath (Join-Path $bundle $hostName) -PathType Leaf)) {
        & (Join-Path $bundle $hostName) remove | Out-Null
    }
    if (Test-Path -LiteralPath $bundle -PathType Container) {
        Remove-Item -LiteralPath $bundle -Recurse -Force
    }
    if ($null -ne $inputPath -and (Test-Path -LiteralPath $inputPath -PathType Leaf)) {
        Remove-Item -LiteralPath $inputPath -Force
    }
}
