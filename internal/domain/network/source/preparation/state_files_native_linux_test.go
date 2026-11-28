//go:build linux && network_signing_native

package preparation

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"syscall"
	"testing"
)

func TestSourcePreparationNativeStateCommand(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("invalid environment: Linux root required")
	}
	uid := unusedSourcePreparationPrincipal(t)
	root, err := os.MkdirTemp("/tmp", "ardents-source-state-native-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Error(err)
		}
	})
	if err := os.Chmod(root, 0755); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(root, "ardents-next")
	build := exec.CommandContext(t.Context(), "go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-w", "-o", program, "./cmd/ardents-next")
	build.Dir = "../../../../.."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build canonical Source command: %v %s", err, out)
	}
	if err := os.Chmod(program, 0555); err != nil {
		t.Fatal(err)
	}
	plan, planPath, bundlePath := sourceInitialFixtureAt(t, root)
	for _, name := range []string{planPath, bundlePath, plan.Network.ClockObservationFile} {
		if err := os.Chmod(name, 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{plan.Network.Root, plan.ParticipationRoot} {
		if err := os.Chown(path, int(uid), int(uid)); err != nil {
			t.Fatal(err)
		}
	}
	run := func(gid uint32, groups []uint32) ([]byte, error) {
		cmd := exec.CommandContext(t.Context(), program, "source", "prepare-state", planPath, bundlePath)
		cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uid, Gid: gid, Groups: groups}}
		return cmd.CombinedOutput()
	}
	for _, test := range []struct {
		name  string
		group uint32
		extra []uint32
	}{
		{name: "foreign primary group", group: 0},
		{name: "foreign supplementary group", group: uid, extra: []uint32{0}},
	} {
		if out, err := run(test.group, test.extra); err == nil || !bytes.Contains(out, []byte(`"first_owner":"input"`)) ||
			bytes.Contains(out, []byte("state-prepared")) || len(sourceHistoryBytes(t, plan.Network.Root)) != 0 ||
			len(sourceHistoryBytes(t, plan.ParticipationRoot)) != 0 {
			t.Fatalf("%s reached Source preparation: %v %s", test.name, err, out)
		}
	}
	const receipt = "{\"operation\":\"source.prepare-state\",\"outcome\":\"state-prepared\"}\n"
	if out, err := run(uid, nil); err != nil || string(out) != receipt {
		t.Fatalf("fixed principal failed preparing Source State: %v %s", err, out)
	}
	stateBefore := sourceHistoryBytes(t, plan.Network.Root)
	participationBefore := sourceHistoryBytes(t, plan.ParticipationRoot)
	if len(stateBefore) == 0 || len(participationBefore) == 0 {
		t.Fatal("preparation receipt without both histories")
	}
	if out, err := run(uid, nil); err == nil || bytes.Contains(out, []byte("state-prepared")) ||
		!reflect.DeepEqual(stateBefore, sourceHistoryBytes(t, plan.Network.Root)) ||
		!reflect.DeepEqual(participationBefore, sourceHistoryBytes(t, plan.ParticipationRoot)) {
		t.Fatalf("repeated preparation changed a retained history: %v %s", err, out)
	}
}

func unusedSourcePreparationPrincipal(t *testing.T) uint32 {
	t.Helper()
	getent, err := exec.LookPath("getent")
	if err != nil {
		t.Fatal("invalid environment: getent is required", err)
	}
	for uid := uint32(990); uid >= 100; uid-- {
		id := strconv.FormatUint(uint64(uid), 10)
		if exec.CommandContext(t.Context(), getent, "passwd", id).Run() != nil &&
			exec.CommandContext(t.Context(), getent, "group", id).Run() != nil {
			return uid
		}
	}
	t.Fatal("invalid environment: no unused system UID/GID")
	return 0
}
