param(
    [string]$VPS = $env:ARDENTS_H4_3B_VPS,
    [string]$SSHKey = $env:ARDENTS_H4_3B_SSH_KEY,
    [int]$Port = $(if ($env:ARDENTS_H4_3B_VPS_PORT) { [int]$env:ARDENTS_H4_3B_VPS_PORT } else { 48026 }),
    [string]$User = $(if ($env:ARDENTS_H4_3B_VPS_USER) { $env:ARDENTS_H4_3B_VPS_USER } else { 'root' }),
    [string]$GoTestTimeout = '-timeout=8m'
)

$ErrorActionPreference = 'Stop'
if ([System.Environment]::OSVersion.Platform -ne [System.PlatformID]::Win32NT) {
    throw 'H4-3B multi-host qualification is owned by its Windows-to-VPS runner.'
}
if ([string]::IsNullOrWhiteSpace($VPS) -or $VPS -notmatch '^(?:\d{1,3}\.){3}\d{1,3}$') {
    throw 'ARDENTS_H4_3B_VPS must name one literal VPS IPv4 address.'
}
if ([string]::IsNullOrWhiteSpace($SSHKey) -or -not (Test-Path -LiteralPath $SSHKey -PathType Leaf)) {
    throw 'ARDENTS_H4_3B_SSH_KEY must name the private key for the declared VPS.'
}
if ($Port -lt 1024 -or $Port -gt 65527) {
    throw 'ARDENTS_H4_3B_VPS_PORT must leave seven following unprivileged test ports.'
}
if ($User -notmatch '^[A-Za-z0-9_-]+$') {
    throw 'ARDENTS_H4_3B_VPS_USER contains unsupported characters.'
}
if ($GoTestTimeout -notmatch '^-timeout=[1-9][0-9]*[smh]$') {
    throw 'H4-3B multi-host Go test timeout must be a positive Go duration in seconds, minutes, or hours.'
}
$sshPath = ''
foreach ($root in @(
    (Join-Path $env:WINDIR 'System32\OpenSSH'),
    (Join-Path $env:WINDIR 'Sysnative\OpenSSH')
)) {
    $candidate = Join-Path $root 'ssh.exe'
    if ([System.IO.File]::Exists($candidate)) {
        $sshPath = $candidate
        break
    }
}
if ([string]::IsNullOrEmpty($sshPath)) {
    $sshCommand = Get-Command ssh.exe -CommandType Application -ErrorAction SilentlyContinue
    if ($null -ne $sshCommand) {
        $sshPath = $sshCommand.Source
    }
}
if ([string]::IsNullOrEmpty($sshPath)) {
    throw 'the Windows OpenSSH ssh command is unavailable'
}

$env:ARDENTS_H4_3B_VPS = $VPS
$env:ARDENTS_H4_3B_SSH_KEY = (Resolve-Path -LiteralPath $SSHKey).Path
$env:ARDENTS_H4_3B_VPS_PORT = [string]$Port
$env:ARDENTS_H4_3B_VPS_USER = $User
$env:ARDENTS_H4_3B_SSH = $sshPath
go test -v -tags=h4_3b_multihost ./tests/e2e/service -run '^TestH43MultiHost(DynamicPublisherApplication|PublisherWithdrawal|PublisherApplicationCrash|PublisherEndpointLoss)$' -count=1 $GoTestTimeout
exit $LASTEXITCODE
