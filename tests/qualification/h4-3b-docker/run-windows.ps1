[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$repository = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..\..\..')).Path
$artifacts = Join-Path ([System.IO.Path]::GetTempPath()) ("ardents-h4-3b-docker-" + [Guid]::NewGuid())

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
            throw 'build failed for H4-3B Docker test binary'
        }
        & go test -c -trimpath -o (Join-Path $artifacts 'h4-6a-control.test') ./tests/e2e/endpoint
        if ($LASTEXITCODE -ne 0) {
            throw 'build failed for H4-6A alpha-control test binary'
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
    $cases = @(
        'TestReferenceC2CarriesOneDynamicPublisherApplication',
        'TestReferenceC2ExplicitlyWithdrawsDynamicPublication',
        'TestReferenceC2ClassifiesPublisherApplicationCrash',
        'TestReferenceC2ClassifiesAbruptPublisherEndpointLoss'
    )
    foreach ($name in $cases) {
        $arguments = @(
            'run', '--rm', '--read-only', '--network', 'none',
            '--pids-limit', '128', '--memory', '1g', '--cpus', '1',
            '--tmpfs', '/tmp:rw,nosuid,nodev,exec,size=768m',
            '--mount', "type=bind,src=$artifacts,dst=/artifacts,readonly",
            '-e', 'HOME=/tmp/home', '-e', 'TMPDIR=/tmp',
            '-e', 'ARDENTS_E2E_PRODUCT_ARDENTS=/artifacts/ardents-linux-amd64',
            '-e', 'ARDENTS_E2E_PRODUCT_ARDENTS_CONTROL=/artifacts/ardents-control-linux-amd64',
            '-e', 'ARDENTS_E2E_PRODUCT_ARDENTS_NODE=/artifacts/ardents-node-linux-amd64',
            '-e', 'ARDENTS_E2E_FIXTURE_REFERENCE_C2=/artifacts/reference-c2-linux-amd64',
            '-w', '/tmp', 'golang:1.26.6', '/artifacts/h4-3b-docker.test',
            '-test.run', "^$name`$", '-test.count=1', '-test.v', '-test.timeout=45s'
        )
        & docker @arguments
        if ($LASTEXITCODE -ne 0) {
            exit $LASTEXITCODE
        }
    }

    $limitArguments = @(
        'run', '--rm', '--read-only', '--network', 'none',
        '--pids-limit', '128', '--memory', '1g', '--cpus', '1',
        '--tmpfs', '/tmp:rw,nosuid,nodev,exec,size=768m',
        '--mount', "type=bind,src=$artifacts,dst=/artifacts,readonly",
        '-e', 'HOME=/tmp/home', '-e', 'TMPDIR=/tmp',
        '-w', '/tmp', 'golang:1.26.6', '/artifacts/h4-3b-http-limits.test',
        '-test.run', '^(TestTransparentAlphaRoute(AcceptsExactKnownBodyLimits|RejectsOversizedKnownRequestBeforeSelectedService|RejectsOversizedKnownResponseBeforeBrowserHeaders|StopsChunkedRequestAtBodyLimit|StopsChunkedResponseAtBodyLimit|RejectsOversizedRequestHeadersBeforeSelectedService|RejectsOversizedResponseHeadersBeforeBrowserHeaders)|TestTransparentServer(AcceptsExactRequestHeadLimit|RejectsRequestHeadAboveExactLimit|AcceptsExactResponseHeadLimit|RejectsResponseHeadAboveExactLimit|StopsIncompleteBrowserRequestAtHeaderTimeout|ClosesIdleBrowserKeepAliveBeforeSecondServiceRequest))$', '-test.count=1', '-test.v', '-test.timeout=45s'
    )
    & docker @limitArguments
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }

    foreach ($name in @(
        'TestAlphaControlReaderVerifiesPinnedBundleAndCachedRestart',
        'TestAlphaControlReaderTwoFreshEnrolledEndpointsAgree'
    )) {
        $arguments = @(
            'run', '--rm', '--read-only', '--network', 'none',
            '--pids-limit', '128', '--memory', '1g', '--cpus', '1',
            '--tmpfs', '/tmp:rw,nosuid,nodev,exec,size=768m',
            '--mount', "type=bind,src=$artifacts,dst=/artifacts,readonly",
            '-e', 'HOME=/tmp/home', '-e', 'TMPDIR=/tmp',
            '-e', 'ARDENTS_E2E_COMMAND=/artifacts/ardents-linux-amd64',
            '-e', 'ARDENTS_E2E_CONTROL=/artifacts/ardents-control-linux-amd64',
            '-w', '/tmp', 'golang:1.26.6', '/artifacts/h4-6a-control.test',
            '-test.run', "^$name`$", '-test.count=1', '-test.v', '-test.timeout=45s'
        )
        & docker @arguments
        if ($LASTEXITCODE -ne 0) {
            exit $LASTEXITCODE
        }
    }
    exit 0
}
finally {
    if (Test-Path -LiteralPath $artifacts -PathType Container) {
        Remove-Item -LiteralPath $artifacts -Recurse -Force
    }
}
