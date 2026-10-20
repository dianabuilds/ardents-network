#!/bin/sh
# Runs only on the dedicated, preinstalled qualification Endpoint service.
set -eu
fail() { printf '%s\n' "$*" >&2; exit 1; }
[ "$(id -u)" = 0 ] || fail 'invalid environment: root installed-profile driver required'
maximum_polls=1500
case "${1-lifecycle}" in
    lifecycle) test_root=TestInstalledTextWorkerLifecycle ;;
    execution) test_root=TestInstalledExecutionLifecycle ;;
    execution-route) test_root=TestInstalledExecutionRouteTCP ;;
    connection-stream) test_root=TestInstalledConnectionStreamTCP ;;
    connection-control-received) test_root=TestInstalledConnectionControlReceivedTCP; maximum_polls=12600 ;;
    connection-control-delivery) test_root=TestInstalledConnectionControlDeliveryTCP; maximum_polls=12600 ;;
    connection-authority) test_root=TestInstalledConnectionAuthorityTCP; maximum_polls=12600 ;;
    connection-control)
        # The complete 12-case TCP matrix, including race instrumentation, uses the
        # pinned unit's twenty-minute test timeout and twenty-one-minute runtime.
        test_root=TestInstalledConnectionControlWritesTCP; maximum_polls=12600
        ;;
    connection-refresh) test_root=TestInstalledConnectionRefreshTCP ;;
    connection-reopen) test_root=TestInstalledConnectionDurableReopenTCP ;;
    connection-withdrawal) test_root=TestInstalledConnectionWithdrawalTCP ;;
    connection-nonce) test_root=TestInstalledConnectionNonceTCP ;;
    publication-timing)
        [ "${2--timeout=15m}" = -timeout=15m ] || fail 'invalid environment: elapsed Publication requires its fixed fifteen-minute bound'
        test_root=TestInstalledPublicationElapsedTCP; maximum_polls=9000
        ;;
    execution-recovery) test_root=TestInstalledExecutionEndpointDeath ;;
    network) test_root=TestInstalledTextWorkersReadTargetThroughJoinedNetwork; maximum_polls=10000 ;;
    recovery) test_root=TestInstalledTextWorkersRecoverAcceptedRequestAcrossJoinedNetwork; maximum_polls=3600 ;;
    tree|execution-tree)
        if [ "$1" = execution-tree ]; then
            test_root=TestInstalledExecutionHostileTree
        else
            test_root=TestInstalledTextWorkerHostileTree
        fi
        worker=/usr/lib/ardents/text-worker-root/ardents-text
        [ -n "${ARDENTS_TEXT_HOSTILE_WORKER_SHA256-}" ] || fail 'hostile worker independent digest required'
        [ -f "$worker" ] && [ ! -L "$worker" ] && [ "$(stat -c %u:%g:%a "$worker")" = 0:0:555 ] ||
            fail 'invalid environment: pinned hostile artifact required'
        printf '%s  %s\n' "$ARDENTS_TEXT_HOSTILE_WORKER_SHA256" "$worker" | sha256sum --check --status ||
            fail 'invalid environment: hostile artifact differs from declared candidate'
        ;;
    escape|execution-escape)
        if [ "$1" = execution-escape ]; then
            test_root=TestInstalledExecutionEscapeMatrix
        else
            test_root=TestInstalledTextWorkerEscapeMatrix
        fi
        worker=/usr/lib/ardents/text-worker-root/ardents-text
        [ -n "${ARDENTS_TEXT_ESCAPE_WORKER_SHA256-}" ] || fail 'escape worker independent digest required'
        [ -f "$worker" ] && [ ! -L "$worker" ] && [ "$(stat -c %u:%g:%a "$worker")" = 0:0:555 ] ||
            fail 'invalid environment: pinned escape artifact required'
        printf '%s  %s\n' "$ARDENTS_TEXT_ESCAPE_WORKER_SHA256" "$worker" | sha256sum --check --status ||
            fail 'invalid environment: escape artifact differs from declared candidate'
        ;;
    *) fail 'unknown installed text-worker profile' ;;
esac
for program in systemctl journalctl sha256sum stat uname sleep grep; do
    command -v "$program" >/dev/null || fail "invalid environment: $program unavailable"
done
. /etc/os-release
[ "$ID" = ubuntu ] && [ "$VERSION_ID" = 24.04 ] && [ "$(uname -m)" = x86_64 ] ||
    fail 'invalid environment: Ubuntu 24.04 x86-64 required'
[ "$(stat -fc %T /sys/fs/cgroup)" = cgroup2fs ] || fail 'invalid environment: cgroup v2 required'
binary=/usr/lib/ardents/qualification/endpoint.test
if [ "$test_root" = TestInstalledConnectionDurableReopenTCP ]; then
    binary=/usr/lib/ardents/qualification/execution-command.test
fi
if [ "$test_root" = TestInstalledExecutionLifecycle ] || [ "$test_root" = TestInstalledExecutionHostileTree ] || [ "$test_root" = TestInstalledExecutionEscapeMatrix ]; then
    binary=/usr/lib/ardents/qualification/execution-runtime.test
fi
if [ "$test_root" = TestInstalledExecutionEndpointDeath ]; then
    binary=/usr/lib/ardents/qualification/execution-runtime.test
    worker=/usr/lib/ardents/text-worker-root/ardents-text
    [ -n "${ARDENTS_TEXT_HOSTILE_WORKER_SHA256-}" ] || fail 'hostile worker independent digest required'
    [ -f "$worker" ] && [ ! -L "$worker" ] && [ "$(stat -c %u:%g:%a "$worker")" = 0:0:555 ] ||
        fail 'invalid environment: pinned hostile artifact required'
    printf '%s  %s\n' "$ARDENTS_TEXT_HOSTILE_WORKER_SHA256" "$worker" | sha256sum --check --status ||
        fail 'invalid environment: hostile artifact differs from declared candidate'
fi
if [ "$test_root" = TestInstalledExecutionRouteTCP ] || [ "$test_root" = TestInstalledPublicationElapsedTCP ] || [ "$test_root" = TestInstalledConnectionStreamTCP ] || [ "$test_root" = TestInstalledConnectionAuthorityTCP ] || [ "$test_root" = TestInstalledConnectionControlDeliveryTCP ] || [ "$test_root" = TestInstalledConnectionControlReceivedTCP ] || [ "$test_root" = TestInstalledConnectionControlWritesTCP ] || [ "$test_root" = TestInstalledConnectionRefreshTCP ] || [ "$test_root" = TestInstalledConnectionWithdrawalTCP ] || [ "$test_root" = TestInstalledConnectionNonceTCP ]; then
    binary=/usr/lib/ardents/qualification/execution-command.test
fi
unit=/run/systemd/system/ardents-endpoint.service
[ -n "$ARDENTS_TEXT_LIFECYCLE_SHA256" ] && [ -n "$ARDENTS_TEXT_LIFECYCLE_UNIT_SHA256" ] ||
    fail 'invalid environment: independent binary and unit digests required'
[ -f "$binary" ] && [ ! -L "$binary" ] && [ "$(stat -c %u:%g:%a "$binary")" = 0:0:555 ] ||
    fail 'invalid environment: immutable root-owned test binary required'
[ -f "$unit" ] && [ ! -L "$unit" ] && [ "$(stat -c %u:%g:%a "$unit")" = 0:0:644 ] ||
    fail 'invalid environment: root-owned temporary qualification unit required'
printf '%s  %s\n' "$ARDENTS_TEXT_LIFECYCLE_SHA256" "$binary" | sha256sum --check --status ||
    fail 'invalid environment: test binary differs from declared candidate'
printf '%s  %s\n' "$ARDENTS_TEXT_LIFECYCLE_UNIT_SHA256" "$unit" | sha256sum --check --status ||
    fail 'invalid environment: test unit differs from declared candidate'
[ "$(systemctl show ardents-endpoint.service -p FragmentPath --value)" = "$unit" ] ||
    fail 'invalid environment: a different Endpoint unit is loaded'
[ "$(systemctl show ardents-endpoint.service -p ActiveState --value)" = inactive ] ||
    fail 'invalid environment: Endpoint is not fresh and inactive'
[ -z "$(systemctl show ardents-endpoint.service -p DropInPaths --value)" ] ||
    fail 'invalid environment: qualification unit has drop-ins'
if [ "$test_root" = TestInstalledExecutionEndpointDeath ]; then
    # The Root observer runs outside Endpoint and intentionally retains its
    # two failed SIGKILL invocations. No killed actor may supply a PASS result.
    controller_argument=${2--timeout=2m}
    [ "$controller_argument" = -timeout=2m ] || fail 'invalid environment: Endpoint-death controller requires its fixed two-minute bound'
    controller_timeout=${controller_argument#-timeout=}
    if journal=$("$binary" -test.run='^TestInstalledExecutionEndpointDeath$' -test.v "-test.timeout=$controller_timeout"); then
        controller_exit=0
    else
        controller_exit=$?
    fi
    printf '%s\n' "$journal"
    [ "$controller_exit" = 0 ] || fail 'original Endpoint-death controller failed'
    for test in "$test_root" "$test_root/initial" "$test_root/restart"; do
        [ "$(printf '%s\n' "$journal" | grep -c "^[[:space:]]*--- PASS: $test (" || true)" = 1 ] ||
            fail 'Endpoint-death profile lacks exact executed controller evidence'
    done
    printf 'installed-profile=%s; result=passed; whole-host-qualification=not-established\n' "$test_root"
    exit 0
fi
systemctl start ardents-endpoint.service
invocation=$(systemctl show ardents-endpoint.service -p InvocationID --value)
[ -n "$invocation" ] || fail 'installed test has no exact running invocation'
attempt=0
while :; do
    state=$(systemctl show ardents-endpoint.service -p ActiveState --value)
    case "$state" in
        inactive|failed) break ;;
        active|activating|deactivating) ;;
        *) fail 'installed lifecycle has an unknown service state' ;;
    esac
    attempt=$((attempt + 1))
    [ "$attempt" -le "$maximum_polls" ] || fail 'installed lifecycle exceeded its declared bound'
    sleep 0.1
done
journal=$(journalctl --no-pager -o cat "_SYSTEMD_INVOCATION_ID=$invocation")
printf '%s\n' "$journal"
[ "$state" = inactive ] && [ "$(systemctl show ardents-endpoint.service -p MainPID --value)" = 0 ] &&
    [ "$(systemctl show ardents-endpoint.service -p Result --value)" = success ] &&
    [ "$(systemctl show ardents-endpoint.service -p ExecMainStatus --value)" = 0 ] ||
    fail 'installed text-worker lifecycle failed or did not terminate'
set -- "$test_root" "$test_root/reader" "$test_root/publisher"
if [ "$test_root" = TestInstalledConnectionDurableReopenTCP ]; then
    set -- "$test_root" "$test_root/ardents-carrier-tcp-tls-v2"
fi
if [ "$test_root" = TestInstalledConnectionWithdrawalTCP ]; then
    set -- "$test_root" "$test_root/ardents-carrier-tcp-tls-v2"
fi
if [ "$test_root" = TestInstalledConnectionNonceTCP ]; then
    set -- "$test_root" "$test_root/closed-peer-write"
    for carrier in ardents-carrier-tcp-tls-v2; do
        set -- "$@" "$test_root/$carrier" "$test_root/$carrier/result" "$test_root/$carrier/join" "$test_root/$carrier/tls"
    done
fi
if [ "$test_root" = TestInstalledConnectionRefreshTCP ]; then
    set -- "$test_root"
    for carrier in ardents-carrier-tcp-tls-v2; do
        set -- "$@" "$test_root/$carrier" "$test_root/$carrier/accepted" "$test_root/$carrier/pending-ack"
    done
fi
if [ "$test_root" = TestInstalledConnectionStreamTCP ]; then
    set -- "$test_root"
    for carrier in ardents-carrier-tcp-tls-v2; do
        set -- "$@" "$test_root/$carrier" "$test_root/$carrier/before-request-loss" "$test_root/$carrier/before-request-terminal-commit-loss" "$test_root/$carrier/responder-parent-loss" "$test_root/$carrier/source-parent-loss" "$test_root/$carrier/source-stock-exhaustion" "$test_root/$carrier/publisher-source-parent-loss" "$test_root/$carrier/response-loss" "$test_root/$carrier/after-request-terminal-loss" "$test_root/$carrier/caller-observation-loss" "$test_root/$carrier/close-observation-loss" "$test_root/$carrier/publisher-observation-loss" "$test_root/$carrier/publisher-close-observation-loss"
    done
fi
if [ "$test_root" = TestInstalledConnectionControlWritesTCP ] || [ "$test_root" = TestInstalledConnectionControlReceivedTCP ] || [ "$test_root" = TestInstalledConnectionControlDeliveryTCP ]; then
    set -- "$test_root"
    for carrier in ardents-carrier-tcp-tls-v2; do
        set -- "$@" "$test_root/$carrier"
        if [ "$test_root" = TestInstalledConnectionControlWritesTCP ]; then
            set -- "$@" "$test_root/$carrier/independent-progress"
        fi
        for role in initiator publisher; do
            set -- "$@" "$test_root/$carrier/$role"
            for kind in terminal receipt confirmation; do
                set -- "$@" "$test_root/$carrier/$role/$kind"
                if [ "$test_root" = TestInstalledConnectionControlWritesTCP ]; then
                    for cut in before-write after-write; do
                        set -- "$@" "$test_root/$carrier/$role/$kind/$cut"
                    done
                fi
                if [ "$test_root" = TestInstalledConnectionControlDeliveryTCP ]; then
                    for cut in before-receive after-finish; do
                        set -- "$@" "$test_root/$carrier/$role/$kind/$cut"
                    done
                fi
            done
        done
    done
fi
if [ "$test_root" = TestInstalledConnectionAuthorityTCP ]; then
    set -- "$test_root"
    for carrier in ardents-carrier-tcp-tls-v2; do
        set -- "$@" "$test_root/$carrier"
        for role in initiator publisher; do
            set -- "$@" "$test_root/$carrier/$role"
            for fault in worker network deadline; do
                set -- "$@" "$test_root/$carrier/$role/$fault"
            done
        done
    done
fi
if [ "$test_root" = TestInstalledPublicationElapsedTCP ]; then
    set -- "$test_root"
    for carrier in ardents-carrier-tcp-tls-v2; do
        set -- "$@" "$test_root/$carrier" "$test_root/$carrier/initial-delayed-ack" "$test_root/$carrier/creation-refresh" "$test_root/$carrier/bounded-overlap"
    done
fi
if [ "$test_root" = TestInstalledExecutionLifecycle ]; then
    set -- "$test_root" "$test_root/Publisher-snapshot-operation" "$test_root/ordinary-Reader-operation" "$test_root/connection" "$test_root/connection/live-operation" "$test_root/connection/attachment-loss" "$test_root/administration" "$test_root/administration/live-operation" "$test_root/administration/attachment-loss" "$test_root/cleanup-failure"
fi
if [ "$test_root" = TestInstalledExecutionHostileTree ] || [ "$test_root" = TestInstalledExecutionEscapeMatrix ]; then
    set -- "$test_root" "$test_root/connection" "$test_root/administration"
fi
if [ "$test_root" = TestInstalledExecutionRouteTCP ]; then
    set -- "$test_root" "$test_root/ardents-carrier-tcp-tls-v2"
    for carrier in ardents-carrier-tcp-tls-v2; do
        set -- "$@" "$test_root/$carrier/service-authentication" "$test_root/$carrier/service-text" "$test_root/$carrier/service-setup-handoff" "$test_root/$carrier/service-recovery"
        set -- "$@" "$test_root/$carrier/publication-worker-ack" "$test_root/$carrier/publication-worker-ack/initial" "$test_root/$carrier/publication-worker-ack/replacement"
        set -- "$@" "$test_root/$carrier/publication-worker-ack/initial/accepted-history" "$test_root/$carrier/publication-worker-ack/retained-history"
        set -- "$@" "$test_root/$carrier/publication-network-ack" "$test_root/$carrier/publication-network-ack/initial" "$test_root/$carrier/publication-network-ack/replacement"
        set -- "$@" "$test_root/$carrier/publication-decoded-network-ack" "$test_root/$carrier/publication-decoded-network-ack/initial" "$test_root/$carrier/publication-decoded-network-ack/replacement"
        set -- "$@" "$test_root/$carrier/publication-delivery-close" "$test_root/$carrier/publication-delivery-close/initial" "$test_root/$carrier/publication-delivery-close/replacement" "$test_root/$carrier/publication-delivery-close/retained-replay"
        set -- "$@" "$test_root/$carrier/publication/delivery" "$test_root/$carrier/publication/delivery/accepted" "$test_root/$carrier/publication/delivery/accepted/concurrent" "$test_root/$carrier/publication/delivery/replay" "$test_root/$carrier/publication/delivery/retained-replay"
        set -- "$@" "$test_root/$carrier/publication" "$test_root/$carrier/publication-ack" "$test_root/$carrier/publication-ack/initial" "$test_root/$carrier/publication-ack/replacement" "$test_root/$carrier/publication-withdraw-ack" "$test_root/$carrier/publication-withdraw-ack/initial" "$test_root/$carrier/publication-withdraw-ack/replacement" "$test_root/$carrier/publication-caller-ack" "$test_root/$carrier/publication-caller-ack/initial" "$test_root/$carrier/publication-caller-ack/replacement" "$test_root/$carrier/joined" "$test_root/$carrier/worker-loss" "$test_root/$carrier/worker-loss-io" "$test_root/$carrier/clock-loss-io" "$test_root/$carrier/caller-loss-io"
    done
fi
if [ "$test_root" = TestInstalledTextWorkersReadTargetThroughJoinedNetwork ]; then
    set -- "$test_root"
    for carrier in ardents-carrier-tcp-tls-v2; do
        set -- "$@" "$test_root/$carrier"
        for size in empty reference maximum refresh; do
            set -- "$@" "$test_root/$carrier/$size"
        done
    done
fi
if [ "$test_root" = TestInstalledTextWorkersRecoverAcceptedRequestAcrossJoinedNetwork ]; then
    set -- "$test_root"
    for carrier in ardents-carrier-tcp-tls-v2; do
        set -- "$@" "$test_root/$carrier"
    done
fi
for test do
    [ "$(printf '%s\n' "$journal" | grep -c "^[[:space:]]*--- PASS: $test (" || true)" = 1 ] ||
        fail 'installed lifecycle lacks exact executed test evidence'
done
printf 'installed-profile=%s; result=passed; whole-host-qualification=not-established\n' "$test_root"
