//go:build linux && amd64

package reachability_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
)

// This is process-crash evidence on the selected Linux amd64 filesystem, not
// power loss. The tracer stops after actual syscall returns; it supplies no
// write, Sync, successful publication, ACK or recovery result.
func TestStoreProcessCrashAtActualPersistenceBoundaries(t *testing.T) {
	if root := os.Getenv("ARDENTS_REACHABILITY_CRASH_ROOT"); root != "" {
		raw, _, network, profile, at := signedDescriptor(3)
		higher := resignDescriptor(raw, func(b []byte) { binary.BigEndian.PutUint64(b[162:170], 10) })
		store, err := reachability.OpenStore(reachability.StoreConfig{Root: root, Network: network})
		if err != nil {
			t.Fatal(err)
		}
		defer store.Close()
		check := func() error { _, err := reachability.VerifyPublish(higher, network, profile, at); return err }
		if outcome, err := store.Publish(higher, profile, at, check); err != nil || outcome != reachability.Accepted {
			t.Fatal(outcome, err)
		}
		t.Fatal("original process survived selected persistence crash")
	}
	for _, point := range []string{"stage-written", "stage-synced", "renamed", "directory-synced"} {
		t.Run(point, func(t *testing.T) {
			raw, target, network, profile, at := signedDescriptor(3)
			higher := resignDescriptor(raw, func(b []byte) { binary.BigEndian.PutUint64(b[162:170], 10) })
			config := reachability.StoreConfig{Root: t.TempDir(), Network: network}
			store, err := reachability.OpenStore(config)
			if err != nil {
				t.Fatal(err)
			}
			check := func() error { _, err := reachability.VerifyPublish(raw, network, profile, at); return err }
			if outcome, err := store.Publish(raw, profile, at, check); err != nil || outcome != reachability.Accepted {
				t.Fatal(outcome, err)
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			crashStoreAtNativeReturn(t, config.Root, target, higher, point)
			stage := filepath.Join(config.Root, "records", ".record-stage")
			if point == "stage-written" || point == "stage-synced" {
				pending, err := os.ReadFile(stage)
				if err != nil || !bytes.Equal(pending, append([]byte{3, 0}, higher...)) {
					t.Fatal("actual stage footprint differs", err)
				}
			} else if _, err := os.Lstat(stage); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("actual rename left stage", err)
			}
			fresh, err := reachability.OpenStore(config)
			if err != nil {
				t.Fatal("independent full recovery after process crash", err)
			}
			defer fresh.Close()
			got, outcome, lookupErr := fresh.Lookup(target, profile, at)
			expected := raw
			if point == "renamed" || point == "directory-synced" {
				expected = higher
			}
			if lookupErr != nil || outcome != reachability.Accepted || !bytes.Equal(got, expected) {
				t.Fatal("crash recovery changed exact retained floor", outcome, lookupErr)
			}
			if _, err := os.Lstat(stage); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("validated recovery left interrupted stage", err)
			}
			if bytes.Equal(expected, higher) {
				if outcome, err := fresh.Publish(raw, profile, at, check); outcome != reachability.Stale || err == nil {
					t.Fatal("crash recovery lost revision floor", outcome, err)
				}
				retained, outcome, err := fresh.Lookup(target, profile, at)
				if err != nil || outcome != reachability.Accepted || !bytes.Equal(retained, higher) {
					t.Fatal("refused rollback poisoned exact recovered floor", outcome, err)
				}
			}
		})
	}
}

type storeNativeSyscall struct {
	number uint64
	file   string
}

func crashStoreAtNativeReturn(t *testing.T, root string, target [32]byte, higher []byte, point string) {
	t.Helper()
	// All ptrace operations belong to the exact original tracer OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	diagnostic, err := os.CreateTemp(t.TempDir(), "child-result")
	if err != nil {
		t.Fatal(err)
	}
	defer diagnostic.Close()
	command := exec.Command(os.Args[0], "-test.run=^TestStoreProcessCrashAtActualPersistenceBoundaries$", "-test.v=true")
	command.Env = append(os.Environ(), "ARDENTS_REACHABILITY_CRASH_ROOT="+root)
	command.SysProcAttr = &syscall.SysProcAttr{Ptrace: true}
	command.Stdout, command.Stderr = diagnostic, diagnostic
	if err := command.Start(); err != nil {
		t.Fatal("selected native ptrace environment unavailable", err)
	}
	pid := command.Process.Pid
	joined := false
	defer func() {
		if !joined {
			_ = command.Process.Kill()
			for {
				var status syscall.WaitStatus
				id, err := syscall.Wait4(-1, &status, syscall.WALL, nil)
				if errors.Is(err, syscall.EINTR) {
					continue
				}
				if err != nil {
					break
				}
				if status.Stopped() {
					_ = syscall.PtraceCont(id, int(syscall.SIGKILL))
				}
			}
		}
		_ = command.Process.Release()
		if t.Failed() {
			raw, _ := os.ReadFile(diagnostic.Name())
			t.Log("original traced child result", string(raw))
		}
	}()
	var initial syscall.WaitStatus
	if id, err := syscall.Wait4(pid, &initial, 0, nil); err != nil || id != pid || !initial.Stopped() {
		t.Fatal("initial original trace stop unavailable", id, initial, err)
	}
	if err := syscall.PtraceSetOptions(pid, syscall.PTRACE_O_TRACECLONE|syscall.PTRACE_O_TRACESYSGOOD); err != nil {
		t.Fatal(err)
	}
	if err := syscall.PtraceSyscall(pid, 0); err != nil {
		t.Fatal(err)
	}
	pending := make(map[int]storeNativeSyscall)
	live := map[int]bool{pid: true}
	stage := filepath.Join(root, "records", ".record-stage")
	record := filepath.Join(root, "records", fmt.Sprintf("%x", target))
	records := filepath.Join(root, "records")
	expected := append([]byte{3, 0}, higher...)
	deadline := time.Now().Add(20 * time.Second)
	crashed := false
	mainJoined := false
	for {
		if time.Now().After(deadline) {
			t.Fatal("selected actual persistence syscall not reached", point)
		}
		var status syscall.WaitStatus
		id, err := syscall.Wait4(-1, &status, syscall.WALL|syscall.WNOHANG, nil)
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if err != nil {
			t.Fatal("original trace wait unavailable", err)
		}
		if id == 0 {
			time.Sleep(100 * time.Microsecond)
			continue
		}
		if status.Exited() || status.Signaled() {
			delete(live, id)
			if id == pid {
				if !crashed || !status.Signaled() || status.Signal() != syscall.SIGKILL {
					t.Fatal("original process did not die at selected boundary", point, status)
				}
				mainJoined = true
			}
			if mainJoined && len(live) == 0 {
				joined = true
				return
			}
			continue
		}
		if !status.Stopped() {
			t.Fatal("unexpected original trace state", status)
		}
		live[id] = true
		if status.StopSignal() == syscall.SIGTRAP && status.TrapCause() == syscall.PTRACE_EVENT_CLONE {
			thread, err := syscall.PtraceGetEventMsg(id)
			if err != nil {
				t.Fatal("original clone trace unavailable", err)
			}
			live[int(thread)] = true
		}
		if crashed {
			_ = syscall.PtraceCont(id, int(syscall.SIGKILL))
			continue
		}
		if status.StopSignal() == syscall.SIGTRAP|0x80 {
			var registers syscall.PtraceRegs
			if err := syscall.PtraceGetRegs(id, &registers); err != nil {
				t.Fatal(err)
			}
			// Linux amd64 marks syscall entry with -ENOSYS. Clone returns may
			// be the new thread's first stop, so no guessed entry/exit toggle.
			if int64(registers.Rax) == -int64(syscall.ENOSYS) {
				file, _ := os.Readlink(fmt.Sprintf("/proc/%d/fd/%d", id, registers.Rdi))
				pending[id] = storeNativeSyscall{number: registers.Orig_rax, file: file}
			} else if call, exists := pending[id]; exists {
				delete(pending, id)
				matched := false
				switch point {
				case "stage-written":
					matched = call.number == syscall.SYS_WRITE && call.file == stage && registers.Rax == uint64(len(expected))
				case "stage-synced":
					matched = call.number == syscall.SYS_FSYNC && call.file == stage && registers.Rax == 0
				case "renamed":
					if (call.number == syscall.SYS_RENAMEAT || call.number == 316 /* Linux amd64 renameat2 */) && registers.Rax == 0 {
						actual, err := os.ReadFile(record)
						matched = err == nil && bytes.Equal(actual, expected)
					}
				case "directory-synced":
					if call.number == syscall.SYS_FSYNC && call.file == records && registers.Rax == 0 {
						actual, err := os.ReadFile(record)
						matched = err == nil && bytes.Equal(actual, expected)
					}
				}
				if matched {
					if err := command.Process.Kill(); err != nil {
						t.Fatal(err)
					}
					crashed = true
					_ = syscall.PtraceCont(id, int(syscall.SIGKILL))
					continue
				}
			}
		}
		deliver := int(status.StopSignal())
		if status.StopSignal() == syscall.SIGTRAP || status.StopSignal() == syscall.SIGTRAP|0x80 || status.StopSignal() == syscall.SIGSTOP {
			deliver = 0
		}
		if err := syscall.PtraceSyscall(id, deliver); err != nil && !errors.Is(err, syscall.ESRCH) {
			t.Fatal(err)
		}
	}
}
