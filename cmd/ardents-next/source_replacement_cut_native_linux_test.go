//go:build linux && network_signing_native

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// This kernel observer runs the genuine enrolled canonical command. It supplies
// no manager reply, Release decision, state, admission callback or product hook.
// Only the original controller may select this installed profile, after the
// separate exact bootstrap decision. The outside scope still owns all users.
func TestSourceReplacementReloadCancellationNative(t *testing.T) {
	var argv []string
	if json.Unmarshal([]byte(os.Getenv("ARDENTS_SOURCE_REPLACEMENT_COMMAND")), &argv) != nil || len(argv) != 9 ||
		argv[1] != "installation" || argv[2] != "replace-source" || os.Geteuid() != 0 || runtime.GOARCH != "amd64" {
		t.Fatal("invalid environment: exact canonical replacement on the native amd64 stand required")
	}
	for _, path := range []string{argv[0], argv[3], argv[4], argv[7], argv[8]} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path {
			t.Fatal("invalid environment: original absolute path required")
		}
	}
	output := os.Getenv("ARDENTS_SOURCE_REPLACEMENT_CUT_OUTPUT")
	if !filepath.IsAbs(output) {
		t.Fatal("invalid environment: private original output required")
	}
	log, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	command := exec.Command(argv[0], argv[1:]...)
	command.Stdout, command.Stderr = writer, writer
	command.SysProcAttr = &syscall.SysProcAttr{Ptrace: true, Pdeathsig: syscall.SIGKILL}
	// Linux ptrace ownership belongs to the creating thread until every tracee
	// has been reaped. A deadline kills only this original command; no takeover.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		t.Fatal(err)
	}
	drained := make(chan error, 1)
	go func() {
		_, err := io.Copy(log, io.LimitReader(reader, 1<<20))
		drained <- err
	}()
	pid := command.Process.Pid
	deadlineDone := make(chan struct{})
	deadline := time.AfterFunc(110*time.Second, func() { _ = command.Process.Kill(); close(deadlineDone) })
	defer func() {
		if !deadline.Stop() {
			<-deadlineDone
		}
	}()
	observation, traceErr := traceSourceReplacementCancellation(pid, command.Process, argv[7])
	// Wait4 in the tracer has already reaped the original. Cmd.Wait still joins
	// its own resources; ECHILD must not be mistaken for successful installation.
	waitErr := command.Wait()
	if err := <-drained; err != nil {
		t.Fatal("original output join", err)
	}
	encoded, encodeErr := json.Marshal(observation)
	if encodeErr != nil {
		t.Fatal(encodeErr)
	}
	t.Log(string(encoded))
	if traceErr != nil {
		t.Fatal("original replacement kernel control failed", traceErr, waitErr)
	}
	if !errors.Is(waitErr, syscall.ECHILD) {
		t.Fatal("original tracee was not reaped by its tracer", waitErr)
	}
	raw, err := os.ReadFile(output)
	// The canonical command deliberately redacts private causes. This control
	// proves signal delivery, failed original exit and absence of success; it
	// cannot inspect the in-process errors.Is chain from public output.
	if err != nil || !bytes.Contains(raw, []byte("source command failed")) || bytes.Contains(raw, []byte("source-replacement-stopped-selected")) {
		t.Fatal("original failed result lost", err, string(raw))
	}
}

type sourceReplacementCutObservation struct {
	PID, ThreadsJoined, ReloadFD, HeldThread int
	ReloadBytes, ReplyBytes                  int64
	OriginalExit                             int
	CancellationAcknowledged                 bool
	NoticeWritesObserved, HeldStopsBeforeAck int
	Pending, Journal, Selected, Unit         sourceReplacementCutFile
}

type sourceReplacementCutFile struct {
	Path           string
	Device, Inode  uint64
	Mode, UID, GID uint32
	Size           int64
	SHA256         string
}

type sourceReplacementTraceCall struct {
	number, fd, address, size uint64
	reload, notice            bool
}

const (
	sourceCutWaitAllThreads = 0x40000000 // Linux __WALL includes traced thread children.
	sourceCutExitKill       = 0x100000   // PTRACE_O_EXITKILL fences a lost original tracer.
)

// Every return drains all attached threads. A held native read cannot report
// completion while the kernel still owns its return; signal delivery precedes
// releasing that read. Neither a phase poll nor elapsed sleep selects the cut.
func traceSourceReplacementCancellation(pid int, process *os.Process, binding string) (result sourceReplacementCutObservation, returned error) {
	result.PID, result.ReloadFD = pid, -1
	threads := map[int]bool{pid: true}
	defer func() {
		if len(threads) == 0 {
			return
		}
		returned = errors.Join(returned, process.Kill())
		for len(threads) != 0 {
			var status syscall.WaitStatus
			tid, err := syscall.Wait4(-1, &status, sourceCutWaitAllThreads, nil)
			if err == syscall.EINTR {
				continue
			}
			if err != nil {
				returned = errors.Join(returned, err)
				return
			}
			if status.Exited() || status.Signaled() {
				delete(threads, tid)
			} else {
				_ = syscall.PtraceCont(tid, int(syscall.SIGKILL))
			}
		}
	}()
	var status syscall.WaitStatus
	if tid, err := syscall.Wait4(pid, &status, 0, nil); err != nil || tid != pid || !status.Stopped() {
		return result, errors.Join(errors.New("original exec trap unavailable"), err)
	}
	if err := syscall.PtraceSetOptions(pid, syscall.PTRACE_O_TRACESYSGOOD|syscall.PTRACE_O_TRACECLONE|sourceCutExitKill); err != nil {
		return result, err
	}
	if err := syscall.PtraceSyscall(pid, 0); err != nil {
		return result, err
	}
	calls := map[int]sourceReplacementTraceCall{}
	held, canceled, delivered := 0, false, false
	for len(threads) != 0 {
		tid, err := syscall.Wait4(-1, &status, sourceCutWaitAllThreads, nil)
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			return result, err
		}
		if !threads[tid] {
			if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid), "task", strconv.Itoa(tid))); err != nil || len(threads) >= 128 {
				return result, errors.Join(errors.New("foreign trace child"), err)
			}
			threads[tid] = true
		}
		if status.Exited() || status.Signaled() {
			delete(threads, tid)
			delete(calls, tid)
			result.ThreadsJoined++
			if tid == pid {
				result.OriginalExit = status.ExitStatus()
			}
			continue
		}
		if !status.Stopped() {
			return result, errors.New("unknown trace status")
		}
		if held != 0 && tid == held {
			result.HeldStopsBeforeAck++
		}
		if status.TrapCause() == syscall.PTRACE_EVENT_CLONE {
			child, err := syscall.PtraceGetEventMsg(tid)
			if err != nil || len(threads) >= 128 {
				return result, errors.Join(errors.New("original thread bound"), err)
			}
			threads[int(child)] = true
		}
		if status.StopSignal() == syscall.SIGTRAP|0x80 {
			var regs syscall.PtraceRegs
			if err := syscall.PtraceGetRegs(tid, &regs); err != nil {
				return result, fmt.Errorf("read original registers thread %d with held thread %d: %w", tid, held, err)
			}
			// Other Linux targets remain compilable; admission above selects the
			// amd64 ABI before any installed execution.
			values := reflect.ValueOf(regs)
			reg := func(name string) uint64 { return values.FieldByName(name).Uint() }
			call, exists := calls[tid]
			if int64(reg("Rax")) == -int64(syscall.ENOSYS) {
				call = sourceReplacementTraceCall{number: reg("Orig_rax"), fd: reg("Rdi"), address: reg("Rsi"), size: reg("Rdx")}
				if held != 0 && call.number == syscall.SYS_WRITE && call.fd == 1 && call.size > 0 && call.size <= 4096 {
					body := make([]byte, int(call.size))
					if n, err := syscall.PtracePeekData(tid, uintptr(call.address), body); err == nil && n == len(body) &&
						bytes.Equal(body, []byte("{\"operation\":\"installation.source\",\"outcome\":\"completion-unavailable\"}\n")) {
						// The existing production notice requires canceled original
						// context and still-unjoined physical work. Observe its real
						// completed write before releasing the native manager read.
						call.notice = true
					}
				}
				if call.number == syscall.SYS_WRITE && call.size > 0 && call.size <= 65536 && !canceled {
					body := make([]byte, int(call.size))
					if n, err := syscall.PtracePeekData(tid, uintptr(call.address), body); err == nil && n == len(body) &&
						bytes.Contains(body, []byte("org.freedesktop.systemd1.Manager\x00")) && bytes.Contains(body, []byte("Reload\x00")) {
						name, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "fd", strconv.FormatUint(call.fd, 10)))
						if err != nil || !strings.HasPrefix(name, "socket:[") || result.ReloadFD != -1 {
							return result, errors.Join(errors.New("original single Reload socket unavailable"), err)
						}
						call.reload = true
					}
				}
				calls[tid] = call
			} else if exists {
				delete(calls, tid)
				returned := int64(reg("Rax"))
				if call.reload {
					if returned != int64(call.size) {
						return result, fmt.Errorf("actual Reload send incomplete: %d", returned)
					}
					result.ReloadFD, result.ReloadBytes = int(call.fd), returned
				}
				if call.notice {
					result.NoticeWritesObserved++
					if !delivered || held == 0 || returned != int64(call.size) {
						return result, errors.New("cancellation notice did not complete under the held read")
					}
					result.CancellationAcknowledged = true
					if err := syscall.PtraceSyscall(held, 0); err != nil {
						return result, fmt.Errorf("release original held read thread %d after notice: %w", held, err)
					}
					held = 0
				}
				if call.number == syscall.SYS_READ && int(call.fd) == result.ReloadFD && returned > 0 && !canceled {
					// The real original socket has read manager bytes after the
					// actual reload send. Keep its native return held until the
					// original SIGTERM has been delivered to another thread.
					pending := filepath.Join(filepath.Dir(binding), "source-replacement-pending")
					var body []byte
					result.Pending, body, err = snapshotSourceReplacementCutFile(pending)
					if err != nil || !bytes.HasPrefix(body, []byte("ardents-source-replacement-pending-v1\n")) {
						return result, errors.Join(errors.New("original blocker absent at Reload read"), err)
					}
					selected, err := os.ReadFile(binding)
					var selection struct {
						Configuration struct {
							Generation string `json:"generation"`
						} `json:"configuration"`
					}
					if err != nil || json.Unmarshal(selected, &selection) != nil || len(selection.Configuration.Generation) != 64 {
						return result, errors.Join(errors.New("selected replacement unavailable"), err)
					}
					journal := filepath.Dir(binding) + ".source-replacement-" + selection.Configuration.Generation + "-journal"
					result.Journal, _, err = snapshotSourceReplacementCutFile(journal)
					if err != nil {
						return result, err
					}
					result.Selected, _, err = snapshotSourceReplacementCutFile(binding)
					if err != nil {
						return result, err
					}
					result.Unit, _, err = snapshotSourceReplacementCutFile("/etc/systemd/system/ardents-source.service")
					if err != nil {
						return result, err
					}
					result.HeldThread, result.ReplyBytes, held, canceled = tid, returned, tid, true
					signalThread := pid
					if signalThread == held {
						signalThread = 0
						for candidate := range threads {
							if candidate != held {
								signalThread = candidate
								break
							}
						}
					}
					if signalThread == 0 {
						return result, errors.New("no independent original signal thread")
					}
					if err := syscall.Tgkill(pid, signalThread, syscall.SIGTERM); err != nil {
						return result, err
					}
					continue
				}
			}
		}
		signal := 0
		if status.StopSignal() != syscall.SIGTRAP && status.StopSignal() != syscall.SIGSTOP && status.StopSignal() != syscall.SIGTRAP|0x80 {
			signal = int(status.StopSignal())
			if canceled && status.StopSignal() == syscall.SIGTERM {
				delivered = true
			}
		}
		if err := syscall.PtraceSyscall(tid, signal); err != nil && err != syscall.ESRCH {
			return result, err
		}
	}
	if !canceled || !delivered || !result.CancellationAcknowledged || held != 0 || result.OriginalExit != 2 || result.ThreadsJoined == 0 {
		return result, errors.New("actual Reload cancellation and original thread join unproven")
	}
	for _, original := range []sourceReplacementCutFile{result.Pending, result.Journal, result.Selected, result.Unit} {
		current, _, err := snapshotSourceReplacementCutFile(original.Path)
		if err != nil || current != original {
			return result, errors.Join(errors.New("original blocker/prefix changed during cancellation or cleanup"), err)
		}
	}
	return result, nil
}

func snapshotSourceReplacementCutFile(path string) (result sourceReplacementCutFile, body []byte, returned error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return result, nil, err
	}
	defer func() { returned = errors.Join(returned, f.Close()) }()
	before, err := f.Stat()
	if err != nil {
		return result, nil, err
	}
	if !before.Mode().IsRegular() || before.Size() > 4<<20 {
		return result, nil, errors.New("invalid original cut file")
	}
	body, err = io.ReadAll(io.LimitReader(f, (4<<20)+1))
	if err != nil {
		return result, nil, err
	}
	after, err := f.Stat()
	named, namedErr := os.Lstat(path)
	if err != nil || namedErr != nil || !os.SameFile(before, after) || !os.SameFile(after, named) ||
		before.Size() != int64(len(body)) || !sameSourceCutMetadata(before, after) || !sameSourceCutMetadata(after, named) {
		return result, nil, errors.Join(errors.New("original cut file changed during read"), err, namedErr)
	}
	native := after.Sys().(*syscall.Stat_t)
	return sourceReplacementCutFile{Path: path, Device: uint64(native.Dev), Inode: native.Ino, Mode: native.Mode, UID: native.Uid, GID: native.Gid, Size: after.Size(), SHA256: fmt.Sprintf("%x", sha256.Sum256(body))}, body, nil
}

func sameSourceCutMetadata(a, b os.FileInfo) bool {
	x, y := a.Sys().(*syscall.Stat_t), b.Sys().(*syscall.Stat_t)
	// Reading may advance atime; it cannot authorize a byte/access/inode change.
	return x.Dev == y.Dev && x.Ino == y.Ino && x.Mode == y.Mode && x.Uid == y.Uid && x.Gid == y.Gid &&
		x.Nlink == 1 && y.Nlink == 1 && x.Size == y.Size && x.Mtim == y.Mtim && x.Ctim == y.Ctim
}
