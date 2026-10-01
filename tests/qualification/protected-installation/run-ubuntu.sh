#!/bin/sh
# Capture the public initial installation route on one admitted system manager.
# This driver does not qualify the two-Endpoint Service journey.
set -eu
umask 077
fail() { printf '%s\n' "$*" >&2; exit 1; }
[ "$#" = 5 ] || fail 'usage: run-ubuntu.sh binary binary-sha256 request installation-root new-evidence-directory'
binary=$1
digest=$2
request=$3
root=$4
evidence=$5
[ "$(id -u)" = 0 ] || fail 'invalid environment: Root required'
for program in systemctl stat uname awk sha256sum python3 timeout; do
	command -v "$program" >/dev/null || fail "invalid environment: $program unavailable"
done
. /etc/os-release
[ "$ID" = ubuntu ] && [ "$VERSION_ID" = 24.04 ] && [ "$(uname -m)" = x86_64 ] || fail 'invalid environment: Ubuntu 24.04 x86-64 required'
[ "$(systemctl --version | awk 'NR == 1 {print $2}')" = 255 ] || fail 'invalid environment: systemd 255 required'
[ "$(stat -fc %T /sys/fs/cgroup)" = cgroup2fs ] || fail 'invalid environment: cgroup v2 required'
[ -f "$binary" ] && [ ! -L "$binary" ] || fail 'invalid environment: direct program required'
printf '%s  %s\n' "$digest" "$binary" | sha256sum --check --status || fail 'invalid environment: independently declared program digest differs'
python3 - "$request" "$root" "$evidence" "$0" "$digest" <<'PY'
import json, os, pathlib, re, sys
request, root, evidence, script, digest = sys.argv[1:]
for path in (request, root, evidence):
    if not os.path.isabs(path) or os.path.normpath(path) != path:
        raise SystemExit('invalid environment: canonical absolute paths required')
if os.path.islink(request):
    raise SystemExit('invalid environment: direct request required')
with open(request, 'rb') as source:
    raw = source.read(65537)
if len(raw) > 65536 or json.loads(raw).get('installation_root') != root:
    raise SystemExit('invalid environment: bounded request/root agreement required')
# The product owns complete canonical grammar and authority validation.
# Do not copy request bytes or any Authority material into diagnostics.
if os.path.commonpath((os.path.realpath(root), os.path.realpath(evidence))) == os.path.realpath(root):
    raise SystemExit('invalid environment: evidence must be outside installation')
checkout = str(pathlib.Path(script).resolve().parents[3])
if os.path.commonpath((checkout, os.path.realpath(evidence))) == checkout:
    raise SystemExit('invalid environment: evidence must be outside repository')
revision = os.environ.get('ARDENTS_INSTALLATION_SOURCE_REVISION', 'unknown')
if revision != 'unknown' and not re.fullmatch(r'[0-9a-f]{40}', revision):
    raise SystemExit('invalid environment: source revision must be a full commit or unknown')
os.mkdir(evidence, 0o700)
with open(os.path.join(evidence, 'run-identity.json'), 'x') as record:
    json.dump({'program_sha256': digest, 'declared_source_revision': revision,
               'scope': 'public initial installation command capture; not full qualification'}, record)
    record.write('\n')
PY
capture() {
	name=$1
	shift
	set +e
	timeout 90 "$@" >"$evidence/$name.stdout" 2>"$evidence/$name.stderr"
	result=$?
	set -e
	printf '%s\n' "$result" >"$evidence/$name.exit"
	[ "$result" = 0 ] || fail "captured original failure: $name (exit $result); no retry or reset"
}
capture manager-before systemctl --system --no-pager show --property=Version
capture provision "$binary" endpoint provision "$request"
capture integrity "$binary" endpoint installation-check "$root"
capture start systemctl --system --no-ask-password --no-pager start ardents-endpoint.service
capture manager-after systemctl --system --no-pager show ardents-endpoint.service
printf '%s\n' 'Commands captured. Manager-start success is not Service readiness, containment or two-host qualification.' >"$evidence/scope.txt"
