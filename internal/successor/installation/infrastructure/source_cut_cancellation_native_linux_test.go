//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The real native controller retains its original boundary through the same
// five-second cancellation notice as the canonical command. Its test journal
// grants no installed Source, manager, Release or accepting recovery authority.
func TestSourceDelayedCutCancellationNative(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-source-stopped-native-stand")
	if err != nil || string(marker) != "ardents-source-stopped-native-v1\n" || os.Geteuid() != 0 {
		t.Fatal("original native file-control stand required", err)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	output := filepath.Join(root, "original.log")
	command := exec.CommandContext(t.Context(), program, "-test.run=^TestSourceCutObserverNative$", "-test.v", "-test.timeout=30s")
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "ARDENTS_SOURCE_CUT_OBSERVER_") && !strings.HasPrefix(value, "ARDENTS_SOURCE_TRANSITION_") {
			command.Env = append(command.Env, value)
		}
	}
	command.Env = append(command.Env,
		"ARDENTS_SOURCE_CUT_OBSERVER_MODE=native-external",
		"ARDENTS_SOURCE_CUT_OBSERVER_PROGRAM="+program,
		"ARDENTS_SOURCE_CUT_OBSERVER_ROOT="+root,
		"ARDENTS_SOURCE_CUT_OBSERVER_OUTPUT="+output,
		"ARDENTS_SOURCE_CUT_NATIVE_DELAYED_NOTICE=1")
	raw, runErr := command.CombinedOutput()
	t.Logf("actual delayed original cut: %s", raw)
	if runErr != nil {
		t.Fatal("delayed original cancellation did not retain its genuine cut and join", runErr)
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
		!report.ExternalCancellationAcknowledged || report.HeldRootThread == 0 || report.SignalledRootThread == 0 ||
		report.AcknowledgementHits != 1 || report.SelectedRecords != 2 || report.Processes != 2 || report.ThreadsJoined < 2 || report.OriginalCutSHA256 == "" {
		t.Fatalf("original delayed cut scope differs: %+v", report)
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
