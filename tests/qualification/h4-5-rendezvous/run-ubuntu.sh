#!/bin/sh
# H4-5 dedicated Rendezvous host eligibility capture. See README.md.
set -eu

fail_reason=''
evidence_dir=''

finish() {
	status=$?
	trap - EXIT INT TERM
	if [ -n "$evidence_dir" ] && [ -d "$evidence_dir" ]; then
		if [ "$status" -eq 0 ]; then
			printf '%s\n' 'eligible-for-h4-5-campaign; no qualification result' >"$evidence_dir/outcome.txt"
		else
			printf 'invalid-environment: %s\n' "${fail_reason:-preflight command failed}" >"$evidence_dir/outcome.txt"
		fi
		chmod 600 "$evidence_dir/outcome.txt"
	fi
	exit "$status"
}
trap finish EXIT INT TERM

invalid() {
	fail_reason=$1
	printf 'H4-5 invalid environment: %s\n' "$fail_reason" >&2
	exit 1
}

require_command() {
	command -v "$1" >/dev/null 2>&1 || invalid "required command is unavailable: $1"
}

case ${ARDENTS_H4_5_EVIDENCE_DIR:-} in
	/*) evidence_dir=$ARDENTS_H4_5_EVIDENCE_DIR ;;
	*) invalid 'ARDENTS_H4_5_EVIDENCE_DIR must be a new absolute path' ;;
esac
listen_port=${ARDENTS_H4_5_LISTEN_PORT:-}
source_commit=${ARDENTS_H4_5_SOURCE_COMMIT:-}

repository_root=''
if [ -z "$source_commit" ]; then
	repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd -P)
	case $evidence_dir in
		"$repository_root"|"$repository_root"/*) invalid 'evidence directory must stay outside the repository' ;;
	esac
else
	case $source_commit in
		*[!0-9a-f]*|'') invalid 'ARDENTS_H4_5_SOURCE_COMMIT must be one lowercase commit digest' ;;
	esac
	[ "${#source_commit}" -eq 40 ] || invalid 'ARDENTS_H4_5_SOURCE_COMMIT must be one lowercase commit digest'
fi
[ ! -e "$evidence_dir" ] || invalid 'evidence directory already exists'
umask 077
mkdir -- "$evidence_dir"

case $listen_port in
	''|*[!0-9]*) invalid 'ARDENTS_H4_5_LISTEN_PORT must be a decimal port' ;;
esac
[ "$listen_port" -ge 1024 ] && [ "$listen_port" -le 65535 ] || invalid 'listen port must be unprivileged'
[ "$(id -u)" -eq 0 ] || invalid 'preflight must run as root for the later exact lifecycle'

for command in awk cp date df findmnt getconf grep id ip journalctl sha256sum ss stat systemctl tr uname; do
	require_command "$command"
done
if [ -n "$repository_root" ]; then
	require_command git
fi

[ "$(uname -s)" = Linux ] || invalid 'host is not Linux'
[ "$(uname -m)" = x86_64 ] || invalid 'host architecture is not x86-64'
[ -r /etc/os-release ] || invalid '/etc/os-release is unavailable'
os_id=$(awk -F= '$1 == "ID" { gsub(/"/, "", $2); print $2; exit }' /etc/os-release)
os_version=$(awk -F= '$1 == "VERSION_ID" { gsub(/"/, "", $2); print $2; exit }' /etc/os-release)
[ "$os_id" = ubuntu ] || invalid "host is not Ubuntu: $os_id"
case $os_version in
	20.04|22.04|24.04|26.04) ;;
	*) invalid "Ubuntu release is not a declared LTS version: $os_version" ;;
esac

[ "$(cat /proc/1/comm)" = systemd ] || invalid 'systemd is not PID 1'
system_state=$(systemctl is-system-running 2>/dev/null || true)
[ "$system_state" = running ] || invalid "systemd is not fully running: $system_state"

online_cpus=$(getconf _NPROCESSORS_ONLN)
memory_kib=$(awk '$1 == "MemTotal:" { print $2; exit }' /proc/meminfo)
case $memory_kib in
	''|*[!0-9]*) invalid 'MemTotal observation is invalid' ;;
esac

[ "$(stat -fc %T /sys/fs/cgroup)" = cgroup2fs ] || invalid 'cgroup v2 is unavailable'
[ -r /sys/fs/cgroup/cgroup.controllers ] || invalid 'cgroup v2 controller list is unavailable'
controllers=$(tr '\n' ' ' </sys/fs/cgroup/cgroup.controllers)
for controller in cpu memory pids; do
	case " $controllers " in
		*" $controller "*) ;;
		*) invalid "cgroup v2 controller is unavailable: $controller" ;;
	esac
done

for path in \
	/usr/lib/ardents-contributor \
	/var/lib/ardents-contributor \
	/var/lib/private/ardents-contributor \
	/etc/systemd/system/ardents-rendezvous-contributor.service; do
	[ ! -e "$path" ] || invalid "candidate host is not fresh; managed path exists: $path"
done
if systemctl list-unit-files ardents-rendezvous-contributor.service --no-legend 2>/dev/null | grep . >/dev/null 2>&1; then
	invalid 'candidate host already knows the Contributor unit'
fi
if ss -H -ltn | awk -v suffix=":$listen_port" '$4 ~ suffix "$" { found=1 } END { exit found ? 0 : 1 }'; then
	invalid "candidate listen port is already in use: $listen_port"
fi

if [ -n "$repository_root" ]; then
	git -C "$repository_root" diff --quiet || invalid 'source worktree has tracked changes'
	git -C "$repository_root" diff --cached --quiet || invalid 'source index has staged changes'
	[ -z "$(git -C "$repository_root" status --short)" ] || invalid 'source worktree is not clean'
	source_commit=$(git -C "$repository_root" rev-parse HEAD)
fi

{
	printf 'schema=ardents-h4-5-host-preflight-v1\n'
	printf 'captured_at_utc='; date -u +%Y-%m-%dT%H:%M:%SZ
	printf 'source_commit=%s\n' "$source_commit"
	printf 'source_status=clean\n'
	printf 'uname='; uname -srmo
	printf 'online_cpus=%s\n' "$online_cpus"
	printf 'memtotal_kib=%s\n' "$memory_kib"
	printf 'systemd_state=%s\n' "$system_state"
	printf 'systemd_version='; systemctl --version | awk 'NR == 1 { print; exit }'
	printf 'cgroup_filesystem='; stat -fc %T /sys/fs/cgroup
	printf 'cgroup_controllers=%s\n' "$controllers"
	printf 'candidate_listen_port=%s\n' "$listen_port"
	printf 'os_release:\n'; cat /etc/os-release
} >"$evidence_dir/host.txt"

{
	printf 'schema=ardents-h4-5-host-observation-v1\n'
	printf 'captured_at_utc='; date -u +%Y-%m-%dT%H:%M:%SZ
	printf 'addresses:\n'; ip -details -statistics address show
	printf 'routes:\n'; ip route show table all
	printf 'links:\n'; ip -details -statistics link show
	printf 'listening_tcp:\n'; ss -H -ltnp
	printf 'filesystems:\n'; df -B1 -P / /var /usr
	printf 'root_mount:\n'; findmnt -T / -o TARGET,SOURCE,FSTYPE,OPTIONS
	printf 'var_mount:\n'; findmnt -T /var -o TARGET,SOURCE,FSTYPE,OPTIONS
	printf 'journal_usage:\n'; journalctl --disk-usage
	printf 'running_services:\n'; systemctl list-units --type=service --state=running --no-pager --plain
} >"$evidence_dir/host-observation.txt"

cp -- "$0" "$evidence_dir/run-ubuntu.sh"
sha256sum \
	"$evidence_dir/host.txt" \
	"$evidence_dir/host-observation.txt" \
	"$evidence_dir/run-ubuntu.sh" >"$evidence_dir/input.sha256"
chmod 600 "$evidence_dir/host.txt" "$evidence_dir/host-observation.txt" \
	"$evidence_dir/run-ubuntu.sh" "$evidence_dir/input.sha256"

printf 'H4-5 host preflight passed: %s\n' "$evidence_dir"
printf '%s\n' 'This proves host eligibility only; it is not an H4-5 qualification result.'
