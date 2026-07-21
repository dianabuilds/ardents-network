#!/bin/sh
set -eu

release_dir=${1:?release directory is required}
installer="$release_dir/scripts/install/linux.sh"
token_file=/var/lib/ardents/secrets/api-token
config_file=/etc/ardents/operator.json
socket_path=/var/lib/ardents/secrets/control.sock

diagnose() {
    systemctl status ardentsd.service --no-pager >&2 || true
    journalctl -u ardentsd.service --no-pager -n 100 >&2 || true
}
trap diagnose HUP INT TERM

wait_ready() {
    attempt=0
    until [ -S "$socket_path" ] && \
        /usr/local/bin/ardentsctl --addr "unix://$socket_path" --token-file "$token_file" --output json node status 2>/dev/null |
            grep -E '"ready":[[:space:]]*true' >/dev/null; do
        attempt=$((attempt + 1))
        if [ "$attempt" -ge 150 ]; then
            diagnose
            return 1
        fi
        sleep 0.1
    done
}

"$installer" install --source-dir "$release_dir" --node-name systemd-smoke --transport-port 61002
systemctl is-enabled --quiet ardentsd.service
systemctl is-active --quiet ardentsd.service
wait_ready

token_before=$(sha256sum "$token_file")
config_before=$(sha256sum "$config_file")
identity_before=$(sha256sum /var/lib/ardents/identity_key.json)
authority_before=$(find /var/lib/ardents-authority -type f -exec sha256sum {} \; | sort | sha256sum)

"$installer" install --source-dir "$release_dir"
systemctl is-active --quiet ardentsd.service
wait_ready
test "$token_before" = "$(sha256sum "$token_file")"
test "$config_before" = "$(sha256sum "$config_file")"
test "$identity_before" = "$(sha256sum /var/lib/ardents/identity_key.json)"
test "$authority_before" = "$(find /var/lib/ardents-authority -type f -exec sha256sum {} \; | sort | sha256sum)"

systemctl restart ardentsd.service
wait_ready
token_before_uninstall=$(sha256sum "$token_file")
config_before_uninstall=$(sha256sum "$config_file")
identity_before_uninstall=$(sha256sum /var/lib/ardents/identity_key.json)
waku_before_uninstall=$(sha256sum /var/lib/ardents/waku_node_key.json)
authority_before_uninstall=$(find /var/lib/ardents-authority -type f -exec sha256sum {} \; | sort | sha256sum)

"$installer" uninstall
! systemctl is-active --quiet ardentsd.service
test ! -e /usr/local/bin/ardentsd
test ! -e /usr/local/bin/ardentsctl
test ! -e /etc/systemd/system/ardentsd.service
test -f /var/lib/ardents/ardents.db
test "$token_before_uninstall" = "$(sha256sum "$token_file")"
test "$config_before_uninstall" = "$(sha256sum "$config_file")"
test "$identity_before_uninstall" = "$(sha256sum /var/lib/ardents/identity_key.json)"
test "$waku_before_uninstall" = "$(sha256sum /var/lib/ardents/waku_node_key.json)"
test "$authority_before_uninstall" = "$(find /var/lib/ardents-authority -type f -exec sha256sum {} \; | sort | sha256sum)"
printf 'native-systemd-smoke=passed\n'
