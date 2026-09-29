#!/bin/sh
set -eu
# Only invoked by explicit make tools-install DIAGNOSTIC_TOOLS=1 in Linux Docker.
archive="$(mktemp /tmp/ardents-powershell.XXXXXX)"
trap 'rm -f "$archive"' EXIT HUP INT TERM
curl --fail --location --retry 2 --output "$archive" https://github.com/PowerShell/PowerShell/releases/download/v7.6.6/powershell-7.6.6-linux-x64.tar.gz
printf '%s  %s\n' ddbc4a2d113bbd46d283cfedcbcd117a70caefd7673f41f2b4e0000badf103bc "$archive" | sha256sum -c -
mkdir -p /opt/ardents-diagnostics/powershell
tar -xzf "$archive" -C /opt/ardents-diagnostics/powershell
chmod 755 /opt/ardents-diagnostics/powershell/pwsh
ln -s /opt/ardents-diagnostics/powershell/pwsh /usr/local/bin/pwsh
