[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateScript({ Test-Path -LiteralPath $_ -PathType Leaf })]
    [string]$FirefoxPath,

    [ValidateRange(10, 120)]
    [int]$TimeoutSeconds = 45,

    [bool]$Headless = $true,

    [switch]$RejectChallengeRevalidation
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
$root = Join-Path ([System.IO.Path]::GetTempPath()) ("ardents-r117-proxy-auth-{0}" -f [Guid]::NewGuid())
$addon = Join-Path $root 'addon'
$profile = Join-Path $root 'profile'
$evidence = Join-Path $root 'evidence'
$ready = Join-Path $evidence 'proxy-ready.txt'
$result = Join-Path $evidence 'requests.json'
$serverError = Join-Path $evidence 'proxy.stderr.txt'
$nativeHostName = 'org.ardents.r117.proxy_auth'
$nativeRoot = Join-Path $root 'native-host'
$nativeManifestPath = Join-Path $nativeRoot ($nativeHostName + '.json')
$nativeRegistryPath = 'Software\Mozilla\NativeMessagingHosts\' + $nativeHostName
$nativeRegistered = $false
$port = Get-FreeLoopbackPort
if ($port -eq 80 -or $port -eq 443) { throw 'the proxy-auth fixture must not use port 80 or 443' }
$authenticationPort = $port
if ($RejectChallengeRevalidation) {
    do { $authenticationPort = Get-FreeLoopbackPort } while ($authenticationPort -eq $port)
}
$passwordBytes = [byte[]]::new(24)
$random = [System.Security.Cryptography.RandomNumberGenerator]::Create()
try { $random.GetBytes($passwordBytes) }
finally { $random.Dispose() }
$password = [Convert]::ToBase64String($passwordBytes)
$server = $null
$webExt = $null

New-Item -ItemType Directory -Path $addon, $profile, $evidence, $nativeRoot -Force | Out-Null
Get-ChildItem -LiteralPath (Join-Path $experiment 'addon') -Force | Copy-Item -Destination $addon -Recurse -Force
Get-ChildItem -LiteralPath (Join-Path $experiment 'native-host') -Force | Copy-Item -Destination $nativeRoot -Recurse -Force
$nativeScriptPath = Join-Path $nativeRoot 'native-host.ps1'
$nativeScript = Get-Content -LiteralPath $nativeScriptPath -Raw
$nativeScript = $nativeScript.Replace('$loopbackPort = 9', ('$loopbackPort = ' + $port))
$nativeScript = $nativeScript.Replace('$authenticationPort = 9', ('$authenticationPort = ' + $authenticationPort))
$nativeScript = $nativeScript.Replace("`$proxyPassword = 'replace-before-run'", ("`$proxyPassword = '" + $password + "'"))
if (-not $nativeScript.Contains('$loopbackPort = ' + $port) -or -not $nativeScript.Contains('$authenticationPort = ' + $authenticationPort) -or
    $nativeScript.Contains('replace-before-run')) {
    throw 'could not inject the temporary native-host response'
}
Write-Utf8NoBom $nativeScriptPath $nativeScript
$nativeManifest = [ordered]@{
    name = $nativeHostName
    description = 'Disposable Ardents R-117 proxy authentication fixture'
    path = (Join-Path $nativeRoot 'native-host.cmd')
    type = 'stdio'
    allowed_extensions = @('r117-proxy-auth@ardents.invalid')
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

$serverScript = @'
param([int]$Port, [string]$Password, [string]$ReadyPath, [string]$ResultPath, [switch]$ExpectAuthenticated)
$ErrorActionPreference = 'Stop'
$listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, $Port)
$listener.Start()
Set-Content -LiteralPath $ReadyPath -Value 'ready' -NoNewline -Encoding utf8
$expected = 'Basic ' + [Convert]::ToBase64String([Text.Encoding]::ASCII.GetBytes('ardents:' + $Password))
$requests = @()
function Serve-One([int]$Index) {
    $client = $listener.AcceptTcpClient()
    try {
        $stream = $client.GetStream()
        $reader = [IO.StreamReader]::new($stream, [Text.Encoding]::ASCII, $false, 8192, $true)
        $line = $reader.ReadLine()
        $headers = @{}
        while ($true) {
            $header = $reader.ReadLine()
            if ($null -eq $header -or $header.Length -eq 0) { break }
            $separator = $header.IndexOf(':')
            if ($separator -gt 0) { $headers[$header.Substring(0, $separator).ToLowerInvariant()] = $header.Substring($separator + 1).Trim() }
        }
        $script:requests += [ordered]@{ request = $line; authorization = $headers['proxy-authorization'] }
        $writer = [IO.StreamWriter]::new($stream, [Text.Encoding]::ASCII, 8192, $true)
        $writer.NewLine = "`r`n"
        if ($Index -eq 0) {
            $writer.Write("HTTP/1.1 407 Proxy Authentication Required`r`nProxy-Authenticate: Basic realm=`"Ardents R-117`"`r`nContent-Length: 0`r`nConnection: close`r`n`r`n")
        }
        elseif ($headers['proxy-authorization'] -eq $expected) {
            $body = '<h1>R-117 proxy authentication passed</h1>'
            $writer.Write("HTTP/1.1 200 OK`r`nContent-Type: text/html`r`nContent-Length: $([Text.Encoding]::UTF8.GetByteCount($body))`r`nConnection: close`r`n`r`n$body")
        }
        else {
            $writer.Write("HTTP/1.1 407 Proxy Authentication Required`r`nContent-Length: 0`r`nConnection: close`r`n`r`n")
        }
        $writer.Flush()
    }
    finally { $client.Close() }
}
try {
    Serve-One 0
    if ($ExpectAuthenticated) {
        Serve-One 1
    }
    else {
        $deadline = [DateTime]::UtcNow.AddSeconds(3)
        while ([DateTime]::UtcNow -lt $deadline) {
            if ($listener.Pending()) { Serve-One 1 }
            else { Start-Sleep -Milliseconds 50 }
        }
    }
    [ordered]@{ expected = $expected; requests = $requests } | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath $ResultPath -Encoding utf8
}
finally { $listener.Stop() }
'@
$serverPath = Join-Path $root 'serve.ps1'
Write-Utf8NoBom $serverPath $serverScript

try {
    $serverArguments = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $serverPath, '-Port', $port,
        '-Password', $password, '-ReadyPath', $ready, '-ResultPath', $result)
    if (-not $RejectChallengeRevalidation) { $serverArguments += '-ExpectAuthenticated' }
    $server = Start-Process -FilePath 'powershell.exe' -ArgumentList (($serverArguments | ForEach-Object { Quote-ProcessArgument $_ }) -join ' ') `
        -WindowStyle Hidden -PassThru -RedirectStandardError $serverError
    $readyDeadline = [DateTime]::UtcNow.AddSeconds(10)
    while (-not (Test-Path -LiteralPath $ready)) {
        if ($server.HasExited) { throw "the synthetic proxy stopped before ready; see $serverError" }
        if ([DateTime]::UtcNow -ge $readyDeadline) { throw 'the synthetic proxy did not become ready' }
        Start-Sleep -Milliseconds 100
    }
    $npx = Get-Command 'npx.cmd' -ErrorAction SilentlyContinue
    if ($null -eq $npx) { $npx = Get-Command 'npx' -ErrorAction Stop }
    $arguments = @('--yes', '--package', 'web-ext@10', 'web-ext', 'run', '--source-dir', $addon,
        '--firefox', (Resolve-Path -LiteralPath $FirefoxPath).Path, '--firefox-profile', $profile,
        '--profile-create-if-missing', '--keep-profile-changes', '--no-input')
    if ($Headless) { $arguments += '--arg=-headless' }
    $webExt = Start-Process -FilePath $npx.Source -ArgumentList (($arguments | ForEach-Object { Quote-ProcessArgument $_ }) -join ' ') -WindowStyle Hidden -PassThru
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    while (-not (Test-Path -LiteralPath $result)) {
        if ($server.HasExited) { throw "the proxy stopped without a complete authenticated request; see $serverError" }
        if ($webExt.HasExited) { throw 'web-ext exited before the authenticated request completed' }
        if ([DateTime]::UtcNow -ge $deadline) { throw 'Firefox did not complete the proxy-authentication experiment before timeout' }
        Start-Sleep -Milliseconds 250
    }
    $proof = Get-Content -LiteralPath $result -Raw | ConvertFrom-Json
    if ($RejectChallengeRevalidation) {
        if ($proof.requests.Count -ne 1 -or $proof.requests[0].request -ne 'GET http://reference.ard/ HTTP/1.1' -or $proof.requests[0].authorization) {
            throw "mismatched native revalidation disclosed a credential: $(Get-Content -LiteralPath $result -Raw)"
        }
    }
    elseif ($proof.requests.Count -ne 2 -or $proof.requests[0].request -ne 'GET http://reference.ard/ HTTP/1.1' -or
        $proof.requests[0].authorization -or $proof.requests[1].request -ne 'GET http://reference.ard/ HTTP/1.1' -or
        $proof.requests[1].authorization -ne $proof.expected) {
        throw "unexpected proxy authentication proof: $(Get-Content -LiteralPath $result -Raw)"
    }
    $authentication = if ($RejectChallengeRevalidation) { 'no credential after native port mismatch' } else { 'exact Basic credential observed' }
    [ordered]@{ evidence = $evidence; proxyPort = $port; firstRequest = $proof.requests[0].request; authentication = $authentication } | ConvertTo-Json -Compress
}
finally {
    Stop-OwnFirefoxProfile $profile
    if ($null -ne $webExt -and -not $webExt.HasExited) { Stop-Process -Id $webExt.Id -Force -ErrorAction SilentlyContinue }
    if ($null -ne $server -and -not $server.HasExited) { Stop-Process -Id $server.Id -Force -ErrorAction SilentlyContinue }
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
    Write-Output "R-117 proxy-auth evidence directory: $evidence"
}
