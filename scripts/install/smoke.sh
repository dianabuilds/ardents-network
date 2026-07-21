#!/bin/sh
set -eu

release_dir=${1:?extracted release directory is required}
installer="$release_dir/scripts/install/linux.sh"
token_file=/var/lib/ardents/secrets/api-token
config_file=/etc/ardents/operator.json
socket_path=/var/lib/ardents/secrets/control.sock
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

"$installer" install --source-dir "$release_dir" --node-name native-smoke --transport-port 61001 --no-start
! command -v docker >/dev/null 2>&1
test -x /usr/local/bin/ardentsd
test -x /usr/local/bin/ardentsctl
test -f /etc/systemd/system/ardentsd.service
test "$(stat -c '%a' "$config_file")" = 600
test "$(stat -c '%a' "$token_file")" = 600

token_before=$(sha256sum "$token_file")
config_before=$(sha256sum "$config_file")
identity_before=$(tree_digest /var/lib/ardents)
authority_before=$(tree_digest /var/lib/ardents-authority)
"$installer" install --source-dir "$release_dir" --no-start
test "$token_before" = "$(sha256sum "$token_file")"
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
attempt=0
until /usr/local/bin/ardentsctl --addr "unix://$socket_path" --token-file "$token_file" --output json node status 2>/dev/null | grep -E '"ready":[[:space:]]*true' >/dev/null; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 100 ]; then
        /usr/local/bin/ardentsctl --addr "unix://$socket_path" --token-file "$token_file" --output json node status >&2 || true
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
test -f "$token_file"
test "$state_before_uninstall" = "$(tree_digest /var/lib/ardents)"
test "$authority_before_uninstall" = "$(tree_digest /var/lib/ardents-authority)"
printf 'native-install-smoke=passed\n'
