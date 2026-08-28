#!/bin/sh
set -eu

# This observer is deliberately read-only. The Windows owner creates and
# removes the exact A11 topology; this process only reports its Docker/cgroup
# envelope at one-second intervals.

if [ "$#" -ne 4 ]; then
  printf '%s\n' 'usage: observe-ubuntu.sh CONTAINER EXPECTED_IMAGE_ID WAIT_SECONDS LIVE_SECONDS' >&2
  exit 2
fi

container=$1
expected_image=$2
wait_seconds=$3
live_seconds=$4

case "$container" in
  ardents-h4-8-a11-[0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]*) ;;
  *) printf '%s\n' 'observer refused an unsafe A11 container name' >&2; exit 2 ;;
esac
suffix=${container#ardents-h4-8-a11-}
case "$suffix" in *[!0-9a-f]*|'') printf '%s\n' 'observer refused a non-lower-hex A11 suffix' >&2; exit 2 ;; esac
if [ "${#suffix}" -lt 12 ] || [ "${#suffix}" -gt 32 ]; then
  printf '%s\n' 'observer refused an A11 suffix outside 12-32 characters' >&2
  exit 2
fi
case "$expected_image" in sha256:[0-9a-f]*) ;;
  *) printf '%s\n' 'observer expected image ID must be sha256:lower-hex' >&2; exit 2 ;;
esac
image_hex=${expected_image#sha256:}
case "$image_hex" in *[!0-9a-f]*|'') printf '%s\n' 'observer expected image ID must be sha256:lower-hex' >&2; exit 2 ;; esac
if [ "${#image_hex}" -ne 64 ]; then
  printf '%s\n' 'observer expected image ID must contain 64 hex characters' >&2
  exit 2
fi
case "$wait_seconds:$live_seconds" in *[!0-9:]*|:*|*:) printf '%s\n' 'observer bounds must be positive decimal seconds' >&2; exit 2 ;; esac
if [ "$wait_seconds" -lt 1 ] || [ "$live_seconds" -lt 1 ]; then
  printf '%s\n' 'observer bounds must be positive decimal seconds' >&2
  exit 2
fi
if ! command -v docker >/dev/null 2>&1 || ! command -v awk >/dev/null 2>&1 || ! command -v date >/dev/null 2>&1; then
  printf '%s\n' 'observer requires docker and POSIX/coreutils tools' >&2
  exit 2
fi
if [ "$(stat -fc %T /sys/fs/cgroup 2>/dev/null || true)" != cgroup2fs ]; then
  printf '%s\n' 'observer requires cgroup v2' >&2
  exit 2
fi

value_for_key() {
  key=$1
  file=$2
  awk -v wanted="$key" '$1 == wanted { print $2; found=1; exit } END { if (!found) exit 1 }' "$file"
}

safe_number() {
  value=$1
  label=$2
  case "$value" in *[!0-9]*|'') printf 'observer %s is not numeric\n' "$label" >&2; exit 3 ;; esac
  printf '%s' "$value"
}

container_is_still_live() {
  current=$(docker container inspect --format '{{.Id}}|{{.State.Running}}' "$container" 2>/dev/null) || return 1
  [ "$current" = "$first_id|true" ]
}

finish_teardown_race_or_fail() {
  detail=$1
  if container_is_still_live; then
    printf 'observer sampling failed while container remained live: %s\n' "$detail" >&2
    exit 3
  fi
  printf 'terminal_container_transition=%s\n' "$detail" >&2
}

printf '%b\n' 'epoch_ms\tcontainer_name\tcontainer_id\timage_id\trunning\trestarting\toom_killed\trestart_count\tnano_cpus\tmemory_limit\tpids_limit\tnetwork_mode\trestart_policy\tcgroup_path\tcpu_usage_usec\tcpu_nr_throttled\tcpu_throttled_usec\tmemory_current\tmemory_max\tmemory_events_oom\tmemory_events_oom_kill\tmemory_events_max\tpids_current\tpids_max\tpids_peak\tpids_events_max\tprocess_count\tfd_count\thost_network_rx_bytes\thost_network_tx_bytes'

started=$(date +%s)
seen=0
first_id=
while :; do
  now=$(date +%s)
  elapsed=$((now - started))
  if ! inspect=$(docker container inspect --format '{{.Id}}|{{.Image}}|{{.State.Running}}|{{.State.Restarting}}|{{.State.OOMKilled}}|{{.RestartCount}}|{{.HostConfig.NanoCpus}}|{{.HostConfig.Memory}}|{{.HostConfig.PidsLimit}}|{{.HostConfig.NetworkMode}}|{{.HostConfig.RestartPolicy.Name}}|{{.State.Pid}}' "$container" 2>/dev/null); then
    if ! docker info >/dev/null 2>&1; then
      printf '%s\n' 'observer lost the Docker daemon' >&2
      exit 3
    fi
    if [ "$seen" -eq 1 ]; then
      exit 0
    fi
    if [ "$elapsed" -ge "$wait_seconds" ]; then
      printf '%s\n' 'observer never saw the selected container' >&2
      exit 3
    fi
    sleep 1
    continue
  fi

  seen=1
  old_ifs=$IFS
  IFS='|'
  set -- $inspect
  IFS=$old_ifs
  if [ "$#" -ne 12 ]; then
    printf '%s\n' 'observer received an ambiguous Docker inspection record' >&2
    exit 3
  fi
  container_id=$1
  image_id=$2
  running=$3
  restarting=$4
  oom_killed=$5
  restart_count=$6
  nano_cpus=$7
  memory_limit=$8
  pids_limit=$9
  network_mode=${10}
  restart_policy=${11}
  init_pid=${12}

  if [ -z "$first_id" ]; then first_id=$container_id; fi
  if [ "$container_id" != "$first_id" ] || [ "$image_id" != "$expected_image" ]; then
    printf '%s\n' 'observer detected container or image identity replacement' >&2
    exit 3
  fi
  if [ "$nano_cpus" != 1000000000 ] || [ "$memory_limit" != 1073741824 ] || [ "$pids_limit" != 128 ] ||
     [ "$network_mode" != host ] || [ "$restart_policy" != no ] || [ "$restart_count" != 0 ]; then
    printf '%s\n' 'observer detected a Docker limit/network/restart-policy mismatch' >&2
    exit 3
  fi
  if [ "$running" != true ]; then
    printf 'terminal_container_state id=%s running=%s restarting=%s oom_killed=%s restart_count=%s\n' \
      "$container_id" "$running" "$restarting" "$oom_killed" "$restart_count" >&2
    if [ "$restarting" != false ] || [ "$oom_killed" != false ]; then exit 3; fi
    exit 0
  fi
  safe_number "$init_pid" init_pid >/dev/null
  if ! cgroup_rel=$(awk -F: '$1 == "0" && $2 == "" { print $3; found=1; exit } END { if (!found) exit 1 }' "/proc/$init_pid/cgroup"); then
    finish_teardown_race_or_fail cgroup-path
    sleep 1
    continue
  fi
  case "$cgroup_rel" in /*) ;; *) printf '%s\n' 'observer received a non-absolute cgroup path' >&2; exit 3 ;; esac
  cgroup=/sys/fs/cgroup$cgroup_rel
  if [ ! -d "$cgroup" ]; then
    finish_teardown_race_or_fail cgroup-directory
    sleep 1
    continue
  fi

  if ! cpu_usage=$(value_for_key usage_usec "$cgroup/cpu.stat") ||
     ! cpu_nr_throttled=$(value_for_key nr_throttled "$cgroup/cpu.stat") ||
     ! cpu_throttled=$(value_for_key throttled_usec "$cgroup/cpu.stat") ||
     ! memory_current=$(cat "$cgroup/memory.current") ||
     ! memory_max=$(cat "$cgroup/memory.max") ||
     ! memory_oom=$(value_for_key oom "$cgroup/memory.events") ||
     ! memory_oom_kill=$(value_for_key oom_kill "$cgroup/memory.events") ||
     ! memory_events_max=$(value_for_key max "$cgroup/memory.events") ||
     ! pids_current=$(cat "$cgroup/pids.current") ||
     ! pids_max=$(cat "$cgroup/pids.max"); then
    finish_teardown_race_or_fail cgroup-files
    sleep 1
    continue
  fi
  pids_peak=unavailable
  if [ -r "$cgroup/pids.peak" ]; then
    if ! pids_peak=$(cat "$cgroup/pids.peak"); then
      finish_teardown_race_or_fail pids-peak
      sleep 1
      continue
    fi
  fi
  if ! pids_events_max=$(value_for_key max "$cgroup/pids.events"); then
    finish_teardown_race_or_fail pids-events
    sleep 1
    continue
  fi
  process_count=0
  fd_count=0
  if ! cgroup_pids=$(cat "$cgroup/cgroup.procs"); then
    finish_teardown_race_or_fail cgroup-processes
    sleep 1
    continue
  fi
  for pid in $cgroup_pids; do
    case "$pid" in *[!0-9]*|'') printf '%s\n' 'observer found an invalid cgroup process ID' >&2; exit 3 ;; esac
    if [ -d "/proc/$pid" ]; then
      process_count=$((process_count + 1))
      if [ ! -r "/proc/$pid/fd" ]; then
        if [ -d "/proc/$pid" ]; then
          printf '%s\n' 'observer cannot read one live container process FD directory' >&2
          exit 3
        fi
        continue
      fi
      process_fds=$(find "/proc/$pid/fd" -mindepth 1 -maxdepth 1 2>/dev/null | wc -l | awk '{print $1}')
      fd_count=$((fd_count + process_fds))
    fi
  done
  network_values=$(awk -F'[: ]+' 'NR > 2 && NF >= 11 { rx += $3; tx += $11 } END { printf "%.0f %.0f\n", rx, tx }' "/proc/$init_pid/net/dev" 2>/dev/null) || {
    finish_teardown_race_or_fail network-counters
    sleep 1
    continue
  }
  set -- $network_values
  if [ "$#" -ne 2 ]; then
    printf '%s\n' 'observer cannot read host-network counters' >&2
    exit 3
  fi
  network_rx=$1
  network_tx=$2
  epoch_ms=$(date +%s%3N)

  for pair in \
    "$epoch_ms:epoch_ms" "$restart_count:restart_count" "$nano_cpus:nano_cpus" "$memory_limit:memory_limit" \
    "$pids_limit:pids_limit" "$cpu_usage:cpu_usage_usec" "$cpu_nr_throttled:cpu_nr_throttled" \
    "$cpu_throttled:cpu_throttled_usec" "$memory_current:memory_current" "$memory_max:memory_max" \
    "$memory_oom:memory_events_oom" "$memory_oom_kill:memory_events_oom_kill" \
    "$memory_events_max:memory_events_max" "$pids_current:pids_current" "$pids_max:pids_max" \
    "$pids_events_max:pids_events_max" "$process_count:process_count" "$fd_count:fd_count" \
    "$network_rx:host_network_rx_bytes" "$network_tx:host_network_tx_bytes"; do
    safe_number "${pair%%:*}" "${pair#*:}" >/dev/null
  done
  if [ "$pids_peak" != unavailable ]; then safe_number "$pids_peak" pids_peak >/dev/null; fi

  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
    "$epoch_ms" "$container" "$container_id" "$image_id" "$running" "$restarting" "$oom_killed" "$restart_count" \
    "$nano_cpus" "$memory_limit" "$pids_limit" "$network_mode" "${restart_policy:-none}" "$cgroup_rel" \
    "$cpu_usage" "$cpu_nr_throttled" "$cpu_throttled" "$memory_current" "$memory_max" "$memory_oom" \
    "$memory_oom_kill" "$memory_events_max" "$pids_current" "$pids_max" "$pids_peak" "$pids_events_max" \
    "$process_count" "$fd_count" "$network_rx" "$network_tx"

  if [ "$elapsed" -ge "$live_seconds" ]; then
    printf '%s\n' 'observer exceeded its bounded live interval' >&2
    exit 3
  fi
  sleep 1
done
