param(
    [Parameter(Mandatory = $true)][string]$ClientBinary,
    [Parameter(Mandatory = $true)][string]$ServerBinary
)

$ErrorActionPreference = 'Stop'
$repository = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$client = (Resolve-Path -LiteralPath $ClientBinary).Path
$server = (Resolve-Path -LiteralPath $ServerBinary).Path
foreach ($candidate in @($client, $server)) {
    if ($candidate.StartsWith($repository + [IO.Path]::DirectorySeparatorChar,
            [StringComparison]::OrdinalIgnoreCase)) {
        throw 'Candidate binaries must remain outside the repository'
    }
}
$expectedClient = 'de581c8dd36193bb4168aee840406294af406bf8187817c10ac2bcd9464fd120'
$expectedServer = '5fe32f8ab736ed54fc66027775761084e68f0e1ec9b5fea7c3417c6617255336'
if ((Get-FileHash -Algorithm SHA256 -LiteralPath $client).Hash.ToLowerInvariant() -ne $expectedClient) {
    throw 'WebTunnel client does not match R-036'
}
if ((Get-FileHash -Algorithm SHA256 -LiteralPath $server).Hash.ToLowerInvariant() -ne $expectedServer) {
    throw 'WebTunnel server does not match R-036'
}

$ownedRoot = Join-Path ([IO.Path]::GetTempPath()) ('ardents-s5-test-' + [guid]::NewGuid().ToString('N'))
$network = 'ardents-s5-' + [guid]::NewGuid().ToString('N').Substring(0, 12)
$runner = $network + '-runner'
$observer = $network + '-observer'
New-Item -ItemType Directory -Path $ownedRoot | Out-Null
$networkCreated = $false
$runnerStarted = $false
$observerStarted = $false
try {
    $env:GOOS = 'linux'
    $env:GOARCH = 'amd64'
    $env:CGO_ENABLED = '0'
    & go test -c -o (Join-Path $ownedRoot 'camouflage.test') ./internal/camouflage
    if ($LASTEXITCODE -ne 0) { throw 'Linux Adapter test build failed' }
    & docker network create --internal --subnet 203.0.113.0/24 $network | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'isolated Docker network creation failed' }
    $networkCreated = $true
    $runnerArguments = @(
		'run', '--detach', '--name', $runner, '--init', '--network', $network,
        '--ip', '203.0.113.7', '--read-only',
		'--pids-limit', '64', '--cpus', '2', '--memory', '256m', '--memory-swap', '256m',
		'--ulimit', 'nofile=128:128',
        '--tmpfs', '/tmp:rw,nosuid,nodev,noexec,size=32m', '--user', '65534:65534',
        '--cap-drop', 'ALL', '--security-opt', 'no-new-privileges',
        '-e', 'ARDENTS_WEBTUNNEL_CLIENT=/candidate/webtunnel-client',
        '-e', 'ARDENTS_WEBTUNNEL_SERVER=/candidate/webtunnel-server',
        '-v', "${client}:/candidate/webtunnel-client:ro",
        '-v', "${server}:/candidate/webtunnel-server:ro",
        '-v', "${ownedRoot}/camouflage.test:/tests/camouflage.test:ro",
        '-v', "${repository}/internal/camouflage/testdata:/tests/testdata:ro",
		'-v', "${ownedRoot}:/evidence",
        '-w', '/tests',
        'ubuntu@sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960',
		'sleep', 'infinity'
    )
    & docker @runnerArguments | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'isolated Adapter container start failed' }
	$runnerStarted = $true
    $observerArguments = @(
		'run', '--detach', '--name', $observer, '--network', "container:${runner}",
		'--pids-limit', '8', '--cpus', '0.25', '--memory', '32m', '--memory-swap', '32m',
		'--ulimit', 'nofile=32:32',
        '--cap-drop', 'ALL', '--cap-add', 'NET_RAW', '--security-opt', 'no-new-privileges',
        '-e', 'ARDENTS_DNS_OBSERVER=1', '-e', 'ARDENTS_DNS_SYNC=/evidence/dns',
        '-v', "${ownedRoot}/camouflage.test:/tests/camouflage.test:ro",
        '-v', "${ownedRoot}:/evidence", '-w', '/tests',
        'ubuntu@sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960',
        '/tests/camouflage.test', '-test.run', '^$'
    )
    & docker @observerArguments | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'DNS observer container start failed' }
	$observerStarted = $true
    $testArguments = @(
        'exec', '--user', '65534:65534', '-e', 'ARDENTS_DNS_SYNC=/evidence/dns',
        $runner, '/tests/camouflage.test', '-test.count=1', '-test.v'
    )
    & docker @testArguments
    if ($LASTEXITCODE -ne 0) { throw 'Linux Adapter test suite failed' }
    & docker wait $observer | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'DNS observer did not finish cleanly' }
} finally {
	if ($observerStarted) { & docker rm --force $observer | Out-Null }
	if ($runnerStarted) { & docker rm --force $runner | Out-Null }
    if ($networkCreated) { & docker network rm $network | Out-Null }
    $resolvedOwned = Resolve-Path -LiteralPath $ownedRoot -ErrorAction SilentlyContinue
    if ($null -ne $resolvedOwned -and
        $resolvedOwned.Path.StartsWith([IO.Path]::GetTempPath(), [StringComparison]::OrdinalIgnoreCase)) {
        Remove-Item -LiteralPath $resolvedOwned.Path -Recurse -Force
    }
}
