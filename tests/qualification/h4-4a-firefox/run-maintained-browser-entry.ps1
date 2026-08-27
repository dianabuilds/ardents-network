[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateScript({ Test-Path -LiteralPath $_ -PathType Leaf })]
    [string]$FirefoxPath,

    [Parameter(Mandatory = $true)]
    [ValidateScript({ Test-Path -LiteralPath $_ -PathType Container })]
    [string]$AddonSource,

    [Parameter(Mandatory = $true)]
    [ValidateScript({ Test-Path -LiteralPath $_ -PathType Leaf })]
    [string]$NativeHostPath,

    [Parameter(Mandatory = $true)]
    [ValidateScript({ [System.IO.Path]::IsPathRooted($_) })]
    [string]$NativeStatePath,

    [ValidateScript({ [string]::IsNullOrEmpty($_) -or [System.IO.Path]::IsPathRooted($_) })]
    [string]$DynamicProofPath = '',

    [ValidateScript({ [string]::IsNullOrEmpty($_) -or [System.IO.Path]::IsPathRooted($_) })]
    [string]$ReferenceProofPath = '',

    [ValidateScript({ [string]::IsNullOrEmpty($_) -or (Test-Path -LiteralPath $_ -PathType Leaf) })]
    [string]$SignedXPI = '',

    [ValidateRange(10, 300)]
    [int]$TimeoutSeconds = 30
)

$ErrorActionPreference = 'Stop'

function Get-FreeLoopbackPort {
    $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
    try { $listener.Start(); return $listener.LocalEndpoint.Port }
    finally { $listener.Stop() }
}

function Write-Utf8NoBom([string]$Path, [string]$Text) {
    [System.IO.File]::WriteAllText($Path, $Text, [System.Text.UTF8Encoding]::new($false))
}

function Get-SHA256([string]$Path) {
    $stream = [System.IO.File]::OpenRead($Path)
    $hasher = [System.Security.Cryptography.SHA256]::Create()
    try { return ([System.BitConverter]::ToString($hasher.ComputeHash($stream))).Replace('-', '').ToLowerInvariant() }
    finally { $hasher.Dispose(); $stream.Dispose() }
}

function Quote-ProcessArgument([string]$Value) {
    if ($Value -notmatch '[\s"]') { return $Value }
    return '"' + $Value.Replace('"', '\"') + '"'
}

function Stop-OwnFirefoxProfile([string]$Profile) {
    $needle = [Regex]::Escape($Profile)
    Get-CimInstance Win32_Process -Filter "Name = 'firefox.exe'" -ErrorAction SilentlyContinue |
        Where-Object { $_.CommandLine -match $needle } |
        ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
}

$root = Join-Path ([System.IO.Path]::GetTempPath()) ("ardents-h4-4-browser-entry-{0}" -f [Guid]::NewGuid())
$addon = Join-Path $root 'addon'
$profile = Join-Path $root 'profile'
$evidence = Join-Path $root 'evidence'
$fallbackReady = Join-Path $evidence 'fallback-ready.txt'
$fallbackRequest = Join-Path $evidence 'fallback-request.txt'
$hostName = 'org.ardents.alpha_browser_entry'
$nativeRoot = Join-Path $root 'native-host'
$nativeManifestPath = Join-Path $nativeRoot ($hostName + '.json')
$nativeWrapperPath = Join-Path $nativeRoot 'native-host.cmd'
$nativeRegistryPath = 'Software\Mozilla\NativeMessagingHosts\' + $hostName
$nativeRegistered = $false
$fallbackPort = Get-FreeLoopbackPort
$fallbackProxy = $null
$webExt = $null
$webExtOut = Join-Path $evidence 'web-ext.stdout.txt'
$webExtErr = Join-Path $evidence 'web-ext.stderr.txt'
$dynamicBrowserFlow = -not [string]::IsNullOrWhiteSpace($DynamicProofPath)
$signedFirefoxFlow = -not [string]::IsNullOrWhiteSpace($SignedXPI)
$firefox = $null
if (-not $dynamicBrowserFlow -and [string]::IsNullOrWhiteSpace($ReferenceProofPath)) {
    throw 'static Browser Entry qualification requires an absolute Reference proof path'
}

New-Item -ItemType Directory -Path $profile, $evidence, $nativeRoot -Force | Out-Null
if (-not $signedFirefoxFlow) {
    New-Item -ItemType Directory -Path $addon -Force | Out-Null
    Get-ChildItem -LiteralPath $AddonSource -Force | Copy-Item -Destination $addon -Recurse -Force
    $backgroundPath = Join-Path $addon 'background.js'
    $background = Get-Content -LiteralPath $backgroundPath -Raw
    $qualificationProbe = if ($dynamicBrowserFlow) { @'

// This is copied only into a temporary web-ext profile by the qualification
// runner. The page itself owns the browser-driven dynamic HTTP flow.
browser.runtime.onInstalled.addListener(({ temporary }) => {
  if (!temporary) return;
  void browser.tabs.create({ url: "http://reference.ard/" }).catch(() => {});
});
'@
    } else { @'

// This is copied only into a temporary web-ext profile by the qualification
// runner. It does not belong to the release add-on behaviour.
browser.runtime.onInstalled.addListener(async ({ temporary }) => {
  if (!temporary) return;
  // The ordinary control comes first so the runner can prove the browser's
  // pre-existing proxy path independently of the deliberately failing alpha
  // probes below. The Go caller separately requires the Reference resources.
  void browser.tabs.create({ url: "http://ordinary.invalid/" }).catch(() => {});
  await new Promise((resolve) => setTimeout(resolve, 250));
  void browser.tabs.create({ url: "http://reference.ard/" }).catch(() => {});
  await new Promise((resolve) => setTimeout(resolve, 1200));
  void browser.tabs.create({ url: "http://unavailable.ard/" }).catch(() => {});
  await new Promise((resolve) => setTimeout(resolve, 800));
  void browser.tabs.create({ url: "https://reference.ard/" }).catch(() => {});
});
'@
    }
    Write-Utf8NoBom $backgroundPath ($background + $qualificationProbe)
}
if (-not $dynamicBrowserFlow) {
    $profilePreferences = @(
        'user_pref("network.proxy.type", 1);',
        'user_pref("network.proxy.http", "127.0.0.1");',
        "user_pref(`"network.proxy.http_port`", $fallbackPort);",
        'user_pref("network.proxy.share_proxy_settings", true);'
    ) -join "`n"
    Write-Utf8NoBom (Join-Path $profile 'user.js') $profilePreferences
}
Write-Utf8NoBom $nativeWrapperPath ("@echo off`r`n`"{0}`" native-host --state `"{1}`"`r`n" -f (Resolve-Path -LiteralPath $NativeHostPath).Path, $NativeStatePath)
$nativeManifest = [ordered]@{
    name = $hostName
    description = 'Ardents H4-4 qualification native messaging host'
    path = 'native-host.cmd'
    type = 'stdio'
    allowed_extensions = @('alpha-browser-entry@ardents.network')
} | ConvertTo-Json
Write-Utf8NoBom $nativeManifestPath $nativeManifest
$existing = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey($nativeRegistryPath, $false)
if ($null -ne $existing) {
    $existing.Close()
    throw "the temporary native-host registry key already exists: $nativeRegistryPath"
}
$registered = [Microsoft.Win32.Registry]::CurrentUser.CreateSubKey($nativeRegistryPath)
try {
    $registered.SetValue('', $nativeManifestPath, [Microsoft.Win32.RegistryValueKind]::String)
    $nativeRegistered = $true
}
finally { $registered.Close() }

try {
    if (-not $dynamicBrowserFlow) {
        $fallbackScript = Join-Path $PSScriptRoot '..\..\..\experiments\r-115-firefox-zone-proxy\serve.ps1'
        $fallbackArguments = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $fallbackScript,
            '-Port', $fallbackPort, '-ReadyPath', $fallbackReady, '-RequestPath', $fallbackRequest, '-MaximumRequests', 4)
        $fallbackProxy = Start-Process -FilePath 'powershell.exe' -ArgumentList (($fallbackArguments | ForEach-Object { Quote-ProcessArgument $_ }) -join ' ') -WindowStyle Hidden -PassThru
        $readyDeadline = [DateTime]::UtcNow.AddSeconds(10)
        while (-not (Test-Path -LiteralPath $fallbackReady)) {
            if ($fallbackProxy.HasExited) { throw 'temporary ordinary-URL fallback proxy exited before ready' }
            if ([DateTime]::UtcNow -ge $readyDeadline) { throw 'temporary ordinary-URL fallback proxy did not become ready' }
            Start-Sleep -Milliseconds 100
        }
    }
    if ($signedFirefoxFlow) {
        if (-not $dynamicBrowserFlow) { throw 'signed Firefox qualification requires the dynamic Browser Entry flow' }
        $resolvedXPI = (Resolve-Path -LiteralPath $SignedXPI).Path
        $expectedDigest = Get-SHA256 $resolvedXPI
        $firefoxArguments = @('-no-remote', '-new-instance', '-profile', $profile, 'about:addons')
        $firefox = Start-Process -FilePath (Resolve-Path -LiteralPath $FirefoxPath).Path -ArgumentList (($firefoxArguments | ForEach-Object { Quote-ProcessArgument $_ }) -join ' ') -PassThru
        Start-Process -FilePath 'explorer.exe' -ArgumentList (Quote-ProcessArgument ('/select,' + $resolvedXPI)) | Out-Null
        $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
        do {
            $proof = if (Test-Path -LiteralPath $DynamicProofPath) { Get-Content -LiteralPath $DynamicProofPath -Raw } else { '' }
            if ($proof -eq "browser-dynamic-http`n") { break }
            if ([DateTime]::UtcNow -ge $deadline) { throw 'signed Browser Entry did not reach its Publisher proof before timeout' }
            Start-Sleep -Milliseconds 250
        } while ($true)
        $installedXPIs = @(Get-ChildItem -LiteralPath (Join-Path $profile 'extensions') -Filter '*.xpi' -File -ErrorAction SilentlyContinue |
            Where-Object { (Get-SHA256 $_.FullName) -eq $expectedDigest })
        if ($installedXPIs.Count -ne 1) { throw 'Firefox did not retain exactly the selected signed XPI in its clean profile' }
        $extensionsIndex = Join-Path $profile 'extensions.json'
        if (-not (Test-Path -LiteralPath $extensionsIndex -PathType Leaf)) { throw 'Firefox did not write its clean-profile extension index' }
        $installed = @((Get-Content -LiteralPath $extensionsIndex -Raw | ConvertFrom-Json).addons |
            Where-Object { $_.id -eq 'alpha-browser-entry@ardents.network' -and $_.version -eq '0.1.0' })
        if ($installed.Count -ne 1) { throw 'Firefox clean profile does not identify the selected fixed Browser Entry add-on' }
        [ordered]@{ firefox = (Resolve-Path -LiteralPath $FirefoxPath).Path; addOnID = 'alpha-browser-entry@ardents.network'; nativeHost = $hostName; signedXPISHA256 = $expectedDigest; dynamicProof = $DynamicProofPath } | ConvertTo-Json -Compress
        return
    }
    $npx = Get-Command 'npx.cmd' -ErrorAction SilentlyContinue
    if ($null -eq $npx) { $npx = Get-Command 'npx' -ErrorAction Stop }
    $arguments = @('--yes', '--package', 'web-ext@10', 'web-ext', 'run', '--source-dir', $addon,
        '--firefox', (Resolve-Path -LiteralPath $FirefoxPath).Path,
        '--firefox-profile', $profile, '--profile-create-if-missing', '--keep-profile-changes',
        '--no-input', '--verbose', '--arg=-headless')
    $webExt = Start-Process -FilePath $npx.Source -ArgumentList (($arguments | ForEach-Object { Quote-ProcessArgument $_ }) -join ' ') -WindowStyle Hidden -PassThru -RedirectStandardOutput $webExtOut -RedirectStandardError $webExtErr
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    if ($dynamicBrowserFlow) {
        do {
            if ($webExt.HasExited) { throw "web-ext exited before dynamic Browser Entry completed; see $webExtOut and $webExtErr" }
            $proof = if (Test-Path -LiteralPath $DynamicProofPath) { Get-Content -LiteralPath $DynamicProofPath -Raw } else { '' }
            if ($proof -eq "browser-dynamic-http`n") { break }
            if ([DateTime]::UtcNow -ge $deadline) { throw "dynamic Browser Entry flow did not reach its Publisher proof; see $webExtOut and $webExtErr" }
            Start-Sleep -Milliseconds 250
        } while ($true)
        [ordered]@{ firefox = (Resolve-Path -LiteralPath $FirefoxPath).Path; addOnID = 'alpha-browser-entry@ardents.network'; nativeHost = $hostName; dynamicProof = $DynamicProofPath } | ConvertTo-Json -Compress
        return
    }
    do {
        if ($webExt.HasExited) { throw "web-ext exited before Browser Entry completed; see $webExtOut and $webExtErr" }
        $fallbackObserved = if (Test-Path -LiteralPath $fallbackRequest) { Get-Content -LiteralPath $fallbackRequest -Raw } else { '' }
        $referenceObserved = if (Test-Path -LiteralPath $ReferenceProofPath) { Get-Content -LiteralPath $ReferenceProofPath -Raw } else { '' }
        if ($fallbackObserved -match 'GET http://ordinary\.invalid/ HTTP/1\.1' -and $referenceObserved -eq "firefox-reference-resources`n") { break }
        if ([DateTime]::UtcNow -ge $deadline) { throw "Browser Entry did not complete ordinary and named resource proofs; see $webExtOut and $webExtErr" }
        Start-Sleep -Milliseconds 250
    } while ($true)
    if ($fallbackObserved -match '(GET http://[^\s]*\.ard/|CONNECT [^\s]*\.ard:443)') {
        throw "an alpha URL reached the configured ordinary fallback proxy: $fallbackObserved"
    }
    [ordered]@{ firefox = (Resolve-Path -LiteralPath $FirefoxPath).Path; addOnID = 'alpha-browser-entry@ardents.network'; nativeHost = $hostName; fallbackRequests = $fallbackObserved; referenceProof = $ReferenceProofPath } | ConvertTo-Json -Compress
}
finally {
    Stop-OwnFirefoxProfile $profile
    if ($null -ne $firefox -and -not $firefox.HasExited) { Stop-Process -Id $firefox.Id -Force -ErrorAction SilentlyContinue }
    if ($null -ne $webExt -and -not $webExt.HasExited) { Stop-Process -Id $webExt.Id -Force -ErrorAction SilentlyContinue }
    if ($null -ne $fallbackProxy -and -not $fallbackProxy.HasExited) { Stop-Process -Id $fallbackProxy.Id -Force -ErrorAction SilentlyContinue }
    if ($nativeRegistered) {
        $registered = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey($nativeRegistryPath, $true)
        if ($null -ne $registered) {
            $value = [string]$registered.GetValue('')
            $registered.Close()
            if ($value -eq $nativeManifestPath) { [Microsoft.Win32.Registry]::CurrentUser.DeleteSubKey($nativeRegistryPath, $false) }
        }
    }
    if ($signedFirefoxFlow) {
        Start-Sleep -Milliseconds 200
        Remove-Item -LiteralPath $root -Recurse -Force -ErrorAction Stop
        Write-Output 'H4-4 signed Firefox temporary profile and native-host registration removed'
    } else {
        Write-Output "H4-4 maintained Browser Entry evidence directory: $evidence"
    }
}
