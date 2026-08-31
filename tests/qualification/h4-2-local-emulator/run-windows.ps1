[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$repository = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..\..\..')).Path
$artifacts = Join-Path ([System.IO.Path]::GetTempPath()) ("ardents-h4-2-local-emulator-" + [Guid]::NewGuid())
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
            if ($entry.Package -eq './tests/e2e/service/fixturecommand/reference-c2') {
                & go build -trimpath -buildvcs=false -tags referencec2 -o (Join-Path $artifacts $entry.Output) $entry.Package
            }
            else {
                & go build -trimpath -buildvcs=false -o (Join-Path $artifacts $entry.Output) $entry.Package
            }
            if ($LASTEXITCODE -ne 0) {
                throw "build failed for $($entry.Package)"
            }
        }
        & go test -c -trimpath -o (Join-Path $artifacts 'h4-2-local-emulator.test') ./tests/e2e/service
        if ($LASTEXITCODE -ne 0) {
            throw 'build failed for H4-2 local emulator test binary'
        }
        foreach ($entry in @(
            @{ Output = 'h4-2-carrier-product.test'; Package = './internal/endpoint' },
            @{ Output = 'h4-2-carrier-state.test'; Package = './internal/network/state' },
            @{ Output = 'h4-2-carrier-route.test'; Package = './internal/route' }
        )) {
            & go test -c -trimpath -o (Join-Path $artifacts $entry.Output) $entry.Package
            if ($LASTEXITCODE -ne 0) {
                throw "build failed for $($entry.Package)"
            }
        }
    }
    finally {
        Pop-Location
    }
    $cases = @(
        @{ Binary = 'h4-2-local-emulator.test'; Pattern = '^TestReferenceC2HardStopsRendezvousWithHeldRoute$'; Timeout = '90s' },
        @{ Binary = 'h4-2-carrier-product.test'; Pattern = '^TestC2RouteCarriesReferenceSiteBetweenTwoEndpoints$'; Timeout = '30s' },
        @{ Binary = 'h4-2-carrier-state.test'; Pattern = '^TestCurrentNodeDutyProjectsSignedCarrierProfilesAndRejectsUnknown$'; Timeout = '30s' },
        @{ Binary = 'h4-2-carrier-route.test'; Pattern = '^Test(FailedQUICAttemptNeverFallsBackToListeningTCP|FailedTCPAttemptNeverFallsBackToListeningQUICSocket|FailedQUICBindingAbortsInsteadOfGracefullyClosing|OpenNodeLegUsesSameBindingOverQUIC|OpenNodeLegConfirmsExactTLSAndLegBinding|OpenNodeLegRejectsUnknownCarrierWithoutDial|QUICCarrierProfileKeepsOptionalSemanticsDisabled|QUICListenerReturnsPendingCarrierBeforeAuthenticationCompletes)$'; Timeout = '30s' }
    )
    foreach ($case in $cases) {
        $arguments = @(
            'run', '--rm', '--read-only', '--network', 'none',
            '--pids-limit', '256', '--memory', '1g', '--cpus', '1',
            '--tmpfs', '/tmp:rw,nosuid,nodev,exec,size=768m',
            '--mount', "type=bind,src=$artifacts,dst=/artifacts,readonly",
            '-e', 'HOME=/tmp/home', '-e', 'TMPDIR=/tmp',
            '-e', 'ARDENTS_E2E_PRODUCT_ARDENTS=/artifacts/ardents-linux-amd64',
            '-e', 'ARDENTS_E2E_PRODUCT_ARDENTS_CONTROL=/artifacts/ardents-control-linux-amd64',
            '-e', 'ARDENTS_E2E_PRODUCT_ARDENTS_NODE=/artifacts/ardents-node-linux-amd64',
            '-e', 'ARDENTS_E2E_FIXTURE_REFERENCE_C2=/artifacts/reference-c2-linux-amd64',
            '-w', '/tmp', 'golang:1.26.6', "/artifacts/$($case.Binary)",
            '-test.run', $case.Pattern, '-test.count=1', '-test.v', "-test.timeout=$($case.Timeout)"
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
