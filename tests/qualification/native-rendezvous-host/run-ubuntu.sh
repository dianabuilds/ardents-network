#!/bin/sh
# R-092 NET-01A host eligibility and evidence preparation. See README.md.
set -eu

fail_reason=''

finish() {
	status=$?
	if [ -n "${evidence_dir:-}" ] && [ -d "$evidence_dir" ]; then
		if [ "$status" -eq 0 ]; then
			printf '%s\n' 'eligible-for-later-campaign; no capacity result' >"$evidence_dir/outcome.txt"
		else
			printf 'invalid-environment: %s\n' "${fail_reason:-preflight command failed}" >"$evidence_dir/outcome.txt"
		fi
	fi
	exit "$status"
}

trap finish EXIT

invalid() {
	fail_reason=$1
	printf 'NET-01A invalid environment: %s\n' "$fail_reason" >&2
	exit 1
}

require_command() {
	command -v "$1" >/dev/null 2>&1 || invalid "required command is unavailable: $1"
}

case ${ARDENTS_NET_01A_EVIDENCE_DIR:-} in
	/*) evidence_dir=$ARDENTS_NET_01A_EVIDENCE_DIR ;;
	*) invalid 'ARDENTS_NET_01A_EVIDENCE_DIR must be a new absolute path' ;;
esac
case ${ARDENTS_NET_01A_LINK_EVIDENCE:-} in
	/*) link_evidence=$ARDENTS_NET_01A_LINK_EVIDENCE ;;
	*) invalid 'ARDENTS_NET_01A_LINK_EVIDENCE must name an absolute regular file' ;;
esac
case ${ARDENTS_NET_01A_HOST_DECLARATION:-} in
	/*) host_declaration=$ARDENTS_NET_01A_HOST_DECLARATION ;;
	*) invalid 'ARDENTS_NET_01A_HOST_DECLARATION must name an absolute regular file' ;;
esac

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd -P)
case $evidence_dir in
	"$repository_root"|"$repository_root"/*) invalid 'evidence directory must stay outside the repository' ;;
esac
[ ! -e "$evidence_dir" ] || invalid 'evidence directory already exists'
umask 077
mkdir -- "$evidence_dir"

require_command awk
require_command git
require_command go
require_command getconf
require_command sha256sum
require_command stat
require_command uname

[ -f "$link_evidence" ] || invalid 'link evidence is not a regular file'
[ -s "$link_evidence" ] || invalid 'link evidence is empty'
[ -f "$host_declaration" ] || invalid 'host declaration is not a regular file'
[ -s "$host_declaration" ] || invalid 'host declaration is empty'
[ "$(uname -s)" = Linux ] || invalid 'host is not Linux'
[ "$(id -u)" -ne 0 ] || invalid 'campaign must run from an unprivileged account'
[ -r /etc/os-release ] || invalid '/etc/os-release is unavailable'

os_id=$(awk -F= '$1 == "ID" { gsub(/\"/, "", $2); print $2; exit }' /etc/os-release)
os_version=$(awk -F= '$1 == "VERSION_ID" { gsub(/\"/, "", $2); print $2; exit }' /etc/os-release)
[ "$os_id" = ubuntu ] || invalid "host is not Ubuntu: $os_id"
case $os_version in
	20.04|22.04|24.04|26.04) ;;
	*) invalid "Ubuntu release is not a declared LTS version: $os_version" ;;
esac

architecture=$(uname -m)
[ "$architecture" = x86_64 ] || invalid "architecture is not x86-64: $architecture"
online_cpus=$(getconf _NPROCESSORS_ONLN)
[ "$online_cpus" = 2 ] || invalid "visible CPU count is not exactly 2: $online_cpus"
[ -r /proc/meminfo ] || invalid '/proc/meminfo is unavailable'
memory_kib=$(awk '$1 == "MemTotal:" { print $2; exit }' /proc/meminfo)
case $memory_kib in
	''|*[!0-9]*) invalid 'MemTotal observation is invalid' ;;
esac
[ "$memory_kib" -ge 1900000 ] && [ "$memory_kib" -le 2200000 ] || invalid "MemTotal is outside the 2-GiB observation band: $memory_kib KiB"

[ -d /sys/fs/cgroup ] || invalid 'cgroup hierarchy is unavailable'
[ "$(stat -fc %T /sys/fs/cgroup)" = cgroup2fs ] || invalid 'cgroup v2 is unavailable'
[ -r /sys/fs/cgroup/cgroup.controllers ] || invalid 'cgroup v2 controller list is unavailable'
controllers=$(tr '\n' ' ' </sys/fs/cgroup/cgroup.controllers)
for controller in cpu memory pids; do
	case " $controllers " in
		*" $controller "*) ;;
		*) invalid "cgroup v2 controller is unavailable: $controller" ;;
	esac
done

{
	printf 'captured_at_utc='; date -u +%Y-%m-%dT%H:%M:%SZ
	printf 'repository_root=%s\n' "$repository_root"
	printf 'source_commit='; git -C "$repository_root" rev-parse HEAD
	printf 'source_status:\n'; git -C "$repository_root" status --short
	printf 'go_version='; go version
	printf 'uname='; uname -srmo
	printf 'os_release:\n'; cat /etc/os-release
	printf 'online_cpus=%s\n' "$online_cpus"
	printf 'memtotal_kib=%s\n' "$memory_kib"
	printf 'cgroup_filesystem='; stat -fc %T /sys/fs/cgroup
	printf 'cgroup_controllers=%s\n' "$controllers"
} >"$evidence_dir/host.txt"
cp -- "$link_evidence" "$evidence_dir/link-evidence.txt"
cp -- "$host_declaration" "$evidence_dir/host-declaration.txt"
sha256sum "$evidence_dir/host.txt" "$evidence_dir/link-evidence.txt" "$evidence_dir/host-declaration.txt" >"$evidence_dir/input.sha256"
chmod 600 "$evidence_dir/host.txt" "$evidence_dir/link-evidence.txt" "$evidence_dir/host-declaration.txt" "$evidence_dir/input.sha256"

printf 'NET-01A preflight passed: %s\n' "$evidence_dir"
printf '%s\n' 'This preflight only establishes host eligibility; it does not select capacity.'
