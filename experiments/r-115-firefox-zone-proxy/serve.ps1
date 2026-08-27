[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateRange(1024, 65535)]
    [int]$Port,

    [Parameter(Mandatory = $true)]
    [string]$ReadyPath,

    [Parameter(Mandatory = $true)]
    [string]$RequestPath,

    [ValidateRange(1, 8)]
    [int]$MaximumRequests = 1,

    [switch]$RejectNonReference
)

$ErrorActionPreference = 'Stop'
$listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, $Port)
$listener.Start()
Set-Content -LiteralPath $ReadyPath -Value 'ready' -NoNewline
try {
    $requests = [System.Collections.Generic.List[string]]::new()
    for ($index = 0; $index -lt $MaximumRequests; $index++) {
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
            [void]$requests.Add(($lines -join "`n"))
            Set-Content -LiteralPath $RequestPath -Value ($requests -join "`n`n---`n`n") -Encoding ascii
            $referenceRequest = $lines.Count -gt 0 -and $lines[0].StartsWith('GET http://reference.ard/')
            if ($lines.Count -eq 0 -or $lines[0].StartsWith('CONNECT ') -or ($RejectNonReference -and -not $referenceRequest)) {
                $response = "HTTP/1.1 502 Bad Gateway`r`nContent-Length: 0`r`nConnection: close`r`n`r`n"
            }
            else {
                $body = '<!doctype html><title>R-115 extension evidence</title><h1>Firefox extension routed .ard to loopback</h1>'
                $response = "HTTP/1.1 200 OK`r`nContent-Type: text/html; charset=utf-8`r`nContent-Length: $([System.Text.Encoding]::UTF8.GetByteCount($body))`r`nConnection: close`r`n`r`n$body"
            }
            $bytes = [System.Text.Encoding]::UTF8.GetBytes($response)
            $stream.Write($bytes, 0, $bytes.Length)
            $stream.Flush()
        }
        finally {
            $client.Dispose()
        }
    }
}
finally {
    $listener.Stop()
}
