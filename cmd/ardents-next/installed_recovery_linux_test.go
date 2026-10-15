//go:build linux && text_worker_installed

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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

// This is a kernel fault controller inside the pinned test binary, not an
// Installation actor. Its observations grant no Release, Startup or recovery
// authority. The parent retains the actual Source/clock and signed Candidate.
type installedRecoveryCut struct {
	RootPID                int
	ACKDigest              string
	GuardSync, JournalSync uint64
	ThreadsJoined          int
}

func installedInterruptedReplacement(t *testing.T, program, request, base string) {
	t.Helper()
	if runtime.GOARCH != "amd64" {
		t.Fatal("invalid environment: selected x86-64 kernel syscall observer required")
	}
	cutPath := filepath.Join(base, "replacement-cut.json")
	command := exec.CommandContext(t.Context(), program, "installation", "upgrade-installed", "--request", request)
	command.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	var output, diagnostic bytes.Buffer
	command.Stdout, command.Stderr = &output, &diagnostic
	// Retain the creating OS thread through original Root join, as for ordinary
	// installed commands. No failed Root outcome is projected to success.
	runtime.LockOSThread()
	if err := command.Start(); err != nil {
		runtime.UnlockOSThread()
		t.Fatal(err)
	}
	rootPin, pinErr := os.Open(filepath.Join("/proc", strconv.Itoa(command.Process.Pid)))
	self, selfErr := os.Executable()
	var controllerErr error
	var controllerOutput []byte
	if pinErr == nil && selfErr == nil {
		controller := exec.CommandContext(t.Context(), self, "-test.run=^TestInstalledRecoveryCutChild$", "-test.timeout=90s")
		controller.Env = append(os.Environ(),
			"ARDENTS_INSTALLATION_CUT_PID="+strconv.Itoa(command.Process.Pid),
			"ARDENTS_INSTALLATION_CUT_BASE="+base,
			"ARDENTS_INSTALLATION_CUT_REQUEST="+request,
			"ARDENTS_INSTALLATION_CUT_PROGRAM="+program,
			"ARDENTS_INSTALLATION_CUT_RECEIPT="+cutPath)
		controllerOutput, controllerErr = controller.CombinedOutput()
	}
	if pinErr != nil || selfErr != nil || controllerErr != nil {
		_ = command.Process.Kill()
	}
	rootErr := command.Wait()
	runtime.UnlockOSThread()
	if rootPin != nil {
		defer rootPin.Close()
	}
	if pinErr != nil || selfErr != nil || controllerErr != nil {
		t.Fatalf("original Root cut controller: %v %v %v; %s; Root: %v stdout=%s diagnostics=%s", pinErr, selfErr, controllerErr, controllerOutput, rootErr, output.String(), diagnostic.String())
	}
	var exit *exec.ExitError
	if !errors.As(rootErr, &exit) || exit.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL || output.Len() != 0 {
		t.Fatalf("original Root did not retain deliberate SIGKILL: %v; %s; %s", rootErr, output.String(), diagnostic.String())
	}
	if _, err := os.ReadFile(filepath.Join("/proc/self/fd", strconv.Itoa(int(rootPin.Fd())), "stat")); !errors.Is(err, os.ErrNotExist) && !errors.Is(err, syscall.ESRCH) {
		t.Fatal("original interrupted Root has not joined", err)
	}
	var cut installedRecoveryCut
	raw, err := os.ReadFile(cutPath)
	if err != nil || json.Unmarshal(raw, &cut) != nil || cut.RootPID != command.Process.Pid || len(cut.ACKDigest) != 64 || cut.GuardSync == 0 || cut.JournalSync <= cut.GuardSync || cut.ThreadsJoined == 0 {
		t.Fatal("original kernel cut lacks full ACK, durable guard record and thread join", err)
	}
	installation := filepath.Join(base, "installation")
	before, floors := installedRecoveryInventory(t, installation), installedRecoveryInventory(t, filepath.Join(base, "floors"))
	identity := installedStartupIdentity(t)
	if identity[0] == "0" || identity[2] != "active" {
		t.Fatal("post-ACK original Endpoint is not running", identity)
	}
	processPin, err := os.Open(filepath.Join("/proc", identity[0]))
	if err != nil {
		t.Fatal(err)
	}
	defer processPin.Close()
	scopePin, err := os.Open("/sys/fs/cgroup/system.slice/ardents-endpoint.service/cgroup.events")
	if err != nil {
		t.Fatal(err)
	}
	defer scopePin.Close()
	var declaration struct {
		Reference string `json:"reference_time"`
	}
	raw, err = os.ReadFile(request)
	if err != nil || json.Unmarshal(raw, &declaration) != nil {
		t.Fatal("original recovery reference unavailable", err)
	}
	reference, err := time.Parse(time.RFC3339Nano, declaration.Reference)
	if err != nil {
		t.Fatal(err)
	}
	// The Release fixture's reference is explicit and separate from its actual
	// live Network clock. This new evaluation is not independent trusted time.
	installedStartupCommand(t, program, nil, "installed-recovered-started", "installation", "recover-installed", installation, reference.Add(time.Second).Format(time.RFC3339Nano))
	if installedStartupIdentity(t) != identity {
		t.Fatal("terminal recovery replayed Start or changed original invocation")
	}
	currentProc, err := os.Stat(filepath.Join("/proc", identity[0]))
	originalProc, originalErr := processPin.Stat()
	if err != nil || originalErr != nil || !os.SameFile(currentProc, originalProc) {
		t.Fatal("recovery lost original process custody", err, originalErr)
	}
	currentScope, err := os.Stat("/sys/fs/cgroup/system.slice/ardents-endpoint.service/cgroup.events")
	originalScope, originalErr := scopePin.Stat()
	if err != nil || originalErr != nil || !os.SameFile(currentScope, originalScope) {
		t.Fatal("recovery replaced original kernel scope", err, originalErr)
	}
	after := installedRecoveryInventory(t, installation)
	guard := filepath.Join(installation, "start-guard.json")
	if _, exists := before[guard]; !exists {
		t.Fatal("cut lacks original guard")
	}
	delete(before, guard)
	if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(floors, installedRecoveryInventory(t, filepath.Join(base, "floors"))) {
		t.Fatal("fresh recovery changed bytes, original files or Release floors beyond guard removal")
	}
	t.Log("original Root SIGKILL joined after full ACK and synced removal records; fresh recovery removed only original guard and preserved running process/scope/floors before genuine Service")
}

type installedCutSync struct {
	Path          string
	Device, Inode uint64
	Digest        string
	Order         uint64
}

// The child owns all original ptrace attachments through physical thread join.
// A missing privilege, ABI, live original or exact cut fails this profile.
func TestInstalledRecoveryCutChild(t *testing.T) {
	pid, err := strconv.Atoi(os.Getenv("ARDENTS_INSTALLATION_CUT_PID"))
	base, request, program := os.Getenv("ARDENTS_INSTALLATION_CUT_BASE"), os.Getenv("ARDENTS_INSTALLATION_CUT_REQUEST"), os.Getenv("ARDENTS_INSTALLATION_CUT_PROGRAM")
	receipt := os.Getenv("ARDENTS_INSTALLATION_CUT_RECEIPT")
	if err != nil || pid <= 1 || runtime.GOARCH != "amd64" || os.Geteuid() != 0 || !filepath.IsAbs(base) || !filepath.IsAbs(program) || !filepath.IsAbs(request) || receipt != filepath.Join(base, "replacement-cut.json") {
		t.Fatal("invalid environment: original root process and exact controller declarations required")
	}
	proc := filepath.Join("/proc", strconv.Itoa(pid))
	pin, err := os.Open(proc)
	if err != nil {
		t.Fatal(err)
	}
	defer pin.Close()
	arguments, err := os.ReadFile(filepath.Join(proc, "cmdline"))
	want := []byte(strings.Join([]string{program, "installation", "upgrade-installed", "--request", request, ""}, "\x00"))
	if err != nil || !bytes.Equal(arguments, want) {
		t.Fatal("foreign original Root", err)
	}
	exe, err := os.Stat(filepath.Join(proc, "exe"))
	expected, expectedErr := os.Stat(program)
	if err != nil || expectedErr != nil || !os.SameFile(exe, expected) {
		t.Fatal("foreign original Root executable", err, expectedErr)
	}
	rootProcess, err := os.FindProcess(pid)
	if err != nil {
		t.Fatal(err)
	}
	defer rootProcess.Release()
	// The true parent retains this unreaped child until this controller joins,
	// preventing PID reuse. EXITKILL additionally fences lost tracer cleanup.
	defer rootProcess.Kill()
	deadline := time.Now().Add(80 * time.Second)
	// Tracing large authenticated generation observations changes their scheduling
	// substantially. Observe the current request's archived-start boundary before
	// attaching; every claimed ACK/removal event is still observed at the kernel,
	// and a missed cut fails. This phase marker grants no Installation authority.
	var declaration struct {
		Bundle string `json:"bundle_root"`
	}
	raw, err := os.ReadFile(request)
	if err != nil || json.Unmarshal(raw, &declaration) != nil || declaration.Bundle == "" {
		t.Fatal("original request declaration", err)
	}
	ready := false
	for time.Now().Before(deadline) {
		var intent struct {
			Request struct {
				Bundle string `json:"bundle_root"`
			} `json:"request"`
			Candidate struct {
				Generation string `json:"generation_digest"`
			} `json:"candidate"`
		}
		var selection struct {
			Generation string `json:"generation_digest"`
		}
		body, readErr := os.ReadFile(filepath.Join(base, "installation", "selection.json"))
		if readErr == nil && json.Unmarshal(body, &selection) == nil && len(selection.Generation) == 64 {
			body, readErr = os.ReadFile(filepath.Join(base, "installation", "journals", selection.Generation, "completed-intent.json"))
			if readErr == nil && json.Unmarshal(body, &intent) == nil && intent.Request.Bundle == declaration.Bundle && intent.Candidate.Generation == selection.Generation {
				ready = true
				break
			}
		}
		stat, err := os.ReadFile(filepath.Join(proc, "stat"))
		if err != nil {
			t.Fatal("original Root ended before trace boundary", err)
		}
		fields := strings.Fields(string(stat[bytes.LastIndexByte(stat, ')')+1:]))
		if len(fields) == 0 || fields[0] == "Z" || fields[0] == "X" {
			t.Fatal("original Root terminated before current-request archived-start boundary")
		}
		time.Sleep(time.Millisecond)
	}
	if !ready {
		t.Fatal("original current-request archived-start boundary unavailable")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	threads := make(map[int]bool)
	entries, err := os.ReadDir(filepath.Join(proc, "task"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		tid, err := strconv.Atoi(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		_, _, nativeErr := syscall.Syscall6(syscall.SYS_PTRACE, 0x4206, uintptr(tid), 0, uintptr(syscall.PTRACE_O_TRACESYSGOOD|syscall.PTRACE_O_TRACECLONE|0x100000), 0, 0)
		if nativeErr == syscall.ESRCH {
			continue
		}
		if nativeErr != 0 {
			t.Fatal("invalid environment: original ptrace seize", nativeErr)
		}
		threads[tid] = true
		_, _, nativeErr = syscall.Syscall6(syscall.SYS_PTRACE, 0x4207, uintptr(tid), 0, 0, 0, 0)
		if nativeErr != 0 && nativeErr != syscall.ESRCH {
			t.Fatal(nativeErr)
		}
	}
	if len(threads) == 0 {
		t.Fatal("no original Root threads")
	}
	joined := 0
	var sequence uint64
	var ack string
	pending := make(map[int]installedCutSync)
	var synced []installedCutSync
	calls := make(map[uint64]uint64)
	cut := false
	var guardOrder, journalOrder uint64
	// Wait in the kernel for the next original trace event instead of polling
	// every stopped thread. The original deadline still kills a missed cut and
	// wakes Wait4; it cannot manufacture an observed ACK or successful cut.
	timedOut := make(chan struct{})
	deadlineKill := time.AfterFunc(time.Until(deadline), func() {
		_ = rootProcess.Kill()
		close(timedOut)
	})
	defer func() {
		if !deadlineKill.Stop() {
			<-timedOut
		}
	}()
	for len(threads) != 0 && time.Now().Before(deadline) {
		var status syscall.WaitStatus
		tid, err := syscall.Wait4(-1, &status, 0x40000000, nil)
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			t.Fatal("original tracer wait", err)
		}
		if !threads[tid] {
			// A new CLONE child's initial stop may precede its parent's
			// event. Accept only a real member of this unreaped Root's
			// original thread group; no unrelated child can enter custody.
			if _, err := os.Stat(filepath.Join(proc, "task", strconv.Itoa(tid))); err != nil || len(threads) >= 128 {
				t.Fatal("foreign or unavailable original trace thread", tid, err)
			}
			threads[tid] = true
		}
		if status.Exited() || status.Signaled() {
			delete(threads, tid)
			joined++
			continue
		}
		if !status.Stopped() {
			t.Fatal("unrecognized original trace outcome")
		}
		if status.TrapCause() == syscall.PTRACE_EVENT_CLONE {
			child, err := syscall.PtraceGetEventMsg(tid)
			if err != nil || len(threads) >= 128 {
				t.Fatal("original thread inventory", err)
			}
			threads[int(child)] = true
		}
		if status.StopSignal() == syscall.SIGTRAP|0x80 && !cut {
			var registers syscall.PtraceRegs
			if err := syscall.PtraceGetRegs(tid, &registers); err != nil {
				t.Fatal(err)
			}
			// Reflection keeps other Linux architectures compilable; the
			// selected profile refuses before effects unless ABI is x86-64.
			values := reflect.ValueOf(registers)
			register := func(name string) uint64 { return values.FieldByName(name).Uint() }
			call, returned := register("Orig_rax"), int64(register("Rax"))
			calls[call]++
			sequence++
			if call == syscall.SYS_FSYNC {
				fd := filepath.Join("/proc", strconv.Itoa(tid), "fd", strconv.FormatUint(register("Rdi"), 10))
				if returned == -int64(syscall.ENOSYS) {
					path, err := os.Readlink(fd)
					if err != nil {
						t.Fatal(err)
					}
					info, err := os.Stat(fd)
					if err != nil {
						t.Fatal(err)
					}
					native := info.Sys().(*syscall.Stat_t)
					observation := installedCutSync{Path: path, Device: uint64(native.Dev), Inode: native.Ino}
					if strings.HasSuffix(path, "/completion-guard-removal.json") {
						body, err := os.ReadFile(fd)
						if err != nil {
							t.Fatal(err)
						}
						digest := sha256.Sum256(body)
						observation.Digest = hex.EncodeToString(digest[:])
					}
					pending[tid] = observation
				} else if returned == 0 {
					if observation, exists := pending[tid]; exists {
						observation.Order = sequence
						synced = append(synced, observation)
						delete(pending, tid)
					}
				}
			}
			if call == syscall.SYS_WRITE && returned == 160 && register("Rdx") == 160 {
				if observed := installedCutACK(t, tid, register("Rdi"), register("Rsi"), base); observed != "" {
					ack = observed
				}
			}
			if call == syscall.SYS_UNLINKAT && returned == -int64(syscall.ENOSYS) {
				name := make([]byte, 256)
				n, err := syscall.PtracePeekData(tid, uintptr(register("Rsi")), name)
				if err != nil || n != len(name) {
					t.Fatal("original unlink pathname", err)
				}
				if end := bytes.IndexByte(name, 0); end >= 0 {
					name = name[:end]
				}
				guard := filepath.Join(base, "installation", "start-guard.json")
				if string(name) == "start-guard.json" || string(name) == guard {
					if string(name) == "start-guard.json" {
						parent, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(tid), "fd", strconv.FormatUint(register("Rdi"), 10)))
						if err != nil || parent != filepath.Dir(guard) {
							t.Fatal("foreign guard parent", err)
						}
					}
					if ack == "" {
						t.Fatal("guard unlink before independently observed full ACK")
					}
					_, _, journal := installedCutSelection(t, base)
					for _, record := range []string{"completion-socket-removal.json", "completion-record-removal.json", "completion-guard-removal.json"} {
						if _, err := os.Stat(filepath.Join(journal, record)); err != nil {
							t.Fatal("missing original removal record", err)
						}
					}
					for _, leaf := range []string{"start-completion.socket", "start-socket.json"} {
						if _, err := os.Lstat(filepath.Join(base, "installation", leaf)); !errors.Is(err, os.ErrNotExist) {
							t.Fatal("earlier completion removal unfinished", err)
						}
					}
					record := filepath.Join(journal, "completion-guard-removal.json")
					info, err := os.Stat(record)
					if err != nil {
						t.Fatal(err)
					}
					native := info.Sys().(*syscall.Stat_t)
					body, err := os.ReadFile(record)
					if err != nil {
						t.Fatal(err)
					}
					digest := sha256.Sum256(body)
					for _, sync := range synced {
						if sync.Device == uint64(native.Dev) && sync.Inode == native.Ino && sync.Digest == hex.EncodeToString(digest[:]) {
							guardOrder = sync.Order
						}
					}
					info, err = os.Stat(journal)
					if err != nil {
						t.Fatal(err)
					}
					native = info.Sys().(*syscall.Stat_t)
					for _, sync := range synced {
						if sync.Device == uint64(native.Dev) && sync.Inode == native.Ino && sync.Order > guardOrder {
							journalOrder = sync.Order
						}
					}
					if guardOrder == 0 || journalOrder <= guardOrder {
						t.Fatal("guard record and original journal not independently synced")
					}
					if err := rootProcess.Kill(); err != nil {
						t.Fatal(err)
					}
					cut = true
				}
			}
		}
		signal := 0
		if status.StopSignal() != syscall.SIGTRAP && status.StopSignal() != syscall.SIGSTOP && status.StopSignal() != syscall.SIGTRAP|0x80 {
			signal = int(status.StopSignal())
		}
		if err := syscall.PtraceSyscall(tid, signal); err != nil && err != syscall.ESRCH {
			t.Fatal("original trace continuation", err)
		}
	}
	if !cut || len(threads) != 0 || joined == 0 {
		t.Fatal("exact original cut/thread join unavailable", cut, len(threads), joined, "kernel stops", sequence, "syscall counts", calls)
	}
	raw, err = json.Marshal(installedRecoveryCut{RootPID: pid, ACKDigest: ack, GuardSync: guardOrder, JournalSync: journalOrder, ThreadsJoined: joined})
	if err != nil {
		t.Fatal(err)
	}
	installedStartupWrite(t, receipt, raw, 0600)
}

func installedCutACK(t *testing.T, tid int, descriptor, address uint64, base string) string {
	t.Helper()
	fd := filepath.Join("/proc", strconv.Itoa(tid), "fd", strconv.FormatUint(descriptor, 10))
	path, err := os.Readlink(fd)
	if err != nil {
		t.Fatal("original output descriptor", err)
	}
	if !strings.HasPrefix(path, "socket:[") {
		return ""
	}
	frame := make([]byte, 160)
	if n, err := syscall.PtracePeekData(tid, uintptr(address), frame); err != nil || n != len(frame) {
		t.Fatal("original full ACK memory", err)
	}
	// Other original bus/TLS writes supply no completion observation. The
	// closed frame consists of precisely 160 lowercase hexadecimal bytes.
	for _, value := range frame {
		if !(value >= '0' && value <= '9' || value >= 'a' && value <= 'f') {
			return ""
		}
	}
	selection, started, _ := installedCutSelection(t, base)
	if !bytes.Equal(frame, []byte(started.InvocationID+selection.GenerationDigest+selection.BindingDigest)) {
		return ""
	}
	digest := sha256.Sum256(frame)
	return hex.EncodeToString(digest[:])
}

func installedCutSelection(t *testing.T, base string) (selection struct {
	GenerationDigest string `json:"generation_digest"`
	BindingDigest    string `json:"binding_digest"`
}, started struct {
	InvocationID string `json:"invocation_id"`
}, journal string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(base, "installation", "selection.json"))
	if err != nil || json.Unmarshal(raw, &selection) != nil || len(selection.GenerationDigest) != 64 || len(selection.BindingDigest) != 64 {
		t.Fatal("original selection unavailable", err)
	}
	journal = filepath.Join(base, "installation", "journals", selection.GenerationDigest)
	raw, err = os.ReadFile(filepath.Join(journal, "started-invocation.json"))
	if err != nil || json.Unmarshal(raw, &started) != nil || len(started.InvocationID) != 32 {
		t.Fatal("original started record unavailable", err)
	}
	return
}

type installedRecoveryMember struct {
	Device, Inode                                            uint64
	UID, GID, Mode                                           uint32
	Size, ChangedSec, ChangedNSec, ModifiedSec, ModifiedNSec int64
	Digest                                                   [32]byte
}

func installedRecoveryInventory(t *testing.T, root string) map[string]installedRecoveryMember {
	t.Helper()
	result := make(map[string]installedRecoveryMember)
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return errors.New("unexpected recovery inventory member")
		}
		native, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return errors.New("missing actual recovery inode")
		}
		member := installedRecoveryMember{Device: uint64(native.Dev), Inode: native.Ino, UID: native.Uid, GID: native.Gid, Mode: native.Mode}
		if info.Mode().IsRegular() {
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			member.Digest = sha256.Sum256(body)
			member.Size = native.Size
			member.ChangedSec, member.ChangedNSec = native.Ctim.Sec, native.Ctim.Nsec
			member.ModifiedSec, member.ModifiedNSec = native.Mtim.Sec, native.Mtim.Nsec
		}
		result[path] = member
		return nil
	})
	if err != nil {
		t.Fatal("original recovery inventory", err)
	}
	return result
}
