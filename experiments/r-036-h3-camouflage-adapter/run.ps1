param(
    [Parameter(Mandatory = $true)][string]$Lyrebird,
    [Parameter(Mandatory = $true)][string]$WebTunnelClient,
    [Parameter(Mandatory = $true)][string]$WebTunnelServer,
    [Parameter(Mandatory = $true)][string]$EvidenceRoot
)

$ErrorActionPreference = 'Stop'
$experiment = Split-Path -Parent $MyInvocation.MyCommand.Path
$ownedRoot = [System.IO.Path]::GetFullPath($EvidenceRoot)
$repositoryRoot = [System.IO.Path]::GetFullPath((Join-Path $experiment '..\..'))
$repositoryPrefix = $repositoryRoot.TrimEnd('\') + '\'
if ($ownedRoot.Equals($repositoryRoot, [StringComparison]::OrdinalIgnoreCase) -or
    $ownedRoot.StartsWith($repositoryPrefix, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'EvidenceRoot must be outside the repository'
}
$binaryDir = Join-Path $ownedRoot 'bin'
$runRoot = Join-Path $ownedRoot 'runs'
$network = 'ardents-r036-' + [Guid]::NewGuid().ToString('N').Substring(0, 12)
$seedBytes = New-Object byte[] 32
$random = [System.Security.Cryptography.RandomNumberGenerator]::Create()
$random.GetBytes($seedBytes)
$random.Dispose()
$workloadSeed = ([BitConverter]::ToString($seedBytes) -replace '-', '').ToLowerInvariant()

New-Item -ItemType Directory -Force -Path $binaryDir, $runRoot | Out-Null
foreach ($candidate in @('obfs4', 'webtunnel')) {
    foreach ($rung in @('stdin', 'sigterm', 'sigkill')) {
        New-Item -ItemType Directory -Force -Path `
            (Join-Path $runRoot "results\$candidate\$rung"), `
            (Join-Path $runRoot "observer-sync\$candidate\$rung") | Out-Null
    }
}
$harness = Join-Path $binaryDir 'r036-adapter-lab'
$image = 'ubuntu@sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960'
$env:CGO_ENABLED = '0'
$env:GOOS = 'linux'
$env:GOARCH = 'amd64'
$sources = Get-ChildItem -LiteralPath $experiment -Filter '*.go' |
    Where-Object { $_.Name -notlike '*_test.go' } |
    Select-Object -ExpandProperty FullName
go build -trimpath -buildvcs=false -ldflags=-buildid= -o $harness $sources
if ($LASTEXITCODE -ne 0) { throw 'harness build failed' }
$harnessSHA = (Get-FileHash -Algorithm SHA256 -LiteralPath $harness).Hash.ToLowerInvariant()
$lyrebirdSHA = (Get-FileHash -Algorithm SHA256 -LiteralPath $Lyrebird).Hash.ToLowerInvariant()
$webClientSHA = (Get-FileHash -Algorithm SHA256 -LiteralPath $WebTunnelClient).Hash.ToLowerInvariant()
$webServerSHA = (Get-FileHash -Algorithm SHA256 -LiteralPath $WebTunnelServer).Hash.ToLowerInvariant()
$pinnedLyrebirdSHA = 'ae8fe150a98263d13967d07378a1f5af924ba8baad26bdffe6d8b489eee7ba62'
$pinnedWebClientSHA = 'de581c8dd36193bb4168aee840406294af406bf8187817c10ac2bcd9464fd120'
$pinnedWebServerSHA = '5fe32f8ab736ed54fc66027775761084e68f0e1ec9b5fea7c3417c6617255336'
if ($lyrebirdSHA -ne $pinnedLyrebirdSHA) { throw 'Lyrebird binary does not match R-036' }
if ($webClientSHA -ne $pinnedWebClientSHA) { throw 'WebTunnel client does not match R-036' }
if ($webServerSHA -ne $pinnedWebServerSHA) { throw 'WebTunnel server does not match R-036' }

docker network create --internal --subnet 192.0.2.0/24 $network | Out-Null
try {
    function Invoke-Campaign([string]$candidate, [string]$rung) {
        $container = $network + '-' + $candidate + '-' + $rung
        $evidence = "/evidence/results/$candidate/$rung"
        $sync = "/evidence/observer-sync/$candidate/$rung"
        $hashes = if ($candidate -eq 'obfs4') {
            @($pinnedLyrebirdSHA, $pinnedLyrebirdSHA)
        } else {
            @($pinnedWebClientSHA, $pinnedWebServerSHA)
        }
        $candidateMounts = if ($candidate -eq 'obfs4') {
            @('-v', "${Lyrebird}:/candidate/lyrebird:ro")
        } else {
            @('-v', "${WebTunnelClient}:/candidate/webtunnel-client:ro",
                '-v', "${WebTunnelServer}:/candidate/webtunnel-server:ro")
        }
        $common = @(
        'run', '-d', '--name', $container, '--network', $network, '--ip', '192.0.2.3',
        '--dns', '127.0.0.53', '--cap-drop', 'ALL',
        '--security-opt', 'no-new-privileges', '--user', '65534:65534',
        '--read-only', '--tmpfs', '/tmp:rw,noexec,nosuid,nodev,size=8m',
        '-v', "${harness}:/lab:ro"
        ) + $candidateMounts + @(
        '-v', "${runRoot}:/evidence:rw",
        $image,
        '/lab', '-candidate', $candidate, '-evidence', $evidence,
        '-observer-sync', $sync, '-shutdown-rung', $rung, '-seed', $workloadSeed,
        '-client-sha256', $hashes[0], '-server-sha256', $hashes[1],
        '-harness-sha256', $harnessSHA, '-image', $image
        )
        docker @common | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "$candidate/$rung container start failed" }
        try {
            docker run --rm --network "container:$container" --cap-drop ALL `
                --cap-add NET_RAW --security-opt no-new-privileges --user 0:0 `
                --read-only -v "${harness}:/lab:ro" -v "${runRoot}:/evidence:rw" `
                $image /lab -dns-observer -observer-sync $sync
            if ($LASTEXITCODE -ne 0) {
                docker logs $container
                throw "$candidate/$rung DNS observer failed"
            }
            $campaignExit = (docker wait $container).Trim()
            if ($LASTEXITCODE -ne 0 -or $campaignExit -ne '0') {
                docker logs $container
                throw "$candidate/$rung campaign failed with exit $campaignExit"
            }
        }
        finally {
            docker rm -f $container 2>$null | Out-Null
        }
    }

    foreach ($candidate in @('obfs4', 'webtunnel')) {
        foreach ($rung in @('stdin', 'sigterm', 'sigkill')) {
            Invoke-Campaign $candidate $rung
        }
    }
}
finally {
    docker network rm $network | Out-Null
}
