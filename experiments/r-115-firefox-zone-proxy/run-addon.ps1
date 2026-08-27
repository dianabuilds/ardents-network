[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateScript({ Test-Path -LiteralPath $_ -PathType Leaf })]
    [string]$FirefoxPath,

    [ValidateRange(10, 120)]
    [int]$TimeoutSeconds = 60,

    [bool]$Headless = $true,

    [switch]$UseNativeHost,

    # When nonzero, exercise the add-on against a listener owned by a
    # qualification caller. The caller, rather than this fixture, must prove
    # the requests it receives. This keeps the fixture's synthetic proxy out
    # of the maintained Endpoint-path qualification.
    [ValidateRange(1024, 65535)]
    [int]$ExternalLoopbackProxyPort = 0
)

$ErrorActionPreference = 'Stop'

function Get-FreeLoopbackPort {
    $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
    try {
        $listener.Start()
        return $listener.LocalEndpoint.Port
    }
    finally {
        $listener.Stop()
    }
}

function Write-Utf8NoBom([string]$Path, [string]$Text) {
    [System.IO.File]::WriteAllText($Path, $Text, [System.Text.UTF8Encoding]::new($false))
}

function Quote-ProcessArgument([string]$Value) {
    if ($Value -notmatch '[\s"]') { return $Value }
    return '"' + $Value.Replace('"', '\"') + '"'
}

function Stop-OwnFirefoxProfile([string]$Profile) {
    $needle = [Regex]::Escape($Profile)
    Get-CimInstance Win32_Process -Filter "Name = 'firefox.exe'" |
        Where-Object { $_.CommandLine -match $needle } |
        ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
}

$experiment = $PSScriptRoot
$root = Join-Path ([System.IO.Path]::GetTempPath()) ("ardents-r115-addon-{0}" -f [Guid]::NewGuid())
$addon = Join-Path $root 'addon'
$profile = Join-Path $root 'profile'
$evidence = Join-Path $root 'evidence'
$ready = Join-Path $evidence 'proxy-ready.txt'
$request = Join-Path $evidence 'request.txt'
$webExtOut = Join-Path $evidence 'web-ext.stdout.txt'
$webExtErr = Join-Path $evidence 'web-ext.stderr.txt'
$fallbackReady = Join-Path $evidence 'fallback-ready.txt'
$fallbackRequest = Join-Path $evidence 'fallback-request.txt'
$nativeHostName = 'org.ardents.r115.fixture'
$nativeRoot = Join-Path $root 'native-host'
$nativeManifestPath = Join-Path $nativeRoot ($nativeHostName + '.json')
$nativeRegistryPath = 'Software\Mozilla\NativeMessagingHosts\' + $nativeHostName
$nativeRegistered = $false
$port = if ($ExternalLoopbackProxyPort -ne 0) { $ExternalLoopbackProxyPort } else { Get-FreeLoopbackPort }
do { $fallbackPort = Get-FreeLoopbackPort } while ($fallbackPort -eq $port)
if ($port -eq 80 -or $port -eq 443 -or $fallbackPort -eq 80 -or $fallbackPort -eq 443) {
    throw 'a Browser Entry fixture listener must not use port 80 or 443'
}
$proxy = $null
$fallbackProxy = $null
$webExt = $null

New-Item -ItemType Directory -Path $addon, $profile, $evidence, $nativeRoot -Force | Out-Null
Get-ChildItem -LiteralPath (Join-Path $experiment 'addon') -Force |
    Copy-Item -Destination $addon -Recurse -Force
$backgroundPath = Join-Path $addon 'background.js'
$background = Get-Content -LiteralPath $backgroundPath -Raw
$background = $background.Replace('const loopbackPort = 9;', "const loopbackPort = $port;")
if ($background -notmatch "const loopbackPort = $port;") { throw 'could not bind the add-on to the temporary loopback proxy' }
if ($UseNativeHost) {
    $background = $background.Replace('const useNativeMessagePort = false;', 'const useNativeMessagePort = true;')
    if ($background -notmatch 'const useNativeMessagePort = true;') { throw 'could not enable the temporary native-messaging probe' }
}
Write-Utf8NoBom $backgroundPath $background
$profilePreferences = @(
    'user_pref("network.proxy.type", 1);',
    'user_pref("network.proxy.http", "127.0.0.1");',
    "user_pref(`"network.proxy.http_port`", $fallbackPort);",
    'user_pref("network.proxy.share_proxy_settings", true);'
)
$profilePreferences = $profilePreferences -join "`n"
Write-Utf8NoBom (Join-Path $profile 'user.js') $profilePreferences
if ($UseNativeHost) {
    Get-ChildItem -LiteralPath (Join-Path $experiment 'native-host') -Force |
        Where-Object { $_.Name -ne 'README.md' } |
        Copy-Item -Destination $nativeRoot -Recurse -Force
    $nativeScriptPath = Join-Path $nativeRoot 'native-host.ps1'
    $nativeScript = Get-Content -LiteralPath $nativeScriptPath -Raw
    $nativePortLine = '$loopbackPort = ' + $port
    $nativeScript = $nativeScript.Replace('$loopbackPort = 9', $nativePortLine)
    if (-not $nativeScript.Contains($nativePortLine)) { throw 'could not bind the temporary native host to the loopback proxy' }
    Write-Utf8NoBom $nativeScriptPath $nativeScript
    $nativeManifest = [ordered]@{
        name = $nativeHostName
        description = 'Disposable Ardents R-115 native messaging fixture'
        path = 'native-host.cmd'
        type = 'stdio'
        allowed_extensions = @('r115-fixture@ardents.invalid')
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
    finally {
        $registered.Close()
    }
}

try {
    if ($ExternalLoopbackProxyPort -eq 0) {
        $proxyArguments = @(
            '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', (Join-Path $experiment 'serve.ps1'),
            '-Port', $port, '-ReadyPath', $ready, '-RequestPath', $request, '-MaximumRequests', 8, '-RejectNonReference'
        )
        $proxyArgumentLine = ($proxyArguments | ForEach-Object { Quote-ProcessArgument $_ }) -join ' '
        $proxy = Start-Process -FilePath 'powershell.exe' -ArgumentList $proxyArgumentLine -WindowStyle Hidden -PassThru
    }
    $fallbackArguments = @(
        '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', (Join-Path $experiment 'serve.ps1'),
        '-Port', $fallbackPort, '-ReadyPath', $fallbackReady, '-RequestPath', $fallbackRequest, '-MaximumRequests', 8
    )
    $fallbackArgumentLine = ($fallbackArguments | ForEach-Object { Quote-ProcessArgument $_ }) -join ' '
    $fallbackProxy = Start-Process -FilePath 'powershell.exe' -ArgumentList $fallbackArgumentLine -WindowStyle Hidden -PassThru
    $readyDeadline = [DateTime]::UtcNow.AddSeconds(10)
    while (($ExternalLoopbackProxyPort -eq 0 -and -not (Test-Path -LiteralPath $ready)) -or -not (Test-Path -LiteralPath $fallbackReady)) {
        if (($null -ne $proxy -and $proxy.HasExited) -or $fallbackProxy.HasExited) { throw 'a synthetic proxy exited before becoming ready' }
        if ([DateTime]::UtcNow -ge $readyDeadline) { throw 'a synthetic proxy did not become ready' }
        Start-Sleep -Milliseconds 100
    }

    $npx = Get-Command 'npx.cmd' -ErrorAction SilentlyContinue
    if ($null -eq $npx) { $npx = Get-Command 'npx' -ErrorAction Stop }
    $arguments = @(
        '--yes', '--package', 'web-ext@10', 'web-ext', 'run', '--source-dir', $addon,
        '--firefox', (Resolve-Path -LiteralPath $FirefoxPath).Path,
        '--firefox-profile', $profile, '--profile-create-if-missing', '--keep-profile-changes',
        '--no-input', '--verbose'
    )
    if ($Headless) { $arguments += '--arg=-headless' }
    $argumentLine = ($arguments | ForEach-Object { Quote-ProcessArgument $_ }) -join ' '
    $webExt = Start-Process -FilePath $npx.Source -ArgumentList $argumentLine -WindowStyle Hidden -PassThru -RedirectStandardOutput $webExtOut -RedirectStandardError $webExtErr

    if ($ExternalLoopbackProxyPort -eq 0) {
        $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
        while (-not (Test-Path -LiteralPath $request)) {
            if ($webExt.HasExited) { throw "web-ext exited before the add-on sent a proxy request; see $webExtOut and $webExtErr" }
            if ([DateTime]::UtcNow -ge $deadline) { throw "the add-on did not send a proxy request before timeout; see $webExtOut and $webExtErr" }
            Start-Sleep -Milliseconds 250
        }
        $firstLine = (Get-Content -LiteralPath $request -TotalCount 1)
        $expectedFirstLine = 'GET http://reference.ard/ HTTP/1.1'
        if ($firstLine -ne $expectedFirstLine) { throw "unexpected add-on proxy request: $firstLine" }
        $alphaDeadline = [DateTime]::UtcNow.AddSeconds(10)
        do {
            $alphaObserved = Get-Content -LiteralPath $request -Raw
            $httpsObserved = $alphaObserved -match 'CONNECT reference\.ard:443 HTTP/1\.1'
            if ($httpsObserved) { break }
            if ([DateTime]::UtcNow -ge $alphaDeadline) { throw 'HTTPS .ard did not reach the local proxy as a CONNECT request' }
            Start-Sleep -Milliseconds 250
        } while ($true)
    }
    else {
        # The caller observes requests at its maintained listener. Leave
        # Firefox running long enough for the fixture's installed probes to
        # complete, but retain the ordinary-traffic check below.
        Start-Sleep -Seconds 8
        if ($webExt.HasExited) { throw "web-ext exited before the external loopback qualification completed; see $webExtOut and $webExtErr" }
        $firstLine = 'observed by external loopback qualification caller'
        $alphaObserved = $firstLine
    }
    $fallbackDeadline = [DateTime]::UtcNow.AddSeconds(10)
    do {
        $fallbackObserved = if (Test-Path -LiteralPath $fallbackRequest) { Get-Content -LiteralPath $fallbackRequest -Raw } else { '' }
        if ($fallbackObserved -match 'GET http://ordinary\.invalid/ HTTP/1\.1') { break }
        if ([DateTime]::UtcNow -ge $fallbackDeadline) { throw 'ordinary URL did not use the configured temporary fallback proxy' }
        Start-Sleep -Milliseconds 250
    } while ($true)
    if ($fallbackObserved -match 'GET http://[^\s]*\.ard/ HTTP/1\.1') { throw "an .ard URL reached the configured fallback proxy: $fallbackObserved" }
    $result = [ordered]@{
        firefox = (Resolve-Path -LiteralPath $FirefoxPath).Path
        url = 'http://reference.ard/'
        loopbackProxyPort = $port
        browserFallbackProxyPort = $fallbackPort
        request = $firstLine
        alphaRequests = $alphaObserved
        fallbackRequests = $fallbackObserved
        addOnID = 'r115-fixture@ardents.invalid'
        nativeMessaging = [bool]$UseNativeHost
        externalLoopbackProxy = [bool]($ExternalLoopbackProxyPort -ne 0)
        profile = $profile
        webExtStandardOutput = $webExtOut
        webExtStandardError = $webExtErr
    }
    $result | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $evidence 'result.json') -Encoding utf8
    $result | ConvertTo-Json -Compress
}
finally {
    Stop-OwnFirefoxProfile $profile
    if ($null -ne $webExt -and -not $webExt.HasExited) { Stop-Process -Id $webExt.Id -Force -ErrorAction SilentlyContinue }
    if ($null -ne $proxy -and -not $proxy.HasExited) { Stop-Process -Id $proxy.Id -Force -ErrorAction SilentlyContinue }
    if ($null -ne $fallbackProxy -and -not $fallbackProxy.HasExited) { Stop-Process -Id $fallbackProxy.Id -Force -ErrorAction SilentlyContinue }
    if ($nativeRegistered) {
        $registered = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey($nativeRegistryPath, $true)
        if ($null -ne $registered) {
            $registeredValue = [string]$registered.GetValue('')
            $registered.Close()
            if ($registeredValue -eq $nativeManifestPath) {
                [Microsoft.Win32.Registry]::CurrentUser.DeleteSubKey($nativeRegistryPath, $false)
            }
        }
    }
    Write-Output "R-115 add-on evidence directory: $evidence"
}
