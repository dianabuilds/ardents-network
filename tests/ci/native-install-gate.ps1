param(
    [string]$ReportDir = "tests/.artifacts/native-install"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location $root
$reportPath = [IO.Path]::GetFullPath($ReportDir)
$runID = [guid]::NewGuid().ToString("N")
$stage = Join-Path $reportPath "stage-$runID"
$container = "ardents-native-systemd-$runID"
$image = "ardents/native-systemd-smoke:$runID"

[IO.Directory]::CreateDirectory((Join-Path $stage "scripts/install")) | Out-Null
[IO.Directory]::CreateDirectory((Join-Path $stage "systemd")) | Out-Null
Copy-Item scripts/install/linux.sh,scripts/install/systemd-smoke.sh -Destination (Join-Path $stage "scripts/install")
Copy-Item deploy/systemd/ardentsd.service -Destination (Join-Path $stage "systemd")

try {
    docker run --rm `
        --mount "type=bind,source=$root,target=/workspace,readonly" `
        --mount "type=bind,source=$stage,target=/release" `
        --mount source=ardents-go-build-cache,target=/root/.cache/go-build `
        --mount source=ardents-go-mod-cache,target=/go/pkg/mod `
        -w /workspace golang:1.26-bookworm sh -c `
        "go build -trimpath -buildvcs=false -o /release/ardentsd ./cmd/ardentsd && go build -trimpath -buildvcs=false -o /release/ardentsctl ./cmd/ardentsctl"
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

    docker exec $container /bin/sh /release/scripts/install/systemd-smoke.sh /release
    if ($LASTEXITCODE -ne 0) { throw "native systemd acceptance failed" }
    [IO.File]::WriteAllText((Join-Path $reportPath "passed.txt"), "native-systemd-smoke=passed`n")
} finally {
    docker rm -f $container 2>$null | Out-Null
    docker image rm $image 2>$null | Out-Null
    if (Test-Path -LiteralPath $stage) { Remove-Item -LiteralPath $stage -Recurse -Force }
}
