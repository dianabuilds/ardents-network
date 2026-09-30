//go:build linux

package state_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The selected system-scope process profile shares placement verification with
// installed qualification. This alone does not qualify installed workers.
func nodeProcessCommand(t *testing.T, binary string, arguments ...string) (*exec.Cmd, func(*testing.T)) {
	t.Helper()
	profile := os.Getenv("ARDENTS_NODE_SCOPE_PROFILE")
	if profile != "" && profile != "system" {
		t.Fatal("invalid prerequisite: unknown Node scope profile")
	}
	if os.Getenv("ARDENTS_TEXT_COMMAND_QUALIFICATION") != "1" && profile == "" {
		return exec.Command(binary, arguments...), nil
	}
	prefix := "ardents-command-node-" + strconv.Itoa(os.Getpid()) + "-"
	if selected := os.Getenv("ARDENTS_TEXT_COMMAND_SCOPE_PREFIX"); selected != "" {
		if !strings.HasPrefix(selected, "ardents-command-node-") || !strings.HasSuffix(selected, "-") || len(selected) > 180 || strings.IndexFunc(selected, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '.')
		}) >= 0 {
			t.Fatal("invalid prerequisite: Node scope prefix is not an owned unit name")
		}
		prefix = selected
	}
	unit := prefix + rand.Text() + ".scope"
	args := append([]string{"--scope", "--quiet", "--unit", unit, binary}, arguments...)
	command := exec.Command("systemd-run", args...)
	var cgroup string
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		stopErr := exec.CommandContext(ctx, "systemctl", "stop", unit).Run()
		state, stateErr := exec.CommandContext(ctx, "systemctl", "show", unit, "--property=ActiveState", "--value").Output()
		if stateErr != nil || strings.TrimSpace(string(state)) != "inactive" {
			t.Errorf("Node scope did not become inactive: stop=%v state=%v", stopErr, stateErr)
		}
		if cgroup != "" {
			populated, err := os.ReadFile(filepath.Join("/sys/fs/cgroup", cgroup, "cgroup.events"))
			if err != nil && !os.IsNotExist(err) || err == nil && !strings.Contains(string(populated), "populated 0\n") {
				t.Errorf("Node scope cleanup did not establish an empty cgroup: %v", err)
			}
		}
	})
	return command, func(t *testing.T) {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		raw, err := exec.CommandContext(ctx, "systemctl", "show", unit, "--property=ControlGroup", "--value").Output()
		if err != nil || strings.TrimSpace(string(raw)) != "/system.slice/"+unit {
			t.Fatalf("Node does not own its selected systemd scope: %v", err)
		}
		cgroup = strings.TrimSpace(string(raw))
		membership, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", command.Process.Pid))
		if err != nil || strings.TrimSpace(string(membership)) != "0::"+cgroup {
			t.Fatalf("READY Node process is outside its selected scope: %v", err)
		}
		pids, err := os.ReadFile(filepath.Join("/sys/fs/cgroup", cgroup, "cgroup.procs"))
		if err != nil || strings.TrimSpace(string(pids)) != strconv.Itoa(command.Process.Pid) {
			t.Fatalf("Node scope includes an unrelated process: %v", err)
		}
		t.Log("verified: READY Node owns a separate systemd scope")
	}
}
