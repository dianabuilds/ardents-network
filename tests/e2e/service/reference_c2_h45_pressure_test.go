//go:build h4_5_rendezvous

package service_test

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func h45ExerciseInstalledStoragePressure(t *testing.T, remote h43RemoteC2) {
	t.Helper()
	root := h43ShellQuote(remote.environment.remoteDirectory)
	pressure := "/var/lib/private/ardents-contributor/role/h45-pressure.bin"
	t.Cleanup(func() {
		cleanup := "set -eu; rm -f " + h43ShellQuote(pressure) + "; test ! -e " + h43ShellQuote(pressure)
		if output, err := remote.runCleanup(cleanup); err != nil {
			t.Errorf("cleanup installed H4-5 pressure input: %v\n%s", err, output)
		}
	})
	command := fmt.Sprintf(`set -eu
wait_resource() {
  wanted=$1
  tries=0
  limit=200
  if [ "$wanted" = NORMAL ]; then limit=1500; fi
  while [ "$tries" -lt "$limit" ]; do
    state=$(sed -n 's/.*"state":"\([^"]*\)".*/\1/p' /var/lib/private/ardents-contributor/diagnostics/resource.json 2>/dev/null || true)
    if [ "$state" = "$wanted" ]; then return 0; fi
    tries=$((tries + 1))
    sleep 0.1
  done
  return 1
}
managed_bytes() {
  find /var/lib/private/ardents-contributor/network /var/lib/private/ardents-contributor/role -type f ! -path %[1]s -printf '%%s\n' |
    awk '{total += $1} END {print total + 0}'
}
base=$(managed_bytes)
protect_size=$((335544320 - base))
test "$protect_size" -gt 0
truncate -s "$protect_size" %[1]s
wait_resource PROTECT
cp /var/lib/private/ardents-contributor/diagnostics/resource.json %[2]s/contributor-resource-protect.json
rm -f %[1]s
wait_resource NORMAL
cp /var/lib/private/ardents-contributor/diagnostics/resource.json %[2]s/contributor-resource-normal.json
base=$(managed_bytes)
drain_size=$((402653184 - base))
test "$drain_size" -gt 0
truncate -s "$drain_size" %[1]s
wait_resource EXIT
cp /var/lib/private/ardents-contributor/diagnostics/resource.json %[2]s/contributor-resource-exit.json
rm -f %[1]s
tries=0
while systemctl is-active --quiet ardents-rendezvous-contributor.service && [ "$tries" -lt 100 ]; do tries=$((tries + 1)); sleep 0.1; done
test ! -e %[1]s
test "$(sed -n 's/.*"state":"\([^"]*\)".*/\1/p' /var/lib/private/ardents-contributor/diagnostics/lifecycle.json)" = WITHDRAWN
grep -F 'resource pressure crossed an emergency threshold' /var/lib/private/ardents-contributor/diagnostics/lifecycle.json >%[2]s/contributor-pressure-withdrawn.json`, h43ShellQuote(pressure), root)
	if output, err := remote.run(t, command); err != nil {
		t.Fatalf("exercise installed H4-5 storage pressure: %v\n%s", err, output)
	}
	h45RunLifecycle(t, remote, "restart-after-pressure", h45ContributorCommand, "contributor restart")
}

func h45AssertConnectionRefused(t *testing.T, remote h43RemoteC2, transition string) {
	t.Helper()
	connection, err := net.DialTimeout("tcp", net.JoinHostPort(remote.environment.host,
		fmt.Sprint(remote.environment.port+1)), time.Second)
	if err == nil {
		_ = connection.Close()
		t.Fatalf("H4-5 Contributor accepted a TCP connection after %s", transition)
	}
	if transition != "drain" && transition != "withdrawal" {
		t.Fatal("H4-5 refusal transition is outside its closed vocabulary")
	}
	if strings.TrimSpace(err.Error()) == "" {
		t.Fatalf("H4-5 refusal after %s lacked a classified dial error", transition)
	}
}
