#!/bin/sh
set -eu

# Explicit tools-install bootstrap only. No normal gate invokes this installer.
test "$(uname -s)" = Linux
test "$(id -u)" = 0
. /etc/os-release
test "$ID" = ubuntu && test "$VERSION_ID" = 24.04
test "$(dpkg --print-architecture)" = amd64

version=4.3-4.1build2
if command -v make >/dev/null; then
    test "$(dpkg-query -W -f='${Version}' make)" = "$version"
    exit
fi
# APT authenticates its configured Noble repository. Never add a repository,
# disable authentication, remove packages or upgrade the existing host closure.
package=${ARDENTS_NODE_NATIVE_MAKE_PACKAGE:-}
if test -n "$package"; then
    case "$package" in /*) ;; *) exit 2;; esac
    test -f "$package"
    # Independent SHA-256 from the authenticated Noble APT package inventory.
    expected=1fe6a815b56c7b6e9ce4086a363f09444bbd0a0d30e230c453d0b78e44b57a99
    actual=$(sha256sum "$package")
    test "${actual%% *}" = "$expected"
    apt-get --yes --no-remove --no-upgrade --no-install-recommends install "$package"
else
    apt-get --yes --no-remove --no-upgrade --no-install-recommends install "make=$version"
fi
test "$(dpkg-query -W -f='${Version}' make)" = "$version"
make --version
