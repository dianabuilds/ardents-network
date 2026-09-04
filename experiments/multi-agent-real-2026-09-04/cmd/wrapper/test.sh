#!/bin/bash
set -eo pipefail

WRAPPER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT="$(mktemp "/tmp/ardents-multi-agent-wrapper.XXXXXX.exe")"
trap 'rm -f "$OUT"' EXIT

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
mapfile -t TEST_NAMES < <(grep -Ev '^[[:space:]]*(#|$)' "$WRAPPER_DIR/test-sources.txt")
SOURCES=()
TEST_SOURCES=()
for name in "${SOURCE_NAMES[@]}"; do
    SOURCES+=("$WIN_WRAPPER_DIR/$name")
done
for name in "${TEST_NAMES[@]}"; do
    TEST_SOURCES+=("$WIN_WRAPPER_DIR/$name")
done

echo "== wrapper: build =="
"$GO" build -o "$WIN_OUT" "${SOURCES[@]}"
echo "built $OUT"

echo "== wrapper: vet =="
"$GO" vet "${SOURCES[@]}"
echo "vet ok"

echo "== wrapper: self-test =="
"$GO" test "${SOURCES[@]}" "${TEST_SOURCES[@]}"
"$OUT" self-test
echo "self-test ok"
