#!/bin/sh
set -eu

v1_dir=${1:?v1 release directory is required}
v2_dir=${2:?v2 release directory is required}
bad_dir=${3:?failing release directory is required}
installer="$v1_dir/scripts/install/linux.sh"
config_file=/etc/ardents/operator.json
socket_path=/var/lib/ardents/secrets/control.sock
bootstrap_ticket=/var/lib/ardents/secrets/operator-bootstrap-ticket
root_signer=/root/ardents-systemd-smoke/root.json
device_signer=/root/ardents-systemd-smoke/device.json
application_dir=/var/lib/ardents-applications
application_socket="$application_dir/application.sock"
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
        --principal "$node_principal" --signer-file "$device_signer" --output json node status 2>/dev/null |
        grep -E '"ready":[[:space:]]*true' >/dev/null; do
        attempt=$((attempt + 1))
        [ "$attempt" -lt 150 ] || { diagnose; return 1; }
        sleep 0.1
    done
}

wait_infra() {
    attempt=0
    until [ -S "$socket_path" ] && [ -f "$bootstrap_ticket" ] &&
        curl --fail --silent http://127.0.0.1:9090/readyz |
        grep -E '"status":[[:space:]]*"ready"' >/dev/null; do
        attempt=$((attempt + 1))
        [ "$attempt" -lt 150 ] || { diagnose; return 1; }
        sleep 0.1
    done
}

version_is() { /usr/local/bin/ardentsd --version | grep -F '"version":"'$1'"' >/dev/null; }
authority_hash() { (cd /var/lib/ardents-authority && find . -type f -exec sha256sum {} \; | sort | sha256sum); }

install_output=$("$installer" install --source-dir "$v1_dir" --node-name systemd-smoke --transport-port 61002)
printf '%s\n' "$install_output"
node_principal=$(printf '%s\n' "$install_output" | sed -n 's/.*principal=\(p1_[^ ]*\).*/\1/p' | head -n 1)
test -n "$node_principal"
systemctl is-enabled --quiet ardentsd.service
systemctl is-active --quiet ardentsd.service
wait_infra
install -d -m 0700 "$(dirname "$root_signer")"
principal_json=$(/usr/local/bin/ardentsctl --output json identity principal create --signer-file "$root_signer")
alice_principal=$(printf '%s\n' "$principal_json" | sed -n 's/.*"principal":"\([^"]*\)".*/\1/p')
/usr/local/bin/ardentsctl --output json identity device create \
    --root-signer-file "$root_signer" --signer-file "$device_signer" --valid-for 720h >/dev/null
/usr/local/bin/ardentsctl --addr "unix://$socket_path" --principal "$node_principal" \
    --signer-file "$device_signer" --output json identity enroll \
    --root-signer-file "$root_signer" --device-signer-file "$device_signer" \
    --bootstrap-ticket-file "$bootstrap_ticket" >/dev/null
/usr/local/bin/ardentsctl --addr "unix://$socket_path" --principal "$node_principal" \
    --signer-file "$device_signer" --output json identity grant issue \
    --subject "$alice_principal" --action node.status --valid-for 720h \
    --request-id systemd-smoke-readiness-v1 --yes >/dev/null
wait_ready
version_is v0.1.0
getent group ardents-apps >/dev/null
test "$(stat -c '%a' "$application_dir")" = 2750
test "$(stat -c '%a' "$application_socket")" = 660
test "$(stat -c '%G' "$application_dir")" = ardents-apps
test "$(stat -c '%G' "$application_socket")" = ardents-apps
test ! -e "$application_dir/application-token"
test ! -e /var/lib/ardents/secrets/api-token

systemctl stop ardentsd.service
/usr/local/bin/ardentsd init --authority-dir /var/lib/ardents-authority --node-dir /var/lib/ardents \
    --secret-dir /var/lib/ardents/secrets --node-name systemd-smoke --transport-port 61002 \
    --runtime-data-dir /var/lib/ardents --runtime-secret-dir /var/lib/ardents/secrets
systemctl start ardentsd.service
wait_ready

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
mv "$application_dir" /tmp/ardents-applications.original
mv /var/lib/ardents-authority /tmp/ardents-authority.original
"$installer" restore --archive "$restore_backup"
! systemctl is-active --quiet ardentsd.service
test "$config_before" = "$(sha256sum "$config_file")"
test "$identity_before" = "$(sha256sum /var/lib/ardents/identity_key.json)"
test "$waku_before" = "$(sha256sum /var/lib/ardents/waku_node_key.json)"
test "$authority_before" = "$(authority_hash)"
systemctl start ardentsd.service; wait_ready
test "$(stat -c '%a' "$application_dir")" = 2750
test "$(stat -c '%a' "$application_socket")" = 660
test ! -e "$application_dir/application-token"

"$installer" uninstall
! systemctl is-active --quiet ardentsd.service
test ! -e /usr/local/bin/ardentsd
test ! -e /usr/local/bin/ardentsctl
test ! -e /etc/systemd/system/ardentsd.service
test -f /var/lib/ardents/ardents.db
test "$config_before" = "$(sha256sum "$config_file")"
test "$identity_before" = "$(sha256sum /var/lib/ardents/identity_key.json)"
test "$waku_before" = "$(sha256sum /var/lib/ardents/waku_node_key.json)"
test "$authority_before" = "$(authority_hash)"
printf 'native-systemd-smoke=passed\n'
