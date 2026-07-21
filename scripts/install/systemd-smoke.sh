#!/bin/sh
set -eu

v1_dir=${1:?v1 release directory is required}
v2_dir=${2:?v2 release directory is required}
bad_dir=${3:?failing release directory is required}
installer="$v1_dir/scripts/install/linux.sh"
token_file=/var/lib/ardents/secrets/api-token
config_file=/etc/ardents/operator.json
socket_path=/var/lib/ardents/secrets/control.sock
backup=/var/backups/ardents/manual-smoke.tar.gz
restore_backup=/var/backups/ardents/restore-smoke.tar.gz

diagnose() {
    systemctl status ardentsd.service --no-pager >&2 || true
    journalctl -u ardentsd.service --no-pager -n 100 >&2 || true
}
trap diagnose HUP INT TERM

wait_ready() {
    attempt=0
    until [ -S "$socket_path" ] && /usr/local/bin/ardentsctl --addr "unix://$socket_path" \
        --token-file "$token_file" --output json node status 2>/dev/null |
        grep -E '"ready":[[:space:]]*true' >/dev/null; do
        attempt=$((attempt + 1))
        [ "$attempt" -lt 150 ] || { diagnose; return 1; }
        sleep 0.1
    done
}

version_is() { /usr/local/bin/ardentsd --version | grep -F '"version":"'$1'"' >/dev/null; }
authority_hash() { (cd /var/lib/ardents-authority && find . -type f -exec sha256sum {} \; | sort | sha256sum); }

"$installer" install --source-dir "$v1_dir" --node-name systemd-smoke --transport-port 61002
systemctl is-enabled --quiet ardentsd.service
systemctl is-active --quiet ardentsd.service
wait_ready
version_is v0.1.0

token_before=$(sha256sum "$token_file")
config_before=$(sha256sum "$config_file")
identity_before=$(sha256sum /var/lib/ardents/identity_key.json)
waku_before=$(sha256sum /var/lib/ardents/waku_node_key.json)
authority_before=$(authority_hash)

"$installer" install --source-dir "$v1_dir"
wait_ready
version_is v0.1.0
if "$installer" install --source-dir "$v2_dir"; then
    echo 'install unexpectedly bypassed the version-transition contract' >&2; exit 1
fi
systemctl is-active --quiet ardentsd.service
wait_ready; version_is v0.1.0

systemctl stop ardentsd.service
if "$installer" backup --output /var/backups/ardents/../escaped.tar.gz; then
    echo 'backup path traversal unexpectedly succeeded' >&2; exit 1
fi
"$installer" backup --output "$backup"
test -s "$backup"; test -s "$backup.manifest"
systemctl start ardentsd.service; wait_ready

"$installer" upgrade --source-dir "$v2_dir"
wait_ready; version_is v0.2.0
test "$token_before" = "$(sha256sum "$token_file")"
test "$config_before" = "$(sha256sum "$config_file")"
test "$identity_before" = "$(sha256sum /var/lib/ardents/identity_key.json)"
test "$waku_before" = "$(sha256sum /var/lib/ardents/waku_node_key.json)"
test "$authority_before" = "$(authority_hash)"

if "$installer" upgrade --source-dir "$v2_dir" --backup "$backup"; then
    echo 'upgrade with an existing backup path unexpectedly succeeded' >&2; exit 1
fi
systemctl is-active --quiet ardentsd.service
wait_ready; version_is v0.2.0

if "$installer" upgrade --source-dir "$bad_dir" --backup /var/backups/ardents/pre-bad.tar.gz; then
    echo 'failing upgrade unexpectedly succeeded' >&2; exit 1
fi
systemctl is-active --quiet ardentsd.service
wait_ready; version_is v0.2.0

"$installer" rollback
wait_ready; version_is v0.1.0
"$installer" rollback
wait_ready; version_is v0.2.0

systemctl stop ardentsd.service
"$installer" rollback
! systemctl is-active --quiet ardentsd.service
version_is v0.1.0
"$installer" rollback
! systemctl is-active --quiet ardentsd.service
version_is v0.2.0
systemctl start ardentsd.service; wait_ready

systemctl stop ardentsd.service
"$installer" backup --output "$restore_backup"
mv /etc/ardents/operator.json /tmp/operator.json.original
mv /var/lib/ardents /tmp/ardents-state.original
mv /var/lib/ardents-authority /tmp/ardents-authority.original
"$installer" restore --archive "$restore_backup"
! systemctl is-active --quiet ardentsd.service
test "$token_before" = "$(sha256sum "$token_file")"
test "$config_before" = "$(sha256sum "$config_file")"
test "$identity_before" = "$(sha256sum /var/lib/ardents/identity_key.json)"
test "$waku_before" = "$(sha256sum /var/lib/ardents/waku_node_key.json)"
test "$authority_before" = "$(authority_hash)"
systemctl start ardentsd.service; wait_ready

"$installer" uninstall
! systemctl is-active --quiet ardentsd.service
test ! -e /usr/local/bin/ardentsd
test ! -e /usr/local/bin/ardentsctl
test ! -e /etc/systemd/system/ardentsd.service
test -f /var/lib/ardents/ardents.db
test "$token_before" = "$(sha256sum "$token_file")"
test "$config_before" = "$(sha256sum "$config_file")"
test "$identity_before" = "$(sha256sum /var/lib/ardents/identity_key.json)"
test "$waku_before" = "$(sha256sum /var/lib/ardents/waku_node_key.json)"
test "$authority_before" = "$(authority_hash)"
printf 'native-systemd-smoke=passed\n'
