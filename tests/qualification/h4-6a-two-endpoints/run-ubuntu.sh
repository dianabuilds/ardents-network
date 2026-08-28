#!/bin/sh
set -eu

fresh_release_decision_line() {
    observed=$1
    cohort=$2
    release=$3
    [ "$observed" = "{\"kind\":\"release-decision\",\"outcome\":\"release-accepted\",\"cohort\":\"$cohort\",\"release\":\"$release\"}" ]
}

report_has_fresh_release() {
    observed=$1
    release_outcome=$(printf '%s\n' "$observed" | sed -n 's/.*"release":"\([^"]*\)".*/\1/p')
    [ "$release_outcome" = release-accepted ]
}

lower_sha256() {
    value=$1
    [ "${#value}" -eq 64 ] || return 1
    case "$value" in *[!0-9a-f]*|'') return 1 ;; esac
}

if [ "$#" -eq 1 ] && [ "$1" = --self-test ]; then
    exact='{"kind":"release-decision","outcome":"release-accepted","cohort":"cohort-1","release":"alpha-1"}'
    cached='{"kind":"release-decision","outcome":"no-update","cohort":"cohort-1","release":"alpha-1"}'
    wrong='{"kind":"release-decision","outcome":"release-accepted","cohort":"other","release":"alpha-1"}'
    fresh_release_decision_line "$exact" cohort-1 alpha-1 || exit 1
    if fresh_release_decision_line "$cached" cohort-1 alpha-1 || fresh_release_decision_line "$wrong" cohort-1 alpha-1; then exit 1; fi
    report_has_fresh_release '{"release":"release-accepted"}' || exit 1
    if report_has_fresh_release '{"release":"no-update"}'; then exit 1; fi
    lower_sha256 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef || exit 1
    if lower_sha256 0123456789ABCDEF0123456789abcdef0123456789abcdef0123456789abcdef; then exit 1; fi
    printf 'h4-6a-run-ubuntu-self-test=accepted\n'
    exit 0
fi

if [ "$#" -ne 9 ]; then
    echo 'usage: run-ubuntu.sh ROOT ARCHIVE_SHA256 MANIFEST_PIN ENDPOINT_SHA256 CONTROL_SHA256 COHORT RELEASE AT ARCHIVE_NAME' >&2
    exit 2
fi

root=$1
expected_archive_sha256=$2
expected_manifest_pin=$3
expected_endpoint_sha256=$4
expected_control_sha256=$5
expected_cohort=$6
expected_release=$7
decision_at=$8
archive_name=$9
evidence=$root/ubuntu
work=$root/work
archive=$root/$archive_name
endpoint_a_pid=
endpoint_b_pid=
endpoint_a_socket=
endpoint_b_socket=

die() {
    echo "H4-6A qualifier: $*" >&2
    exit 1
}

process_is_running() {
    pid=$1
    [ -r "/proc/$pid/stat" ] || return 1
    state=$(awk '{print $3}' "/proc/$pid/stat" 2>/dev/null || true)
    [ "$state" != Z ] && [ -n "$state" ]
}

cleanup_process() {
    name=$1
    pid=$2
    socket=$3
    [ -n "$pid" ] || {
        [ -z "$socket" ] || [ ! -e "$socket" ]
        return
    }
    if process_is_running "$pid"; then
        echo "$name cleanup sent SIGTERM to pid $pid" >>"$evidence/cleanup.log"
        kill -TERM "$pid" 2>/dev/null || true
        count=0
        while process_is_running "$pid" && [ "$count" -lt 50 ]; do
            sleep 0.1
            count=$((count + 1))
        done
    fi
    if process_is_running "$pid"; then
        echo "$name cleanup required SIGKILL for pid $pid" >>"$evidence/cleanup.log"
        kill -KILL "$pid" 2>/dev/null || true
        count=0
        while process_is_running "$pid" && [ "$count" -lt 50 ]; do
            sleep 0.1
            count=$((count + 1))
        done
    fi
    if process_is_running "$pid"; then
        echo "$name cleanup left live pid $pid after SIGKILL" >>"$evidence/cleanup.log"
        return 1
    fi
    set +e
    wait "$pid" 2>/dev/null
    code=$?
    set -e
    printf '%s cleanup_wait_exit=%s\n' "$name" "$code" >>"$evidence/cleanup.log"
    if [ -n "$socket" ] && [ -e "$socket" ]; then
        echo "$name cleanup left socket $socket" >>"$evidence/cleanup.log"
        return 1
    fi
    return 0
}

finish() {
    original=$?
    trap - EXIT HUP INT TERM
    final=$original
    cleanup_process endpoint-a "$endpoint_a_pid" "$endpoint_a_socket" || final=1
    cleanup_process endpoint-b "$endpoint_b_pid" "$endpoint_b_socket" || final=1
    {
        printf 'endpoint_a_pid=%s running=%s socket=%s socket_present=%s\n' \
            "${endpoint_a_pid:-none}" "$(if [ -n "$endpoint_a_pid" ] && process_is_running "$endpoint_a_pid"; then echo yes; else echo no; fi)" \
            "${endpoint_a_socket:-unknown}" "$(if [ -n "$endpoint_a_socket" ] && [ -e "$endpoint_a_socket" ]; then echo yes; else echo no; fi)"
        printf 'endpoint_b_pid=%s running=%s socket=%s socket_present=%s\n' \
            "${endpoint_b_pid:-none}" "$(if [ -n "$endpoint_b_pid" ] && process_is_running "$endpoint_b_pid"; then echo yes; else echo no; fi)" \
            "${endpoint_b_socket:-unknown}" "$(if [ -n "$endpoint_b_socket" ] && [ -e "$endpoint_b_socket" ]; then echo yes; else echo no; fi)"
    } >"$evidence/process-residue.txt" 2>&1 || final=1
    if { [ -n "$endpoint_a_pid" ] && process_is_running "$endpoint_a_pid"; } ||
        { [ -n "$endpoint_b_pid" ] && process_is_running "$endpoint_b_pid"; } ||
        { [ -n "$endpoint_a_socket" ] && [ -e "$endpoint_a_socket" ]; } ||
        { [ -n "$endpoint_b_socket" ] && [ -e "$endpoint_b_socket" ]; }; then
        final=1
    fi
    if [ -d "$work" ]; then
        find "$work" -mindepth 1 -maxdepth 5 -printf '%y %m %U:%G %s %p\n' 2>/dev/null | LC_ALL=C sort >"$evidence/work-tree-residue.txt" || final=1
    fi
    printf '%s\n' "$final" >"$evidence/remote-exit-code.txt"
    if [ "$final" -eq 0 ]; then
        printf 'accepted\n' >"$evidence/outcome.txt"
    else
        printf 'failed\n' >"$evidence/outcome.txt"
    fi
    exit "$final"
}

trap finish EXIT
trap 'exit 130' HUP INT TERM

case "$root" in
    /tmp/ardents-h4-6a-two-endpoints-[0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]) ;;
    *) die 'remote root is outside the exact owned namespace' ;;
esac
[ "$(id -u)" -eq 0 ] || die 'remote orchestration requires UID 0'
[ -d "$root" ] && [ -d "$evidence" ] && [ ! -e "$work" ] || die 'remote root is ambiguous or already used'
case "$archive_name" in
    candidate.tar.gz) ;;
    *) die 'archive name is not the fixed staged name' ;;
esac
for value in "$expected_archive_sha256" "$expected_manifest_pin" "$expected_endpoint_sha256" "$expected_control_sha256"; do
    case "$value" in
        *[!0-9a-f]*|'') die 'expected digest is not lowercase hexadecimal' ;;
    esac
    [ "${#value}" -eq 64 ] || die 'expected digest is not SHA-256 length'
done
case "$expected_cohort" in *[!A-Za-z0-9._-]*|'') die 'cohort is unsafe' ;; esac
case "$expected_release" in *[!A-Za-z0-9._-]*|'') die 'release is unsafe' ;; esac
case "$decision_at" in
    [0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z) ;;
    *) die 'decision time is not the fixed UTC RFC3339-second form' ;;
esac

command -v sha256sum >/dev/null || die 'sha256sum is unavailable'
command -v tar >/dev/null || die 'tar is unavailable'
command -v setpriv >/dev/null || die 'setpriv is unavailable'
command -v awk >/dev/null || die 'awk is unavailable'
command -v sed >/dev/null || die 'sed is unavailable'
command -v grep >/dev/null || die 'grep is unavailable'
command -v cmp >/dev/null || die 'cmp is unavailable'
command -v readlink >/dev/null || die 'readlink is unavailable'

{
    printf 'captured_at_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    printf 'hostname=%s\n' "$(hostname)"
    uname -srmo
    printf 'machine=%s\n' "$(uname -m)"
    printf 'online_processors=%s\n' "$(getconf _NPROCESSORS_ONLN)"
    awk '/MemTotal|MemAvailable/ {print $1 "=" $2 " " $3}' /proc/meminfo
    printf 'orchestrator_uid=%s gid=%s\n' "$(id -u)" "$(id -g)"
    printf 'setpriv=%s\n' "$(setpriv --version | sed -n '1p')"
    printf 'tar=%s\n' "$(tar --version | sed -n '1p')"
    printf 'sha256sum=%s\n' "$(sha256sum --version | sed -n '1p')"
    printf '%s\n' '--- /etc/os-release ---'
    sed -n '1,32p' /etc/os-release
} >"$evidence/host-envelope.txt"

. /etc/os-release
[ "${ID:-}" = ubuntu ] && [ "${VERSION_ID:-}" = 22.04 ] || die 'selected remote host is not Ubuntu 22.04'
[ "$(uname -m)" = x86_64 ] || die 'selected remote host is not x86_64'
[ -f "$archive" ] && [ ! -L "$archive" ] || die 'staged archive is not a regular file'
actual_archive_sha256=$(sha256sum "$archive" | awk '{print $1}')
printf '%s  %s\n' "$actual_archive_sha256" "$archive_name" >"$evidence/archive-sha256.txt"
[ "$actual_archive_sha256" = "$expected_archive_sha256" ] || die 'remote archive digest differs from the approved input'

mkdir -m 700 "$work"
tar -tzf "$archive" >"$evidence/archive-inventory.txt" || die 'archive inventory cannot be read'
[ -s "$evidence/archive-inventory.txt" ] || die 'archive inventory is empty'
normalized=$work/archive-inventory.normalized
while IFS= read -r entry || [ -n "$entry" ]; do
    case "$entry" in
        ./*) entry=${entry#./} ;;
    esac
    case "$entry" in
        ''|/*|*\\*|../*|*/../*|*/..|*//* ) die 'archive inventory contains an unsafe path' ;;
    esac
    trimmed=${entry%/}
    case "$trimmed" in
        */*/*) die 'archive inventory has an unexpected nested path' ;;
    esac
    printf '%s\n' "$trimmed" >>"$normalized"
done <"$evidence/archive-inventory.txt"
[ -z "$(LC_ALL=C sort "$normalized" | uniq -d)" ] || die 'archive inventory contains duplicate paths'
top=$(sed 's#^\./##; s#/$##; s#/.*##' "$evidence/archive-inventory.txt" | LC_ALL=C sort -u)
[ "$(printf '%s\n' "$top" | awk 'NF {count++} END {print count+0}')" -eq 1 ] || die 'archive does not have one exact top-level directory'
case "$top" in *[!A-Za-z0-9._-]*|'') die 'archive top-level directory is unsafe' ;; esac
tar -xzf "$archive" --no-same-owner --no-same-permissions -C "$work" || die 'archive extraction failed'
bundle=$work/$top
[ -d "$bundle" ] && [ ! -L "$bundle" ] || die 'extracted bundle root is invalid'
[ "$(find "$work" -mindepth 1 -maxdepth 1 | wc -l)" -eq 1 ] || die 'archive extracted more than one top-level entry'
if find "$bundle" -mindepth 1 -maxdepth 1 ! -type f -print -quit | grep -q .; then
    die 'bundle has a non-regular top-level entry'
fi

manifest=$bundle/SHA256SUMS
[ -f "$manifest" ] && [ ! -L "$manifest" ] || die 'bundle manifest is not a regular file'
actual_manifest_pin=$(sha256sum "$manifest" | awk '{print $1}')
printf '%s  SHA256SUMS\n' "$actual_manifest_pin" >"$evidence/manifest-pin.txt"
[ "$actual_manifest_pin" = "$expected_manifest_pin" ] || die 'bundle manifest differs from the Enrollment Pin'
expected_names=$work/expected-names.txt
actual_names=$work/actual-names.txt
: >"$expected_names"
while IFS= read -r line || [ -n "$line" ]; do
    digest=${line%% *}
    [ "${#digest}" -eq 64 ] || die 'manifest has a non-canonical digest'
    case "$digest" in *[!0-9a-f]*|'') die 'manifest has a non-canonical digest' ;; esac
    prefix="$digest  "
    name=${line#"$prefix"}
    [ "$name" != "$line" ] || die 'manifest separator is not canonical'
    case "$name" in *[!A-Za-z0-9._-]*|'') die 'manifest filename is not a simple top-level name' ;; esac
    printf '%s\n' "$name" >>"$expected_names"
done <"$manifest"
printf '%s\n' SHA256SUMS >>"$expected_names"
LC_ALL=C sort -o "$expected_names" "$expected_names"
[ -z "$(uniq -d "$expected_names")" ] || die 'manifest inventory contains duplicates'
find "$bundle" -mindepth 1 -maxdepth 1 -type f -printf '%f\n' | LC_ALL=C sort >"$actual_names"
cmp -s "$expected_names" "$actual_names" || die 'manifest and bundle inventories differ'
(cd "$bundle" && sha256sum --strict --check SHA256SUMS) >"$evidence/manifest-check.txt" 2>&1 || die 'a manifested bundle byte failed verification'
cp "$manifest" "$evidence/SHA256SUMS"
cp "$bundle/RELEASE" "$evidence/RELEASE"

descriptor=$bundle/RELEASE
field() {
    key=$1
    matches=$(sed -n "s/^${key}=//p" "$descriptor")
    [ "$(printf '%s\n' "$matches" | awk 'NF {count++} END {print count+0}')" -eq 1 ] || die "RELEASE field $key is absent or duplicated"
    printf '%s' "$matches"
}
schema=$(field schema)
cohort=$(field cohort)
release=$(field release)
platform=$(field platform)
environment=$(field environment)
network=$(field network)
target_path=$(field target_path)
endpoint_name=$(field artifact)
control_name=$(field control_artifact)
catalog_name=$(field control_catalog)
release_component_name=$(field control_release)
network_component_name=$(field control_network)
compatibility_component_name=$(field control_compatibility)
release_root_name=$(field control_release_root)
network_root_name=$(field control_network_root)
compatibility_root_name=$(field control_compatibility_root)
[ "$schema" = ardents-closed-alpha-enrollment-v3 ] || die 'RELEASE schema is not the selected enrollment v3'
[ "$cohort" = "$expected_cohort" ] || die 'RELEASE cohort differs from the approved input'
[ "$release" = "$expected_release" ] || die 'RELEASE release differs from the approved input'
[ "$platform" = linux-amd64 ] || die 'RELEASE platform is not linux-amd64'
[ "$target_path" = ardents/linux-amd64/endpoint ] || die 'RELEASE target path is not the selected Endpoint target'
[ "$endpoint_name" = ardents-linux-amd64 ] || die 'RELEASE Endpoint artifact name is unexpected'
[ "$control_name" = ardents-control-linux-amd64 ] || die 'RELEASE control artifact name is unexpected'
[ "$catalog_name" = catalog.ac1 ] || die 'RELEASE control catalog name is unexpected'
[ "$release_component_name" = release.ac1 ] && [ "$network_component_name" = network.ac1 ] && \
    [ "$compatibility_component_name" = compatibility.ac1 ] || die 'RELEASE component names are unexpected'
[ "$release_root_name" = release.pub ] && [ "$network_root_name" = network.pub ] && \
    [ "$compatibility_root_name" = compatibility.pub ] || die 'RELEASE component-root names are unexpected'
case "$environment" in *[!A-Za-z0-9._-]*|'') die 'RELEASE environment is unsafe' ;; esac
case "$network" in *[!A-Za-z0-9._-]*|'') die 'RELEASE network is unsafe' ;; esac
actual_endpoint_sha256=$(sha256sum "$bundle/$endpoint_name" | awk '{print $1}')
actual_control_sha256=$(sha256sum "$bundle/$control_name" | awk '{print $1}')
[ "$actual_endpoint_sha256" = "$expected_endpoint_sha256" ] || die 'Endpoint digest differs from the approved input'
[ "$actual_control_sha256" = "$expected_control_sha256" ] || die 'control companion digest differs from the approved input'
printf '%s  %s\n%s  %s\n' "$actual_endpoint_sha256" "$endpoint_name" "$actual_control_sha256" "$control_name" >"$evidence/product-sha256.txt"
catalog_identity=$(sha256sum "$bundle/$catalog_name" | awk '{print $1}')
release_root_id=$(sha256sum "$bundle/$release_root_name" | awk '{print $1}')
network_root_id=$(sha256sum "$bundle/$network_root_name" | awk '{print $1}')
compatibility_root_id=$(sha256sum "$bundle/$compatibility_root_name" | awk '{print $1}')
release_component_digest=$(sha256sum "$bundle/$release_component_name" | awk '{print $1}')
network_component_digest=$(sha256sum "$bundle/$network_component_name" | awk '{print $1}')
compatibility_component_digest=$(sha256sum "$bundle/$compatibility_component_name" | awk '{print $1}')
printf 'catalog_identity=%s\nclass_1_root_id=%s\nclass_1_digest=%s\nclass_2_root_id=%s\nclass_2_digest=%s\nclass_3_root_id=%s\nclass_3_digest=%s\nrelease_identity=%s\n' \
    "$catalog_identity" "$release_root_id" "$release_component_digest" \
    "$network_root_id" "$network_component_digest" "$compatibility_root_id" \
    "$compatibility_component_digest" "$expected_release" >"$evidence/expected-control-identities.txt"

make_endpoint_root() {
    name=$1
    destination=$work/$name
    mkdir -m 700 "$destination"
    cp -a "$bundle" "$destination/bundle"
    mkdir -m 700 "$destination/home" "$destination/config" "$destination/state" "$destination/cache" "$destination/runtime" "$destination/tmp"
    input=$destination/alpha-enrollment.json
    printf '{"schema":"ardents-alpha-enrollment-input-v1","bundle_root":"%s","cohort":"%s","release":"%s","platform":"linux-amd64","manifest_sha256":"%s","environment":"%s","network":"%s","target_path":"ardents/linux-amd64/endpoint"}\n' \
        "$destination/bundle" "$cohort" "$release" "$actual_manifest_pin" "$environment" "$network" >"$input"
    chmod 600 "$input"
    chmod 700 "$destination/bundle/$endpoint_name" "$destination/bundle/$control_name"
    chown -R 65534:65534 "$destination"
}

make_endpoint_root endpoint-a
make_endpoint_root endpoint-b
chmod 711 "$root" "$work"
: >"$evidence/freshness-preflight.txt"
for name in endpoint-a endpoint-b; do
    endpoint_floor=$work/$name/state/ardents/floors/release-decision
    inspection_root=$work/$name/control-inspection
    [ ! -e "$endpoint_floor" ] || die "$name Release floor existed before first Endpoint observation"
    [ ! -e "$inspection_root" ] || die "$name control inspection root existed before first control observation"
    printf '%s_endpoint_release_floor=absent\n%s_control_inspection_root=absent\n' \
        "$name" "$name" >>"$evidence/freshness-preflight.txt"
done
for name in endpoint-a endpoint-b; do
    copy=$work/$name/bundle
    (cd "$copy" && sha256sum --strict --check SHA256SUMS) >"$evidence/$name-bundle-check.txt" 2>&1 || die "$name bundle copy differs from its manifest"
    find "$copy" -mindepth 1 -maxdepth 1 -type f -printf '%f\n' | LC_ALL=C sort >"$work/$name.names"
done
cmp -s "$work/endpoint-a.names" "$work/endpoint-b.names" || die 'Endpoint bundle copy inventories differ'
while IFS= read -r name; do
    cmp -s "$work/endpoint-a/bundle/$name" "$work/endpoint-b/bundle/$name" || die "Endpoint bundle copies differ at $name"
done <"$work/endpoint-a.names"
(cd "$work/endpoint-a/bundle" && sha256sum $(cat "$work/endpoint-a.names")) >"$evidence/endpoint-a-bundle-sha256.txt"
(cd "$work/endpoint-b/bundle" && sha256sum $(cat "$work/endpoint-b.names")) >"$evidence/endpoint-b-bundle-sha256.txt"
cmp -s "$evidence/endpoint-a-bundle-sha256.txt" "$evidence/endpoint-b-bundle-sha256.txt" || die 'Endpoint bundle copy digest inventories differ'
printf 'cohort=%s\nrelease=%s\nplatform=linux-amd64\nmanifest_sha256=%s\nenvironment=%s\nnetwork=%s\ntarget_path=ardents/linux-amd64/endpoint\ndecision_at=%s\n' \
    "$cohort" "$release" "$actual_manifest_pin" "$environment" "$network" "$decision_at" >"$evidence/identical-authenticated-statements.txt"

start_endpoint() {
    name=$1
    destination=$work/$name
    env -i PATH=/usr/bin:/bin HOME="$destination/home" TMPDIR="$destination/tmp" \
        XDG_CONFIG_HOME="$destination/config" XDG_STATE_HOME="$destination/state" \
        XDG_CACHE_HOME="$destination/cache" XDG_RUNTIME_DIR="$destination/runtime" \
        LANG=C LC_ALL=C setpriv --reuid=65534 --regid=65534 --clear-groups \
        --no-new-privs --bounding-set=-all --inh-caps=-all --ambient-caps=-all \
        "$destination/bundle/$endpoint_name" endpoint enroll "$destination/alpha-enrollment.json" \
        >"$evidence/$name-lifecycle.stdout.log" 2>"$evidence/$name-lifecycle.stderr.log" &
    printf '%s\n' "$!" >"$work/$name.pid"
}

start_endpoint endpoint-a
endpoint_a_pid=$(cat "$work/endpoint-a.pid")
start_endpoint endpoint-b
endpoint_b_pid=$(cat "$work/endpoint-b.pid")
[ "$endpoint_a_pid" != "$endpoint_b_pid" ] || die 'fresh Endpoints unexpectedly share one process ID'
process_is_running "$endpoint_a_pid" || die 'endpoint-a exited before both fresh Endpoints were started'
process_is_running "$endpoint_b_pid" || die 'endpoint-b exited before both fresh Endpoints were started'
printf '%s\n' "$endpoint_a_pid" >"$evidence/endpoint-a.pid"
printf '%s\n' "$endpoint_b_pid" >"$evidence/endpoint-b.pid"

wait_ready() {
    name=$1
    pid=$2
    output=$evidence/$name-lifecycle.stdout.log
    count=0
    while [ "$count" -lt 300 ]; do
        process_is_running "$pid" || die "$name exited before readiness"
        lines=$(awk 'END {print NR+0}' "$output")
        [ "$lines" -ge 3 ] && break
        sleep 0.1
        count=$((count + 1))
    done
    [ "$(awk 'END {print NR+0}' "$output")" -ge 3 ] || die "$name did not reach readiness within 30 seconds"
    first=$(sed -n '1p' "$output")
    second=$(sed -n '2p' "$output")
    third=$(sed -n '3p' "$output")
    printf '%s' "$first" | grep -F '"kind":"endpoint-lifecycle"' >/dev/null && printf '%s' "$first" | grep -F '"state":"starting"' >/dev/null || die "$name first event is not starting"
    fresh_release_decision_line "$second" "$expected_cohort" "$expected_release" || \
        die "$name second event is not the exact fresh release-accepted decision for the selected cohort/release"
    printf '%s' "$third" | grep -F '"kind":"endpoint-lifecycle"' >/dev/null && printf '%s' "$third" | grep -F '"state":"ready"' >/dev/null || die "$name third event is not ready"
    socket=$(printf '%s\n' "$third" | sed -n 's/.*"attachment":"\([^"]*\)".*/\1/p')
    case "$socket" in "$work/$name/runtime/"*) ;; *) die "$name ready socket is outside its fresh runtime root" ;; esac
    [ -S "$socket" ] || die "$name ready attachment is not a live socket"
    printf '%s' "$socket"
}

endpoint_a_socket=$(wait_ready endpoint-a "$endpoint_a_pid")
endpoint_b_socket=$(wait_ready endpoint-b "$endpoint_b_pid")
[ "$endpoint_a_socket" != "$endpoint_b_socket" ] || die 'fresh Endpoints unexpectedly share one socket'
endpoint_a_release=$(sed -n '2p' "$evidence/endpoint-a-lifecycle.stdout.log" | sed -n 's/.*"outcome":"\([^"]*\)".*/\1/p')
endpoint_b_release=$(sed -n '2p' "$evidence/endpoint-b-lifecycle.stdout.log" | sed -n 's/.*"outcome":"\([^"]*\)".*/\1/p')
[ "$endpoint_a_release" = release-accepted ] || die 'fresh endpoint-a did not report release-accepted'
[ "$endpoint_b_release" = release-accepted ] || die 'fresh endpoint-b did not report release-accepted'
[ "$endpoint_a_release" = "$endpoint_b_release" ] || die 'fresh Endpoint lifecycle Release outcomes differ'
printf 'endpoint-a=%s\nendpoint-b=%s\n' "$endpoint_a_release" "$endpoint_b_release" >"$evidence/lifecycle-release-outcomes.txt"

record_release_floor_inventory() {
    name=$1
    floor_root=$2
    destination=$3
    [ -d "$floor_root" ] && [ ! -L "$floor_root" ] || die "$name Release floor root is not a real directory"
    if find "$floor_root" -mindepth 1 ! -type f ! -type d -print -quit | grep -q .; then
        die "$name Release floor contains a non-regular entry"
    fi
    actual_top=$work/$name-release-floor-top.actual
    expected_top=$work/$name-release-floor-top.expected
    find "$floor_root" -mindepth 1 -maxdepth 1 -printf '%f\n' | LC_ALL=C sort >"$actual_top"
    printf '%s\n' .ardents-release-decision-v1 current generations | LC_ALL=C sort >"$expected_top"
    cmp -s "$expected_top" "$actual_top" || die "$name Release floor top-level inventory is not canonical"
    marker_expected=$work/$name-release-floor-marker.expected
    printf 'ardents-release-decision-v1\n' >"$marker_expected"
    cmp -s "$marker_expected" "$floor_root/.ardents-release-decision-v1" || die "$name Release floor marker is invalid"
    [ "$(awk 'END {print NR+0}' "$floor_root/current")" -eq 1 ] || die "$name Release floor pointer is not one line"
    generation=$(sed -n '1p' "$floor_root/current")
    lower_sha256 "$generation" || die "$name Release floor pointer is not one canonical generation"
    [ -d "$floor_root/generations/$generation" ] && [ ! -L "$floor_root/generations/$generation" ] || \
        die "$name Release floor pointer does not name a retained generation"
    generation_names=$work/$name-release-floor-generations.txt
    find "$floor_root/generations" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' | LC_ALL=C sort >"$generation_names"
    [ -s "$generation_names" ] || die "$name Release floor has no retained generation"
    if find "$floor_root/generations" -mindepth 1 -maxdepth 1 ! -type d -print -quit | grep -q .; then
        die "$name Release floor generations contain a non-directory entry"
    fi
    while IFS= read -r generation_name; do
        lower_sha256 "$generation_name" || die "$name retained Release generation name is not canonical"
        generation_root=$floor_root/generations/$generation_name
        generation_actual=$work/$name-$generation_name.actual
        generation_expected=$work/$name-$generation_name.expected
        find "$generation_root" -mindepth 1 -maxdepth 1 -printf '%f\n' | LC_ALL=C sort >"$generation_actual"
        printf 'roots\nstate.bin\n' >"$generation_expected"
        cmp -s "$generation_expected" "$generation_actual" || die "$name retained Release generation inventory is not canonical"
        [ -s "$generation_root/state.bin" ] && [ -f "$generation_root/state.bin" ] && [ ! -L "$generation_root/state.bin" ] || \
            die "$name retained Release generation state is invalid"
        roots=$generation_root/roots
        [ -d "$roots" ] && [ ! -L "$roots" ] || die "$name retained Release root archive is invalid"
        if find "$roots" -mindepth 1 -maxdepth 1 ! -type f -print -quit | grep -q .; then
            die "$name retained Release root archive contains a non-file entry"
        fi
        [ "$(find "$roots" -mindepth 1 -maxdepth 1 -type f | awk 'END {print NR+0}')" -gt 0 ] || \
            die "$name retained Release root archive is empty"
        find "$roots" -mindepth 1 -maxdepth 1 -type f -printf '%f\n' | while IFS= read -r archived; do
            version=${archived%.root.json}
            [ "$version" != "$archived" ] || die "$name archived Release root filename is invalid"
            case "$version" in *[!0-9]*|'') die "$name archived Release root version is invalid" ;; esac
            [ "$version" -gt 0 ] || die "$name archived Release root version is not positive"
        done
    done <"$generation_names"
    (
        cd "$floor_root"
        find . -mindepth 1 -type d -printf 'd  %P\n'
        find . -mindepth 1 -type f -printf '%P\n' | LC_ALL=C sort | while IFS= read -r relative; do
            digest=$(sha256sum "$relative" | awk '{print $1}')
            printf 'f %s  %s\n' "$digest" "$relative"
        done
    ) | LC_ALL=C sort >"$destination"
    sha256sum "$floor_root/current" >"$evidence/$name-release-floor-sha256.txt"
}

for name in endpoint-a endpoint-b; do
    record_release_floor_inventory "$name" "$work/$name/state/ardents/floors/release-decision" \
        "$evidence/$name-release-floor-inventory.txt"
done
cmp -s "$evidence/endpoint-a-release-floor-inventory.txt" "$evidence/endpoint-b-release-floor-inventory.txt" || \
    die 'fresh Endpoint Release floor inventories differ'
for pair in "endpoint-a:$endpoint_a_pid" "endpoint-b:$endpoint_b_pid"; do
    name=${pair%%:*}
    pid=${pair#*:}
    executable=$(readlink -f "/proc/$pid/exe")
    [ "$executable" = "$work/$name/bundle/$endpoint_name" ] || die "$name process is not the exact separate manifested Endpoint copy"
    printf '%s\n' "$executable" >"$evidence/$name-executable.txt"
    awk '/^(Name|State|Pid|PPid|Uid|Gid|Threads|CapInh|CapPrm|CapEff|CapBnd|CapAmb|NoNewPrivs):/ {print}' "/proc/$pid/status" >"$evidence/$name-process-status.txt"
    awk '$1 == "Uid:" {ok = ($2 == 65534 && $3 == 65534 && $4 == 65534 && $5 == 65534)} END {exit !ok}' "$evidence/$name-process-status.txt" || die "$name did not run as UID 65534"
    awk '$1 == "Gid:" {ok = ($2 == 65534 && $3 == 65534 && $4 == 65534 && $5 == 65534)} END {exit !ok}' "$evidence/$name-process-status.txt" || die "$name did not run as GID 65534"
    awk '$1 == "NoNewPrivs:" {ok = ($2 == 1)} END {exit !ok}' "$evidence/$name-process-status.txt" || die "$name lacks no_new_privs"
    awk '$1 == "CapEff:" {ok = ($2 == "0000000000000000")} END {exit !ok}' "$evidence/$name-process-status.txt" || die "$name retained effective capabilities"
done

run_control() {
    name=$1
    destination=$work/$name
    state_root=$destination/control-inspection
    [ ! -e "$state_root" ] || die "$name control inspection root is not fresh"
    set +e
    env -i PATH=/usr/bin:/bin HOME="$destination/home" TMPDIR="$destination/tmp" LANG=C LC_ALL=C \
        setpriv --reuid=65534 --regid=65534 --clear-groups --no-new-privs \
        --bounding-set=-all --inh-caps=-all --ambient-caps=-all \
        "$destination/bundle/$control_name" inspect-bundle \
        --enrollment "$destination/alpha-enrollment.json" \
        --artifact "$destination/bundle/$endpoint_name" \
        --state-root "$state_root" --at "$decision_at" \
        >"$evidence/$name-control.stdout.json" 2>"$evidence/$name-control.stderr.log"
    code=$?
    set -e
    printf '%s\n' "$code" >"$evidence/$name-control.exitcode"
    [ "$code" -eq 0 ] || die "$name control inspection failed"
}

run_control endpoint-a
run_control endpoint-b

validate_report() {
    name=$1
    report=$evidence/$name-control.stdout.json
    [ "$(awk 'END {print NR+0}' "$report")" -eq 1 ] || die "$name control report is not exactly one JSON line"
    grep -F '"schema":"ardents-alpha-control-report-v1"' "$report" >/dev/null || die "$name control report schema is absent"
    grep -F '"catalog":"accepted"' "$report" >/dev/null || die "$name catalog is not accepted"
    grep -F '"catalog_cohort":"'"$cohort"'"' "$report" >/dev/null || die "$name catalog cohort differs"
    grep -F '"catalog_identity":"'"$catalog_identity"'"' "$report" >/dev/null || die "$name catalog identity differs from catalog.ac1"
    grep -E '"catalog_generation":[1-9][0-9]*' "$report" >/dev/null || die "$name catalog generation is absent"
    grep -E '"catalog_not_before":"[^"]+"' "$report" >/dev/null || die "$name catalog not-before is absent"
    grep -E '"catalog_not_after":"[^"]+"' "$report" >/dev/null || die "$name catalog not-after is absent"
    grep -E '"class":1,"outcome":"accepted","root_id":"'"$release_root_id"'","generation":[1-9][0-9]*,"digest":"'"$release_component_digest"'","not_before":"[^"]+","not_after":"[^"]+"' "$report" >/dev/null || die "$name class-1 identity differs from release.pub/release.ac1"
    grep -E '"class":2,"outcome":"accepted","root_id":"'"$network_root_id"'","generation":[1-9][0-9]*,"digest":"'"$network_component_digest"'","not_before":"[^"]+","not_after":"[^"]+"' "$report" >/dev/null || die "$name class-2 identity differs from network.pub/network.ac1"
    grep -E '"class":3,"outcome":"accepted","root_id":"'"$compatibility_root_id"'","generation":[1-9][0-9]*,"digest":"'"$compatibility_component_digest"'","not_before":"[^"]+","not_after":"[^"]+"' "$report" >/dev/null || die "$name class-3 identity differs from compatibility.pub/compatibility.ac1"
    report_has_fresh_release "$(cat "$report")" || die "$name fresh control Release outcome is not release-accepted"
    grep -F '"release_identity":"'"$expected_release"'"' "$report" >/dev/null || die "$name Release identity differs from the selected release"
    grep -E '"build_identity":"[^"]+"' "$report" >/dev/null || die "$name build identity is absent"
    grep -F '"artifact_digest":"'"$expected_endpoint_sha256"'"' "$report" >/dev/null || die "$name artifact digest differs from the Endpoint"
    grep -E '"protocol_phase":"[^"]+"' "$report" >/dev/null || die "$name protocol phase is absent"
    grep -E '"network_id":"[0-9a-f]{64}"' "$report" >/dev/null || die "$name Network identity is absent"
    grep -E '"network_epoch":[1-9][0-9]*' "$report" >/dev/null || die "$name Network epoch is absent"
    grep -E '"network_digest":"[0-9a-f]{64}"' "$report" >/dev/null || die "$name Network digest is absent"
    grep -E '"network_profile":"[^"]+"' "$report" >/dev/null || die "$name Network profile is absent"
}

validate_report endpoint-a
validate_report endpoint-b
cmp -s "$evidence/endpoint-a-control.stdout.json" "$evidence/endpoint-b-control.stdout.json" || die 'fresh Endpoint control reports differ'
record_release_floor_inventory endpoint-a-control "$work/endpoint-a/control-inspection/release" \
    "$evidence/endpoint-a-control-release-floor-inventory.txt"
record_release_floor_inventory endpoint-b-control "$work/endpoint-b/control-inspection/release" \
    "$evidence/endpoint-b-control-release-floor-inventory.txt"
for inventory in \
    "$evidence/endpoint-b-release-floor-inventory.txt" \
    "$evidence/endpoint-a-control-release-floor-inventory.txt" \
    "$evidence/endpoint-b-control-release-floor-inventory.txt"; do
    cmp -s "$evidence/endpoint-a-release-floor-inventory.txt" "$inventory" || \
        die 'the four fresh Release floor inventories are not byte-for-byte equal'
done
endpoint_a_control_release=$(sed -n 's/.*"release":"\([^"]*\)".*/\1/p' "$evidence/endpoint-a-control.stdout.json")
endpoint_b_control_release=$(sed -n 's/.*"release":"\([^"]*\)".*/\1/p' "$evidence/endpoint-b-control.stdout.json")
[ "$endpoint_a_control_release" = release-accepted ] || die 'fresh endpoint-a control did not report release-accepted'
[ "$endpoint_b_control_release" = release-accepted ] || die 'fresh endpoint-b control did not report release-accepted'
[ "$endpoint_a_control_release" = "$endpoint_a_release" ] || die 'endpoint-a lifecycle and control Release outcomes differ'
[ "$endpoint_b_control_release" = "$endpoint_b_release" ] || die 'endpoint-b lifecycle and control Release outcomes differ'
printf 'reports=byte-for-byte-equal\nendpoint-a=%s\nendpoint-b=%s\n' \
    "$endpoint_a_control_release" "$endpoint_b_control_release" >"$evidence/control-report-comparison.txt"
printf 'schema=ardents-h4-6a-two-endpoints-summary-v1\nendpoint_a_release=release-accepted\nendpoint_b_release=release-accepted\nendpoint_a_control_release=release-accepted\nendpoint_b_control_release=release-accepted\nrelease_floor_inventories=canonical-byte-for-byte-equal\nlifecycle_identity=selected-cohort-release\n' \
    >"$evidence/qualification-summary.txt"

kill -TERM "$endpoint_a_pid" || die 'sending SIGTERM to endpoint-a failed'
kill -TERM "$endpoint_b_pid" || die 'sending SIGTERM to endpoint-b failed'

wait_stopped() {
    name=$1
    pid=$2
    socket=$3
    count=0
    while process_is_running "$pid" && [ "$count" -lt 150 ]; do
        sleep 0.1
        count=$((count + 1))
    done
    process_is_running "$pid" && die "$name did not stop within 15 seconds"
    set +e
    wait "$pid"
    code=$?
    set -e
    printf '%s\n' "$code" >"$evidence/$name.exitcode"
    [ "$code" -eq 0 ] || die "$name exited non-zero after SIGTERM"
    last=$(sed -n '$p' "$evidence/$name-lifecycle.stdout.log")
    printf '%s' "$last" | grep -F '"kind":"endpoint-lifecycle"' >/dev/null && printf '%s' "$last" | grep -F '"state":"stopped"' >/dev/null || die "$name did not emit the stopped event"
    [ ! -e "$socket" ] || die "$name left its runtime socket"
}

wait_stopped endpoint-a "$endpoint_a_pid" "$endpoint_a_socket"
endpoint_a_pid=
wait_stopped endpoint-b "$endpoint_b_pid" "$endpoint_b_socket"
endpoint_b_pid=
printf 'endpoint-a=stopped-exit-0-socket-absent\nendpoint-b=stopped-exit-0-socket-absent\n' >"$evidence/lifecycle-verdict.txt"
printf 'h4-6a-two-fresh-endpoints=accepted\n' >"$evidence/verdict.txt"
