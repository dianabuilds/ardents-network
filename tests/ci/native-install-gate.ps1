param(
    [string]$ReportDir = "tests/.artifacts/native-install"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location $root
$reportPath = [IO.Path]::GetFullPath($ReportDir)
[IO.Directory]::CreateDirectory($reportPath) | Out-Null
$passedPath = Join-Path $reportPath "passed.txt"
$systemdLogPath = Join-Path $reportPath "systemd.log"
$lifecycleLogPath = Join-Path $reportPath "lifecycle.log"
$environmentPath = Join-Path $reportPath "environment.json"
$checksumPath = Join-Path $reportPath "SHA256SUMS"
Remove-Item -LiteralPath $passedPath -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath $systemdLogPath,$lifecycleLogPath,$environmentPath,$checksumPath `
    -Force -ErrorAction SilentlyContinue
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

$dockerServer = ""
try {
    $dockerServer = (docker version --format '{{.Server.Version}}' 2>$null)
} catch {
    $dockerServer = ""
}
[ordered]@{
    contract = "privileged-systemd-container"
    qualification_equivalence = "requires-explicit-maintainer-decision"
    host_os = [System.Runtime.InteropServices.RuntimeInformation]::OSDescription
    docker_server = $dockerServer
    container_image = $image
    container_image_id = ""
    systemd = ""
    github_run_id = $env:GITHUB_RUN_ID
    github_run_attempt = $env:GITHUB_RUN_ATTEMPT
    source_commit = $env:GITHUB_SHA
    captured_at = (Get-Date).ToUniversalTime().ToString("o")
} | ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 -LiteralPath $environmentPath

$gateError = $null
try {
    docker run --rm `
        --mount "type=bind,source=$root,target=/workspace,readonly" `
        --mount "type=bind,source=$stage,target=/release" `
        --mount source=ardents-go-build-cache,target=/root/.cache/go-build `
        --mount source=ardents-go-mod-cache,target=/go/pkg/mod `
        -w /workspace docker.io/library/golang:1.26.5-bookworm@sha256:3f6236bd765f898a2a3c2946112b04097814c4529d44534674700cd07b9c6b4c sh tests/ci/build-native-install-fixtures.sh
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

    docker exec $container /bin/sh /release/v1/scripts/install/systemd-smoke.sh `
        /release/v1 /release/v2 /release/bad 2>&1 |
        Tee-Object -FilePath $lifecycleLogPath | Out-Host
    if ($LASTEXITCODE -ne 0) { throw "native systemd acceptance failed" }
    [IO.File]::WriteAllText($passedPath, "native-systemd-smoke=passed`n")
} catch {
    $gateError = $_
} finally {
    if (docker ps -a --format '{{.Names}}' | Select-String -SimpleMatch $container -Quiet) {
        docker logs $container 2>&1 | Set-Content -Encoding utf8 -LiteralPath $systemdLogPath
        $systemdVersion = (docker exec $container systemctl --version 2>$null | Select-Object -First 1)
        $imageIdentity = (docker image inspect $image --format '{{.Id}}' 2>$null)
        [ordered]@{
            contract = "privileged-systemd-container"
            qualification_equivalence = "requires-explicit-maintainer-decision"
            host_os = [System.Runtime.InteropServices.RuntimeInformation]::OSDescription
            docker_server = (docker version --format '{{.Server.Version}}' 2>$null)
            container_image = $image
            container_image_id = $imageIdentity
            systemd = $systemdVersion
            github_run_id = $env:GITHUB_RUN_ID
            github_run_attempt = $env:GITHUB_RUN_ATTEMPT
            source_commit = $env:GITHUB_SHA
            captured_at = (Get-Date).ToUniversalTime().ToString("o")
        } | ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 -LiteralPath $environmentPath
        docker rm -f $container 2>$null | Out-Null
    }
    if (docker image inspect $image 2>$null) {
        docker image rm $image 2>$null | Out-Null
    }
    if (Test-Path -LiteralPath $stage) { Remove-Item -LiteralPath $stage -Recurse -Force }
}

if (-not (Test-Path -LiteralPath $systemdLogPath)) {
    Set-Content -Encoding utf8 -LiteralPath $systemdLogPath -Value "systemd container logs unavailable"
}
if (-not (Test-Path -LiteralPath $lifecycleLogPath)) {
    $detail = if ($null -eq $gateError) { "native lifecycle output unavailable" } else { $gateError.Exception.Message }
    Set-Content -Encoding utf8 -LiteralPath $lifecycleLogPath -Value $detail
}
$checksumLines = foreach ($file in Get-ChildItem -LiteralPath $reportPath -File |
    Where-Object Name -ne "SHA256SUMS" | Sort-Object Name) {
    $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $file.FullName).Hash.ToLowerInvariant()
    "$hash  $($file.Name)"
}
[IO.File]::WriteAllLines($checksumPath, $checksumLines, [Text.UTF8Encoding]::new($false))
if ($null -ne $gateError) { throw $gateError }
