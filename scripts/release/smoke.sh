#!/bin/sh
set -eu

release_dir=${1:?release directory is required}
version=${2:?version is required}
commit=${3:?commit is required}
extract_dir=/tmp/ardents-release-smoke

cd "$release_dir"
sha256sum -c SHA256SUMS >/dev/null
mkdir -p "$extract_dir"
tar -xzf "ardents-${version}-linux-amd64.tar.gz" -C "$extract_dir"
test -f "$extract_dir/LICENSE"
test -f "$extract_dir/ardents.ps1"
test -f "$extract_dir/scripts/deploy/cluster.ps1"
test -x "$extract_dir/scripts/install/linux.sh"
test -x "$extract_dir/scripts/install/smoke.sh"
test -f "$extract_dir/systemd/ardentsd.service"
test -f "$extract_dir/docker/docker-compose.multinode.yml"
"$extract_dir/ardentsctl" version | grep -F "$version" >/dev/null
"$extract_dir/ardentsd" --version | grep -F "$version" >/dev/null
"$extract_dir/ardentsctl" --output json version | grep -F "\"commit\":\"$commit\"" >/dev/null
"$extract_dir/scripts/install/smoke.sh" "$extract_dir"
printf 'release-smoke=passed\n'
