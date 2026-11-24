//go:build linux && (endpoint_platform_native || endpoint_launch_native || endpoint_pair_native)

package endpoint

import (
	"os/exec"
	"strconv"
	"testing"
)

// CreateNativeAccount provisions the dedicated account with an unused system
// UID and GID. The test stand owns its cleanup.
func CreateNativeAccount(t *testing.T) int {
	t.Helper()
	var id int
	for n := 990; n >= 100; n-- {
		candidate := strconv.Itoa(n)
		if exec.Command("getent", "passwd", candidate).Run() != nil && exec.Command("getent", "group", candidate).Run() != nil {
			id = n
			break
		}
	}
	if id == 0 {
		t.Fatal("no free Endpoint UID/GID")
	}
	numeric := strconv.Itoa(id)
	for _, args := range [][]string{
		{"groupadd", "-g", numeric, "ardents-endpoint"},
		{"useradd", "-u", numeric, "-g", numeric, "-d", "/nonexistent", "-s", "/usr/sbin/nologin", "-M", "ardents-endpoint"},
	} {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("%s: %v: %s", args[0], err, out)
		}
	}
	return id
}
