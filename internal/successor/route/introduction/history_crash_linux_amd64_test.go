//go:build linux && amd64

package introduction

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"
)

// These are actual syscall-return process crashes on a mounted Linux amd64
// filesystem, never power-loss evidence. Native tracing observes and kills only
// its own child and joins every traced original thread before independent reopen.
func TestHistoryProcessCrashAtActualPersistenceBoundaries(t *testing.T) {
	binding := Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	now := time.Unix(1_800_000_000, 0).UTC()
	if root := os.Getenv("ARDENTS_INTRODUCTION_CRASH_ROOT"); root != "" {
		history, err := OpenHistory(root, binding)
		if err != nil {
			t.Fatal(err)
		}
		defer history.Close()
		if err := history.Claim([32]byte{8}, now.Add(time.Minute), now.Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		t.Fatal("original process survived selected persistence crash")
	}
	for _, point := range []string{"stage-written", "stage-synced", "renamed", "directory-synced"} {
		t.Run(point, func(t *testing.T) {
			history, _, root, _ := newHistoryFixture(t)
			if err := history.Claim([32]byte{7}, now.Add(time.Minute), now); err != nil {
				t.Fatal(err)
			}
			if err := history.Close(); err != nil {
				t.Fatal(err)
			}
			prior, err := os.ReadFile(filepath.Join(root, slotHistoryName))
			if err != nil {
				t.Fatal(err)
			}
			// Independently construct canonical bytes with stdlib primitives;
			// no production encoder supplies the crash/reopen oracle.
			expected := append([]byte(nil), prior[:120]...)
			binary.BigEndian.PutUint64(expected[112:120], uint64(now.Add(time.Second).Unix()))
			first, second := [32]byte{7}, [32]byte{8}
			keys := [][32]byte{sha256.Sum256(first[:]), sha256.Sum256(second[:])}
			sort.Slice(keys, func(i, j int) bool { return bytes.Compare(keys[i][:], keys[j][:]) < 0 })
			for _, key := range keys {
				expected = append(expected, key[:]...)
				expected = binary.BigEndian.AppendUint64(expected, uint64(now.Add(time.Minute).Unix()))
			}
			crashHistoryAtNativeReturn(t, root, expected, point)
			names, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			var stages []string
			for _, name := range names {
				if strings.HasPrefix(name.Name(), ".introduction-snapshot-") {
					stages = append(stages, name.Name())
				}
			}
			actual, err := os.ReadFile(filepath.Join(root, slotHistoryName))
			if err != nil {
				t.Fatal(err)
			}
			if point == "stage-written" || point == "stage-synced" {
				if len(stages) != 1 || !bytes.Equal(actual, prior) {
					t.Fatal("pre-rename crash changed prior floor or lost stage")
				}
				pending, err := os.ReadFile(filepath.Join(root, stages[0]))
				if err != nil || !bytes.Equal(pending, expected) {
					t.Fatal("actual interrupted stage differs", err)
				}
				if reopened, err := OpenHistory(root, binding); err == nil {
					_ = reopened.Close()
					t.Fatal("interrupted stage silently adopted or repaired")
				}
				after, err := os.ReadFile(filepath.Join(root, stages[0]))
				if err != nil || !bytes.Equal(after, pending) {
					t.Fatal("refused reopen changed conservative footprint", err)
				}
				return
			}
			if len(stages) != 0 || !bytes.Equal(actual, expected) {
				t.Fatal("post-rename crash lost exact next floor")
			}
			reopened, err := OpenHistory(root, binding)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			if err := reopened.Claim([32]byte{8}, now.Add(time.Minute), now.Add(2*time.Second)); err == nil {
				t.Fatal("process crash refunded durable next claim")
			}
			if err := reopened.Claim([32]byte{9}, now.Add(time.Minute), now); err == nil {
				t.Fatal("process crash reset time floor")
			}
		})
	}
}

type historyNativeSyscall struct {
	number uint64
	file   string
}

func crashHistoryAtNativeReturn(t *testing.T, root string, expected []byte, point string) {
	t.Helper()
	// All ptrace operations belong to the exact original tracer OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	diagnostic, err := os.CreateTemp(t.TempDir(), "child-result")
	if err != nil {
		t.Fatal(err)
	}
	defer diagnostic.Close()
	command := exec.Command(os.Args[0], "-test.run=^TestHistoryProcessCrashAtActualPersistenceBoundaries$", "-test.v=true")
	command.Env = append(os.Environ(), "ARDENTS_INTRODUCTION_CRASH_ROOT="+root)
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
	pending := make(map[int]historyNativeSyscall)
	live := map[int]bool{pid: true}
	stagePrefix := filepath.Join(root, ".introduction-snapshot-")
	record := filepath.Join(root, slotHistoryName)
	records := root
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
				pending[id] = historyNativeSyscall{number: registers.Orig_rax, file: file}
			} else if call, exists := pending[id]; exists {
				delete(pending, id)
				matched := false
				switch point {
				case "stage-written":
					matched = call.number == syscall.SYS_WRITE && strings.HasPrefix(call.file, stagePrefix) && registers.Rax == uint64(len(expected))
				case "stage-synced":
					matched = call.number == syscall.SYS_FSYNC && strings.HasPrefix(call.file, stagePrefix) && registers.Rax == 0
				case "renamed":
					if (call.number == syscall.SYS_RENAME || call.number == syscall.SYS_RENAMEAT || call.number == 316 /* Linux amd64 renameat2 */) && registers.Rax == 0 {
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
