#!/bin/sh
set -eu

usage() {
    cat <<'EOF'
Usage:
  linux.sh install [options]
  linux.sh uninstall [--root PATH] [--no-start]

Install options:
  --node-name NAME       canonical node name; required for first install
  --transport-port PORT  Waku TCP listen port (default: 61000)
  --bootstrap-peer ADDR  optional validated Waku bootstrap multiaddr
  --source-dir PATH      directory containing ardentsd and ardentsctl
  --root PATH            alternate filesystem root for packaging tests
  --no-start             install files without invoking systemctl

Uninstall removes binaries and the systemd unit. Configuration, identity,
authority material, secrets, and node data are deliberately retained.
EOF
}

fail() {
    printf 'ardents-install: %s\n' "$*" >&2
    exit 1
}

command_name=${1:-}
case "$command_name" in
    install|uninstall) shift ;;
    -h|--help|help) usage; exit 0 ;;
    *) usage >&2; exit 2 ;;
esac

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
source_dir=$(CDPATH= cd -- "$script_dir/../.." && pwd)
root=/
node_name=
transport_port=61000
bootstrap_peer=
no_start=false

while [ "$#" -gt 0 ]; do
    case "$1" in
        --node-name) [ "$#" -ge 2 ] || fail "--node-name requires a value"; node_name=$2; shift 2 ;;
        --transport-port) [ "$#" -ge 2 ] || fail "--transport-port requires a value"; transport_port=$2; shift 2 ;;
        --bootstrap-peer) [ "$#" -ge 2 ] || fail "--bootstrap-peer requires a value"; bootstrap_peer=$2; shift 2 ;;
        --source-dir) [ "$#" -ge 2 ] || fail "--source-dir requires a value"; source_dir=$2; shift 2 ;;
        --root) [ "$#" -ge 2 ] || fail "--root requires a value"; root=$2; shift 2 ;;
        --no-start) no_start=true; shift ;;
        -h|--help) usage; exit 0 ;;
        *) fail "unknown option: $1" ;;
    esac
done

[ "$(id -u)" -eq 0 ] || fail "run as root"
case "$root" in
    /*) ;;
    *) fail "--root must be an absolute path" ;;
esac
root=${root%/}
[ -n "$root" ] || root=/
path_root=$root
[ "$path_root" = / ] && path_root=

bin_dir="$path_root/usr/local/bin"
config_dir="$path_root/etc/ardents"
state_dir="$path_root/var/lib/ardents"
secret_dir="$state_dir/secrets"
authority_dir="$path_root/var/lib/ardents-authority"
unit_dir="$path_root/etc/systemd/system"
unit_path="$unit_dir/ardentsd.service"
config_path="$config_dir/operator.json"

systemctl_available=false
if [ "$no_start" = false ]; then
    [ "$root" = / ] || fail "--root requires --no-start"
    command -v systemctl >/dev/null 2>&1 || fail "systemctl is required unless --no-start is explicit"
    systemctl_available=true
fi

install_binaries() {
    for name in ardentsd ardentsctl; do
        source_path="$source_dir/$name"
        [ -f "$source_path" ] || fail "missing release binary: $source_path"
        install -m 0755 "$source_path" "$bin_dir/$name.new"
    done
    if ! daemon_identity=$("$bin_dir/ardentsd.new" --version) ||
        ! client_identity=$("$bin_dir/ardentsctl.new" --output json version); then
        rm -f "$bin_dir/ardentsd.new" "$bin_dir/ardentsctl.new"
        fail "staged executables failed their role/version preflight"
    fi
    if [ "$daemon_identity" != "$client_identity" ]; then
        rm -f "$bin_dir/ardentsd.new" "$bin_dir/ardentsctl.new"
        fail "ardentsd and ardentsctl do not have the same build identity"
    fi
    for name in ardentsd ardentsctl; do
        if [ -e "$bin_dir/$name" ]; then
            cp -p "$bin_dir/$name" "$bin_dir/$name.previous"
        fi
    done
    if ! mv -f "$bin_dir/ardentsd.new" "$bin_dir/ardentsd"; then
        restore_previous_binaries
        fail "replace ardentsd"
    fi
    if ! mv -f "$bin_dir/ardentsctl.new" "$bin_dir/ardentsctl"; then
        rm -f "$bin_dir/ardentsd"
        restore_previous_binaries
        fail "replace ardentsctl"
    fi
    rm -f "$bin_dir/ardentsd.previous" "$bin_dir/ardentsctl.previous"
}

restore_previous_binaries() {
    for name in ardentsd ardentsctl; do
        if [ -e "$bin_dir/$name.previous" ]; then
            mv -f "$bin_dir/$name.previous" "$bin_dir/$name"
        fi
        rm -f "$bin_dir/$name.new"
    done
}

ensure_service_account() {
    if [ "$root" != / ]; then
        return
    fi
    for required_command in getent groupadd useradd; do
        command -v "$required_command" >/dev/null 2>&1 || fail "required account-management command is unavailable: $required_command"
    done
    if ! getent group ardents >/dev/null 2>&1; then
        groupadd --system ardents
    fi
    if ! id ardents >/dev/null 2>&1; then
        useradd --system --gid ardents --home-dir /var/lib/ardents --shell /usr/sbin/nologin ardents
    fi
    account=$(getent passwd ardents) || fail "cannot resolve ardents service account"
    account_group=$(printf '%s' "$account" | cut -d: -f4)
    expected_group=$(getent group ardents | cut -d: -f3)
    [ "$account_group" = "$expected_group" ] || fail "existing ardents account must use the ardents primary group"
    account_home=$(printf '%s' "$account" | cut -d: -f6)
    [ "$account_home" = /var/lib/ardents ] || fail "existing ardents account must use /var/lib/ardents as its home"
    account_shell=$(printf '%s' "$account" | cut -d: -f7)
    case "$account_shell" in
        */nologin|*/false) ;;
        *) fail "existing ardents account must have a locked login shell" ;;
    esac
}

if [ "$command_name" = uninstall ]; then
    if [ "$systemctl_available" = true ]; then
        if systemctl is-active --quiet ardentsd.service; then
            systemctl stop ardentsd.service
        fi
        if systemctl is-enabled --quiet ardentsd.service; then
            systemctl disable ardentsd.service >/dev/null
        fi
    fi
    rm -f "$unit_path" "$bin_dir/ardentsd" "$bin_dir/ardentsctl"
    if [ "$systemctl_available" = true ]; then
        systemctl daemon-reload
    fi
    printf 'ardents-install: uninstalled; retained config=%s state=%s\n' "$config_dir" "$state_dir"
    exit 0
fi

case "$transport_port" in
    ''|*[!0-9]*) fail "--transport-port must be an integer from 1 to 65535" ;;
esac
[ "$transport_port" -ge 1 ] && [ "$transport_port" -le 65535 ] || fail "--transport-port must be an integer from 1 to 65535"

unit_source="$source_dir/systemd/ardentsd.service"
if [ ! -f "$unit_source" ]; then
    unit_source="$script_dir/../../deploy/systemd/ardentsd.service"
fi
[ -f "$unit_source" ] || fail "missing systemd unit template"

ensure_service_account
install -d -m 0755 "$bin_dir" "$unit_dir"
install -d -m 0750 "$config_dir"
install -d -m 0750 "$state_dir"
install -d -m 0700 "$secret_dir" "$authority_dir"
install_binaries

if [ ! -f "$config_path" ]; then
    [ -n "$node_name" ] || fail "--node-name is required for first install"
    if [ -n "$bootstrap_peer" ]; then
        "$bin_dir/ardentsd" init \
            --authority-dir "$authority_dir" --node-dir "$state_dir" --secret-dir "$secret_dir" \
            --node-name "$node_name" --transport-port "$transport_port" \
            --runtime-data-dir /var/lib/ardents --runtime-secret-dir /var/lib/ardents/secrets \
            --bootstrap-peer "$bootstrap_peer"
    else
        "$bin_dir/ardentsd" init \
            --authority-dir "$authority_dir" --node-dir "$state_dir" --secret-dir "$secret_dir" \
            --node-name "$node_name" --transport-port "$transport_port" \
            --runtime-data-dir /var/lib/ardents --runtime-secret-dir /var/lib/ardents/secrets
    fi
    mv "$secret_dir/operator.json" "$config_path"
fi

install -m 0644 "$unit_source" "$unit_path"
chmod 0600 "$config_path"
if [ "$root" = / ]; then
    chown -R ardents:ardents "$state_dir" "$config_dir"
    chown -R root:root "$authority_dir"
    chmod 0700 "$authority_dir"
fi

if [ "$systemctl_available" = true ]; then
    systemctl daemon-reload
    systemctl enable ardentsd.service >/dev/null
    if systemctl is-active --quiet ardentsd.service; then
        systemctl restart ardentsd.service
    else
        systemctl start ardentsd.service
    fi
fi

printf 'ardents-install: installed config=%s state=%s service=ardentsd.service\n' "$config_path" "$state_dir"
