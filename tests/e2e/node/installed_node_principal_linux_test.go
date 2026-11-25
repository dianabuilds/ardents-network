//go:build linux && (forwarding_platform_native || introduction_repair_native)

package node_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func chownInstalledNodeTree(root string, uid int) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(path, uid, uid)
	})
}

func createInstalledNodeAccount(t *testing.T) int {
	t.Helper()
	if exec.Command("getent", "passwd", "ardents-node").Run() == nil || exec.Command("getent", "group", "ardents-node").Run() == nil {
		t.Fatal("ardents-node principal already present; invalid environment")
	}
	uid := 0
	for candidate := 990; candidate >= 100; candidate-- {
		name := fmt.Sprintf("%d", candidate)
		if exec.Command("getent", "passwd", name).Run() != nil && exec.Command("getent", "group", name).Run() != nil {
			uid = candidate
			break
		}
	}
	if uid == 0 {
		t.Fatal("no free unprivileged system UID/GID available")
	}
	if err := exec.Command("groupadd", "-g", fmt.Sprintf("%d", uid), "ardents-node").Run(); err != nil {
		t.Fatal("groupadd", err)
	}
	if err := exec.Command("useradd", "-u", fmt.Sprintf("%d", uid), "-g", fmt.Sprintf("%d", uid), "-d", "/nonexistent", "-s", "/usr/sbin/nologin", "-M", "ardents-node").Run(); err != nil {
		t.Fatal("useradd", err)
	}
	return uid
}

func runAsInstalledNode(t *testing.T, name string, args ...string) ([]byte, error) {
	t.Helper()
	full := append([]string{"-u", "ardents-node", "--", name}, args...)
	cmd := exec.CommandContext(t.Context(), "runuser", full...)
	cmd.Dir = "/"
	return cmd.CombinedOutput()
}
