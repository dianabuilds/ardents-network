#!/bin/sh
set -eu

release_dir=${1:?extracted release directory is required}
installer="$release_dir/scripts/install/linux.sh"
config_file=/etc/ardents/operator.json
socket_path=/var/lib/ardents/secrets/control.sock
bootstrap_ticket=/var/lib/ardents/secrets/operator-bootstrap-ticket
root_signer=/var/lib/ardents-smoke/operator-root.json
device_signer=/var/lib/ardents-smoke/operator-device.json
log_file=/tmp/ardentsd-native-smoke.log
daemon_pid=

tree_digest() {
    directory=$1
    find "$directory" -type f -exec sha256sum {} \; | sort | sha256sum | cut -d' ' -f1
}

cleanup() {
    if [ -n "$daemon_pid" ]; then
        kill "$daemon_pid" >/dev/null 2>&1 || true
        wait "$daemon_pid" >/dev/null 2>&1 || true
    fi
}
trap cleanup EXIT INT TERM

install_output=$("$installer" install --source-dir "$release_dir" --node-name native-smoke --transport-port 61001 --no-start)
printf '%s\n' "$install_output"
node_principal=$(printf '%s\n' "$install_output" | sed -n 's/.*principal=\(p1_[^ ]*\).*/\1/p' | head -n 1)
test -n "$node_principal"
! command -v docker >/dev/null 2>&1
test -x /usr/local/bin/ardentsd
test -x /usr/local/bin/ardentsctl
test -f /etc/systemd/system/ardentsd.service
test "$(stat -c '%a' "$config_file")" = 600
test ! -e /var/lib/ardents/secrets/api-token
test ! -e /var/lib/ardents-applications/application-token

config_before=$(sha256sum "$config_file")
identity_before=$(tree_digest /var/lib/ardents)
authority_before=$(tree_digest /var/lib/ardents-authority)
"$installer" install --source-dir "$release_dir" --no-start
test "$config_before" = "$(sha256sum "$config_file")"
test "$identity_before" = "$(tree_digest /var/lib/ardents)"
test "$authority_before" = "$(tree_digest /var/lib/ardents-authority)"

runuser -u ardents -- env ARDENTS_CONFIG_FILE="$config_file" /usr/local/bin/ardentsd >"$log_file" 2>&1 &
daemon_pid=$!
attempt=0
until [ -S "$socket_path" ]; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 100 ]; then
        cat "$log_file" >&2
        exit 1
    fi
    sleep 0.1
done
test -f "$bootstrap_ticket"
install -d -m 0700 "$(dirname "$root_signer")"
principal_json=$(/usr/local/bin/ardentsctl --output json identity principal create --signer-file "$root_signer")
alice_principal=$(printf '%s\n' "$principal_json" | sed -n 's/.*"principal":"\([^"]*\)".*/\1/p')
test -n "$alice_principal"
/usr/local/bin/ardentsctl --output json identity device create \
    --root-signer-file "$root_signer" --signer-file "$device_signer" --valid-for 24h >/dev/null
/usr/local/bin/ardentsctl --addr "unix://$socket_path" --principal "$node_principal" \
    --signer-file "$device_signer" --output json identity enroll \
    --root-signer-file "$root_signer" --device-signer-file "$device_signer" \
    --bootstrap-ticket-file "$bootstrap_ticket" >/dev/null
test ! -e "$bootstrap_ticket"
/usr/local/bin/ardentsctl --addr "unix://$socket_path" --principal "$node_principal" \
    --signer-file "$device_signer" --output json identity grant issue \
    --subject "$alice_principal" --action node.status --valid-for 24h \
    --request-id native-smoke-readiness-v1 --yes >/dev/null
attempt=0
until /usr/local/bin/ardentsctl --addr "unix://$socket_path" --principal "$node_principal" \
    --signer-file "$device_signer" --output json node status 2>/dev/null |
    grep -E '"ready":[[:space:]]*true' >/dev/null; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 100 ]; then
        /usr/local/bin/ardentsctl --addr "unix://$socket_path" --principal "$node_principal" \
            --signer-file "$device_signer" --output json node status >&2 || true
        cat "$log_file" >&2
        exit 1
    fi
    sleep 0.1
done
cleanup
daemon_pid=

state_before_uninstall=$(tree_digest /var/lib/ardents)
authority_before_uninstall=$(tree_digest /var/lib/ardents-authority)

"$installer" uninstall --no-start
test ! -e /usr/local/bin/ardentsd
test ! -e /usr/local/bin/ardentsctl
test ! -e /etc/systemd/system/ardentsd.service
test -f "$config_file"
test ! -e /var/lib/ardents/secrets/api-token
test "$state_before_uninstall" = "$(tree_digest /var/lib/ardents)"
test "$authority_before_uninstall" = "$(tree_digest /var/lib/ardents-authority)"
printf 'native-install-smoke=passed\n'
