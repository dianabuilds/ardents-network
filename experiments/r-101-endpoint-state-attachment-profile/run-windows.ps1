$ErrorActionPreference = 'Stop'

$root = Join-Path ([System.IO.Path]::GetTempPath()) 'ardents-r101-state-ipc-20260824'
if (Test-Path -LiteralPath $root) {
    throw "Refusing to reuse existing experiment root: $root"
}

try {
    go run main.go -root $root
}
finally {
    if ((Test-Path -LiteralPath $root) -and ((Resolve-Path -LiteralPath $root).Path -eq $root)) {
        Remove-Item -LiteralPath $root -Recurse -Force
    }
}
