#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo 'usage: run-ubuntu-lifecycle.sh V1_BINARY V2_BINARY' >&2
  exit 64
fi
if [[ $(id -u) -eq 0 ]]; then
  echo 'refusing root lifecycle run' >&2
  exit 65
fi

source_v1=$1
source_v2=$2
. /etc/os-release
os_id=${ID:-unknown}
os_version=${VERSION_ID:-unknown}
name=ardents-r095-lifecycle-20260824
unit=$name.service
fixture_root=/tmp/$name
program_root=$fixture_root/program
incoming=$fixture_root/incoming
gnupg=$fixture_root/gnupg
config_base=${XDG_CONFIG_HOME:-$HOME/.config}
state_base=${XDG_STATE_HOME:-$HOME/.local/state}
cache_base=${XDG_CACHE_HOME:-$HOME/.cache}
runtime_base=${XDG_RUNTIME_DIR:-}
config_root=$config_base/$name
state_root=$state_base/$name
cache_root=$cache_base/$name
runtime_root=$runtime_base/$name
unit_directory=$config_base/systemd/user
unit_file=$unit_directory/$unit
wants_link=$unit_directory/default.target.wants/$unit
installed=$program_root/ardents-endpoint-fixture
staged=$program_root/.ardents-endpoint-fixture.next
cleanup_failed=0

for path in "$config_base" "$state_base" "$cache_base" "$runtime_base"; do
  if [[ -z $path || $path != /* ]]; then
    echo "invalid XDG base path: $path" >&2
    exit 66
  fi
done
if ! systemctl --user is-system-running >/dev/null 2>&1; then
  echo 'systemd user manager is unavailable' >&2
  exit 67
fi
runtime_owner=$(stat -c %u "$runtime_base")
runtime_mode=$(stat -c %a "$runtime_base")
if [[ $runtime_owner != "$(id -u)" || $runtime_mode != 700 ]]; then
  echo "invalid XDG runtime directory owner=$runtime_owner mode=$runtime_mode" >&2
  exit 68
fi

for path in "$fixture_root" "$config_root" "$state_root" "$cache_root" "$runtime_root" "$unit_file" "$wants_link"; do
  if [[ -e $path || -L $path ]]; then
    echo "refusing occupied lifecycle path: $path" >&2
    exit 69
  fi
done

safe_remove_tree() {
  local path=$1
  if [[ ! -e $path ]]; then
    return
  fi
  if [[ -L $path ]]; then
    echo "refusing symlink cleanup target: $path" >&2
    cleanup_failed=1
    return
  fi
  local parent target actual
  parent=$(realpath "$(dirname "$path")")
  target=$parent/$(basename "$path")
  actual=$(realpath "$path")
  if [[ $actual != "$target" ]]; then
    echo "refusing unexpected cleanup target: $actual" >&2
    cleanup_failed=1
    return
  fi
  rm -rf -- "$actual"
}

cleanup() {
  systemctl --user stop "$unit" >/dev/null 2>&1 || true
  systemctl --user disable "$unit" >/dev/null 2>&1 || true
  if [[ -f $unit_file && ! -L $unit_file ]]; then
    rm -f -- "$unit_file"
  elif [[ -e $unit_file || -L $unit_file ]]; then
    echo "refusing unexpected unit cleanup target: $unit_file" >&2
    cleanup_failed=1
  fi
  systemctl --user daemon-reload >/dev/null 2>&1 || true
  safe_remove_tree "$runtime_root"
  safe_remove_tree "$cache_root"
  safe_remove_tree "$state_root"
  safe_remove_tree "$config_root"
  gpgconf --homedir "$gnupg" --kill all >/dev/null 2>&1 || true
  safe_remove_tree "$fixture_root"
  if [[ $cleanup_failed -ne 0 ]]; then
    echo 'lifecycle cleanup was incomplete' >&2
  fi
}
trap cleanup EXIT

mkdir -p -- "$unit_directory" "$state_base" "$cache_base"
mkdir -m 700 -- "$fixture_root" "$program_root" "$incoming" "$gnupg"
mkdir -m 700 -- "$config_root" "$state_root" "$cache_root" "$runtime_root"
cp -- "$source_v1" "$incoming/v1"
cp -- "$source_v2" "$incoming/v2"
chmod 700 "$incoming/v1" "$incoming/v2"

if ! file "$incoming/v1" | grep -q 'ELF 64-bit.*x86-64'; then
  echo 'v1 fixture is not Linux x86-64' >&2
  exit 70
fi
if ! file "$incoming/v2" | grep -q 'ELF 64-bit.*x86-64'; then
  echo 'v2 fixture is not Linux x86-64' >&2
  exit 70
fi
v1_digest=$(sha256sum "$incoming/v1" | awk '{print $1}')
v2_digest=$(sha256sum "$incoming/v2" | awk '{print $1}')
if [[ $v1_digest == "$v2_digest" ]]; then
  echo 'fixture releases have the same digest' >&2
  exit 71
fi

export GNUPGHOME=$gnupg
gpg --batch --passphrase '' --quick-generate-key \
  'R-095 Lifecycle Fixture <r095-lifecycle@example.invalid>' ed25519 sign 0 >/dev/null 2>&1
fingerprint=$(gpg --batch --with-colons --list-keys 'r095-lifecycle@example.invalid' |
  awk -F: '$1 == "fpr" {print $10; exit}')
gpg --batch --export "$fingerprint" >"$fixture_root/trustedkeys.gpg"
for release in v1 v2; do
  gpg --batch --armor --detach-sign --local-user "$fingerprint" \
    --output "$incoming/$release.asc" "$incoming/$release"
  gpgv --keyring "$fixture_root/trustedkeys.gpg" "$incoming/$release.asc" \
    "$incoming/$release" >/dev/null 2>&1
done
cp -- "$incoming/v2" "$incoming/v2-corrupt"
printf 'changed\n' >>"$incoming/v2-corrupt"
if gpgv --keyring "$fixture_root/trustedkeys.gpg" "$incoming/v2.asc" \
  "$incoming/v2-corrupt" >/dev/null 2>&1; then
  echo 'corrupt fixture unexpectedly verified' >&2
  exit 72
fi

replace_verified() {
  local candidate=$1 signature=$2 inject=${3:-commit}
  if systemctl --user is-active --quiet "$unit"; then
    return 73
  fi
  gpgv --keyring "$fixture_root/trustedkeys.gpg" "$signature" "$candidate" \
    >/dev/null 2>&1 || return 74
  install -m 700 -- "$candidate" "$staged"
  sync -f "$staged"
  if [[ $inject == before-rename ]]; then
    return 75
  fi
  mv -f -- "$staged" "$installed"
  sync -f "$program_root"
}

replace_verified "$incoming/v1" "$incoming/v1.asc"
cat >"$unit_file" <<EOF
[Unit]
Description=Ardents R-095 disposable user lifecycle fixture

[Service]
Type=exec
ExecStart=$installed -mode serve -state-root $state_root -runtime-root $runtime_root
Restart=no
UMask=0077
NoNewPrivileges=yes
TimeoutStartSec=5s
TimeoutStopSec=5s

[Install]
WantedBy=default.target
EOF
chmod 600 "$unit_file"
systemd-analyze --user verify "$unit_file" >/dev/null
systemctl --user daemon-reload
systemctl --user enable "$unit" >/dev/null
enabled=$(systemctl --user is-enabled "$unit")
unit_mode=$(stat -c %a "$unit_file")
linger_before=$(loginctl show-user "$(id -u)" -p Linger --value)

probe_until_ready() {
  local proof=''
  for _ in $(seq 1 100); do
    if proof=$("$installed" -mode probe -runtime-root "$runtime_root" 2>/dev/null); then
      printf '%s\n' "$proof"
      return 0
    fi
    sleep 0.05
  done
  systemctl --user status "$unit" --no-pager >&2 || true
  return 1
}

json_field() {
  local json=$1 field=$2
  printf '%s\n' "$json" | sed -n "s/.*\"$field\":\"\([^\"]*\)\".*/\1/p"
}
json_number() {
  local json=$1 field=$2
  printf '%s\n' "$json" | sed -n "s/.*\"$field\":\([0-9][0-9]*\).*/\1/p"
}

systemctl --user start "$unit"
proof1=$(probe_until_ready)
identity=$(json_field "$proof1" identity)
[[ $(json_field "$proof1" build) == v1 ]]
[[ $(json_number "$proof1" starts) == 1 ]]
[[ $(json_number "$proof1" floor) == 7 ]]
systemctl --user stop "$unit"
[[ $(systemctl --user show "$unit" -p ActiveState --value) == inactive ]]
[[ ! -e $runtime_root/endpoint.sock && ! -e $runtime_root/ready.json ]]

systemctl --user start "$unit"
proof2=$(probe_until_ready)
[[ $(json_field "$proof2" build) == v1 ]]
[[ $(json_field "$proof2" identity) == "$identity" ]]
[[ $(json_number "$proof2" starts) == 2 ]]
[[ $(json_number "$proof2" floor) == 7 ]]
installed_v1=$(sha256sum "$installed" | awk '{print $1}')
if replace_verified "$incoming/v2" "$incoming/v2.asc"; then
  echo 'active replacement unexpectedly succeeded' >&2
  exit 76
else
  active_refusal=$?
fi
[[ $active_refusal == 73 ]]
[[ $(sha256sum "$installed" | awk '{print $1}') == "$installed_v1" ]]
systemctl --user stop "$unit"

if replace_verified "$incoming/v2-corrupt" "$incoming/v2.asc"; then
  echo 'corrupt replacement unexpectedly succeeded' >&2
  exit 77
else
  corrupt_refusal=$?
fi
[[ $corrupt_refusal == 74 ]]
[[ $(sha256sum "$installed" | awk '{print $1}') == "$installed_v1" ]]

if replace_verified "$incoming/v2" "$incoming/v2.asc" before-rename; then
  echo 'interrupted replacement unexpectedly committed' >&2
  exit 78
else
  interrupted_status=$?
fi
[[ $interrupted_status == 75 && -f $staged ]]
[[ $(sha256sum "$installed" | awk '{print $1}') == "$installed_v1" ]]
rm -f -- "$staged"

replace_verified "$incoming/v2" "$incoming/v2.asc"
[[ $(sha256sum "$installed" | awk '{print $1}') == "$v2_digest" ]]
systemctl --user start "$unit"
proof3=$(probe_until_ready)
[[ $(json_field "$proof3" build) == v2 ]]
[[ $(json_field "$proof3" identity) == "$identity" ]]
[[ $(json_number "$proof3" starts) == 3 ]]
[[ $(json_number "$proof3" floor) == 7 ]]
systemctl --user stop "$unit"
systemctl --user disable "$unit" >/dev/null
rm -f -- "$unit_file"
systemctl --user daemon-reload
safe_remove_tree "$program_root"
[[ -f $state_root/fixture-state.json ]]
state_after_program_removal=true
linger_after=$(loginctl show-user "$(id -u)" -p Linger --value)
[[ $linger_after == "$linger_before" ]]

insecure=$fixture_root/insecure-state
mkdir -m 755 "$insecure"
if "$incoming/v2" -mode serve -state-root "$insecure" -runtime-root "$runtime_root" \
  >"$fixture_root/insecure.out" 2>"$fixture_root/insecure.err"; then
  echo 'insecure state root unexpectedly accepted' >&2
  exit 79
fi
grep -q 'state-root-permissions' "$fixture_root/insecure.err"

state_mode=$(stat -c %a "$state_root")
config_mode=$(stat -c %a "$config_root")
cache_mode=$(stat -c %a "$cache_root")
profile_runtime_mode=$(stat -c %a "$runtime_root")
cleanup
trap - EXIT
for path in "$fixture_root" "$config_root" "$state_root" "$cache_root" "$runtime_root" "$unit_file" "$wants_link"; do
  [[ ! -e $path && ! -L $path ]]
done
[[ $cleanup_failed -eq 0 ]]
linger_final=$(loginctl show-user "$(id -u)" -p Linger --value)
[[ $linger_final == "$linger_before" ]]

printf '{"schema":"ardents-r095-ubuntu-lifecycle-v1","passed":true,'
printf '"os":"%s","os_version":"%s","uid":%s,"systemd":"%s",' \
  "$os_id" "$os_version" "$(id -u)" "$(systemctl --version | head -n 1 | tr ' ' '_')"
printf '"linger_before":"%s","linger_after":"%s",' "$linger_before" "$linger_final"
printf '"unit_enabled":"%s","v1_sha256":"%s","v2_sha256":"%s",' \
  "$enabled" "$v1_digest" "$v2_digest"
printf '"state_identity":"%s","starts":[1,2,3],"floor":7,' "$identity"
printf '"modes":"config=%s,state=%s,cache=%s,runtime=%s,unit=%s",' \
  "$config_mode" "$state_mode" "$cache_mode" "$profile_runtime_mode" "$unit_mode"
printf '"active_replacement_refused":true,"corrupt_replacement_refused":true,'
printf '"interrupted_precommit_preserved_v1":true,"v2_activated":true,'
printf '"insecure_state_refused":true,"state_after_program_removal":%s,' \
  "$state_after_program_removal"
printf '"cleanup_complete":true}\n'
