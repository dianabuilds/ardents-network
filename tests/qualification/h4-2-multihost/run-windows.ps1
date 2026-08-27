param(
    [string]$VPS = $env:ARDENTS_H4_2_VPS,
    [string]$SSHKey = $env:ARDENTS_H4_2_SSH_KEY,
    [string]$Candidate = $env:ARDENTS_H4_2_CANDIDATE,
    [string]$CandidateSHA256 = $env:ARDENTS_H4_2_CANDIDATE_SHA256,
    [int]$Port = $(if ($env:ARDENTS_H4_2_VPS_PORT) { [int]$env:ARDENTS_H4_2_VPS_PORT } else { 47926 }),
    [string]$User = $(if ($env:ARDENTS_H4_2_VPS_USER) { $env:ARDENTS_H4_2_VPS_USER } else { 'root' })
)

$ErrorActionPreference = 'Stop'
if ([System.Environment]::OSVersion.Platform -ne [System.PlatformID]::Win32NT) {
    throw 'H4-2 multi-host qualification is owned by its Windows-to-VPS runner.'
}
if ([string]::IsNullOrWhiteSpace($VPS) -or $VPS -notmatch '^(?:\d{1,3}\.){3}\d{1,3}$') {
    throw 'ARDENTS_H4_2_VPS must name one literal VPS IPv4 address.'
}
if ([string]::IsNullOrWhiteSpace($SSHKey) -or -not (Test-Path -LiteralPath $SSHKey -PathType Leaf)) {
    throw 'ARDENTS_H4_2_SSH_KEY must name the private key for the declared VPS.'
}
if ([string]::IsNullOrWhiteSpace($Candidate) -or -not (Test-Path -LiteralPath $Candidate -PathType Leaf)) {
    throw 'ARDENTS_H4_2_CANDIDATE must name the exact Linux amd64 candidate under qualification.'
}
if ($CandidateSHA256 -notmatch '^[0-9a-fA-F]{64}$') {
    throw 'ARDENTS_H4_2_CANDIDATE_SHA256 must be the expected candidate SHA-256 digest.'
}
$resolvedCandidate = (Resolve-Path -LiteralPath $Candidate).Path
$actualCandidateSHA256 = (Get-FileHash -LiteralPath $resolvedCandidate -Algorithm SHA256).Hash.ToLowerInvariant()
if ($actualCandidateSHA256 -ne $CandidateSHA256.ToLowerInvariant()) {
    throw "ARDENTS_H4_2_CANDIDATE digest is $actualCandidateSHA256, want $($CandidateSHA256.ToLowerInvariant())."
}
if ($Port -lt 1024 -or $Port -gt 65532) {
    throw 'ARDENTS_H4_2_VPS_PORT must leave three following unprivileged local test ports.'
}
if ($User -notmatch '^[A-Za-z0-9_-]+$') {
    throw 'ARDENTS_H4_2_VPS_USER contains unsupported characters.'
}
$sshPath = 'C:\Windows\System32\OpenSSH\ssh.exe'
if (-not (Test-Path -LiteralPath $sshPath -PathType Leaf) -and -not [string]::IsNullOrWhiteSpace($env:WINDIR)) {
    $sshPath = Join-Path $env:WINDIR 'System32\OpenSSH\ssh.exe'
}

$env:ARDENTS_H4_2_VPS = $VPS
$env:ARDENTS_H4_2_SSH_KEY = (Resolve-Path -LiteralPath $SSHKey).Path
$env:ARDENTS_H4_2_CANDIDATE = $resolvedCandidate
$env:ARDENTS_H4_2_CANDIDATE_SHA256 = $CandidateSHA256.ToLowerInvariant()
$env:ARDENTS_H4_2_VPS_PORT = [string]$Port
$env:ARDENTS_H4_2_VPS_USER = $User
$env:ARDENTS_H4_2_SSH = $sshPath
go test -v -tags=h4_2_multihost ./tests/e2e/node -run '^TestH42MultiHostRendezvous(Qualification|AbruptRemoteNodeLoss|TCPFaultRelay|KernelNetemRelay)$' -count=1 -timeout=3m
exit $LASTEXITCODE
