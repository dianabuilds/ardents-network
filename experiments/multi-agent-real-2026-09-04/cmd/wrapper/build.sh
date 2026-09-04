#!/bin/bash
set -eo pipefail

OUT="${1:-/tmp/wrapper.exe}"

GO=go
if ! command -v "$GO" >/dev/null 2>&1; then
    if [ -x "/c/Program Files/Go/bin/go.exe" ]; then
        GO="/c/Program Files/Go/bin/go.exe"
    elif [ -x "/mnt/c/Program Files/Go/bin/go.exe" ]; then
        GO="/mnt/c/Program Files/Go/bin/go.exe"
    else
        echo "wrapper: go executable not found" >&2
        exit 2
    fi
fi

WRAPPER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPOSITORY_DIR="$(cd "$WRAPPER_DIR/../../../.." && pwd -P)"
OUT_PARENT="$(cd "$(dirname "$OUT")" && pwd -P)"

case "$OUT_PARENT/" in
    "$REPOSITORY_DIR/"*)
        echo "wrapper: output must stay outside the repository" >&2
        exit 2
        ;;
esac

if [ -e "$OUT" ]; then
    echo "wrapper: output already exists: $OUT" >&2
    exit 2
fi

case "$GO" in
    *.exe)
        win_translate() {
            local p="$1"
            case "$p" in
                /mnt/c/*) echo "C:\\${p#/mnt/c/}" | tr '/' '\\' ;;
                /c/*)      echo "C:\\${p#/c/}"      | tr '/' '\\' ;;
                *)         echo "$p" ;;
            esac
        }
        WIN_WRAPPER_DIR="$(win_translate "$WRAPPER_DIR")"
        WIN_OUT="$(win_translate "$OUT")"
        ;;
    *)
        WIN_WRAPPER_DIR="$WRAPPER_DIR"
        WIN_OUT="$OUT"
        ;;
esac

mapfile -t SOURCE_NAMES < <(grep -Ev '^[[:space:]]*(#|$)' "$WRAPPER_DIR/sources.txt")
SOURCES=()
for name in "${SOURCE_NAMES[@]}"; do
    SOURCES+=("$WIN_WRAPPER_DIR/$name")
done

echo "== wrapper: go version =="
"$GO" version

echo "== wrapper: build =="
"$GO" build -o "$WIN_OUT" "${SOURCES[@]}"

echo "built $OUT"
