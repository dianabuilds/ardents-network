[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateScript({ Test-Path -LiteralPath $_ -PathType Leaf })]
    [string]$FirefoxPath
)

$ErrorActionPreference = 'Stop'
$root = Join-Path ([System.IO.Path]::GetTempPath()) ("ardents-r115-firefox-proxy-{0}" -f [Guid]::NewGuid())
$profile = Join-Path $root 'profile'
$evidence = Join-Path $root 'evidence'
$job = $null

try {
    New-Item -ItemType Directory -Path $profile, $evidence | Out-Null

    $reservation = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
    $reservation.Start()
    $port = $reservation.LocalEndpoint.Port
    $reservation.Stop()
    if ($port -in 80, 443) {
        throw 'the selected loopback port is reserved for the no-port assertion'
    }

    $ready = Join-Path $evidence 'proxy-ready.txt'
    $requestTrace = Join-Path $evidence 'request.txt'
    $job = Start-Job -ArgumentList $port, $ready, $requestTrace -ScriptBlock {
        param($ListenerPort, $ReadyPath, $TracePath)

        $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, $ListenerPort)
        $listener.Start()
        Set-Content -LiteralPath $ReadyPath -Value 'ready' -NoNewline
        try {
            $client = $listener.AcceptTcpClient()
            try {
                $stream = $client.GetStream()
                $reader = [System.IO.StreamReader]::new($stream, [System.Text.Encoding]::ASCII, $false, 4096, $true)
                $lines = [System.Collections.Generic.List[string]]::new()
                while ($true) {
                    $line = $reader.ReadLine()
                    if ($null -eq $line -or $line -eq '') { break }
                    [void]$lines.Add($line)
                }
                Set-Content -LiteralPath $TracePath -Value $lines -Encoding ascii
                $body = '<!doctype html><title>R-115 proxy evidence</title><h1>Firefox reached the ephemeral loopback proxy</h1>'
                $response = "HTTP/1.1 200 OK`r`nContent-Type: text/html; charset=utf-8`r`nContent-Length: $([System.Text.Encoding]::UTF8.GetByteCount($body))`r`nConnection: close`r`n`r`n$body"
                $bytes = [System.Text.Encoding]::UTF8.GetBytes($response)
                $stream.Write($bytes, 0, $bytes.Length)
                $stream.Flush()
            }
            finally {
                $client.Dispose()
            }
        }
        finally {
            $listener.Stop()
        }
    }

    $deadline = [DateTime]::UtcNow.AddSeconds(10)
    while (-not (Test-Path -LiteralPath $ready)) {
        if ([DateTime]::UtcNow -ge $deadline) { throw 'the synthetic proxy did not become ready' }
        Start-Sleep -Milliseconds 100
    }

    $pacPath = Join-Path $profile 'ardents-r115.pac'
    @"
function FindProxyForURL(url, host) {
  if (host === 'reference.ard' || shExpMatch(host, '*.ard')) {
    return 'PROXY 127.0.0.1:$port';
  }
  return 'DIRECT';
}
"@ | Set-Content -LiteralPath $pacPath -Encoding ascii -NoNewline

    $pacURI = ([Uri]$pacPath).AbsoluteUri.Replace('\', '/')
    @"
user_pref("network.proxy.type", 2);
user_pref("network.proxy.autoconfig_url", "$pacURI");
user_pref("network.trr.mode", 5);
user_pref("network.captive-portal-service.enabled", false);
user_pref("network.connectivity-service.enabled", false);
"@ | Set-Content -LiteralPath (Join-Path $profile 'user.js') -Encoding ascii -NoNewline

    $screenshot = Join-Path $evidence 'screenshot.png'
    $process = Start-Process -FilePath $FirefoxPath -ArgumentList @('-headless', '-profile', $profile, '-screenshot', $screenshot, 'http://reference.ard/') -WindowStyle Hidden -Wait -PassThru
    if ($process.ExitCode -ne 0) { throw "Firefox exited with $($process.ExitCode)" }
    Wait-Job -Job $job -Timeout 10 | Out-Null
    Receive-Job -Job $job -ErrorAction Stop | Out-Null
    if (-not (Test-Path -LiteralPath $requestTrace) -or -not (Test-Path -LiteralPath $screenshot)) {
        throw 'Firefox did not leave the expected local proxy evidence'
    }
    [string]$firstLine = @(Get-Content -LiteralPath $requestTrace -TotalCount 1)[0]
    if ($firstLine -ne 'GET http://reference.ard/ HTTP/1.1') {
        throw "unexpected proxy request: $firstLine"
    }
    [pscustomobject]@{
        firefox = $FirefoxPath
        loopbackProxyPort = $port
        requestedURI = 'http://reference.ard/'
        capturedRequest = $firstLine
        screenshot = $screenshot
    } | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $evidence 'result.json') -Encoding utf8
    Write-Output "R-115 evidence: $evidence"
}
finally {
    if ($null -ne $job) {
        Stop-Job -Job $job -ErrorAction SilentlyContinue
        Remove-Job -Job $job -Force -ErrorAction SilentlyContinue
    }
    if (Test-Path -LiteralPath $profile) {
        Remove-Item -LiteralPath $profile -Recurse -Force
    }
}
