Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$source = Join-Path $env:WINDIR 'System32\wsl.exe'
$root = Join-Path ([IO.Path]::GetTempPath()) 'ardents-r095-authenticode-20260824'
$copy = Join-Path $root 'signed-fixture.exe'

if (-not (Test-Path -LiteralPath $source -PathType Leaf)) {
    throw "signed fixture is unavailable: $source"
}
if (Test-Path -LiteralPath $root) {
    throw "refusing occupied experiment root: $root"
}

function Remove-ExperimentRoot {
    if (-not (Test-Path -LiteralPath $root)) {
        return
    }
    $resolved = (Resolve-Path -LiteralPath $root).Path
    if ($resolved -ne $root) {
        throw "refusing unexpected experiment root: $resolved"
    }
    Remove-Item -LiteralPath $resolved -Recurse -Force
}

try {
    New-Item -ItemType Directory -Path $root | Out-Null
    Copy-Item -LiteralPath $source -Destination $copy

    $original = Get-AuthenticodeSignature -LiteralPath $copy
    $before = (Get-FileHash -LiteralPath $copy -Algorithm SHA256).Hash.ToLowerInvariant()

    $stream = [IO.File]::Open($copy, [IO.FileMode]::Open, [IO.FileAccess]::ReadWrite,
        [IO.FileShare]::None)
    try {
        $first = $stream.ReadByte()
        if ($first -lt 0) {
            throw 'signed fixture was unexpectedly empty'
        }
        $stream.Position = 0
        $stream.WriteByte([byte](($first + 1) % 256))
    }
    finally {
        $stream.Dispose()
    }

    $changed = Get-AuthenticodeSignature -LiteralPath $copy
    $after = (Get-FileHash -LiteralPath $copy -Algorithm SHA256).Hash.ToLowerInvariant()

    "original_status=$($original.Status)"
    "changed_status=$($changed.Status)"
    "before_sha256=$before"
    "after_sha256=$after"

    if ($original.Status -ne 'Valid') {
        throw "expected a valid signed fixture, got $($original.Status)"
    }
    if ($changed.Status -eq 'Valid') {
        throw 'changed fixture unexpectedly retained a valid signature'
    }
}
finally {
    Remove-ExperimentRoot
}
