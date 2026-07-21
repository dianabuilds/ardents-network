#!/bin/sh
set -eu

usage() {
    cat <<'EOF'
Usage:
  linux.sh install [options]
  linux.sh upgrade [--source-dir PATH] [--backup PATH]
  linux.sh rollback
  linux.sh backup --output /var/backups/ardents/NAME.tar.gz
  linux.sh restore --archive PATH
  linux.sh uninstall [--root PATH] [--no-start]

Install options:
  --node-name NAME       canonical node name; required for first install
  --transport-port PORT  Waku TCP listen port (default: 61000)
  --bootstrap-peer ADDR  optional validated Waku bootstrap multiaddr
  --source-dir PATH      directory containing ardentsd and ardentsctl
  --root PATH            alternate filesystem root for packaging tests
  --no-start             operate without systemd (packaging tests only)

Upgrade creates a stopped-node backup, retains one previous binary pair, and
rolls back automatically unless the new daemon reaches local API readiness.
Restore accepts only an empty target and deliberately leaves the service stopped.
Uninstall retains configuration, identity, authority material, secrets, and data.
EOF
}

fail() { printf 'ardents-install: %s\n' "$*" >&2; exit 1; }

command_name=${1:-}
case "$command_name" in
    install|upgrade|rollback|backup|restore|uninstall) shift ;;
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
backup_path=
archive_path=

while [ "$#" -gt 0 ]; do
    case "$1" in
        --node-name) [ "$#" -ge 2 ] || fail "--node-name requires a value"; node_name=$2; shift 2 ;;
        --transport-port) [ "$#" -ge 2 ] || fail "--transport-port requires a value"; transport_port=$2; shift 2 ;;
        --bootstrap-peer) [ "$#" -ge 2 ] || fail "--bootstrap-peer requires a value"; bootstrap_peer=$2; shift 2 ;;
        --source-dir) [ "$#" -ge 2 ] || fail "--source-dir requires a value"; source_dir=$2; shift 2 ;;
        --root) [ "$#" -ge 2 ] || fail "--root requires a value"; root=$2; shift 2 ;;
        --no-start) no_start=true; shift ;;
        --backup|--output) [ "$#" -ge 2 ] || fail "$1 requires a value"; backup_path=$2; shift 2 ;;
        --archive) [ "$#" -ge 2 ] || fail "--archive requires a value"; archive_path=$2; shift 2 ;;
        -h|--help) usage; exit 0 ;;
        *) fail "unknown option: $1" ;;
    esac
done

[ "$(id -u)" -eq 0 ] || fail "run as root"
case "$root" in /*) ;; *) fail "--root must be an absolute path" ;; esac
root=${root%/}; [ -n "$root" ] || root=/
path_root=$root; [ "$path_root" = / ] && path_root=

bin_dir="$path_root/usr/local/bin"
config_dir="$path_root/etc/ardents"
state_dir="$path_root/var/lib/ardents"
secret_dir="$state_dir/secrets"
application_dir="$state_dir/applications"
authority_dir="$path_root/var/lib/ardents-authority"
upgrade_dir="$path_root/var/lib/ardents-upgrades"
backup_dir="$path_root/var/backups/ardents"
unit_dir="$path_root/etc/systemd/system"
unit_path="$unit_dir/ardentsd.service"
config_path="$config_dir/operator.json"
token_file="$secret_dir/api-token"
socket_path="$secret_dir/control.sock"

systemctl_available=false
if [ "$no_start" = false ]; then
    [ "$root" = / ] || fail "--root requires --no-start"
    command -v systemctl >/dev/null 2>&1 || fail "systemctl is required unless --no-start is explicit"
    systemctl_available=true
fi
case "$command_name" in
    upgrade|rollback|backup|restore) [ "$systemctl_available" = true ] || fail "$command_name requires systemd" ;;
esac

service_active() { systemctl is-active --quiet ardentsd.service; }
require_stopped() { ! service_active || fail "stop ardentsd.service before $1"; }

wait_ready() {
    attempt=0
    while [ "$attempt" -lt 150 ]; do
        if [ -S "$socket_path" ] && "$bin_dir/ardentsctl" --addr "unix://$socket_path" \
            --token-file "$token_file" --output json node status 2>/dev/null |
            grep -E '"ready":[[:space:]]*true' >/dev/null; then
            return 0
        fi
        attempt=$((attempt + 1)); sleep 0.1
    done
    return 1
}

tree_hash() {
    target=$1
    if [ ! -d "$target" ]; then printf '%s\n' absent; return; fi
    (CDPATH= cd -- "$target" && find . -type f -exec sha256sum {} \; | LC_ALL=C sort | sha256sum | cut -d' ' -f1)
}

file_hash() { [ -f "$1" ] && sha256sum "$1" | cut -d' ' -f1 || printf '%s\n' absent; }

manifest_value() {
    key=$1
    sed -n "s/^${key}=//p" "$2" | head -n 1
}

create_backup() {
    output=$1
    require_stopped backup
    for required_file in "$config_path" "$token_file" "$state_dir/identity_key.json" "$state_dir/waku_node_key.json" "$authority_dir/authority.json"; do
        [ -f "$required_file" ] && [ ! -L "$required_file" ] || fail "cannot back up: missing or unsafe regular file $required_file"
    done
    [ -d "$state_dir" ] && [ ! -L "$state_dir" ] || fail "cannot back up: missing or unsafe $state_dir"
    [ -d "$authority_dir" ] && [ ! -L "$authority_dir" ] || fail "cannot back up: missing or unsafe $authority_dir"
    case "$output" in "$backup_dir"/*) backup_name=${output#"$backup_dir"/} ;; *) fail "backup path must be under $backup_dir" ;; esac
    case "$backup_name" in ''|.|..|*/*) fail "backup output must be a direct child of $backup_dir" ;; esac
    install -d -m 0700 "$backup_dir"
    [ ! -L "$backup_dir" ] || fail "backup root must not be a symbolic link"
    [ "$(stat -c '%u' "$backup_dir")" -eq 0 ] || fail "backup root must be owned by root"
    chmod 0700 "$backup_dir"
    [ ! -e "$output" ] && [ ! -e "$output.manifest" ] || fail "backup already exists: $output"
    tmp=$(mktemp "$backup_dir/.backup.XXXXXX") || fail "create backup temporary file"
    tmp_manifest=$(mktemp "$backup_dir/.manifest.XXXXXX") || { rm -f "$tmp"; fail "create backup manifest temporary file"; }
    trap 'rm -f "$tmp" "$tmp_manifest"' HUP INT TERM EXIT
    tar -czf "$tmp" -C "$path_root/" etc/ardents/operator.json var/lib/ardents var/lib/ardents-authority
    archive_sha=$(file_hash "$tmp")
    {
        printf 'schema=ardents.native-backup/v1\n'
        printf 'archive_sha256=%s\n' "$archive_sha"
        printf 'config_sha256=%s\n' "$(file_hash "$config_path")"
        printf 'identity_sha256=%s\n' "$(file_hash "$state_dir/identity_key.json")"
        printf 'waku_identity_sha256=%s\n' "$(file_hash "$state_dir/waku_node_key.json")"
        printf 'authority_sha256=%s\n' "$(tree_hash "$authority_dir")"
    } >"$tmp_manifest"
    chmod 0600 "$tmp" "$tmp_manifest"
    if ! ln "$tmp" "$output"; then fail "publish backup archive without overwrite"; fi
    if ! ln "$tmp_manifest" "$output.manifest"; then rm -f "$output"; fail "publish backup manifest without overwrite"; fi
    rm -f "$tmp" "$tmp_manifest"
    trap - HUP INT TERM EXIT
    printf 'ardents-install: backup=%s manifest=%s\n' "$output" "$output.manifest"
}

validate_archive_paths() {
    tar -tvzf "$1" | awk '{ kind=substr($1,1,1); if (kind != "-" && kind != "d") exit 1 }' || return 1
    tar -tzf "$1" | while IFS= read -r entry; do
        case "$entry" in
            /*|../*|*/../*|*/..) exit 1 ;;
            etc/ardents/operator.json|var/lib/ardents|var/lib/ardents/*|var/lib/ardents-authority|var/lib/ardents-authority/*) ;;
            *) exit 1 ;;
        esac
    done
}

directory_empty() { [ ! -d "$1" ] || [ -z "$(find "$1" -mindepth 1 -maxdepth 1 -print -quit)" ]; }

restore_backup() {
    archive=$1; manifest="$archive.manifest"
    require_stopped restore
    [ -f "$archive" ] || fail "backup archive not found: $archive"
    [ -f "$manifest" ] || fail "backup manifest not found: $manifest"
    directory_empty "$config_dir" || fail "restore target is not empty: $config_dir"
    directory_empty "$state_dir" || fail "restore target is not empty: $state_dir"
    directory_empty "$authority_dir" || fail "restore target is not empty: $authority_dir"
    install -d -m 0700 "$upgrade_dir"
    staging=$(mktemp -d "$upgrade_dir/restore.XXXXXX") || fail "create restore staging directory"
    trap 'rm -rf "$staging"' HUP INT TERM EXIT
    archive_snapshot="$staging/archive.tar.gz"
    manifest_snapshot="$staging/archive.manifest"
    install -m 0600 "$archive" "$archive_snapshot"
    install -m 0600 "$manifest" "$manifest_snapshot"
    [ "$(manifest_value schema "$manifest_snapshot")" = ardents.native-backup/v1 ] || fail "unsupported backup manifest"
    [ "$(file_hash "$archive_snapshot")" = "$(manifest_value archive_sha256 "$manifest_snapshot")" ] || fail "backup checksum mismatch"
    validate_archive_paths "$archive_snapshot" || fail "backup contains an unsafe or unexpected path"
    content="$staging/content"; install -d -m 0700 "$content"
    tar -xzf "$archive_snapshot" -C "$content"
    staged_config="$content/etc/ardents/operator.json"
    staged_state="$content/var/lib/ardents"
    staged_authority="$content/var/lib/ardents-authority"
    [ "$(file_hash "$staged_config")" = "$(manifest_value config_sha256 "$manifest_snapshot")" ] || fail "restored config checksum mismatch"
    [ "$(file_hash "$staged_state/identity_key.json")" = "$(manifest_value identity_sha256 "$manifest_snapshot")" ] || fail "restored identity checksum mismatch"
    [ "$(file_hash "$staged_state/waku_node_key.json")" = "$(manifest_value waku_identity_sha256 "$manifest_snapshot")" ] || fail "restored Waku identity checksum mismatch"
    [ "$(tree_hash "$staged_authority")" = "$(manifest_value authority_sha256 "$manifest_snapshot")" ] || fail "restored authority checksum mismatch"
    install -d -m 0750 "$config_dir" "$state_dir"; install -d -m 0700 "$authority_dir"
    cp -a "$staged_state/." "$state_dir/"; cp -a "$staged_authority/." "$authority_dir/"
    install -m 0600 "$staged_config" "$config_path"
    chown -R ardents:ardents "$state_dir" "$config_dir"
    chown -R root:root "$authority_dir"; chmod 0700 "$authority_dir"
    rm -rf "$staging"; trap - HUP INT TERM EXIT
    printf 'ardents-install: restored=%s; service remains stopped\n' "$archive"
}

stage_binaries() {
    install -d -m 0755 "$bin_dir"
    for name in ardentsd ardentsctl; do
        [ -f "$source_dir/$name" ] || fail "missing release binary: $source_dir/$name"
        install -m 0755 "$source_dir/$name" "$bin_dir/$name.new"
    done
    if ! daemon_identity=$("$bin_dir/ardentsd.new" --version) ||
        ! client_identity=$("$bin_dir/ardentsctl.new" --output json version); then
        rm -f "$bin_dir/ardentsd.new" "$bin_dir/ardentsctl.new"
        fail "staged executables failed their role/version preflight"
    fi
    [ "$daemon_identity" = "$client_identity" ] || { rm -f "$bin_dir/ardentsd.new" "$bin_dir/ardentsctl.new"; fail "ardentsd and ardentsctl do not have the same build identity"; }
}

replace_staged_binaries() {
    destination=$1
    install -d -m 0700 "$destination" || return 2
    for name in ardentsd ardentsctl; do
        if [ -e "$bin_dir/$name" ] && ! cp -p "$bin_dir/$name" "$destination/$name"; then
            return 1
        fi
    done
    mv -f "$bin_dir/ardentsd.new" "$bin_dir/ardentsd" || return 1
    if ! mv -f "$bin_dir/ardentsctl.new" "$bin_dir/ardentsctl"; then
        if [ -f "$destination/ardentsd" ] && ! cp -p "$destination/ardentsd" "$bin_dir/ardentsd"; then
            return 2
        fi
        return 1
    fi
}

restore_binary_pair() {
    from=$1
    [ -x "$from/ardentsd" ] && [ -x "$from/ardentsctl" ] || return 1
    suffix=$$
    for name in ardentsd ardentsctl; do
        install -m 0755 "$from/$name" "$bin_dir/$name.restore.$suffix" || return 1
        cp -p "$bin_dir/$name" "$bin_dir/$name.current.$suffix" || return 1
    done
    restore_daemon_identity=$("$bin_dir/ardentsd.restore.$suffix" --version) || return 1
    restore_client_identity=$("$bin_dir/ardentsctl.restore.$suffix" --output json version) || return 1
    [ "$restore_daemon_identity" = "$restore_client_identity" ] || return 1
    mv -f "$bin_dir/ardentsd.restore.$suffix" "$bin_dir/ardentsd" || return 1
    if ! mv -f "$bin_dir/ardentsctl.restore.$suffix" "$bin_dir/ardentsctl"; then
        cp -p "$bin_dir/ardentsd.current.$suffix" "$bin_dir/ardentsd" || return 2
        return 1
    fi
    rm -f "$bin_dir/ardentsd.current.$suffix" "$bin_dir/ardentsctl.current.$suffix" || true
}

ensure_service_account() {
    [ "$root" = / ] || return 0
    for required_command in getent groupadd useradd; do command -v "$required_command" >/dev/null 2>&1 || fail "required account-management command is unavailable: $required_command"; done
    getent group ardents >/dev/null 2>&1 || groupadd --system ardents
    getent group ardents-apps >/dev/null 2>&1 || groupadd --system ardents-apps
    id ardents >/dev/null 2>&1 || useradd --system --gid ardents --home-dir /var/lib/ardents --shell /usr/sbin/nologin ardents
    account=$(getent passwd ardents) || fail "cannot resolve ardents service account"
    [ "$(printf '%s' "$account" | cut -d: -f4)" = "$(getent group ardents | cut -d: -f3)" ] || fail "existing ardents account must use the ardents primary group"
    [ "$(printf '%s' "$account" | cut -d: -f6)" = /var/lib/ardents ] || fail "existing ardents account must use /var/lib/ardents as its home"
    case "$(printf '%s' "$account" | cut -d: -f7)" in */nologin|*/false) ;; *) fail "existing ardents account must have a locked login shell" ;; esac
}

if [ "$command_name" = uninstall ]; then
    if [ "$systemctl_available" = true ]; then
        service_active && systemctl stop ardentsd.service
        if systemctl is-enabled --quiet ardentsd.service; then
            systemctl disable ardentsd.service >/dev/null
        fi
    fi
    rm -f "$unit_path" "$bin_dir/ardentsd" "$bin_dir/ardentsctl"
    [ "$systemctl_available" = false ] || systemctl daemon-reload
    printf 'ardents-install: uninstalled; retained config=%s state=%s\n' "$config_dir" "$state_dir"; exit 0
fi

if [ "$command_name" = backup ]; then [ -n "$backup_path" ] || fail "backup requires --output PATH"; create_backup "$backup_path"; exit 0; fi
if [ "$command_name" = restore ]; then [ -n "$archive_path" ] || fail "restore requires --archive PATH"; ensure_service_account; restore_backup "$archive_path"; exit 0; fi

if [ "$command_name" = upgrade ]; then
    [ -x "$bin_dir/ardentsd" ] && [ -x "$bin_dir/ardentsctl" ] && [ -f "$unit_path" ] || fail "upgrade requires an existing native installation"
    stage_binaries
    was_active=false; service_active && was_active=true
    [ "$was_active" = false ] || systemctl stop ardentsd.service
    if [ -z "$backup_path" ]; then backup_path="$backup_dir/pre-upgrade-$(date -u +%Y%m%dT%H%M%SZ)-$$.tar.gz"; fi
    if ! (create_backup "$backup_path"); then
        rm -f "$bin_dir/ardentsd.new" "$bin_dir/ardentsctl.new"
        [ "$was_active" = false ] || systemctl start ardentsd.service
        fail "upgrade backup failed; existing binaries were not changed"
    fi
    transaction="$upgrade_dir/transaction.$$"
    rm -rf "$transaction"; install -d -m 0700 "$transaction"
    replace_status=0
    replace_staged_binaries "$transaction" || replace_status=$?
    if [ "$replace_status" -ne 0 ]; then
        if [ "$replace_status" -eq 1 ] && [ "$was_active" = true ]; then
            systemctl start ardentsd.service
        fi
        fail "could not activate staged binary pair; service left stopped if compensation failed; transaction evidence retained at $transaction"
    fi
    systemctl daemon-reload
    if ! systemctl start ardentsd.service || ! wait_ready; then
        systemctl stop ardentsd.service >/dev/null 2>&1 || true
        restore_binary_pair "$transaction" || fail "upgrade failed and previous binary pair could not be restored"
        systemctl start ardentsd.service || fail "upgrade failed; previous binaries restored but service did not start"
        wait_ready || fail "upgrade failed; previous binaries restored but readiness did not recover"
        [ "$was_active" = true ] || systemctl stop ardentsd.service
        rm -rf "$transaction"
        fail "upgrade failed readiness; previous version restored and healthy; backup=$backup_path"
    fi
    [ "$was_active" = true ] || systemctl stop ardentsd.service
    rm -rf "$upgrade_dir/previous"; mv "$transaction" "$upgrade_dir/previous"
    printf 'ardents-install: upgraded; rollback=%s backup=%s\n' "$upgrade_dir/previous" "$backup_path"; exit 0
fi

if [ "$command_name" = rollback ]; then
    [ -x "$upgrade_dir/previous/ardentsd" ] && [ -x "$upgrade_dir/previous/ardentsctl" ] || fail "no previous native binary pair is available"
    was_active=false; service_active && was_active=true
    [ "$was_active" = false ] || systemctl stop ardentsd.service
    current="$upgrade_dir/current.$$"; install -d -m 0700 "$current"
    cp -p "$bin_dir/ardentsd" "$bin_dir/ardentsctl" "$current/"
    restore_binary_pair "$upgrade_dir/previous" || fail "could not restore previous binary pair"
    if ! systemctl start ardentsd.service || ! wait_ready; then
        systemctl stop ardentsd.service >/dev/null 2>&1 || true
        restore_binary_pair "$current" || fail "rollback failed and current binary pair could not be restored"
        systemctl start ardentsd.service || fail "rollback failed; current binaries restored but service did not start"
        wait_ready || fail "rollback failed; current binaries restored but readiness did not recover"
        [ "$was_active" = true ] || systemctl stop ardentsd.service
        rm -rf "$current"; fail "rollback candidate failed readiness; current version restored and healthy"
    fi
    [ "$was_active" = true ] || systemctl stop ardentsd.service
    rm -rf "$upgrade_dir/previous"; mv "$current" "$upgrade_dir/previous"
    printf 'ardents-install: rollback complete; displaced version retained for undo\n'; exit 0
fi

case "$transport_port" in ''|*[!0-9]*) fail "--transport-port must be an integer from 1 to 65535" ;; esac
[ "$transport_port" -ge 1 ] && [ "$transport_port" -le 65535 ] || fail "--transport-port must be an integer from 1 to 65535"
unit_source="$source_dir/systemd/ardentsd.service"; [ -f "$unit_source" ] || unit_source="$script_dir/../../deploy/systemd/ardentsd.service"
[ -f "$unit_source" ] || fail "missing systemd unit template"
ensure_service_account
install -d -m 0755 "$bin_dir" "$unit_dir"; install -d -m 0750 "$config_dir" "$state_dir"; install -d -m 0700 "$secret_dir" "$application_dir" "$authority_dir"
stage_binaries
if [ -x "$bin_dir/ardentsd" ] || [ -x "$bin_dir/ardentsctl" ]; then
    if [ ! -x "$bin_dir/ardentsd" ] || [ ! -x "$bin_dir/ardentsctl" ]; then
        rm -f "$bin_dir/ardentsd.new" "$bin_dir/ardentsctl.new"
        fail "installed native binary pair is incomplete; preserve evidence and repair explicitly"
    fi
    current_daemon_identity=$("$bin_dir/ardentsd" --version) || { rm -f "$bin_dir/ardentsd.new" "$bin_dir/ardentsctl.new"; fail "installed ardentsd identity is unreadable"; }
    current_client_identity=$("$bin_dir/ardentsctl" --output json version) || { rm -f "$bin_dir/ardentsd.new" "$bin_dir/ardentsctl.new"; fail "installed ardentsctl identity is unreadable"; }
    if [ "$current_daemon_identity" != "$current_client_identity" ]; then
        rm -f "$bin_dir/ardentsd.new" "$bin_dir/ardentsctl.new"
        fail "installed native binary pair has mismatched build identities"
    fi
    if [ "$current_daemon_identity" != "$daemon_identity" ]; then
        rm -f "$bin_dir/ardentsd.new" "$bin_dir/ardentsctl.new"
        fail "candidate build differs from installed build; use the upgrade command"
    fi
fi
replace_staged_binaries "$upgrade_dir/install-previous" || fail "replace native binary pair"

if [ ! -f "$config_path" ]; then
    [ -n "$node_name" ] || fail "--node-name is required for first install"
    if [ -n "$bootstrap_peer" ]; then
        "$bin_dir/ardentsd" init --authority-dir "$authority_dir" --node-dir "$state_dir" --secret-dir "$secret_dir" --node-name "$node_name" --transport-port "$transport_port" --runtime-data-dir /var/lib/ardents --runtime-secret-dir /var/lib/ardents/secrets --bootstrap-peer "$bootstrap_peer"
    else
        "$bin_dir/ardentsd" init --authority-dir "$authority_dir" --node-dir "$state_dir" --secret-dir "$secret_dir" --node-name "$node_name" --transport-port "$transport_port" --runtime-data-dir /var/lib/ardents --runtime-secret-dir /var/lib/ardents/secrets
    fi
    mv "$secret_dir/operator.json" "$config_path"
fi
install -m 0644 "$unit_source" "$unit_path"; chmod 0600 "$config_path"
if [ "$root" = / ]; then
    chown -R ardents:ardents "$state_dir" "$config_dir"
    chown ardents:ardents-apps "$application_dir"; chmod 2750 "$application_dir"
    if [ -f "$application_dir/application-token" ]; then
        chown ardents:ardents-apps "$application_dir/application-token"; chmod 0640 "$application_dir/application-token"
    fi
    chown -R root:root "$authority_dir"; chmod 0700 "$authority_dir"
fi
if [ "$systemctl_available" = true ]; then
    systemctl daemon-reload
    systemctl enable ardentsd.service >/dev/null
    if service_active; then systemctl restart ardentsd.service; else systemctl start ardentsd.service; fi
fi
printf 'ardents-install: installed config=%s state=%s service=ardentsd.service\n' "$config_path" "$state_dir"
