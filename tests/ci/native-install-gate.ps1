param(
    [string]$ReportDir = "tests/.artifacts/native-install"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location $root
$reportPath = [IO.Path]::GetFullPath($ReportDir)
$runID = [guid]::NewGuid().ToString("N")
$stage = Join-Path ([IO.Path]::GetTempPath()) "ardents-native-install-$runID"
$v1 = Join-Path $stage "v1"
$v2 = Join-Path $stage "v2"
$bad = Join-Path $stage "bad"
$container = "ardents-native-systemd-$runID"
$image = "ardents/native-systemd-smoke:$runID"

[IO.Directory]::CreateDirectory((Join-Path $v1 "scripts/install")) | Out-Null
[IO.Directory]::CreateDirectory((Join-Path $v1 "systemd")) | Out-Null
[IO.Directory]::CreateDirectory((Join-Path $v2 "systemd")) | Out-Null
[IO.Directory]::CreateDirectory($bad) | Out-Null
Copy-Item scripts/install/linux.sh,scripts/install/systemd-smoke.sh -Destination (Join-Path $v1 "scripts/install")
Copy-Item deploy/systemd/ardentsd.service -Destination (Join-Path $v1 "systemd")
Copy-Item deploy/systemd/ardentsd.service -Destination (Join-Path $v2 "systemd")

try {
    docker run --rm `
        --mount "type=bind,source=$root,target=/workspace,readonly" `
        --mount "type=bind,source=$stage,target=/release" `
        --mount source=ardents-go-build-cache,target=/root/.cache/go-build `
        --mount source=ardents-go-mod-cache,target=/go/pkg/mod `
        -w /workspace golang:1.26-bookworm sh tests/ci/build-native-install-fixtures.sh
    if ($LASTEXITCODE -ne 0) { throw "native smoke binaries failed to build" }

    docker build --provenance=false -t $image -f tests/fixtures/native-systemd.Dockerfile .
    if ($LASTEXITCODE -ne 0) { throw "native systemd smoke image failed to build" }
    docker run -d --privileged --name $container --tmpfs /run --tmpfs /run/lock $image | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "native systemd smoke container failed to start" }
    docker cp "$stage/." "${container}:/release"
    if ($LASTEXITCODE -ne 0) { throw "native release staging copy failed" }

    $deadline = [DateTime]::UtcNow.AddSeconds(30)
    do {
        docker exec $container systemctl is-system-running --quiet
        if ($LASTEXITCODE -eq 0) { break }
        Start-Sleep -Milliseconds 250
    } while ([DateTime]::UtcNow -lt $deadline)
    if ($LASTEXITCODE -ne 0) {
        docker logs $container
        throw "systemd did not become ready"
    }

    docker exec $container /bin/sh /release/v1/scripts/install/systemd-smoke.sh /release/v1 /release/v2 /release/bad
    if ($LASTEXITCODE -ne 0) { throw "native systemd acceptance failed" }
    [IO.File]::WriteAllText((Join-Path $reportPath "passed.txt"), "native-systemd-smoke=passed`n")
} finally {
    if (docker ps -a --format '{{.Names}}' | Select-String -SimpleMatch $container -Quiet) {
        docker rm -f $container 2>$null | Out-Null
    }
    if (docker image inspect $image 2>$null) {
        docker image rm $image 2>$null | Out-Null
    }
    if (Test-Path -LiteralPath $stage) { Remove-Item -LiteralPath $stage -Recurse -Force }
}
