//go:build linux && amd64 && network_signing_native

// Exact native observer preparation. Native mode executes only the genuine
// file/process test driver. Installed mode requires a separately pinned finite
// profile and the real private Source stand. Do not invoke installed mode before
// direct PO acceptance of its NEW completed exact ADR-0123 Decision6 receipt.
// A receipt hash is identification, never evidence of that human decision.
//
// ABI/protocol references inspected 2026-10-24:
// https://codebrowser.dev/linux/linux/arch/x86/include/asm/user_64.h.html
// https://codebrowser.dev/linux/linux/arch/x86/kernel/ptrace.c.html
// https://man7.org/linux/man-pages/man2/ptrace.2.html
package infrastructure

import (
	"bytes"
	"crypto/sha256"
	"debug/buildinfo"
	"debug/elf"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

const (
	sourceCutSeize     = 0x4206
	sourceCutInterrupt = 0x4207
	sourceCutListen    = 0x4208
	sourceCutEventStop = 128
	sourceCutWaitAll   = 0x40000000
	sourceCutExitKill  = 0x100000
	// Linux x86-64 struct user: 216-byte regs, 8-byte fpvalid/pad,
	// 512-byte i387, ten64-bit words/pads, then32-byte comm =>848.
	sourceCutDebugRegisters        = 848
	sourceCutAcknowledgementSymbol = "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure.(*sourceTransitionControl).acknowledge"
)

type sourceCutObservation struct {
	PhysicalWindow                   string `json:"physical_window,omitempty"`
	HeldCoordinatorThread            int    `json:"held_coordinator_thread,omitempty"`
	HeldCoordinatorKilled            bool   `json:"held_coordinator_killed,omitempty"`
	RootCloseEntry                   uint64 `json:"root_close_entry,omitempty"`
	RootOwnershipReleasedBeforeExit  bool   `json:"root_ownership_released_before_exit,omitempty"`
	RootCloseObservations            int    `json:"root_close_observations,omitempty"`
	InspectionOnly                   bool   `json:"inspection_only"`
	ProfileSHA256                    string `json:"profile_sha256,omitempty"`
	PublicReceiptSHA256              string `json:"public_receipt_sha256,omitempty"`
	InstalledExecution               bool   `json:"installed_execution"`
	ExternalCancellationAcknowledged bool   `json:"external_cancellation_acknowledged"`
	HeldRootThread                   int    `json:"held_root_thread"`
	SignalledRootThread              int    `json:"signalled_root_thread"`
	SelectedJournalSHA256            string `json:"selected_journal_sha256"`
	OriginalCutSHA256                string `json:"original_cut_sha256"`
	ProgramSHA256                    string `json:"program_sha256"`
	Entry                            uint64 `json:"entry"`
	RootPID                          int    `json:"root_pid"`
	RootExit                         int    `json:"root_exit"`
	ThreadsJoined                    int    `json:"threads_joined"`
	Processes                        int    `json:"processes"`
	AcknowledgementHits              int    `json:"acknowledgement_hits"`
	GroupStops                       int    `json:"group_stops"`
	SelectedRecords                  int    `json:"selected_records"`
	OutputSHA256                     string `json:"output_sha256"`
}

// This describes only an observed initial/replacement process cut. It cannot
// start a Service or run recovery and makes no qualification/authority claim.
type sourceCutObserverProfile struct {
	Schema              string   `json:"schema"`
	ProgramSHA256       string   `json:"program_sha256"`
	PublicReceiptSHA256 string   `json:"public_receipt_sha256"`
	Arguments           []string `json:"arguments"`
	Journal             string   `json:"journal"`
	PrefixRecords       int      `json:"prefix_records"`
	LastPhase           string   `json:"last_phase"`
	sha256              string
}

func sourceCutObserverSHA(value string) bool {
	raw, err := hex.DecodeString(value)
	return err == nil && len(raw) == 32 && value == strings.ToLower(value)
}

// This bounds the explicit closed-alpha observer's local declaration. Release
// alone compares the actual identifier with its authenticated metadata; this
// is not a new product-wide Network grammar or authority rule.
func sourceCutObserverNetwork(value string) bool {
	if len(value) < 1 || len(value) > 256 {
		return false
	}
	for _, c := range value {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == ':') {
			return false
		}
	}
	return true
}

func readSourceCutObserverFile(path string, maximum int64) (raw []byte, info os.FileInfo, returned error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		returned = errors.Join(returned, f.Close())
		if returned != nil {
			raw = nil
		}
	}()
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maximum {
		return nil, nil, errors.Join(errors.New("bounded original native regular file required"), err)
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	if !ok || native.Uid != 0 || native.Gid != 0 || info.Mode().Perm() != 0600 {
		return nil, nil, errors.New("original root0600 file required")
	}
	var fs syscall.Statfs_t
	if err := syscall.Fstatfs(int(f.Fd()), &fs); err != nil || fs.Type != 0xef53 {
		return nil, nil, errors.Join(errors.New("original ext4 file required"), err)
	}
	raw, err = io.ReadAll(io.LimitReader(f, maximum+1))
	after, statErr := f.Stat()
	named, namedErr := os.Lstat(path)
	if err != nil || statErr != nil || namedErr != nil || int64(len(raw)) != info.Size() || !os.SameFile(info, after) || !os.SameFile(info, named) || info.Mode() != after.Mode() || info.Size() != after.Size() {
		return nil, nil, errors.Join(errors.New("original native file changed during read"), err, statErr, namedErr)
	}
	return raw, info, nil
}

func readSourceCutObserverProfile(path, pin, program, root string) (profile *sourceCutObserverProfile, returned error) {
	if !filepath.IsAbs(path) || !sourceCutObserverSHA(pin) || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return nil, errors.New("exact absolute profile and SHA256 required")
	}
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer func() {
		returned = errors.Join(returned, f.Close())
		if returned != nil {
			profile = nil
		}
	}()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 64<<10 {
		return nil, errors.Join(errors.New("bounded regular profile required"), err)
	}
	raw, err := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	sum := sha256.Sum256(raw)
	if err != nil || int64(len(raw)) != info.Size() || hex.EncodeToString(sum[:]) != pin {
		return nil, errors.Join(errors.New("profile pin differs"), err)
	}
	p := new(sourceCutObserverProfile)
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(p); err != nil {
		return nil, err
	}
	canonical, encoded := json.Marshal(p)
	if encoded != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return nil, errors.New("canonical exact profile required")
	}
	if decoder.Decode(new(any)) != io.EOF || p.Schema != "source-original-cut-observer-v1" ||
		!sourceCutObserverSHA(p.ProgramSHA256) || !sourceCutObserverSHA(p.PublicReceiptSHA256) || len(p.Arguments) != 8 ||
		p.Arguments[0] != "installation" || p.PrefixRecords < 2 || p.PrefixRecords > 115 ||
		len(p.LastPhase) == 0 || len(p.LastPhase) > 64 || !filepath.IsAbs(program) ||
		program != filepath.Join(p.Arguments[2], "ardents-node-linux-amd64") {
		return nil, errors.New("closed canonical observer profile differs")
	}
	initial := p.Arguments[1] == "install-initial-stopped"
	if !initial && p.Arguments[1] != "replace-source" {
		return nil, errors.New("only original initial/replacement cut sourceCutObservation permitted")
	}
	paths, environment, network := []int{2, 3, 6, 7}, 4, 5
	if initial {
		paths, environment, network = []int{2, 4, 7}, 5, 6
		if !sourceCutObserverSHA(p.Arguments[3]) {
			return nil, errors.New("initial independent pin differs")
		}
	}
	if p.Arguments[environment] != "alpha" || !sourceCutObserverNetwork(p.Arguments[network]) {
		return nil, errors.New("closed-alpha environment/network identifiers required")
	}
	for _, i := range paths {
		argument := p.Arguments[i]
		if !filepath.IsAbs(argument) || filepath.Clean(argument) != argument || len(argument) > 4096 {
			return nil, errors.New("exact finite absolute command arguments required")
		}
	}
	if initial {
		if p.Journal != root+".source-initial-journal" {
			return nil, errors.New("initial original journal differs")
		}
	} else {
		prefix := root + ".source-replacement-"
		generation := strings.TrimSuffix(strings.TrimPrefix(p.Journal, prefix), "-journal")
		if !sourceCutObserverSHA(generation) || p.Journal != prefix+generation+"-journal" || p.PrefixRecords > 55 || p.Arguments[6] != filepath.Join(root, "binding.json") {
			return nil, errors.New("replacement original journal/binding differs")
		}
	}
	p.sha256 = pin
	return p, nil
}

func TestSourceCutObserverNative(t *testing.T) {
	mode := os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_MODE")
	if mode == "" {
		return
	} // Dedicated helper, selected by its outside caller.
	program := os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_PROGRAM")
	root := os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_ROOT")
	output := os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_OUTPUT")
	profilePath := os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_PROFILE")
	profilePin := os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_PROFILE_SHA256")
	var profile *sourceCutObserverProfile
	var err error
	nativeCase, inspectOnly, nativeRecords := "external-cut", false, 2
	switch mode {
	case "inspect-only", "installed-cut":
		profile, err = readSourceCutObserverProfile(profilePath, profilePin, program, root)
		if err != nil {
			t.Fatal(err)
		}
		inspectOnly = mode == "inspect-only"
		if os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_NATIVE_RECORDS") != "" {
			t.Fatal("native target cannot accompany installed profile")
		}
	case "native-complete", "native-external":
		if profilePath != "" || profilePin != "" {
			t.Fatal("installed profile cannot accompany native mechanism control")
		}
		if mode == "native-complete" {
			nativeCase = "complete"
		}
		if value := os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_NATIVE_RECORDS"); value != "" {
			nativeRecords, err = strconv.Atoi(value)
			if err != nil {
				t.Fatal(err)
			}
		}
	default:
		t.Fatal("unknown finite Source cut observer mode")
	}
	previous := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(previous)
	result, err := observeSourceCut(program, root, output, nativeCase, profile, inspectOnly, nativeRecords, nil)
	raw, encoded := json.Marshal(result)
	var reported error
	if encoded == nil {
		_, reported = fmt.Fprintln(os.Stdout, "SOURCE_CUT_OBSERVATION "+string(raw))
	}
	if err = errors.Join(err, encoded, reported); err != nil {
		t.Fatal(err)
	}
}
func sourceCutPtrace(request, pid int, address, data uintptr) error {
	_, _, e := syscall.Syscall6(syscall.SYS_PTRACE, uintptr(request), uintptr(pid), address, data, 0, 0)
	if e != 0 {
		return e
	}
	return nil
}

func armSourceCutBreakpoint(pid int, entry uint64) error {
	if err := sourceCutPtrace(syscall.PTRACE_POKEUSR, pid, sourceCutDebugRegisters, uintptr(entry)); err != nil {
		return err
	}
	return sourceCutPtrace(syscall.PTRACE_POKEUSR, pid, sourceCutDebugRegisters+7*8, 1)
}

func resumeSourceCutBreakpoint(pid int, registers *syscall.PtraceRegs) error {
	// RF suppresses the same hardware execution trap for one instruction.
	// Program bytes, instruction pointer and application data are unchanged.
	registers.Eflags |= 1 << 16
	if err := syscall.PtraceSetRegs(pid, registers); err != nil {
		return err
	}
	return syscall.PtraceCont(pid, 0)
}

func sourceCutThreadGroup(pid int) (int, error) {
	f, err := os.Open("/proc/" + strconv.Itoa(pid) + "/status")
	if err != nil {
		return 0, err
	}
	raw, read := io.ReadAll(io.LimitReader(f, 64<<10))
	if err := errors.Join(read, f.Close()); err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "Tgid:") {
			n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Tgid:")))
			if err != nil || n <= 0 {
				return 0, errors.Join(errors.New("invalid Tgid"), err)
			}
			return n, nil
		}
	}
	return 0, errors.New("native Tgid missing")
}

func observeSourceCut(program, root, output, nativeCase string, profile *sourceCutObserverProfile, inspectOnly bool, nativeRecords int, physical *sourcePhysicalCut) (result sourceCutObservation, returned error) {
	result.InspectionOnly, result.RootExit = inspectOnly, -1
	if nativeRecords < 2 || nativeRecords > 55 || (nativeRecords != 2 && (profile != nil || nativeCase != "external-cut")) {
		return result, errors.New("finite native-only target differs")
	}
	if physical != nil && (profile != nil || inspectOnly || nativeRecords != 2 || !physical.valid() ||
		nativeCase != "unsafe-external-cut" && !physical.root || nativeCase != "unsafe-root-cut" && physical.root) {
		return result, errors.New("closed native-only physical cut required")
	}
	if nativeCase != "complete" && nativeCase != "external-cut" && !((nativeCase == "unsafe-external-cut" || nativeCase == "unsafe-root-cut") && physical != nil) {
		return result, errors.New("unknown native file-only case")
	}
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" || os.Geteuid() != 0 ||
		!filepath.IsAbs(program) || (profile == nil && !strings.HasSuffix(program, ".test")) || !filepath.IsAbs(root) || (!inspectOnly && !filepath.IsAbs(output)) {
		return result, errors.New("native root amd64 file-only test profile required")
	}
	file, err := os.OpenFile(program, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return result, err
	}
	defer func() { returned = errors.Join(returned, file.Close()) }()
	info, err := file.Stat()
	if err != nil {
		return result, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 256<<20 {
		return result, errors.New("bounded native test executable required")
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, io.LimitReader(file, (256<<20)+1)); err != nil {
		return result, err
	}
	result.ProgramSHA256 = hex.EncodeToString(hash.Sum(nil))
	journal := filepath.Join(root, "original.source-initial-journal")
	var selectedInfo os.FileInfo
	selectedRecords, selectedHit := nativeRecords, nativeRecords-1
	acknowledgement := []byte("native-original-cancellation-observed\n")
	bound := 25 * time.Second
	if profile != nil {
		if result.ProgramSHA256 != profile.ProgramSHA256 {
			return result, errors.New("actual installed program pin differs")
		}
		result.ProfileSHA256, result.PublicReceiptSHA256 = profile.sha256, profile.PublicReceiptSHA256
		journal, selectedRecords, selectedHit = profile.Journal, profile.PrefixRecords, profile.PrefixRecords-1
		acknowledgement = []byte("{\"operation\":\"installation.source\",\"outcome\":\"completion-unavailable\"}\n")
		// The actual command retains its unchanged two-minute product deadline.
		bound = 150 * time.Second
	}
	image, err := elf.NewFile(file)
	if err != nil {
		return result, err
	}
	if image.Type != elf.ET_EXEC || image.Machine != elf.EM_X86_64 {
		return result, errors.New("fixed amd64 executable required")
	}
	symbols, err := image.Symbols()
	if err != nil {
		return result, err
	}
	selectedSymbol := sourceCutAcknowledgementSymbol
	if physical != nil {
		build, err := buildinfo.Read(file)
		if err != nil || build.GoVersion != "go1.27.3" {
			return result, errors.Join(errors.New("physical observer requires exact inspected Go ABI"), err)
		}
		selectedSymbol, result.PhysicalWindow = physical.symbol, physical.window
		selectedRecords = physical.records
	}
	for _, symbol := range symbols {
		if physical != nil && physical.root && symbol.Name == sourcePhysicalRootReturnSymbol && elf.ST_TYPE(symbol.Info) == elf.STT_FUNC && symbol.Size != 0 {
			if result.RootCloseEntry != 0 {
				return result, errors.New("duplicate native root return entry")
			}
			result.RootCloseEntry = symbol.Value
		}
		if symbol.Name == selectedSymbol && elf.ST_TYPE(symbol.Info) == elf.STT_FUNC && symbol.Size != 0 {
			if result.Entry != 0 {
				return result, errors.New("duplicate acknowledgement symbol")
			}
			result.Entry = symbol.Value
		}
	}
	text := image.Section(".text")
	if result.Entry == 0 || text == nil || result.Entry < text.Addr || result.Entry >= text.Addr+text.Size {
		return result, errors.New("actual acknowledgement entry missing from executable text")
	}
	if physical != nil && physical.root && (result.RootCloseEntry == 0 || result.RootCloseEntry == result.Entry || result.RootCloseEntry < text.Addr || result.RootCloseEntry >= text.Addr+text.Size) {
		return result, errors.New("actual native root return entry missing from executable text")
	}
	armEntries := func(tid int) error {
		if err := armSourceCutBreakpoint(tid, result.Entry); err != nil {
			return err
		}
		if physical != nil && physical.root {
			if err := sourceCutPtrace(syscall.PTRACE_POKEUSR, tid, sourceCutDebugRegisters+8, uintptr(result.RootCloseEntry)); err != nil {
				return err
			}
			return sourceCutPtrace(syscall.PTRACE_POKEUSR, tid, sourceCutDebugRegisters+7*8, 5)
		}
		return nil
	}
	if inspectOnly {
		return result, nil
	}
	if profile != nil {
		marker, err := os.ReadFile("/etc/ardents-source-startup-native-stand")
		container, containerErr := os.ReadFile("/run/systemd/container")
		cgroup, cgroupErr := os.ReadFile("/proc/self/cgroup")
		if err != nil || containerErr != nil || cgroupErr != nil || string(marker) != "ardents-source-startup-native-v1\n" || strings.TrimSpace(string(container)) != "ardents-source-startup" || string(cgroup) != "0::/../supervisor\n" {
			return result, errors.Join(errors.New("original private installed supervisor stand required"), err, containerErr, cgroupErr)
		}
		for _, name := range []string{"mnt", "pid", "cgroup", "uts"} {
			a, e := os.Stat("/proc/1/ns/" + name)
			b, be := os.Stat("/proc/self/ns/" + name)
			if e != nil || be != nil || !os.SameFile(a, b) {
				return result, errors.Join(errors.New("original manager namespace differs"), e, be)
			}
		}
	}
	log, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return result, err
	}
	defer func() { returned = errors.Join(returned, log.Sync(), log.Close()) }()
	reader, writer, err := os.Pipe()
	if err != nil {
		return result, err
	}
	defer func() { returned = errors.Join(returned, reader.Close()) }()
	writerClosed := false
	defer func() {
		if !writerClosed {
			returned = errors.Join(returned, writer.Close())
		}
	}()
	readFD := int(reader.Fd())
	if err := syscall.SetNonblock(readFD, true); err != nil {
		return result, err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pidfd := -1
	command := exec.Command(program, "-test.run=^TestSourceTransitionNativeDriver$", "-test.v")
	if physical != nil && physical.root {
		command = exec.Command(program, "-test.run=^TestSourcePhysicalRootNativeDriver$", "-test.v")
	}
	if profile != nil {
		command = exec.Command(program, profile.Arguments...)
	}
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "ARDENTS_SOURCE_TRANSITION_") && !strings.HasPrefix(value, "GOMAXPROCS=") && !strings.HasPrefix(value, "ARDENTS_DEBUG_SOCKET=") {
			command.Env = append(command.Env, value)
		}
	}
	if profile == nil {
		command.Env = append(command.Env, "ARDENTS_SOURCE_TRANSITION_NATIVE_ROLE=controller", "ARDENTS_SOURCE_TRANSITION_NATIVE_CASE="+nativeCase, "ARDENTS_SOURCE_TRANSITION_NATIVE_ROOT="+root, "ARDENTS_SOURCE_TRANSITION_NATIVE_RECORDS="+strconv.Itoa(nativeRecords))
	}
	command.Env = append(command.Env, "GOMAXPROCS=2")
	command.Stdout, command.Stderr = writer, writer
	command.SysProcAttr = &syscall.SysProcAttr{Ptrace: true, Pdeathsig: syscall.SIGKILL, PidFD: &pidfd}
	started := time.Now()
	if err := command.Start(); err != nil {
		return result, err
	}
	pid := command.Process.Pid
	result.InstalledExecution = profile != nil
	result.RootPID, result.RootExit = pid, -1
	tracked := map[int]bool{pid: false}
	groups := map[int]int{pid: pid}
	processes := map[int]bool{pid: true}
	physicalSelected := false
	var physicalImage *sourcePhysicalImage
	failed := false
	var expired atomic.Bool
	var deadlineErr error
	deadlineDone := make(chan struct{})
	deadline := time.AfterFunc(bound, func() { expired.Store(true); deadlineErr = command.Process.Kill(); close(deadlineDone) })
	defer func() {
		if !deadline.Stop() {
			<-deadlineDone
			returned = errors.Join(returned, deadlineErr)
		}
	}()
	var captured []byte
	outputEOF := false
	drain := func() error {
		var scratch [8192]byte
		for {
			n, err := syscall.Read(readFD, scratch[:])
			if n > 0 {
				if len(captured)+n > 1<<20 {
					return errors.New("native output exceeds finite bound")
				}
				captured = append(captured, scratch[:n]...)
				if written, e := log.Write(scratch[:n]); e != nil || written != n {
					return errors.Join(io.ErrShortWrite, e)
				}
			}
			if n == 0 && err == nil {
				outputEOF = true
				return nil
			}
			if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
				return nil
			}
			if err != nil && err != syscall.EINTR {
				return err
			}
		}
	}
	lastTraceTID, lastTraceStatus := 0, syscall.WaitStatus(0)
	fail := func(err error) {
		if err == nil {
			return
		}
		returned = errors.Join(returned, fmt.Errorf("native trace tid=%d status=%#x: %w", lastTraceTID, uint32(lastTraceStatus), err))
		if failed {
			return
		}
		failed = true
		returned = errors.Join(returned, command.Process.Kill())
		unique := map[int]bool{}
		for _, group := range groups {
			if !unique[group] {
				unique[group] = true
				returned = errors.Join(returned, syscall.Kill(group, syscall.SIGKILL))
			}
		}
	}
	// No return after native birth precedes this complete owned-tree drain.
	defer func() {
		if len(tracked) != 0 {
			fail(errors.New("trace ended with native borrowers"))
			for {
				var status syscall.WaitStatus
				tid, err := syscall.Wait4(-1, &status, sourceCutWaitAll, nil)
				if errors.Is(err, syscall.ECHILD) {
					break
				}
				if err == syscall.EINTR {
					continue
				}
				if err != nil {
					returned = errors.Join(returned, err)
					break
				}
				if status.Exited() || status.Signaled() {
					delete(tracked, tid)
					continue
				}
				if status.Stopped() {
					group, e := sourceCutThreadGroup(tid)
					if e == nil {
						e = syscall.Kill(group, syscall.SIGKILL)
					}
					returned = errors.Join(returned, e, syscall.PtraceCont(tid, int(syscall.SIGKILL)))
				}
			}
		}
		returned = errors.Join(returned, drain())
		if !outputEOF {
			returned = errors.Join(returned, errors.New("original output not EOF after native joins"))
		}
		waited := command.Wait()
		if !errors.Is(waited, syscall.ECHILD) {
			returned = errors.Join(returned, errors.New("tracer did not own original root reap"), waited)
		}
		returned = errors.Join(returned, command.Process.Release())
		if pidfd >= 0 {
			returned = errors.Join(returned, syscall.Close(pidfd))
		}
		if expired.Load() {
			returned = errors.Join(returned, errors.New("original finite mechanism deadline exhausted"))
		}
		result.Processes = len(processes)
		sum := sha256.Sum256(captured)
		result.OutputSHA256 = hex.EncodeToString(sum[:])
	}()
	writerClosed = true
	if err := writer.Close(); err != nil {
		return result, err
	}
	if pidfd < 0 {
		return result, errors.New("from-birth pidfd unavailable")
	}
	var status syscall.WaitStatus
	if got, err := syscall.Wait4(pid, &status, sourceCutWaitAll, nil); err != nil || got != pid || !status.Stopped() || status.StopSignal() != syscall.SIGTRAP {
		return result, errors.Join(errors.New("original exec stop missing"), err)
	}
	actual, err := os.Stat("/proc/" + strconv.Itoa(pid) + "/exe")
	if err != nil || !os.SameFile(info, actual) {
		return result, errors.Join(errors.New("original executable identity changed"), err)
	}
	// Detach directly into SIGSTOP before the first user instruction, then
	// adopt SEIZE semantics so real child group stops remain visible to root.
	if err := sourceCutPtrace(syscall.PTRACE_DETACH, pid, 0, uintptr(syscall.SIGSTOP)); err != nil {
		return result, err
	}
	if got, err := syscall.Wait4(pid, &status, syscall.WUNTRACED, nil); err != nil || got != pid || !status.Stopped() || status.StopSignal() != syscall.SIGSTOP {
		return result, errors.Join(errors.New("pre-instruction native group stop missing"), err)
	}
	options := syscall.PTRACE_O_TRACECLONE | syscall.PTRACE_O_TRACEFORK | syscall.PTRACE_O_TRACEVFORK | syscall.PTRACE_O_TRACEEXEC | syscall.PTRACE_O_TRACEEXIT | sourceCutExitKill
	if err := sourceCutPtrace(sourceCutSeize, pid, 0, uintptr(options)); err != nil {
		return result, err
	}
	if err := sourceCutPtrace(sourceCutInterrupt, pid, 0, 0); err != nil {
		return result, err
	}
	if got, err := syscall.Wait4(pid, &status, sourceCutWaitAll, nil); err != nil || got != pid || !status.Stopped() || int(uint32(status)>>16) != sourceCutEventStop {
		return result, errors.Join(errors.New("seized original stop missing"), err)
	}
	if err := armEntries(pid); err != nil {
		return result, err
	}
	if err := sourceCutPtrace(sourceCutListen, pid, 0, 0); err != nil {
		return result, err
	}
	if err := command.Process.Signal(syscall.SIGCONT); err != nil {
		return result, err
	}
	held := 0
	for len(tracked) != 0 {
		if err := drain(); err != nil {
			fail(err)
		}
		if held != 0 && !failed && bytes.Contains(captured, acknowledgement) {
			var registers syscall.PtraceRegs
			if e := syscall.PtraceGetRegs(held, &registers); e != nil {
				fail(e)
			} else if registers.Rip != result.Entry {
				fail(errors.New("held original acknowledgement entry moved"))
			} else if e := resumeSourceCutBreakpoint(held, &registers); e != nil {
				fail(e)
			} else {
				result.ExternalCancellationAcknowledged = true
				held = 0
			}
		}
		if expired.Load() && !failed {
			fail(errors.New("finite original deadline"))
		}
		var status syscall.WaitStatus
		tid, err := syscall.Wait4(-1, &status, sourceCutWaitAll|syscall.WNOHANG, nil)
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			return result, err
		}
		if tid == 0 {
			time.Sleep(time.Millisecond)
			continue
		}
		lastTraceTID, lastTraceStatus = tid, status
		if status.Exited() || status.Signaled() {
			if tid == result.HeldCoordinatorThread && status.Signaled() && status.Signal() == syscall.SIGKILL && result.SignalledRootThread != 0 {
				result.HeldCoordinatorKilled = true
			}
			delete(tracked, tid)
			delete(groups, tid)
			result.ThreadsJoined++
			if tid == pid && status.Exited() {
				result.RootExit = status.ExitStatus()
			}
			continue
		}
		if !status.Stopped() {
			fail(errors.New("unknown native wait state"))
			continue
		}
		if fresh, exists := tracked[tid]; !exists || fresh {
			group, e := sourceCutThreadGroup(tid)
			if e != nil {
				fail(e)
				_ = syscall.PtraceCont(tid, int(syscall.SIGKILL))
				continue
			}
			tracked[tid], groups[tid], processes[group] = false, group, true
			if len(processes) > 32 {
				fail(errors.New("observer original process bound exceeded"))
			}
			if failed {
				returned = errors.Join(returned, syscall.Kill(group, syscall.SIGKILL))
			}
			arm := group == pid
			if physical != nil {
				arm = false
				// A newly reported thread can already be at its final EXIT stop.
				// Its group leader may have exited; there is no application entry
				// left to arm. Continue to the real wait/reap below instead of
				// inspecting a vanished /proc/<leader>/exe during teardown.
				if int(uint32(status)>>16) != syscall.PTRACE_EVENT_EXIT {
					arm, e = sourcePhysicalSameProgram(group, info)
					if e != nil {
						fail(e)
					}
				}
			}
			if arm && !failed {
				if e := armEntries(tid); e != nil {
					fail(e)
				}
			}
			if e := syscall.PtraceCont(tid, 0); e != nil {
				fail(e)
			}
			continue
		}
		switch int(uint32(status) >> 16) {
		case syscall.PTRACE_EVENT_CLONE, syscall.PTRACE_EVENT_FORK, syscall.PTRACE_EVENT_VFORK:
			child, e := syscall.PtraceGetEventMsg(tid)
			if e != nil || child == 0 {
				fail(errors.Join(errors.New("native birth event invalid"), e))
			} else {
				tracked[int(child)] = true
				if len(tracked) > 256 {
					fail(errors.New("observer original native thread bound exceeded"))
				}
			}
			if e := syscall.PtraceCont(tid, 0); e != nil {
				fail(e)
			}
		case syscall.PTRACE_EVENT_EXEC:
			if physical != nil && !failed {
				same, e := sourcePhysicalSameProgram(groups[tid], info)
				if e != nil {
					fail(e)
				} else if same {
					if e := armEntries(tid); e != nil {
						fail(e)
					}
				} else if e := sourceCutPtrace(syscall.PTRACE_POKEUSR, tid, sourceCutDebugRegisters+7*8, 0); e != nil {
					fail(e)
				}
			}
			if e := syscall.PtraceCont(tid, 0); e != nil {
				fail(e)
			}
		case syscall.PTRACE_EVENT_EXIT:
			if e := syscall.PtraceCont(tid, 0); e != nil {
				fail(e)
			}
		case sourceCutEventStop:
			signal := status.StopSignal()
			if signal == syscall.SIGSTOP || signal == syscall.SIGTSTP || signal == syscall.SIGTTIN || signal == syscall.SIGTTOU {
				result.GroupStops++
				if e := sourceCutPtrace(sourceCutListen, tid, 0, 0); e != nil {
					fail(e)
				}
			} else if e := syscall.PtraceCont(tid, 0); e != nil {
				fail(e)
			}
		default:
			signal := status.StopSignal()
			if signal == syscall.SIGTRAP {
				var registers syscall.PtraceRegs
				if e := syscall.PtraceGetRegs(tid, &registers); e != nil {
					fail(e)
					continue
				}
				if physical != nil && physical.root && registers.Rip == result.RootCloseEntry {
					result.RootCloseObservations++
					if e := drain(); e != nil {
						fail(e)
					}
					if groups[tid] != pid || !physicalSelected || held != 0 || !result.ExternalCancellationAcknowledged || result.RootOwnershipReleasedBeforeExit || !bytes.Contains(captured, []byte("native-root-writer-closed\n")) {
						fail(errors.New("native root return preceded cancellation or original Close"))
					} else if e := physical.verifyRetained(journal, root, physicalImage); e != nil {
						fail(e)
					} else if e := verifySourcePhysicalRootFDs(pid, physicalImage); e != nil {
						fail(e)
					} else {
						actual, e := os.Stat("/proc/" + strconv.Itoa(pid) + "/exe")
						if e != nil || !os.SameFile(info, actual) {
							fail(errors.Join(errors.New("original root not alive at post-Close probe"), e))
						} else if e := sourceCutPtrace(syscall.PTRACE_POKEUSR, tid, sourceCutDebugRegisters+7*8, 1); e != nil {
							fail(e)
						} else {
							// DR1 is a one-use observation, disarmed only after the
							// real lease/FD proof while this caller is still alive.
							// Go's preemption prologue may restart the same entry
							// after RF resume; that is not another semantic call.
							// DR0 and every native exit/join check remain active.
							result.RootOwnershipReleasedBeforeExit = true
						}
					}
					// Optional native scheduler stress after the ownership proof.
					// No Source effect or original cut is selected by this delay.
					if os.Getenv("ARDENTS_SOURCE_ROOT_RETURN_STRESS") == "1" {
						time.Sleep(25 * time.Millisecond)
					}
					if e := resumeSourceCutBreakpoint(tid, &registers); e != nil {
						fail(e)
					}
					continue
				}
				if physical != nil && registers.Rip == result.Entry {
					if !failed && !physicalSelected && (physical.root && groups[tid] == pid || !physical.root && groups[tid] != pid) {
						matched, observed, e := physical.match(tid, &registers, journal, root)
						if e != nil {
							fail(e)
						} else if matched {
							if time.Since(started) >= 8*time.Second {
								fail(errors.New("physical selection did not precede original ten-second deadline"))
							} else {
								physicalSelected, physicalImage = true, observed
								result.SelectedRecords = physical.records
								result.SelectedJournalSHA256 = observed.journalSHA
								if physical.root {
									held, result.HeldRootThread = tid, tid
								} else {
									result.HeldCoordinatorThread = tid
								}
								for candidate, group := range groups {
									if group == pid && candidate != tid && (result.SignalledRootThread == 0 || candidate < result.SignalledRootThread) {
										result.SignalledRootThread = candidate
									}
								}
								if result.SignalledRootThread == 0 {
									fail(errors.New("independent original root signal thread missing"))
								} else if e := syscall.Tgkill(pid, result.SignalledRootThread, syscall.SIGTERM); e != nil {
									fail(e)
								} else {
									// The original controller must kill and join its held child.
									// No RF resume can finish the unrecorded effect.
									continue
								}
							}
						}
					}
					if e := resumeSourceCutBreakpoint(tid, &registers); e != nil {
						fail(e)
					}
					continue
				}
				if groups[tid] == pid && registers.Rip == result.Entry {
					result.AcknowledgementHits++
					if result.AcknowledgementHits == selectedHit {
						raw, info, e := readSourceCutObserverFile(journal, 4<<20)
						if e != nil {
							fail(e)
						} else {
							selectedInfo = info
							result.SelectedRecords = bytes.Count(raw, []byte{'\n'})
							sum := sha256.Sum256(raw)
							result.SelectedJournalSHA256 = hex.EncodeToString(sum[:])
							if profile != nil {
								lines := bytes.Split(bytes.TrimSuffix(raw, []byte{'\n'}), []byte{'\n'})
								var record struct {
									Sequence int    `json:"sequence"`
									Phase    string `json:"phase"`
								}
								if len(raw) > 4<<20 || len(lines) != selectedRecords || json.Unmarshal(lines[len(lines)-1], &record) != nil || record.Sequence != selectedRecords-1 || record.Phase != profile.LastPhase {
									fail(errors.New("exact selected original prefix/phase differs"))
								}
							}
						}
					}
					if nativeCase == "external-cut" && result.AcknowledgementHits >= selectedHit && !failed {
						if result.AcknowledgementHits != selectedHit || result.SelectedRecords != selectedRecords {
							fail(errors.New("external cut missed exact first frozen boundary"))
							continue
						}
						other := 0
						for candidate, group := range groups {
							if group == pid && candidate != tid && (other == 0 || candidate < other) {
								other = candidate
							}
						}
						if other == 0 {
							fail(errors.New("no independent original root signal thread"))
							continue
						}
						held, result.HeldRootThread, result.SignalledRootThread = tid, tid, other
						if e := syscall.Tgkill(pid, other, syscall.SIGTERM); e != nil {
							fail(e)
						}
						continue
					}
					if e := resumeSourceCutBreakpoint(tid, &registers); e != nil {
						fail(e)
					}
					continue
				}
			}
			if e := syscall.PtraceCont(tid, int(signal)); e != nil {
				fail(e)
			}
		}
	}
	if physical != nil {
		result.ExternalCancellationAcknowledged = bytes.Contains(captured, acknowledgement)
		joined := !physical.root && result.HeldCoordinatorKilled && len(processes) == 2
		if physical.root {
			joined = result.RootOwnershipReleasedBeforeExit && result.RootCloseObservations == 1 && result.HeldRootThread != 0 && held == 0 && len(processes) == 1
		}
		if failed || !physicalSelected || !joined || !result.ExternalCancellationAcknowledged || result.RootExit != 0 {
			return result, errors.Join(returned, fmt.Errorf("actual unsafe physical cut differs: %+v", result))
		}
		return result, physical.verifyRetained(journal, root, physicalImage)
	}
	wantedHits := 4
	if nativeCase == "external-cut" {
		wantedHits = selectedHit
	}
	wantedExit := 0
	if profile != nil {
		wantedExit = 2
	}
	if result.RootExit != wantedExit || result.AcknowledgementHits != wantedHits || result.SelectedRecords != selectedRecords || result.GroupStops < wantedHits || (profile == nil && len(processes) != 2) || (profile != nil && len(processes) < 2) {
		return result, fmt.Errorf("actual original-controller outcome differs: %+v", result)
	}
	if nativeCase == "external-cut" {
		if !result.ExternalCancellationAcknowledged || held != 0 {
			return result, errors.New("original cancellation acknowledgement did not precede held-thread release")
		}
		raw, after, err := readSourceCutObserverFile(journal, 4<<20)
		if err != nil {
			return result, err
		}
		sum := sha256.Sum256(raw)
		if selectedInfo == nil || !os.SameFile(selectedInfo, after) || selectedInfo.Mode() != after.Mode() || hex.EncodeToString(sum[:]) != result.SelectedJournalSHA256 {
			return result, errors.New("original interrupted prefix changed")
		}
		if _, err := os.Lstat(journal + ".original-cut-pending"); !errors.Is(err, os.ErrNotExist) {
			return result, errors.Join(errors.New("original cut proof remains guarded"), err)
		}
		if profile == nil {
			if _, err := os.Lstat(filepath.Join(root, "next-effect")); !errors.Is(err, os.ErrNotExist) {
				return result, errors.Join(errors.New("denied original performed next effect"), err)
			}
		}
		proof, _, err := readSourceCutObserverFile(journal+".original-cut-result", 64<<10)
		if err != nil {
			return result, err
		}
		sum = sha256.Sum256(proof)
		result.OriginalCutSHA256 = hex.EncodeToString(sum[:])
	}
	return result, nil
}
