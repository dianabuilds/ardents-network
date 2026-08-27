[CmdletBinding()]
param(
    [string]$VPS = $env:ARDENTS_H4_3B_VPS,
    [string]$SSHKey = $env:ARDENTS_H4_3B_SSH_KEY,
    [string]$User = $(if ($env:ARDENTS_H4_3B_VPS_USER) { $env:ARDENTS_H4_3B_VPS_USER } else { 'root' })
)

$ErrorActionPreference = 'Stop'
if ([System.Environment]::OSVersion.Platform -ne [System.PlatformID]::Win32NT) {
    throw 'H4-3B VPS qualification is owned by its Windows-to-VPS runner.'
}
if ([string]::IsNullOrWhiteSpace($VPS) -or $VPS -notmatch '^(?:\d{1,3}\.){3}\d{1,3}$') {
    throw 'ARDENTS_H4_3B_VPS must name one literal VPS IPv4 address.'
}
if ([string]::IsNullOrWhiteSpace($SSHKey) -or -not (Test-Path -LiteralPath $SSHKey -PathType Leaf)) {
    throw 'ARDENTS_H4_3B_SSH_KEY must name the private key for the declared VPS.'
}
if ($User -notmatch '^[A-Za-z0-9_-]+$') {
    throw 'ARDENTS_H4_3B_VPS_USER contains unsupported characters.'
}

$repository = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..\..\..')).Path
$artifacts = Join-Path ([System.IO.Path]::GetTempPath()) ("ardents-h4-3b-vps-" + [Guid]::NewGuid())
$sshPath = 'C:\Windows\System32\OpenSSH\ssh.exe'
$scpPath = 'C:\Windows\System32\OpenSSH\scp.exe'
if ((-not (Test-Path -LiteralPath $sshPath -PathType Leaf) -or -not (Test-Path -LiteralPath $scpPath -PathType Leaf)) -and -not [string]::IsNullOrWhiteSpace($env:WINDIR)) {
    $sshPath = Join-Path $env:WINDIR 'System32\OpenSSH\ssh.exe'
    $scpPath = Join-Path $env:WINDIR 'System32\OpenSSH\scp.exe'
}
if (-not (Test-Path -LiteralPath $sshPath -PathType Leaf) -or -not (Test-Path -LiteralPath $scpPath -PathType Leaf)) {
    throw 'the Windows OpenSSH ssh/scp commands are unavailable'
}

$remoteRoot = ''

function Write-ArtifactDigests([string]$Root) {
    Get-ChildItem -LiteralPath $Root -File | Sort-Object Name | ForEach-Object {
        $hasher = [System.Security.Cryptography.SHA256]::Create()
        try {
            $digest = $hasher.ComputeHash([System.IO.File]::ReadAllBytes($_.FullName))
        }
        finally {
            $hasher.Dispose()
        }
        [PSCustomObject]@{ Name = $_.Name; SHA256 = ([System.BitConverter]::ToString($digest) -replace '-', '').ToLowerInvariant(); Bytes = $_.Length }
    } | Format-Table -AutoSize | Out-Host
}

try {
    [System.IO.Directory]::CreateDirectory($artifacts) | Out-Null
    Push-Location $repository
    try {
        $env:GOOS = 'linux'
        $env:GOARCH = 'amd64'
        $env:CGO_ENABLED = '0'
        foreach ($entry in @(
            @{ Output = 'ardents-linux-amd64'; Package = './cmd/ardents' },
            @{ Output = 'ardents-control-linux-amd64'; Package = './cmd/ardents-control' },
            @{ Output = 'ardents-node-linux-amd64'; Package = './cmd/ardents-node' },
            @{ Output = 'reference-c2-linux-amd64'; Package = './tests/e2e/service/fixturecommand/reference-c2' }
        )) {
            & go build -trimpath -buildvcs=false -o (Join-Path $artifacts $entry.Output) $entry.Package
            if ($LASTEXITCODE -ne 0) {
                throw "build failed for $($entry.Package)"
            }
        }
        & go test -c -trimpath -o (Join-Path $artifacts 'h4-3b-docker.test') ./tests/e2e/service
        if ($LASTEXITCODE -ne 0) {
            throw 'build failed for H4-3B Service test binary'
        }
        & go test -c -trimpath -o (Join-Path $artifacts 'h4-6a-control.test') ./tests/e2e/endpoint
        if ($LASTEXITCODE -ne 0) {
            throw 'build failed for H4-6A control test binary'
        }
        & go test -c -trimpath -o (Join-Path $artifacts 'h4-3b-http-limits.test') ./internal/endpoint/reference
        if ($LASTEXITCODE -ne 0) {
            throw 'build failed for H4-3B HTTP limit test binary'
        }
    }
    finally {
        Pop-Location
    }

    Write-ArtifactDigests $artifacts
    $sshArguments = @('-i', (Resolve-Path -LiteralPath $SSHKey).Path, '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=15', '-o', 'StrictHostKeyChecking=accept-new', "$User@$VPS")
    $remoteRoot = (& $sshPath @sshArguments 'mktemp -d /tmp/ardents-h4-3b-vps-XXXXXX').Trim()
    if ($LASTEXITCODE -ne 0 -or $remoteRoot -notmatch '^/tmp/ardents-h4-3b-vps-[A-Za-z0-9]+$') {
        throw 'VPS did not return a valid H4-3B temporary directory'
    }
    $copyArguments = @('-i', (Resolve-Path -LiteralPath $SSHKey).Path, '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=15', '-o', 'StrictHostKeyChecking=accept-new')
    $copyArguments += Get-ChildItem -LiteralPath $artifacts -File | ForEach-Object { $_.FullName }
    $copyArguments += "$User@$VPS`:$remoteRoot/"
    & $scpPath @copyArguments
    if ($LASTEXITCODE -ne 0) {
        throw 'copying H4-3B qualification bytes to the VPS failed'
    }

    & $sshPath @sshArguments "uname -m; uname -r; nproc; awk '/MemAvailable/ {print `$2 " kB"}' /proc/meminfo; docker version --format '{{.Server.Version}}'; docker image inspect golang:1.26.6 --format '{{.Id}}'"
    if ($LASTEXITCODE -ne 0) {
        throw 'VPS Docker envelope is unavailable'
    }
    $serviceCases = @(
        'TestReferenceC2CarriesOneDynamicPublisherApplication',
        'TestReferenceC2ExplicitlyWithdrawsDynamicPublication',
        'TestReferenceC2ClassifiesPublisherApplicationCrash',
        'TestReferenceC2ClassifiesAbruptPublisherEndpointLoss'
    )
    foreach ($name in $serviceCases) {
        $remoteCommand = "docker run --rm --read-only --network none --pids-limit 128 --memory 1g --cpus 1 --tmpfs /tmp:rw,nosuid,nodev,exec,size=768m --mount type=bind,src=$remoteRoot,dst=/artifacts,readonly -e HOME=/tmp/home -e TMPDIR=/tmp -e ARDENTS_E2E_PRODUCT_ARDENTS=/artifacts/ardents-linux-amd64 -e ARDENTS_E2E_PRODUCT_ARDENTS_CONTROL=/artifacts/ardents-control-linux-amd64 -e ARDENTS_E2E_PRODUCT_ARDENTS_NODE=/artifacts/ardents-node-linux-amd64 -e ARDENTS_E2E_FIXTURE_REFERENCE_C2=/artifacts/reference-c2-linux-amd64 -w /tmp golang:1.26.6 /artifacts/h4-3b-docker.test -test.run '^$name`$' -test.count=1 -test.v -test.timeout=45s"
        & $sshPath @sshArguments $remoteCommand
        if ($LASTEXITCODE -ne 0) {
            throw "VPS H4-3B Service cell $name failed"
        }
    }
    $limitCommand = "docker run --rm --read-only --network none --pids-limit 128 --memory 1g --cpus 1 --tmpfs /tmp:rw,nosuid,nodev,exec,size=768m --mount type=bind,src=$remoteRoot,dst=/artifacts,readonly -e HOME=/tmp/home -e TMPDIR=/tmp -w /tmp golang:1.26.6 /artifacts/h4-3b-http-limits.test -test.run '^(TestTransparentAlphaRoute(AcceptsExactKnownBodyLimits|RejectsOversizedKnownRequestBeforeSelectedService|RejectsOversizedKnownResponseBeforeBrowserHeaders|StopsChunkedRequestAtBodyLimit|StopsChunkedResponseAtBodyLimit|RejectsOversizedRequestHeadersBeforeSelectedService|RejectsOversizedResponseHeadersBeforeBrowserHeaders)|TestTransparentServer(AcceptsExactRequestHeadLimit|RejectsRequestHeadAboveExactLimit|AcceptsExactResponseHeadLimit|RejectsResponseHeadAboveExactLimit|StopsIncompleteBrowserRequestAtHeaderTimeout|ClosesIdleBrowserKeepAliveBeforeSecondServiceRequest))`$' -test.count=1 -test.v -test.timeout=45s"
    & $sshPath @sshArguments $limitCommand
    if ($LASTEXITCODE -ne 0) {
        throw 'VPS H4-3B HTTP limit cell failed'
    }
    foreach ($name in @('TestAlphaControlReaderVerifiesPinnedBundleAndCachedRestart', 'TestAlphaControlReaderTwoFreshEnrolledEndpointsAgree')) {
        $remoteCommand = "docker run --rm --read-only --network none --pids-limit 128 --memory 1g --cpus 1 --tmpfs /tmp:rw,nosuid,nodev,exec,size=768m --mount type=bind,src=$remoteRoot,dst=/artifacts,readonly -e HOME=/tmp/home -e TMPDIR=/tmp -e ARDENTS_E2E_COMMAND=/artifacts/ardents-linux-amd64 -e ARDENTS_E2E_CONTROL=/artifacts/ardents-control-linux-amd64 -w /tmp golang:1.26.6 /artifacts/h4-6a-control.test -test.run '^$name`$' -test.count=1 -test.v -test.timeout=45s"
        & $sshPath @sshArguments $remoteCommand
        if ($LASTEXITCODE -ne 0) {
            throw "VPS H4-6A control cell $name failed"
        }
    }
}
finally {
    if ($remoteRoot -match '^/tmp/ardents-h4-3b-vps-[A-Za-z0-9]+$') {
        $cleanupArguments = @('-i', (Resolve-Path -LiteralPath $SSHKey).Path, '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=15', '-o', 'StrictHostKeyChecking=accept-new', "$User@$VPS")
        & $sshPath @cleanupArguments "rm -rf -- $remoteRoot; test ! -e $remoteRoot"
        if ($LASTEXITCODE -ne 0) {
            throw "VPS H4-3B temporary directory cleanup failed: $remoteRoot"
        }
    }
    if (Test-Path -LiteralPath $artifacts -PathType Container) {
        Remove-Item -LiteralPath $artifacts -Recurse -Force
    }
}
