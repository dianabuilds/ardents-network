//go:build linux && endpoint_launch_native

package endpoint

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/application/administration"
)

// PublishAndFindSnapshotWorker publishes through the original Administration
// socket and identifies the live installed Publisher worker in its cgroup.
func PublishAndFindSnapshotWorker(t testing.TB, administrationSocket, endpointPID string, uid uint32) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), administration.SnapshotTimeout)
	defer cancel()
	path := filepath.Join(t.TempDir(), "snapshot-input")
	if err := os.WriteFile(path, []byte("Endpoint replacement snapshot\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := administration.PublishLocal(ctx, administrationSocket, path); err != nil {
		t.Fatal("publish before replacement:", err)
	}
	if link, err := administration.RequestPublishedLink(ctx, administrationSocket); err != nil || link == "" {
		t.Fatal("publication was not reachable before replacement:", err)
	}
	const directory = "/sys/fs/cgroup/system.slice"
	scopes, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	suffix := "-" + endpointPID + "-" + strconv.FormatUint(uint64(uid), 10) + ".service"
	for _, scope := range scopes {
		name := scope.Name()
		if scope.IsDir() && strings.HasPrefix(name, "ardents-text-publisher@") && strings.HasSuffix(name, suffix) {
			path := filepath.Join(directory, name)
			if LiveSnapshotWorker(path) {
				return path
			}
		}
	}
	t.Fatal("published snapshot has no original installed Publisher worker")
	return ""
}

// LiveSnapshotWorker checks the original cgroup against the installed worker inode.
func LiveSnapshotWorker(scope string) bool {
	body, err := os.ReadFile(filepath.Join(scope, "cgroup.procs"))
	if err != nil {
		return false
	}
	executable, err := os.Stat("/usr/lib/ardents/text-worker-root/ardents-text")
	if err != nil {
		return false
	}
	for _, pid := range strings.Fields(string(body)) {
		if _, err := strconv.ParseUint(pid, 10, 32); err != nil {
			return false
		}
		active, err := os.Stat(filepath.Join("/proc", pid, "exe"))
		if err == nil && os.SameFile(executable, active) {
			return true
		}
	}
	return false
}
