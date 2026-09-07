#!/bin/sh
# Disposable root-run probe; inputs are an already built binary and evidence dir.
set -eu
test "$#" -eq 2 || { printf '%s\n' 'usage: run_confinement.sh PROBE_BINARY EVIDENCE_DIR' >&2; exit 2; }
test "$(id -u)" -eq 0 || { printf '%s\n' 'the system-manager probe requires root' >&2; exit 2; }
probe_binary=$(realpath -- "$1")
evidence_dir=$(realpath -- "$2")
test -f "$probe_binary" && test -d "$evidence_dir"
test ! -e "$evidence_dir/baseline.json" && test ! -e "$evidence_dir/sandbox.json"
probe_root=$(mktemp -d /tmp/ardents-r152.XXXXXXXX)
export ARDENTS_PROBE_ROOT="$probe_root"
mkdir -p "$probe_root/root/tmp" "$probe_root/root/proc" "$probe_root/root/dev" "$probe_root/root/run" "$probe_root/outside"
cp -- "$probe_binary" "$probe_root/root/probe"
chmod 0755 "$probe_root/root/probe"
printf 'synthetic probe sentinel\n' > "$probe_root/outside/sentinel"
printf '%s' 'local-channel-probe' | "$probe_root/root/probe" baseline > "$evidence_dir/baseline.json"
printf '%s' 'local-channel-probe' | systemd-run --quiet --wait --collect --pipe \
 --unit="$(basename "$probe_root")-probe" \
 -p "Environment=ARDENTS_PROBE_ROOT=$probe_root" -p WorkingDirectory=/ \
 -p "RootDirectory=$probe_root/root" -p DynamicUser=yes -p PrivateNetwork=yes \
 -p PrivateIPC=yes -p PrivateDevices=yes -p PrivateTmp=yes -p NoNewPrivileges=yes \
 -p CapabilityBoundingSet= -p AmbientCapabilities= -p RestrictAddressFamilies=AF_UNIX \
 -p RestrictNamespaces=yes -p ProtectSystem=strict -p ProtectHome=yes \
 -p ProtectControlGroups=yes -p ProtectKernelTunables=yes -p ProtectKernelModules=yes \
 -p ProtectKernelLogs=yes -p LockPersonality=yes -p RestrictSUIDSGID=yes \
 -p SystemCallArchitectures=native -p MemoryDenyWriteExecute=yes \
 -p 'SystemCallFilter=~io_uring_setup io_uring_enter io_uring_register bpf ptrace process_vm_readv process_vm_writev perf_event_open keyctl add_key request_key userfaultfd' \
 -p MemoryMax=128M -p TasksMax=32 -p CPUQuota=50% -p LimitCORE=0 \
 -p RuntimeMaxSec=15s -p TimeoutStopSec=2s -p KillMode=control-group \
 /probe sandbox > "$evidence_dir/sandbox.json"
printf 'retained probe root: %s\n' "$probe_root"
# Keep failures and the generated root for inspection; do not erase evidence.
