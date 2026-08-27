param(
    [Parameter(Mandatory = $true)][string]$VPS,
    [Parameter(Mandatory = $true)][string]$SSHKey,
    [Parameter(Mandatory = $true)][string]$Candidate,
    [Parameter(Mandatory = $true)][string]$CandidateSHA256,
    [Parameter(Mandatory = $true)][string]$SSHPath,
    [string]$User = 'root'
)

$ErrorActionPreference = 'Stop'
$stageRoot = Join-Path ([IO.Path]::GetTempPath()) ('ardents-h4-2-exact-' + [Guid]::NewGuid().ToString('N'))
$knownHosts = Join-Path $stageRoot 'known_hosts'
$archive = Join-Path $stageRoot 'stage.tar.gz'
$payload = Join-Path $stageRoot 'payload'
$suffix = [Guid]::NewGuid().ToString('N').Substring(0, 16)
$remoteRoot = "/tmp/ardents-h4-2-exact-$suffix"
$remoteArchive = "/tmp/ardents-h4-2-exact-$suffix.tar.gz"
$container = "ardents-h4-2-exact-$suffix"
$scpPath = Join-Path (Split-Path -Parent $SSHPath) 'scp.exe'
$remote = "${User}@${VPS}"
$sshOptions = @('-i', $SSHKey, '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=15', '-o', 'ConnectionAttempts=3',
    '-o', 'ServerAliveInterval=5', '-o', 'ServerAliveCountMax=6', '-o', 'StrictHostKeyChecking=accept-new', '-o', "UserKnownHostsFile=$knownHosts")

function Invoke-Remote([string]$Command) {
    & $SSHPath @sshOptions $remote $Command
    if ($LASTEXITCODE -ne 0) {
        throw "remote H4-2 exact-candidate command failed with exit $LASTEXITCODE"
    }
}

New-Item -ItemType Directory -Path $payload | Out-Null
try {
    Copy-Item -LiteralPath $Candidate -Destination (Join-Path $payload 'ardents')
    $savedGOOS, $savedGOARCH = $env:GOOS, $env:GOARCH
    $savedCGO, $savedToolchain = $env:CGO_ENABLED, $env:GOTOOLCHAIN
    try {
        $env:GOOS = 'linux'
        $env:GOARCH = 'amd64'
        $env:CGO_ENABLED = '0'
        $env:GOTOOLCHAIN = 'local'
        go test -c -trimpath -tags=h4_2_exact_candidate -o (Join-Path $payload 'service.test') ./tests/e2e/service
        if ($LASTEXITCODE -ne 0) { throw 'cross-build exact-candidate service test failed' }
        go build -trimpath -buildvcs=false -o (Join-Path $payload 'ardents-node') ./cmd/ardents-node
        if ($LASTEXITCODE -ne 0) { throw 'cross-build product Node failed' }
        go build -trimpath -buildvcs=false -o (Join-Path $payload 'publish-app') ./tests/e2e/service/fixturecommand/publish-app
        if ($LASTEXITCODE -ne 0) { throw 'cross-build publish-app fixture failed' }
        go build -trimpath -buildvcs=false -o (Join-Path $payload 'stream-app') ./tests/e2e/service/fixturecommand/stream-app
        if ($LASTEXITCODE -ne 0) { throw 'cross-build stream-app fixture failed' }
    }
    finally {
        $env:GOOS = $savedGOOS
        $env:GOARCH = $savedGOARCH
        $env:CGO_ENABLED = $savedCGO
        $env:GOTOOLCHAIN = $savedToolchain
    }
    $stagedDigest = (Get-FileHash -LiteralPath (Join-Path $payload 'ardents') -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($stagedDigest -ne $CandidateSHA256.ToLowerInvariant()) {
        throw "staged exact candidate digest is $stagedDigest"
    }
    $nodeDigest = (Get-FileHash -LiteralPath (Join-Path $payload 'ardents-node') -Algorithm SHA256).Hash.ToLowerInvariant()
    tar.exe -czf $archive -C $payload .
    if ($LASTEXITCODE -ne 0) { throw 'archive exact-candidate qualification payload failed' }

    Invoke-Remote "set -eu; test ! -e $remoteRoot; test ! -e $remoteArchive; ! docker container inspect $container >/dev/null 2>&1; docker image inspect golang:1.26.6 >/dev/null; mkdir -m 700 $remoteRoot"
    if (-not (Test-Path -LiteralPath $scpPath -PathType Leaf)) { throw 'scp.exe is unavailable beside ssh.exe' }
    & $scpPath @sshOptions $archive "${remote}:${remoteArchive}"
    if ($LASTEXITCODE -ne 0) { throw 'upload exact-candidate qualification payload failed' }
    Invoke-Remote "set -eu; tar -xzf $remoteArchive -C $remoteRoot; rm -f $remoteArchive; chmod 700 $remoteRoot/ardents $remoteRoot/ardents-node $remoteRoot/service.test $remoteRoot/publish-app $remoteRoot/stream-app; test `$(sha256sum $remoteRoot/ardents | awk '{print `$1}') = $($CandidateSHA256.ToLowerInvariant()); test `$(sha256sum $remoteRoot/ardents-node | awk '{print `$1}') = $nodeDigest; printf 'remote_host '; uname -srmo; printf 'remote_container_image='; docker image inspect golang:1.26.6 --format '{{.Id}}'; sha256sum $remoteRoot/ardents $remoteRoot/ardents-node $remoteRoot/service.test $remoteRoot/publish-app $remoteRoot/stream-app"
    Invoke-Remote "docker run --rm --name $container --network host --pids-limit 128 --memory 1g --cpus 1 --read-only --tmpfs /tmp:rw,noexec,nosuid,size=256m -v ${remoteRoot}:/work:ro -e ARDENTS_E2E_PRODUCT_ARDENTS=/work/ardents -e ARDENTS_E2E_PRODUCT_ARDENTS_NODE=/work/ardents-node -e ARDENTS_E2E_FIXTURE_PUBLISH_APP=/work/publish-app -e ARDENTS_E2E_FIXTURE_STREAM_APP=/work/stream-app -e ARDENTS_H4_2_CANDIDATE_SHA256=$($CandidateSHA256.ToLowerInvariant()) -e ARDENTS_H4_2_NODE_SHA256=$nodeDigest golang:1.26.6 /work/service.test -test.v -test.run '^TestH42ExactCandidateProductNodesOwnTCPCarrier$' -test.count=1 -test.timeout=90s"
}
finally {
    & $SSHPath @sshOptions $remote "set -eu; docker info >/dev/null; if docker container inspect $container >/dev/null 2>&1; then docker rm -f $container >/dev/null; fi; rm -f $remoteArchive; if [ -d $remoteRoot ]; then rm -rf -- $remoteRoot; fi; test ! -e $remoteRoot; test ! -e $remoteArchive; if docker container inspect $container >/dev/null 2>&1; then exit 1; fi" | Out-Null
    $cleanupExit = $LASTEXITCODE
    $resolvedStage = Resolve-Path -LiteralPath $stageRoot -ErrorAction SilentlyContinue
    if ($null -ne $resolvedStage -and $resolvedStage.Path.StartsWith([IO.Path]::GetTempPath(), [StringComparison]::OrdinalIgnoreCase)) {
        Remove-Item -LiteralPath $resolvedStage.Path -Recurse -Force
    }
    if ($cleanupExit -ne 0) { throw "remote H4-2 exact-candidate cleanup failed with exit $cleanupExit" }
}
