//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// This private native fixture drives genuine Go GC preemption at the first
// original ACK. It grants no installed Source or accepting recovery authority.
type sourceCutNativePreemption struct {
	enabled bool
	started bool
	held    int
}

func (p *sourceCutNativePreemption) hold(pid, tid, hits int, groups map[int]int) (bool, error) {
	if !p.enabled || p.started || hits != 1 {
		return false, nil
	}
	other := 0
	for candidate, group := range groups {
		if group == pid && candidate != tid && (other == 0 || candidate < other) {
			other = candidate
		}
	}
	if other == 0 {
		return false, errors.New("native GC independent original root thread missing")
	}
	p.started, p.held = true, tid
	return true, syscall.Tgkill(pid, other, syscall.SIGUSR1)
}

func (p *sourceCutNativePreemption) poll(entry uint64, result *sourceCutObservation) error {
	if p.held == 0 {
		return nil
	}
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", p.held))
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(line, "SigPnd:") {
			continue
		}
		pending, err := strconv.ParseUint(strings.TrimSpace(strings.TrimPrefix(line, "SigPnd:")), 16, 64)
		if err != nil {
			return err
		}
		if pending&(1<<(uint(syscall.SIGURG)-1)) == 0 {
			return nil
		}
		var registers syscall.PtraceRegs
		if err := syscall.PtraceGetRegs(p.held, &registers); err != nil {
			return err
		}
		if registers.Rip != entry {
			return errors.New("native GC original ACK entry moved")
		}
		if err := resumeSourceCutBreakpoint(p.held, &registers); err != nil {
			return err
		}
		p.held = 0
		result.NativeACKPreemptionObserved = true
		return nil
	}
	return errors.New("native original thread pending signals absent")
}

func TestSourcePreemptedAcknowledgementNative(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-source-stopped-native-stand")
	if err != nil || string(marker) != "ardents-source-stopped-native-v1\n" || os.Geteuid() != 0 {
		t.Fatal("original native file-control stand required", err)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	command := exec.CommandContext(t.Context(), program, "-test.run=^TestSourceCutObserverNative$", "-test.v", "-test.timeout=30s")
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "ARDENTS_SOURCE_CUT_") && !strings.HasPrefix(value, "ARDENTS_SOURCE_TRANSITION_") && !strings.HasPrefix(value, "GOMEMLIMIT=") {
			command.Env = append(command.Env, value)
		}
	}
	command.Env = append(command.Env,
		"GOMEMLIMIT=off",
		"ARDENTS_SOURCE_CUT_OBSERVER_MODE=native-external",
		"ARDENTS_SOURCE_CUT_OBSERVER_NATIVE_RECORDS=3",
		"ARDENTS_SOURCE_CUT_OBSERVER_PROGRAM="+program,
		"ARDENTS_SOURCE_CUT_OBSERVER_ROOT="+root,
		"ARDENTS_SOURCE_CUT_OBSERVER_OUTPUT="+filepath.Join(root, "original.log"),
		"ARDENTS_SOURCE_CUT_NATIVE_PREEMPT_ACK=1")
	raw, runErr := command.CombinedOutput()
	t.Logf("actual preempted original ACK: %s", raw)
	if runErr != nil {
		t.Fatal("preempted original ACK did not preserve exact cut and join", runErr)
	}
	var reports []sourceCutObservation
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		if !bytes.HasPrefix(line, []byte("SOURCE_CUT_OBSERVATION ")) {
			continue
		}
		var report sourceCutObservation
		if err := json.Unmarshal(bytes.TrimPrefix(line, []byte("SOURCE_CUT_OBSERVATION ")), &report); err != nil {
			t.Fatal(err)
		}
		reports = append(reports, report)
	}
	if len(reports) != 1 {
		t.Fatal("exact original observation required", len(reports))
	}
	report := reports[0]
	if report.InstalledExecution || report.InspectionOnly || report.PublicReceiptSHA256 != "" || report.RootExit != 0 || report.RootSignal != 0 ||
		!report.NativeACKPreemptionObserved || report.AcknowledgementReplays < 1 || !report.ExternalCancellationAcknowledged ||
		report.HeldRootThread == 0 || report.SignalledRootThread == 0 || report.AcknowledgementHits != 2 || report.SelectedRecords != 3 ||
		report.Processes != 2 || report.ThreadsJoined < 2 || report.OriginalCutSHA256 == "" {
		t.Fatalf("original preempted ACK scope differs: %+v", report)
	}
	proof, err := os.ReadFile(filepath.Join(root, "original.source-initial-journal.original-cut-result"))
	if err != nil {
		t.Fatal("joined original proof absent", err)
	}
	pin := sha256.Sum256(proof)
	if report.OriginalCutSHA256 != hex.EncodeToString(pin[:]) {
		t.Fatal("observer did not retain the exact original proof")
	}
	if _, err := os.Lstat(filepath.Join(root, "next-effect")); !os.IsNotExist(err) {
		t.Fatal("unacknowledged next effect occurred", err)
	}
}
