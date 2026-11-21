//go:build linux && text_worker_installed

package runtime

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/execution/worker"
)

func pinInstalledExecutionWorker(t *testing.T, instance worker.Instance) *hostileExecutionTree {
	t.Helper()
	// Retain original kernel/proc descriptors independently of runtime
	// cleanup. Pathname disappearance or manager MainPID=0 is no join.
	group, err := os.OpenRoot("/sys/fs/cgroup" + instance.Cgroup)
	if err != nil {
		t.Fatal(err)
	}
	events, openErr := group.Open("cgroup.events")
	if err := errors.Join(openErr, group.Close()); err != nil {
		if events != nil {
			_ = events.Close()
		}
		t.Fatal(err)
	}
	observer := &hostileExecutionTree{events: events}
	t.Cleanup(func() {
		for _, process := range observer.processes {
			if err := process.Close(); err != nil {
				t.Error(err)
			}
		}
		if err := events.Close(); err != nil {
			t.Error(err)
		}
	})
	var filesystem syscall.Statfs_t
	if syscall.Fstatfs(int(events.Fd()), &filesystem) != nil || filesystem.Type != 0x63677270 {
		t.Fatal("worker observer is not an original kernel cgroup-v2 descriptor")
	}
	process, err := os.Open("/proc/" + strconv.FormatUint(uint64(instance.PID), 10))
	if err != nil {
		t.Fatal(err)
	}
	observer.processes = append(observer.processes, process)
	if gone, populated, err := observer.observe(); err != nil || gone || !populated {
		t.Fatalf("original worker scope positive control unavailable: %v", err)
	}
	return observer
}

// Unit collection is observed after the original kernel and proc observers
// have established physical join. Listing loaded units does not load a name.
func requireInstalledExecutionUnitCollected(t *testing.T, parent context.Context, unit string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		output, err := exec.CommandContext(ctx, "systemctl", "list-units", "--no-legend", "--no-pager", "--plain", "--all", "--", unit).Output()
		if err != nil {
			t.Fatalf("observe collection of %s: %v", unit, err)
		}
		if strings.TrimSpace(string(output)) == "" {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("joined worker unit was not collected: %s", unit)
		case <-ticker.C:
		}
	}
}
