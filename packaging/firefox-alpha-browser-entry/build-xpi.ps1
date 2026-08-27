[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateScript({ [System.IO.Path]::IsPathRooted($_) })]
    [string]$OutputPath
)

$ErrorActionPreference = 'Stop'

function Require-ExactFile([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "required Browser Entry source file is unavailable: $Path"
    }
}

$source = (Resolve-Path -LiteralPath $PSScriptRoot).Path
$repository = (Resolve-Path -LiteralPath (Join-Path $source '..\..')).Path
$outputDirectory = Split-Path -Parent $OutputPath
if ([string]::IsNullOrWhiteSpace($outputDirectory)) {
    throw 'XPI output directory is unavailable'
}
if (-not (Test-Path -LiteralPath $outputDirectory -PathType Container)) {
    [System.IO.Directory]::CreateDirectory($outputDirectory) | Out-Null
}
$output = [System.IO.Path]::GetFullPath($OutputPath)
if ($output.StartsWith($repository + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw 'generated XPI output must stay outside the repository'
}
if ([System.IO.Path]::GetExtension($output) -ne '.xpi') {
    throw 'Browser Entry output must use the .xpi extension'
}
if (Test-Path -LiteralPath $output) {
    throw "refusing to overwrite an existing XPI: $output"
}

$manifestPath = Join-Path $source 'manifest.json'
$backgroundPath = Join-Path $source 'background.js'
Require-ExactFile $manifestPath
Require-ExactFile $backgroundPath
$manifestRaw = [System.IO.File]::ReadAllBytes($manifestPath)
$manifest = [System.Text.Encoding]::UTF8.GetString($manifestRaw) | ConvertFrom-Json
if ($manifest.manifest_version -ne 3 -or $manifest.browser_specific_settings.gecko.id -ne 'alpha-browser-entry@ardents.network' -or
    [string]::IsNullOrWhiteSpace($manifest.version) -or $manifest.background.scripts.Count -ne 1 -or $manifest.background.scripts[0] -ne 'background.js') {
    throw 'Browser Entry manifest does not have the selected fixed release identity'
}

$temporary = Join-Path ([System.IO.Path]::GetTempPath()) ("ardents-firefox-xpi-" + [Guid]::NewGuid())
[System.IO.Directory]::CreateDirectory($temporary) | Out-Null
try {
    Copy-Item -LiteralPath $manifestPath -Destination (Join-Path $temporary 'manifest.json')
    Copy-Item -LiteralPath $backgroundPath -Destination (Join-Path $temporary 'background.js')
    $candidate = Join-Path $temporary 'browser-entry.zip'
    Compress-Archive -LiteralPath @((Join-Path $temporary 'manifest.json'), (Join-Path $temporary 'background.js')) -DestinationPath $candidate -CompressionLevel Optimal

    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $archive = [System.IO.Compression.ZipFile]::OpenRead($candidate)
    try {
        $entries = @($archive.Entries | ForEach-Object { $_.FullName } | Sort-Object)
        if (@($entries | Where-Object { $_ -notin @('background.js', 'manifest.json') }).Count -ne 0 -or $entries.Count -ne 2) {
            throw "Browser Entry XPI contains an unexpected release surface: $($entries -join ', ')"
        }
        $packedManifest = $archive.GetEntry('manifest.json')
        $reader = [System.IO.StreamReader]::new($packedManifest.Open(), [System.Text.UTF8Encoding]::new($false))
        try { $packedRaw = $reader.ReadToEnd() }
        finally { $reader.Dispose() }
        if ($packedRaw -ne [System.Text.Encoding]::UTF8.GetString($manifestRaw)) {
            throw 'Browser Entry XPI manifest bytes differ from the reviewed source'
        }
    }
    finally { $archive.Dispose() }
    Move-Item -LiteralPath $candidate -Destination $output
    $digest = (Get-FileHash -LiteralPath $output -Algorithm SHA256).Hash.ToLowerInvariant()
    [ordered]@{ kind = 'ardents-firefox-alpha-browser-entry-unsigned-xpi-v1'; xpi = $output; sha256 = $digest; addOnID = $manifest.browser_specific_settings.gecko.id; version = $manifest.version } |
        ConvertTo-Json -Compress
}
finally {
    if (Test-Path -LiteralPath $temporary -PathType Container) {
        Remove-Item -LiteralPath $temporary -Recurse -Force
    }
}
