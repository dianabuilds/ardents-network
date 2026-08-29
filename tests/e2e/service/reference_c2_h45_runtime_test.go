//go:build h4_5_rendezvous

package service_test

import (
	"fmt"
	"testing"
)

func h45RuntimeSampler() string {
	return `#!/bin/sh
set -eu
root=$1
port=$2
output=$root/contributor-runtime-samples.tsv
printf 'captured_at\tcpu_usage_usec\tmemory_bytes\trss_kib\tfds\tsockets\tthreads\tpids\n' >"$output"
samples=0
while [ "$samples" -lt 900 ]; do
  cgroup_relative=$(systemctl show ardents-rendezvous-contributor.service -p ControlGroup --value)
  main_pid=$(systemctl show ardents-rendezvous-contributor.service -p MainPID --value)
  cgroup=/sys/fs/cgroup$cgroup_relative
  if [ -d "$cgroup" ] && [ "$main_pid" -gt 0 ] && [ -r "/proc/$main_pid/status" ]; then
    captured_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    cpu=$(awk '$1 == "usage_usec" {print $2}' "$cgroup/cpu.stat")
    memory=$(cat "$cgroup/memory.current")
    rss=$(awk '$1 == "VmRSS:" {print $2}' "/proc/$main_pid/status")
    threads=$(awk '$1 == "Threads:" {print $2}' "/proc/$main_pid/status")
    fds=$(find "/proc/$main_pid/fd" -mindepth 1 -maxdepth 1 2>/dev/null | wc -l)
    sockets=$(find "/proc/$main_pid/fd" -mindepth 1 -maxdepth 1 -lname 'socket:*' 2>/dev/null | wc -l)
    pids=$(cat "$cgroup/pids.current")
    printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$captured_at" "$cpu" "$memory" "$rss" "$fds" "$sockets" "$threads" "$pids" >>"$output"
    samples=$((samples + 1))
  fi
  sleep 1
done
`
}

func h45StartRuntimeSampler(t *testing.T, remote h43RemoteC2) {
	t.Helper()
	root := remote.environment.remoteDirectory
	command := fmt.Sprintf(`set -eu
test ! -e %[1]s/contributor-runtime-sampler.pid
nohup %[1]s/runtime-sampler.sh %[1]s %d >/dev/null 2>&1 &
printf '%%s\n' "$!" >%[1]s/contributor-runtime-sampler.pid`, h43ShellQuote(root), remote.environment.port+1)
	if output, err := remote.run(t, command); err != nil {
		t.Fatalf("start H4-5 runtime sampler: %v\n%s", err, output)
	}
	t.Cleanup(func() {
		if output, err := remote.runCleanup(h45StopRuntimeSamplerCommand(root, 0)); err != nil {
			t.Errorf("cleanup H4-5 runtime sampler: %v\n%s", err, output)
		}
	})
}

func h45StopRuntimeSampler(t *testing.T, remote h43RemoteC2, minimumSamples int) {
	t.Helper()
	if output, err := remote.run(t, h45StopRuntimeSamplerCommand(remote.environment.remoteDirectory, minimumSamples)); err != nil {
		t.Fatalf("stop H4-5 runtime sampler: %v\n%s", err, output)
	}
}

func h45StopRuntimeSamplerCommand(root string, minimumSamples int) string {
	quoted := h43ShellQuote(root)
	return fmt.Sprintf(`set -eu
pid_file=%[1]s/contributor-runtime-sampler.pid
if [ -f "$pid_file" ]; then
  pid=$(cat "$pid_file")
  kill "$pid" 2>/dev/null || true
  tries=0
  while kill -0 "$pid" 2>/dev/null && [ "$tries" -lt 50 ]; do tries=$((tries + 1)); sleep 0.1; done
  if kill -0 "$pid" 2>/dev/null; then kill -KILL "$pid" 2>/dev/null || true; fi
  tries=0
  while kill -0 "$pid" 2>/dev/null && [ "$tries" -lt 50 ]; do tries=$((tries + 1)); sleep 0.1; done
  ! kill -0 "$pid" 2>/dev/null
  rm -f "$pid_file"
fi
test -f %[1]s/contributor-runtime-samples.tsv
data_lines=$(($(wc -l <%[1]s/contributor-runtime-samples.tsv) - 1))
test "$data_lines" -ge %d`, quoted, minimumSamples)
}
