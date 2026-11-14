#!/bin/sh
set -eu
# Only invoked by explicit make tools-install DIAGNOSTIC_TOOLS=1.
if [ "$(uname -s)" = Darwin ]; then
    case "$(uname -m)" in
        arm64) asset=osx-arm64; digest=6df833d094ebac1c1a74340d7b3437f4aaf5e03ce640484a1c4359f3ce8b3db1 ;;
        x86_64) asset=osx-x64; digest=e325ed9f666894eb39a5ea52800b602da2fb4242bbe9747ceddb39cdc66de805 ;;
        *) echo 'unsupported PowerShell developer architecture' >&2; exit 1 ;;
    esac
    destination="${ARDENTS_POWERSHELL_ROOT:?set an absolute fresh external PowerShell directory}"
    case "$destination" in /*) ;; *) echo 'PowerShell directory must be absolute' >&2; exit 1 ;; esac
    destination="$(CDPATH= cd -- "$(dirname "$destination")" && pwd -P)/$(basename "$destination")"
    case "$destination/" in "$(pwd -P)/"*) echo 'PowerShell directory must be outside the repository' >&2; exit 1 ;; esac
    if [ -e "$destination" ] || [ -L "$destination" ]; then
        echo 'PowerShell destination already exists' >&2
        exit 1
    fi
    archive="$(mktemp /tmp/ardents-powershell.XXXXXX)"
    staging="$(mktemp -d "$(dirname "$destination")/.ardents-powershell.XXXXXX")"
    trap 'rm -f "$archive"; rm -rf "$staging"' EXIT HUP INT TERM
    curl --fail --location --retry 2 --output "$archive" "https://github.com/PowerShell/PowerShell/releases/download/v7.6.6/powershell-7.6.6-$asset.tar.gz"
    printf '%s  %s\n' "$digest" "$archive" | shasum -a 256 -c -
    mkdir "$staging/lib" "$staging/bin"
    tar -xzf "$archive" -C "$staging/lib"
    chmod 755 "$staging/lib/pwsh"
    cat > "$staging/bin/pwsh" <<'MAC_WRAPPER'
#!/bin/sh
tool_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd -P)
export XDG_CACHE_HOME="$tool_root/cache" XDG_CONFIG_HOME="$tool_root/config" XDG_DATA_HOME="$tool_root/data"
export POWERSHELL_TELEMETRY_OPTOUT=1 DOTNET_CLI_TELEMETRY_OPTOUT=1
exec "$tool_root/lib/pwsh" "$@"
MAC_WRAPPER
    chmod 755 "$staging/bin/pwsh"
    # Publish the verified tool with an exclusive destination. The parent's
    # temporary directory is ours; no existing installation is overwritten.
    mkdir "$destination"
    mv "$staging/lib" "$staging/bin" "$destination/"
    "$destination/bin/pwsh" --version
    exit 0
fi
# Existing Linux Docker installation retains its reviewed x64 artifact.
archive="$(mktemp /tmp/ardents-powershell.XXXXXX)"
trap 'rm -f "$archive"' EXIT HUP INT TERM
curl --fail --location --retry 2 --output "$archive" https://github.com/PowerShell/PowerShell/releases/download/v7.6.6/powershell-7.6.6-linux-x64.tar.gz
printf '%s  %s\n' ddbc4a2d113bbd46d283cfedcbcd117a70caefd7673f41f2b4e0000badf103bc "$archive" | sha256sum -c -
mkdir -p /opt/ardents-diagnostics/powershell
tar -xzf "$archive" -C /opt/ardents-diagnostics/powershell
chmod 755 /opt/ardents-diagnostics/powershell/pwsh
cat > /usr/local/bin/pwsh <<'WRAPPER'
#!/bin/sh
# Tool-only XDG placement must not alter inherited Endpoint test state.
export XDG_CACHE_HOME=/tmp/ardents-powershell-cache
export XDG_CONFIG_HOME=/tmp/ardents-powershell-config
export XDG_DATA_HOME=/tmp/ardents-powershell-data
export POWERSHELL_TELEMETRY_OPTOUT=1 DOTNET_CLI_TELEMETRY_OPTOUT=1
exec /opt/ardents-diagnostics/powershell/pwsh "$@"
WRAPPER
chmod 755 /usr/local/bin/pwsh
