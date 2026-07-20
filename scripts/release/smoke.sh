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
test -f "$extract_dir/docker/docker-compose.multinode.yml"
"$extract_dir/ard" version | grep -F "$version" >/dev/null
"$extract_dir/ardd" --version | grep -F "$version" >/dev/null
"$extract_dir/ard" --output json version | grep -F "\"commit\":\"$commit\"" >/dev/null
printf 'release-smoke=passed\n'
