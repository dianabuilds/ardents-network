//go:build linux && document_worker_installed

package endpoint

import (
	"context"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"testing"

	executionruntime "github.com/dianabuilds/ardents-network/internal/execution/runtime"
)

// The observed unique unit selects only the fault target. The original native
// operation is checked again before the existing non-root StopUnit permission
// is exercised. No Job is revoked by the fixture; IPC observes physical loss.
func InstalledConnectionWorkerIdentity(t *testing.T, invocation *executionruntime.Invocation) (name, role string, pid, uid uint64) {
	t.Helper()
	activation := reflect.ValueOf(invocation).Elem().FieldByName("activation")
	if activation.IsNil() {
		t.Fatal("original activation absent")
	}
	instance := activation.Elem().FieldByName("Instance")
	pid, uid = instance.FieldByName("PID").Uint(), instance.FieldByName("UID").Uint()
	name, role = instance.FieldByName("Name").String(), instance.FieldByName("Role").String()
	suffix := "-" + strconv.Itoa(os.Getpid()) + "-" + strconv.Itoa(os.Geteuid()) + ".service"
	if pid == 0 || uid == 0 || pid == uint64(os.Getpid()) || role != "reader" && role != "publisher" || !strings.HasPrefix(name, "ardents-text-"+role+"@") || !strings.HasSuffix(name, suffix) {
		t.Fatal("original worker process identity differs")
	}
	return name, role, pid, uid
}

func StopInstalledConnectionWorker(t *testing.T, ctx context.Context, invocation *executionruntime.Invocation, check func() error) {
	t.Helper()
	name, _, _, _ := InstalledConnectionWorkerIdentity(t, invocation)
	if err := check(); err != nil {
		t.Fatal("original worker unavailable before fault", err)
	}
	if err := exec.CommandContext(ctx, "systemctl", "--system", "stop", name).Run(); err != nil {
		t.Fatal("original worker stop", err)
	}
}
