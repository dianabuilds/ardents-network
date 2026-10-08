//go:build installation_native

package process

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestInstallationNativeProcessSelfCredentialBoundary(t *testing.T) {
	if os.Getenv("ARDENTS_PROCESS_SELF_CHILD") == "1" {
		if os.Geteuid() != 65534 || os.Getegid() != 65534 {
			t.Fatal("invalid environment: child did not drop actual kernel credentials")
		}
		identity, err := os.Stat("/proc/self/exe")
		if err != nil {
			t.Fatal(err)
		}
		expected := Expected{PID: uint32(os.Getpid()), UID: 65534, GID: 65534,
			Invocation: [16]byte{1}, InstallationRoot: "/foreign-installation",
			Generation: strings.Repeat("a", 64), ProgramIdentity: identity}
		if pin, err := RetainSelf(t.Context(), expected); pin != nil || !errors.Is(err, errBinding) {
			if pin != nil {
				pin.Close()
			}
			t.Fatal("actual non-root foreign executable was not refused", err)
		}
		if pin, err := Retain(t.Context(), expected); pin != nil || !errors.Is(err, errInput) {
			if pin != nil {
				pin.Close()
			}
			t.Fatal("service account acquired privileged predecessor custody", err)
		}
		foreign := expected
		foreign.PID = uint32(os.Getppid())
		if pin, err := RetainSelf(t.Context(), foreign); pin != nil || !errors.Is(err, errInput) {
			if pin != nil {
				pin.Close()
			}
			t.Fatal("self opening inspected a foreign PID", err)
		}
		foreign = expected
		foreign.UID++
		if pin, err := RetainSelf(t.Context(), foreign); pin != nil || !errors.Is(err, errInput) {
			if pin != nil {
				pin.Close()
			}
			t.Fatal("self opening accepted supplied foreign credentials", err)
		}
		cancelled, cancel := context.WithCancel(t.Context())
		cancel()
		if pin, err := RetainSelf(cancelled, expected); pin != nil || !errors.Is(err, context.Canceled) {
			if pin != nil {
				pin.Close()
			}
			t.Fatal("cancelled self opening acquired custody", err)
		}
		return
	}
	nativeDirectory(t)
	if pin, err := RetainSelf(t.Context(), Expected{PID: uint32(os.Getpid())}); pin != nil || !errors.Is(err, errInput) {
		if pin != nil {
			pin.Close()
		}
		t.Fatal("root became the installed service process", err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// Go's private build parent is inaccessible after credential drop. Copy
	// only this executable into a new root-owned searchable fixture directory;
	// do not change build-directory or host-account permissions.
	directory, err := os.MkdirTemp("/tmp", "installation-process-self-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Error(err)
		}
	})
	if err := os.Chmod(directory, 0711); err != nil {
		t.Fatal(err)
	}
	input, err := os.Open(executable)
	if err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(directory, "reader.test")
	outputFile, err := os.OpenFile(child, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0555)
	if err != nil {
		t.Fatal(errors.Join(err, input.Close()))
	}
	_, copyErr := io.Copy(outputFile, input)
	if err := errors.Join(copyErr, outputFile.Close(), input.Close()); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), child, "-test.run=^TestInstallationNativeProcessSelfCredentialBoundary$", "-test.count=1")
	command.Env = []string{"ARDENTS_PROCESS_SELF_CHILD=1"}
	command.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65534, Gid: 65534, Groups: []uint32{65534}}}
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("actual credential child failed: %v\n%s", err, output)
	}
}

func nativeDirectory(t *testing.T) string {
	t.Helper()
	parent := os.Getenv("ARDENTS_INSTALLATION_NATIVE_ROOT")
	if os.Geteuid() != 0 || parent == "" || !filepath.IsAbs(parent) {
		t.Fatal("invalid environment: installation_native requires root and a trusted temporary parent")
	}
	for current := parent; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil || info == nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 {
			t.Fatal("invalid trusted temporary parent", err)
		}
		native, ok := info.Sys().(*syscall.Stat_t)
		if !ok || native.Uid != 0 {
			t.Fatal("invalid trusted temporary parent owner")
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	directory, err := os.MkdirTemp(parent, "installation-process-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Error(err)
		}
	})
	return directory
}

func TestInstallationNativeProcessRefusesOrdinaryProcImitationAndClosesOriginal(t *testing.T) {
	directory := nativeDirectory(t)
	// This is a deliberately foreign filesystem imitation, never a positive
	// installed process, manager or authority fixture.
	body := "123 (fixture) S " + strings.Repeat("0 ", 18) + "456 0\n"
	if err := os.WriteFile(filepath.Join(directory, "stat"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	original, err := root.Stat(".")
	if err != nil {
		root.Close()
		t.Fatal(err)
	}
	retained := &Invocation{root: root, identity: original, pid: 123}
	t.Cleanup(func() { retained.Close() })
	if err := retained.Observe(t.Context()); err == nil {
		t.Fatal("ordinary proc imitation became an installed invocation")
	}
	if _, err := root.Stat("."); err != nil {
		t.Fatal("refusal released original descriptor custody", err)
	}
	if err := retained.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := root.Stat("."); err == nil {
		t.Fatal("physical close retained original descriptor")
	}
	if err := retained.Observe(t.Context()); err == nil {
		t.Fatal("closed descriptor became process evidence")
	}
}

func TestInstallationNativeProcessRetainRefusesForeignExecutable(t *testing.T) {
	directory := nativeDirectory(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	identity, err := os.Stat(executable)
	if err != nil {
		t.Fatal(err)
	}
	expected := Expected{PID: uint32(os.Getpid()), UID: 65534, GID: 65534,
		Invocation: [16]byte{1}, InstallationRoot: directory,
		Generation: strings.Repeat("a", 64), ProgramIdentity: identity}
	// The actual current process and actual program inode do not make the
	// test executable the fixed generation program or its qualified invocation.
	retained, err := Retain(t.Context(), expected)
	if retained != nil {
		retained.Close()
		t.Fatal("foreign actual executable became installed process evidence")
	}
	if !errors.Is(err, errBinding) {
		t.Fatal("foreign executable was not refused at identity", err)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if retained, err := Retain(cancelled, expected); retained != nil || !errors.Is(err, context.Canceled) {
		if retained != nil {
			retained.Close()
		}
		t.Fatal("cancelled caller acquired process custody", err)
	}
	if retained, err := Retain(nil, expected); retained != nil || !errors.Is(err, errInput) {
		if retained != nil {
			retained.Close()
		}
		t.Fatal("nil caller acquired process custody", err)
	}
}
