//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const sourceRefusalReturnSymbol = "main.(*sourceCommandFailure).Error"
const sourceRefusalNativeReturnSymbol = "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure.sourceRefusalNativeReturn"
const sourceRefusalAdmissionSymbol = "main.runSourceInstallation"
const sourceRefusalNativeAdmissionSymbol = "github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure.TestSourceRefusalNativeDriver"

// This is an external observation declaration, not permission to run recovery.
// Installed use requires the completed exact cohort's direct D6 acceptance.
// The journal hash pins even a deliberately damaged original negative control;
// no parser here can upgrade its bytes into Source provenance.
type sourceRefusalObserverProfile struct {
	Schema               string                     `json:"schema"`
	ProgramSHA256        string                     `json:"program_sha256"`
	PublicReceiptSHA256  string                     `json:"public_receipt_sha256"`
	Arguments            []string                   `json:"arguments"`
	JournalSHA256        string                     `json:"journal_sha256"`
	JournalMode          string                     `json:"journal_mode"`
	ExternalJournalLease bool                       `json:"external_journal_lease"`
	ExternalReleaseLease bool                       `json:"external_release_lease"`
	RecoveryHold         *sourceRecoveryHoldProfile `json:"recovery_hold,omitempty"`
	MissingOriginalCut   bool                       `json:"missing_original_cut,omitempty"`
	StartupPending       *sourcePendingGuardProfile `json:"startup_pending,omitempty"`
	RetirementPending    *sourcePendingGuardProfile `json:"retirement_pending,omitempty"`
	EarlyInitialPrefix   int                        `json:"early_initial_prefix,omitempty"`
	sha256               string
}

func (p *sourceRefusalObserverProfile) validCancellationObservation(root string) bool {
	return p != nil && p.EarlyInitialPrefix == 0 && p.RecoveryHold != nil && len(p.Arguments) == 8 && p.Schema == "source-recovery-cancellation-observer-v1" &&
		p.JournalMode == "0600" && p.StartupPending == nil && p.RetirementPending == nil && !p.MissingOriginalCut && !p.ExternalJournalLease && !p.ExternalReleaseLease && p.RecoveryHold.BodySHA256 == p.ProgramSHA256 &&
		p.RecoveryHold.valid(root, p.Arguments[7])
}

func (p *sourceRefusalObserverProfile) validMissingCutObservation() bool {
	return p != nil && p.EarlyInitialPrefix == 0 && p.MissingOriginalCut && p.Schema == "source-recovery-missing-cut-observer-v1" && len(p.Arguments) == 8 &&
		p.JournalMode == "0600" && p.StartupPending == nil && p.RetirementPending == nil && p.RecoveryHold == nil && !p.ExternalJournalLease && !p.ExternalReleaseLease
}

func readSourceRefusalObserverProfile(path, pin, program, root string) (*sourceRefusalObserverProfile, error) {
	if !filepath.IsAbs(path) || !sourceCutObserverSHA(pin) || !sourcePath(root) {
		return nil, errors.New("exact refusal profile path/pin required")
	}
	raw, _, err := readSourceCutObserverFile(path, 64<<10)
	if err != nil || sourcePhysicalSHA(raw) != pin {
		return nil, errors.Join(errors.New("original refusal profile differs"), err)
	}
	p := new(sourceRefusalObserverProfile)
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(p); err != nil {
		return nil, err
	}
	canonical, err := json.Marshal(p)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) || decoder.Decode(new(any)) != io.EOF ||
		!sourceCutObserverSHA(p.ProgramSHA256) ||
		!sourceCutObserverSHA(p.PublicReceiptSHA256) || !sourceCutObserverSHA(p.JournalSHA256) || (p.JournalMode != "0600" && p.JournalMode != "0640") || len(p.Arguments) != 8 {
		return nil, errors.New("closed canonical recovery refusal profile required")
	}
	if p.Arguments[0] != "installation" || p.Arguments[1] != "recover-source" || p.Arguments[4] != "alpha" ||
		!sourceCutObserverNetwork(p.Arguments[5]) || program != filepath.Join(p.Arguments[2], ProgramName) {
		return nil, errors.New("exact canonical recovery argv required")
	}
	for _, index := range []int{2, 3, 6, 7} {
		if !sourcePath(p.Arguments[index]) {
			return nil, errors.New("canonical bounded recovery path required")
		}
	}
	journal := p.Arguments[7]
	prefix := root + ".source-replacement-"
	generation := strings.TrimSuffix(strings.TrimPrefix(journal, prefix), "-journal")
	if p.RetirementPending != nil {
		if !p.validRetirementPendingObservation(root) {
			return nil, errors.New("distinct exact retirement pending refusal purpose required")
		}
	} else if p.StartupPending != nil {
		if !p.validStartupPendingObservation(root) {
			return nil, errors.New("distinct exact startup pending refusal purpose required")
		}
	} else if journal != root+".source-initial-journal" && (journal != prefix+generation+"-journal" || !sourceCutObserverSHA(generation)) {
		return nil, errors.New("original initial/replacement journal required")
	}
	if p.EarlyInitialPrefix != 0 {
		if !p.validEarlyPrefixObservation(root) {
			return nil, errors.New("distinct fresh early-prefix custody purpose required")
		}
	} else if p.StartupPending != nil || p.RetirementPending != nil {
		// Its exact purpose/command was checked above; it borrows no other cause.
	} else if p.MissingOriginalCut {
		if !p.validMissingCutObservation() {
			return nil, errors.New("distinct missing-original-cut read purpose required")
		}
	} else if p.RecoveryHold == nil {
		if p.Schema != "source-recovery-refusal-observer-v1" {
			return nil, errors.New("distinct closed refusal purpose required")
		}
	} else if !p.validCancellationObservation(root) {
		return nil, errors.New("distinct closed original-born cancellation purpose required")
	}
	p.sha256 = pin
	return p, nil
}

type sourceRefusalRuntimeCPU struct {
	fd    int
	name  string
	info  os.FileInfo
	flags uint64
}

func readSourceRefusalFDFlags(pid int, fd string) (uint64, error) {
	f, err := os.Open(filepath.Join("/proc", strconv.Itoa(pid), "fdinfo", fd))
	if err != nil {
		return 0, err
	}
	raw, read := io.ReadAll(io.LimitReader(f, 4097))
	if err := errors.Join(read, f.Close()); err != nil || len(raw) > 4096 {
		return 0, errors.Join(errors.New("finite original descriptor flags required"), err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "flags:\t") {
			return strconv.ParseUint(strings.TrimPrefix(line, "flags:\t"), 8, 64)
		}
	}
	return 0, errors.New("original descriptor flags missing")
}

// Go1.27.3 src/runtime/cgroup_linux.go defaultGOMAXPROCSInit and
// src/internal/runtime/cgroup/cgroup_linux.go OpenCPU retain the read-only
// cpu.max descriptor for process lifetime, even with explicit GOMAXPROCS.
// Capture its actual descriptor/inode/flags before Source operation entry;
// a pathname wildcard at the later refusal would not prove original custody.
func captureSourceRefusalRuntimeCPU(pid int) (*sourceRefusalRuntimeCPU, error) {
	directory := filepath.Join("/proc", strconv.Itoa(pid), "fd")
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) > 128 {
		return nil, errors.Join(errors.New("finite original runtime descriptor inventory required"), err)
	}
	var original *sourceRefusalRuntimeCPU
	for _, entry := range entries {
		path := filepath.Join(directory, entry.Name())
		name, err := os.Readlink(path)
		if err != nil {
			return nil, err
		}
		if !strings.HasPrefix(name, "/sys/fs/cgroup/") || !strings.HasSuffix(name, "/cpu.max") {
			continue
		}
		fd, err := strconv.Atoi(entry.Name())
		if err != nil || fd < 3 || original != nil {
			return nil, errors.Join(errors.New("one original runtime CPU descriptor required"), err)
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil, errors.Join(errors.New("original runtime CPU file required"), err)
		}
		var filesystem syscall.Statfs_t
		if err := syscall.Statfs(path, &filesystem); err != nil || filesystem.Type != 0x63677270 {
			return nil, errors.Join(errors.New("original kernel cgroup-v2 CPU file required"), err)
		}
		flags, err := readSourceRefusalFDFlags(pid, entry.Name())
		if err != nil || flags&syscall.O_ACCMODE != syscall.O_RDONLY || flags&syscall.O_CLOEXEC == 0 {
			return nil, errors.Join(errors.New("original read-only runtime CPU descriptor required"), err)
		}
		original = &sourceRefusalRuntimeCPU{fd: fd, name: name, info: info, flags: flags}
	}
	return original, nil
}

func verifySourceRefusalFDs(pid int, pipe os.FileInfo, cpu *sourceRefusalRuntimeCPU) error {
	directory := filepath.Join("/proc", strconv.Itoa(pid), "fd")
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) < 3 || len(entries) > 128 {
		return errors.Join(errors.New("finite original refusal descriptor inventory required"), err)
	}
	var standard [3]bool
	epoll, eventfd, cpuObserved := 0, 0, cpu == nil
	for _, entry := range entries {
		fd, err := strconv.Atoi(entry.Name())
		if err == nil && fd >= 0 && fd < len(standard) {
			standard[fd] = true
		}
	}
	if standard != [3]bool{true, true, true} {
		return errors.New("original refusal standard descriptor missing")
	}
	for _, entry := range entries {
		fd, err := strconv.Atoi(entry.Name())
		if err != nil || fd < 0 {
			return errors.Join(errors.New("invalid refusal descriptor number"), err)
		}
		path := filepath.Join(directory, entry.Name())
		name, err := os.Readlink(path)
		if err != nil {
			return err // All original threads are stopped; disappearance is not success.
		}
		switch fd {
		case 0:
			if name != "/dev/null" {
				return errors.New("original refusal stdin differs")
			}
		case 1, 2:
			info, err := os.Stat(path)
			if err != nil || pipe == nil || !os.SameFile(pipe, info) || !strings.HasPrefix(name, "pipe:[") {
				return errors.Join(errors.New("original refusal output pipe differs"), err)
			}
		default:
			if cpu != nil && fd == cpu.fd {
				info, statErr := os.Stat(path)
				flags, flagsErr := readSourceRefusalFDFlags(pid, entry.Name())
				if statErr != nil || flagsErr != nil || name != cpu.name || !os.SameFile(cpu.info, info) || flags != cpu.flags {
					return errors.Join(errors.New("original runtime CPU descriptor changed"), statErr, flagsErr)
				}
				cpuObserved = true
			} else if name == "anon_inode:[eventpoll]" {
				epoll++
			} else if name == "anon_inode:[eventfd]" {
				eventfd++
			} else {
				return fmt.Errorf("product descriptor still retained before refusal return: fd%d (%s)", fd, name)
			}
		}
	}
	if !cpuObserved || epoll > 1 || eventfd > 1 {
		return errors.New("original runtime descriptor inventory differs")
	}
	return nil
}

func verifySourceRefusalStoppedThreads(pid int, tracked, stopped map[int]bool) error {
	entries, err := os.ReadDir(filepath.Join("/proc", strconv.Itoa(pid), "task"))
	if err != nil || len(entries) < 2 || len(entries) > 256 || len(entries) != len(tracked) || len(stopped) != len(tracked) {
		return errors.Join(errors.New("complete original stopped refusal thread inventory required"), err)
	}
	for _, entry := range entries {
		tid, err := strconv.Atoi(entry.Name())
		fresh, exists := tracked[tid]
		if err != nil || !exists || fresh || !stopped[tid] {
			return errors.Join(errors.New("original refusal thread was not observed stopped"), err)
		}
	}
	return nil
}

func verifySourceRefusalResources(pid int, root string, pipe os.FileInfo, cpu *sourceRefusalRuntimeCPU, profile *sourceRefusalObserverProfile, scenario string) error {
	if err := verifySourceRefusalFDs(pid, pipe, cpu); err != nil {
		return err
	}
	journal, lock, pin, external := filepath.Join(root, "original-journal"), filepath.Join(root, "release-lock"), "", scenario == "refusal-held" || scenario == "refusal-held-both" || scenario == "refusal-held-unattempted"
	mode, releaseExternal := os.FileMode(0600), scenario == "refusal-held-release" || scenario == "refusal-held-both"
	if profile != nil {
		journal, lock, pin, external = profile.Arguments[7], filepath.Join(profile.Arguments[3], ".ardents-release-decision-lock"), profile.JournalSHA256, profile.ExternalJournalLease
		if profile.JournalMode == "0640" {
			mode = 0640
		}
		releaseExternal = profile.ExternalReleaseLease
	}
	if err := probeSourceRefusalLease(journal, pin, mode, external); err != nil {
		return err
	}
	if err := probeSourceRefusalLease(lock, "", 0600, releaseExternal); err != nil {
		return err
	}
	if profile != nil {
		return verifySourceRefusalManagerUsers(pid)
	}
	return nil
}

func probeSourceRefusalLease(path, pin string, mode os.FileMode, external bool) (returned error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	defer func() { returned = errors.Join(returned, f.Close()) }()
	info, err := f.Stat()
	if err != nil || !sourceAccess(info, mode, 0, 0, false) || info.Size() > 4<<20 {
		return errors.Join(errors.New("original private refusal lease file required"), err)
	}
	var filesystem syscall.Statfs_t
	if err := syscall.Fstatfs(int(f.Fd()), &filesystem); err != nil || filesystem.Type != 0xef53 {
		return errors.Join(errors.New("original native refusal lease required"), err)
	}
	if pin != "" {
		raw, err := io.ReadAll(io.LimitReader(f, (4<<20)+1))
		if err != nil || int64(len(raw)) != info.Size() || sourcePhysicalSHA(raw) != pin {
			return errors.Join(errors.New("preserved refusal journal differs"), err)
		}
	}
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if external {
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return errors.Join(errors.New("original external journal lease no longer held"), err)
		}
	} else {
		if err != nil {
			return errors.Join(errors.New("original refusal lease still held before return"), err)
		}
		defer func() { returned = errors.Join(returned, syscall.Flock(int(f.Fd()), syscall.LOCK_UN)) }()
	}
	named, namedErr := os.Lstat(path)
	after, statErr := f.Stat()
	if namedErr != nil || statErr != nil || !sameSourceFile(info, named) || !sameSourceFile(info, after) {
		return errors.Join(errors.New("original refusal lease identity changed"), namedErr, statErr)
	}
	return nil
}

type sourceRefusalBusOutput struct{ bytes.Buffer }

func (w *sourceRefusalBusOutput) Write(p []byte) (int, error) {
	if w.Len()+len(p) > 64<<10 {
		return 0, errors.New("native manager peer inventory exceeds bound")
	}
	return w.Buffer.Write(p)
}

func verifySourceRefusalManagerUsers(pid int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// The original root is still alive with every thread stopped and no socket
	// descriptor. Observe actual broker retirement, never substitute process exit.
	for {
		var output sourceRefusalBusOutput
		command := exec.CommandContext(ctx, "busctl", "--system", "--no-pager", "--no-legend", "list")
		command.Stdout, command.Stderr = &output, &output
		if err := command.Run(); err != nil {
			return errors.Join(errors.New("actual native manager peer inventory unavailable"), err)
		}
		retained, manager := false, false
		for _, line := range strings.Split(output.String(), "\n") {
			fields := strings.Fields(line)
			if len(fields) > 1 && fields[0] == "org.freedesktop.systemd1" && fields[1] == "1" {
				manager = true
			}
			if len(fields) > 1 && fields[1] == strconv.Itoa(pid) {
				retained = true
			}
		}
		if !manager {
			return errors.New("original private manager peer absent from actual broker inventory")
		}
		if !retained {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.New("original refusal manager user retained before return")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// The outer native control uses actual files/flocks/processes only. None of
// these fixture bytes represent Release history or successful Source authority.
func sourceRefusalNativeReturn() error {
	f, err := os.Open("/proc/self/exe")
	if err != nil {
		return err
	}
	_, observed := f.Stat()
	return errors.Join(observed, f.Close())
}

func TestSourceRefusalNativeDriver(t *testing.T) {
	scenario := os.Getenv("ARDENTS_SOURCE_TRANSITION_NATIVE_CASE")
	if !strings.HasPrefix(scenario, "refusal-") {
		return
	}
	root := os.Getenv("ARDENTS_SOURCE_TRANSITION_NATIVE_ROOT")
	if !sourcePath(root) || os.Geteuid() != 0 || os.Getenv("GOGC") != "off" || os.Getenv("GOMEMLIMIT") != "off" {
		t.Fatal("original no-GC native refusal driver required")
	}
	held := scenario == "refusal-held" || scenario == "refusal-held-release" || scenario == "refusal-held-both" || scenario == "refusal-held-unattempted"
	missingCut := sourceMissingCutNativeCase(scenario)
	startupPending := sourceStartupPendingNativeCase(scenario) || sourceRetirementPendingNativeCase(scenario)
	earlyKind, _ := sourceEarlyPrefixNativeCase(scenario)
	if !held && !missingCut && !startupPending && earlyKind == "" {
		if err := os.Mkdir(root, 0700); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"original-journal", "release-lock"} {
			if err := os.WriteFile(filepath.Join(root, name), []byte("native file fixture only\n"), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if earlyKind != "" {
		runSourceEarlyPrefixNative(t, root, earlyKind)
		if err := sourceRefusalNativeReturn(); err != nil {
			t.Fatal(err)
		}
		return
	}
	if startupPending {
		if !strings.HasSuffix(scenario, "-unattempted") {
			path := filepath.Join(root, "source-start-pending")
			if sourceRetirementPendingNativeCase(scenario) {
				path = filepath.Join(root, "source-retirement-pending")
			}
			if scenario == "refusal-retirement-pending-startup" {
				path = filepath.Join(root, "source-start-pending")
			}
			if strings.HasSuffix(scenario, "-foreign") {
				path = filepath.Join(root, "foreign-pending")
			}
			var err error
			if strings.HasSuffix(scenario, "-follow") {
				_, err = os.Stat(path)
			} else {
				_, err = os.Lstat(path)
			}
			if err != nil {
				t.Fatal("actual startup pending native lookup required", err)
			}
		}
		if err := sourceRefusalNativeReturn(); err != nil {
			t.Fatal(err)
		}
		return
	}
	if missingCut {
		journal := filepath.Join(root, "original-journal")
		if scenario == "refusal-missing-cut" {
			prefix, err := openSourceRecoveryPrefix(context.Background(), journal, "initial")
			if err != nil {
				t.Fatal(err)
			}
			missing := prefix.readOriginalCut(strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64))
			closed := prefix.close()
			var pathError *os.PathError
			if !errors.Is(missing, syscall.ENOENT) || !errors.As(missing, &pathError) || pathError.Path != journal+".original-cut-journal" || !sourcePhysicalRefusalClosed(missing, closed) {
				t.Fatal("actual original-cut read/ENOENT and joined refusal required", missing, closed)
			}
		} else if scenario == "refusal-missing-cut-path-only" {
			// Linux amd64 O_PATH is a pathname-only lookup, not a read open.
			const openPath = 0x200000
			fd, err := syscall.Open(journal+".original-cut-journal", openPath|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
			if fd >= 0 {
				err = errors.Join(err, syscall.Close(fd))
			}
			if !errors.Is(err, syscall.ENOENT) {
				t.Fatal("actual pathname-only missing-file negative required", err)
			}
		} else if scenario == "refusal-missing-cut-foreign" {
			// A real ENOENT at a different name cannot prove this owner failed.
			f, err := os.OpenFile(journal+".foreign-missing", os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
			if f != nil {
				err = errors.Join(err, f.Close())
			}
			if !errors.Is(err, syscall.ENOENT) {
				t.Fatal("actual unrelated missing-file negative required", err)
			}
		}
		// The unattempted negative performs no original-cut read at all.
		if err := sourceRefusalNativeReturn(); err != nil {
			t.Fatal(err)
		}
		return
	}
	lease := "original-journal"
	if scenario == "refusal-held-release" {
		lease = "release-lock"
	}
	f, err := os.OpenFile(filepath.Join(root, lease), os.O_RDONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if scenario == "refusal-held-unattempted" {
		// The outside lease is real, but this caller never attempts flock.
		// Merely returning with clean descriptors must not prove exclusion.
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		if err := sourceRefusalNativeReturn(); err != nil {
			t.Fatal(err)
		}
		return
	}
	locked := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if held {
		if !errors.Is(locked, syscall.EWOULDBLOCK) {
			t.Fatal("external lease did not refuse actual driver", locked)
		}
	} else if locked != nil {
		t.Fatal(locked)
	}
	if scenario != "refusal-leaked" {
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if scenario == "refusal-held-both" {
		release, err := os.Open(filepath.Join(root, "release-lock"))
		if err != nil {
			t.Fatal(err)
		}
		locked := syscall.Flock(int(release.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		closed := release.Close()
		if !errors.Is(locked, syscall.EWOULDBLOCK) || closed != nil {
			t.Fatal("original recovery must retain both actual leases", locked, closed)
		}
	}
	if scenario == "refusal-cpu-duplicate" {
		// A second product-owned handle to the same kernel CPU file must not
		// inherit the runtime's original descriptor exception.
		f, err = os.Open("/sys/fs/cgroup/cpu.max")
		if err != nil {
			t.Fatal(err)
		}
	}
	if scenario == "refusal-missing-stdout" {
		if err := syscall.Close(1); err != nil {
			t.Fatal(err)
		}
	}
	if err := sourceRefusalNativeReturn(); err != nil {
		t.Fatal(err)
	}
	runtime.KeepAlive(f)
	if scenario == "refusal-leaked" || scenario == "refusal-cpu-duplicate" {
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSourceRefusalLeaseProbeNative(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-source-stopped-native-stand")
	if os.Geteuid() != 0 || err != nil || string(marker) != "ardents-source-stopped-native-v1\n" {
		t.Fatal("original native file stand required", err)
	}
	path := filepath.Join(t.TempDir(), "original")
	raw := []byte("damaged native fixture only\n")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	pin := sourcePhysicalSHA(raw)
	if err := probeSourceRefusalLease(path, pin, 0600, false); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0640); err != nil {
		t.Fatal(err)
	}
	if err := probeSourceRefusalLease(path, pin, 0600, false); err == nil {
		t.Fatal("changed journal access was not independently observed")
	}
	if err := probeSourceRefusalLease(path, pin, 0640, false); err != nil {
		t.Fatal("damaged access fixture cannot be observed without repair", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	if err := probeSourceRefusalLease(path, pin, 0640, false); err == nil {
		t.Fatal("actual foreign lease accepted as released")
	}
	if err := probeSourceRefusalLease(path, pin, 0640, true); err != nil {
		t.Fatal("explicit original external lease lost", err)
	}
}

func TestSourceRefusalObserverNative(t *testing.T) {
	mode := os.Getenv("ARDENTS_SOURCE_REFUSAL_OBSERVER_MODE")
	if mode == "" {
		return // Dedicated outside caller selects this helper.
	}
	program, root, output := os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_PROGRAM"), os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_ROOT"), os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_OUTPUT")
	var profile *sourceRefusalObserverProfile
	inspect := mode == "inspect-only" || mode == "inspect-cancellation" || mode == "inspect-missing-cut" || mode == "inspect-startup-pending" || mode == "inspect-retirement-pending" || mode == "inspect-early-prefix"
	if inspect || mode == "installed-refusal" || mode == "installed-cancellation" || mode == "installed-missing-cut" || mode == "installed-startup-pending" || mode == "installed-retirement-pending" || mode == "installed-early-prefix" {
		var err error
		profile, err = readSourceRefusalObserverProfile(os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_PROFILE"), os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_PROFILE_SHA256"), program, root)
		if err != nil {
			t.Fatal(err)
		}
		if (profile.RecoveryHold != nil) != (mode == "inspect-cancellation" || mode == "installed-cancellation") {
			t.Fatal("observer mode differs from exact profile purpose")
		}
		if profile.MissingOriginalCut != (mode == "inspect-missing-cut" || mode == "installed-missing-cut") {
			t.Fatal("observer mode differs from missing-original-cut purpose")
		}
		if (profile.StartupPending != nil) != (mode == "inspect-startup-pending" || mode == "installed-startup-pending") {
			t.Fatal("observer mode differs from exact startup pending purpose")
		}
		if (profile.RetirementPending != nil) != (mode == "inspect-retirement-pending" || mode == "installed-retirement-pending") {
			t.Fatal("observer mode differs from exact retirement pending purpose")
		}
		if (profile.EarlyInitialPrefix != 0) != (mode == "inspect-early-prefix" || mode == "installed-early-prefix") {
			t.Fatal("observer mode differs from exact early-prefix purpose")
		}
	} else if mode != "native-mechanism" && mode != "native-recovery-hold" && mode != "native-held-both" && mode != "native-early-notice" {
		t.Fatal("unknown closed refusal observer mode")
	}
	if profile == nil && (os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_PROFILE") != "" || os.Getenv("ARDENTS_SOURCE_CUT_OBSERVER_PROFILE_SHA256") != "") {
		t.Fatal("installed profile cannot accompany native refusal mechanism")
	}
	if mode == "native-recovery-hold" || mode == "native-held-both" || mode == "native-early-notice" {
		scenario := "refusal-held-both"
		if mode == "native-recovery-hold" {
			scenario = "refusal-recovery-cancel"
		} else if mode == "native-early-notice" {
			scenario = "refusal-recovery-early-notice"
		}
		result, err := observeSourceProcess(program, root, output, scenario, nil, false, 2, nil, nil)
		raw, encoded := json.Marshal(result)
		fmt.Printf("SOURCE_REFUSAL_OBSERVATION %s\n", raw)
		if err != nil || encoded != nil {
			t.Fatal(err, encoded)
		}
		return
	}
	if profile != nil {
		result, err := observeSourceProcess(program, root, output, "refusal-closed", nil, inspect, 2, nil, profile)
		raw, encoded := json.Marshal(result)
		fmt.Printf("SOURCE_REFUSAL_OBSERVATION %s\n", raw)
		if err != nil || encoded != nil {
			t.Fatal(err, encoded)
		}
		return
	}
	scenarios := []string{"refusal-closed", "refusal-held", "refusal-held-release", "refusal-leaked", "refusal-missing-stdout", "refusal-cpu-duplicate", "refusal-held-unattempted", "refusal-missing-cut", "refusal-missing-cut-unattempted", "refusal-missing-cut-foreign", "refusal-missing-cut-path-only", "refusal-startup-pending", "refusal-startup-pending-unattempted", "refusal-startup-pending-foreign", "refusal-startup-pending-follow", "refusal-retirement-pending", "refusal-retirement-pending-unattempted", "refusal-retirement-pending-foreign", "refusal-retirement-pending-follow", "refusal-retirement-pending-startup"}
	for _, count := range []int{24, 42} {
		for _, kind := range []string{"read", "unattempted", "unlocked", "partial", "foreign", "released", "shared"} {
			scenarios = append(scenarios, "refusal-early-"+kind+"-"+strconv.Itoa(count))
		}
	}
	for _, scenario := range scenarios {
		t.Run(scenario, func(t *testing.T) {
			caseRoot := root + "-" + scenario
			var external *os.File
			journalPin := sourcePhysicalSHA([]byte("native file fixture only\n"))
			earlyKind, earlyCount := sourceEarlyPrefixNativeCase(scenario)
			if earlyCount != 0 {
				journalPin = prepareSourceEarlyPrefixNative(t, caseRoot, earlyCount)
			}
			if sourceStartupPendingNativeCase(scenario) || sourceRetirementPendingNativeCase(scenario) {
				if err := os.Mkdir(caseRoot, 0700); err != nil {
					t.Fatal(err)
				}
				for _, name := range []string{"original-journal", "release-lock", "source-start-pending", "source-retirement-pending", "foreign-pending"} {
					if err := os.WriteFile(filepath.Join(caseRoot, name), []byte("native file fixture only\n"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			if sourceMissingCutNativeCase(scenario) {
				if err := os.Mkdir(caseRoot, 0700); err != nil {
					t.Fatal(err)
				}
				births, err := openSourceBirths(context.Background(), filepath.Join(caseRoot, "original-journal"), map[string]bool{})
				if err != nil {
					t.Fatal(err)
				}
				if err := births.close(); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(caseRoot, "release-lock"), []byte("native file fixture only\n"), 0600); err != nil {
					t.Fatal(err)
				}
				raw, err := os.ReadFile(filepath.Join(caseRoot, "original-journal"))
				if err != nil {
					t.Fatal(err)
				}
				journalPin = sourcePhysicalSHA(raw)
			}
			if scenario == "refusal-held" || scenario == "refusal-held-release" || scenario == "refusal-held-unattempted" {
				if err := os.Mkdir(caseRoot, 0700); err != nil {
					t.Fatal(err)
				}
				for _, name := range []string{"original-journal", "release-lock"} {
					if err := os.WriteFile(filepath.Join(caseRoot, name), []byte("native file fixture only\n"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				var err error
				lease := "original-journal"
				if scenario == "refusal-held-release" {
					lease = "release-lock"
				}
				external, err = os.Open(filepath.Join(caseRoot, lease))
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := external.Close(); err != nil {
						t.Error(err)
					}
				}()
				if err := syscall.Flock(int(external.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
					t.Fatal(err)
				}
			}
			result, err := observeSourceProcess(program, caseRoot, output+"-"+scenario, scenario, nil, false, 2, nil, nil)
			raw, encoded := json.Marshal(result)
			fmt.Printf("SOURCE_REFUSAL_NATIVE_OBSERVATION %s %s\n", scenario, raw)
			if encoded != nil {
				t.Fatal(encoded)
			}
			if earlyKind != "" && earlyKind != "read" {
				if err == nil || !strings.Contains(err.Error(), "original fresh-prefix lease/full read/close not observed before return") || result.RefusalEarlyPrefixObserved || result.RefusalEarlyPrefixClosed || result.RefusalOwnershipReleasedBeforeExit || result.ThreadsJoined < 2 {
					t.Fatal("unattempted, unlocked, partial or foreign read escaped original early-prefix oracle", result, err)
				}
				return
			}
			if sourceStartupPendingNativeCase(scenario) && scenario != "refusal-startup-pending" {
				if err == nil || !strings.Contains(err.Error(), "original pending guard Lstat success not observed before return") || result.RefusalStartupPendingObserved || result.RefusalOwnershipReleasedBeforeExit || result.ThreadsJoined < 2 {
					t.Fatal("unattempted, foreign or following stat escaped original nofollow guard oracle", result, err)
				}
				return
			}
			if sourceRetirementPendingNativeCase(scenario) && scenario != "refusal-retirement-pending" {
				if err == nil || !strings.Contains(err.Error(), "original pending guard Lstat success not observed before return") || result.RefusalRetirementPendingObserved || result.RefusalStartupPendingObserved || result.RefusalOwnershipReleasedBeforeExit || result.ThreadsJoined < 2 {
					t.Fatal("unattempted, foreign, following or startup stat escaped original retirement oracle", result, err)
				}
				return
			}
			if sourceMissingCutNativeCase(scenario) && scenario != "refusal-missing-cut" {
				if err == nil || !strings.Contains(err.Error(), "original-cut read ENOENT not observed before return") || result.RefusalMissingCutObserved || result.RefusalOwnershipReleasedBeforeExit || result.ThreadsJoined < 2 {
					t.Fatal("absent target or unrelated ENOENT escaped exact causal read oracle", result, err)
				}
				return
			}
			if scenario == "refusal-held-unattempted" {
				if err == nil || !strings.Contains(err.Error(), "original exclusive flock refusal not observed before return") || result.RefusalExclusionObserved || result.RefusalOwnershipReleasedBeforeExit || result.ThreadsJoined < 2 {
					t.Fatal("real external lease without actual caller attempt escaped causal oracle", result, err)
				}
				return
			}
			if scenario == "refusal-leaked" {
				if err == nil || !strings.Contains(err.Error(), "product descriptor still retained before refusal return") || !strings.Contains(err.Error(), filepath.Join(caseRoot, "original-journal")) || result.RefusalOwnershipReleasedBeforeExit {
					t.Fatal("actual retained descriptor escaped before-return oracle", result, err)
				}
				return
			}
			if scenario == "refusal-missing-stdout" {
				if err == nil || !strings.Contains(err.Error(), "original refusal standard descriptor missing") || result.RefusalOwnershipReleasedBeforeExit {
					t.Fatal("missing actual stdout escaped before-return oracle", result, err)
				}
				return
			}
			if scenario == "refusal-cpu-duplicate" {
				if err == nil || !strings.Contains(err.Error(), "product descriptor still retained before refusal return") || !strings.Contains(err.Error(), "/sys/fs/cgroup/cpu.max") || result.RefusalOwnershipReleasedBeforeExit {
					t.Fatal("extra CPU handle escaped original-runtime-only exception", result, err)
				}
				return
			}
			if err != nil || !result.RefusalOwnershipReleasedBeforeExit || result.RefusalStoppedThreads < 2 || result.InstalledExecution || result.RootExit != 0 {
				t.Fatal("actual native refusal lifetime", result, err)
			}
			if earlyKind == "read" && (!result.RefusalEarlyPrefixObserved || !result.RefusalEarlyPrefixClosed || result.RefusalEarlyPrefixRecords != earlyCount || result.RefusalEarlyPrefixDevice == 0 || result.RefusalEarlyPrefixInode == 0 || result.RefusalEarlyPrefixSHA256 != journalPin || result.RefusalExclusionObserved || result.RefusalMissingCutObserved) {
				t.Fatal("actual fresh journal lease, full read and close missing", result)
			}
			if scenario == "refusal-held" || scenario == "refusal-held-release" {
				kind := "journal"
				if scenario == "refusal-held-release" {
					kind = "release"
				}
				if !result.RefusalExclusionObserved || result.RefusalExclusionKind != kind || result.RefusalExclusionDevice == 0 || result.RefusalExclusionInode == 0 {
					t.Fatal("actual original kernel flock exclusion missing", result)
				}
			}
			if scenario == "refusal-missing-cut" && (!result.RefusalMissingCutObserved || result.RefusalMissingCutParentDevice == 0 || result.RefusalMissingCutParentInode == 0) {
				t.Fatal("actual paired original control-file read ENOENT missing", result)
			}
			if scenario == "refusal-startup-pending" && (!result.RefusalStartupPendingObserved || result.RefusalStartupPendingDevice == 0 || result.RefusalStartupPendingInode == 0 || result.RefusalStartupPendingSHA256 != journalPin) {
				t.Fatal("actual paired original guard nofollow stat missing", result)
			}
			if scenario == "refusal-retirement-pending" && (!result.RefusalRetirementPendingObserved || result.RefusalRetirementPendingDevice == 0 || result.RefusalRetirementPendingInode == 0 || result.RefusalRetirementPendingSHA256 != journalPin || result.RefusalStartupPendingObserved) {
				t.Fatal("actual paired retirement guard nofollow stat missing", result)
			}
			if err := probeSourceRefusalLease(filepath.Join(caseRoot, "original-journal"), journalPin, 0600, scenario == "refusal-held"); err != nil {
				t.Fatal("post-exit independent lease observation", err)
			}
		})
	}
}
