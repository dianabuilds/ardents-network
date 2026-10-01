param([Parameter(Mandatory=$true)][string]$EvidenceRoot)
$ErrorActionPreference='Stop'
$taskRoot=[IO.Path]::GetFullPath($EvidenceRoot)
$taskAncestor=[IO.DirectoryInfo]::new($taskRoot)
while ($null -ne $taskAncestor) {
    if ($taskAncestor.Exists -and ($taskAncestor.Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw 'Refuse redirected installation root.' }
    if (Test-Path -LiteralPath (Join-Path $taskAncestor.FullName '.git')) { throw 'Public installation artifacts must be outside Git.' }
    $taskAncestor=$taskAncestor.Parent
}
if (Test-Path -LiteralPath $taskRoot) { throw 'Refuse installation evidence reuse.' }
New-Item -ItemType Directory -Path $taskRoot | Out-Null
$taskHelper=docker image inspect ardents-diagnostics:prebuilt-parsers-final-389 --format '{{.Id}}'
if ($LASTEXITCODE -ne 0) { throw 'Explicitly installed inspection helper required.' }
$taskReceipt=[ordered]@{started_utc=(Get-Date).ToUniversalTime().ToString('o');scope='public OTel Builder inspection only; target not executed';helper_image=$taskHelper;complete=$false;assets=@()}
function Receive-PinnedAsset([string]$Name,[long]$Size,[string]$Hash) {
    $taskUrl='https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/cmd%2Fbuilder%2Fv0.162.0/'+$Name
    $taskRequest=[Net.HttpWebRequest]::Create($taskUrl)
    $taskRequest.Timeout=20000; $taskRequest.ReadWriteTimeout=20000
    $taskResponse=$taskRequest.GetResponse()
    $taskInput=$taskResponse.GetResponseStream()
    $taskOutput=[IO.File]::Open((Join-Path $taskRoot $Name),[IO.FileMode]::CreateNew,[IO.FileAccess]::Write)
    $taskBuffer=New-Object byte[] 65536; $taskTotal=0L; $taskClock=[Diagnostics.Stopwatch]::StartNew()
    try {
        while (($taskRead=$taskInput.Read($taskBuffer,0,$taskBuffer.Length)) -gt 0) {
            $taskTotal+=$taskRead
            if ($taskTotal -gt $Size -or $taskClock.Elapsed.TotalSeconds -gt 120) { throw 'Public asset exceeded finite download budget.' }
            $taskOutput.Write($taskBuffer,0,$taskRead)
        }
    } finally { $taskOutput.Dispose(); $taskInput.Dispose(); $taskResponse.Dispose() }
    $taskHasher=[Security.Cryptography.SHA256]::Create(); $taskFile=[IO.File]::OpenRead((Join-Path $taskRoot $Name))
    try { $taskActual=[BitConverter]::ToString($taskHasher.ComputeHash($taskFile)).Replace('-','').ToLowerInvariant() }
    finally { $taskFile.Dispose(); $taskHasher.Dispose() }
    $taskReceipt.assets += [ordered]@{url=$taskUrl;bytes=$taskTotal;sha256=$taskActual;published_digest_matches=($taskActual -eq $Hash -and $taskTotal -eq $Size)}
    if ($taskActual -ne $Hash -or $taskTotal -ne $Size) { throw 'Public archive/SBOM identity mismatch.' }
}
try {
    Receive-PinnedAsset 'ocb_0.162.0_linux_amd64' 8417442 '7c74640d726f23689d8853e0d5a55707ad8b524417ca7416c036c4ecb9ddb01f'
    docker run --rm --network none --read-only --cap-drop ALL --security-opt no-new-privileges --user 10001:10001 --memory 128m --cpus 0.5 --pids-limit 8 --mount "type=bind,source=$taskRoot,target=/assets,readonly" --entrypoint go $taskHelper version -m /assets/ocb_0.162.0_linux_amd64 | Out-File (Join-Path $taskRoot 'binary-buildinfo.txt') -Encoding utf8
    if ($LASTEXITCODE -ne 0) { throw 'Builder build metadata inspection failed.' }
    docker run --rm --network none --read-only --cap-drop ALL --security-opt no-new-privileges --user 10001:10001 --memory 128m --cpus 0.5 --pids-limit 8 --mount "type=bind,source=$taskRoot,target=/assets,readonly" --entrypoint readelf $taskHelper -l /assets/ocb_0.162.0_linux_amd64 | Out-File (Join-Path $taskRoot 'elf-program-headers.txt') -Encoding utf8
    if ($LASTEXITCODE -ne 0) { throw 'Builder ELF metadata inspection failed.' }
    $taskReceipt.complete=$true
} finally {
    $taskReceipt.ended_utc=(Get-Date).ToUniversalTime().ToString('o')
    $taskReceipt.signature='not verified; matching published digest is not signature or admission proof'
    $taskReceipt | ConvertTo-Json -Depth 6 | Out-File (Join-Path $taskRoot 'receipt.json') -Encoding utf8
}