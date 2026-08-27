$ErrorActionPreference = 'Stop'
$loopbackPort = 9
$authenticationPort = 9
$proxyPassword = 'replace-before-run'

function Read-Exact([System.IO.Stream]$Stream, [byte[]]$Buffer) {
    $offset = 0
    while ($offset -lt $Buffer.Length) {
        $count = $Stream.Read($Buffer, $offset, $Buffer.Length - $offset)
        if ($count -le 0) { return $false }
        $offset += $count
    }
    return $true
}

$stdin = [Console]::OpenStandardInput()
$lengthBytes = [byte[]]::new(4)
if (-not (Read-Exact $stdin $lengthBytes)) { exit 0 }
$length = [BitConverter]::ToUInt32($lengthBytes, 0)
if ($length -lt 2 -or $length -gt 4096) { exit 1 }
$messageBytes = [byte[]]::new($length)
if (-not (Read-Exact $stdin $messageBytes)) { exit 1 }
try { $message = [Text.Encoding]::UTF8.GetString($messageBytes) | ConvertFrom-Json -ErrorAction Stop }
catch { exit 1 }
switch ($message.operation) {
    'loopback-proxy-port' { $response = @{ port = $loopbackPort } }
    'loopback-proxy-authentication' { $response = @{ port = $authenticationPort; password = $proxyPassword } }
    default { exit 1 }
}
$responseBytes = [Text.Encoding]::UTF8.GetBytes(($response | ConvertTo-Json -Compress))
$output = [Console]::OpenStandardOutput()
$output.Write([BitConverter]::GetBytes([uint32]$responseBytes.Length), 0, 4)
$output.Write($responseBytes, 0, $responseBytes.Length)
$output.Flush()
