//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/pprof"
	"runtime/trace"
	"strings"
	"syscall"
	"testing"
	"time"

	processdiag "github.com/dianabuilds/ardents-network/internal/diagnostics/process"
)

func TestEvidencePackagePreservesCommandFailureAndMissingOwner(t *testing.T) {
	source := reportFixture(t, 7)
	if err := os.Chmod(source, 0700); err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(parent, "evidence.json")
	if err := dispatch([]string{"bundle", "-dir", source, "-out", out}); err != nil {
		t.Fatalf("assemble complete capture of failed command: %v", err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var packet struct {
		Schema   string `json:"schema"`
		Assembly string `json:"assembly"`
		Run      struct {
			Status   string          `json:"status"`
			Exit     int             `json:"exit_code"`
			Complete bool            `json:"complete"`
			Failure  json.RawMessage `json:"first_observed_failure"`
			Gaps     []string        `json:"gaps"`
		} `json:"run"`
		Artifacts []struct {
			Name   string `json:"name"`
			SHA256 string `json:"sha256"`
			Bytes  int64  `json:"bytes"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(body, &packet); err != nil {
		t.Fatal(err)
	}
	if packet.Schema != "ardents-evidence-package-v1" || packet.Assembly != "complete" || packet.Run.Status != "command-failed" || packet.Run.Exit != 7 || !packet.Run.Complete {
		t.Fatalf("command outcome or assembly/capture distinction lost: %s", body)
	}
	if len(packet.Run.Failure) != 0 && string(packet.Run.Failure) != "null" {
		t.Fatal("invented first owner failure")
	}
	if !strings.Contains(strings.Join(packet.Run.Gaps, " "), "first-owner-failure-not-observed") {
		t.Fatal("missing owner gap lost")
	}
	if len(packet.Artifacts) != 4 {
		t.Fatalf("expected four fixed report inputs, got%d", len(packet.Artifacts))
	}
	for _, a := range packet.Artifacts {
		if len(a.SHA256) != 64 || a.Bytes < 0 {
			t.Fatalf("unverified artifact: %+v", a)
		}
	}
	info, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatal("package not private")
	}
	if err := dispatch([]string{"bundle", "-dir", source, "-out", out}); err == nil {
		t.Fatal("overwrote existing evidence package")
	}
	after, err := os.ReadFile(out)
	if err != nil || string(after) != string(body) {
		t.Fatal("original package changed on output refusal")
	}
}

func TestEvidencePackageRefusesOversizedCaptureFile(t *testing.T) {
	source := reportFixture(t, 0)
	if err := os.Chmod(source, 0700); err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(source, "events.ndjson")
	if err := os.Truncate(name, (4<<20)+1); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(parent, "evidence.json")
	if err := dispatch([]string{"bundle", "-dir", source, "-out", out}); err == nil {
		t.Fatal("accepted event evidence beyond collector's 4 MiB budget")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("created an apparently complete package from invalid input")
	}
	info, err := os.Stat(name)
	if err != nil || info.Size() != (4<<20)+1 {
		t.Fatal("modified original oversized evidence")
	}
}

func TestEvidencePackageInventoriesToolVersionsUsedByReport(t *testing.T) {
	source := reportFixture(t, 0)
	if err := os.Chmod(source, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "tools.txt"), []byte("staticcheck 2025.1.1 (0.6.1)\ntool-private-secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(parent, "evidence.json")
	if err := dispatch([]string{"bundle", "-dir", source, "-out", out}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var packet struct {
		Run struct {
			Conditions struct {
				Tools map[string]string `json:"tools"`
			} `json:"conditions"`
		} `json:"run"`
		Artifacts []struct {
			Name  string `json:"name"`
			Bytes int64  `json:"bytes"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(body, &packet); err != nil {
		t.Fatal(err)
	}
	if packet.Run.Conditions.Tools["staticcheck"] != "2025.1.1" {
		t.Fatal("validated tool identity not retained")
	}
	found := false
	for _, a := range packet.Artifacts {
		if a.Name == "tools.txt" && a.Bytes == 49 {
			found = true
		}
	}
	if !found {
		t.Fatal("report consumed tool versions without an inventoried input")
	}
	if strings.Contains(string(body), "tool-private-secret") {
		t.Fatal("raw tool output copied into package")
	}
}

func TestEvidencePackageIncludesSelectedReaderObservationWithoutInventingAssociation(t *testing.T) {
	source := reportFixture(t, 7)
	if err := os.Chmod(source, 0700); err != nil {
		t.Fatal(err)
	}
	private := t.TempDir()
	if err := os.Chmod(private, 0700); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(private, "evidence.json")
	socket := filepath.Join(private, "process.sock")
	err := processdiag.Run(t.Context(), socket, func(ctx context.Context) error {
		capture := processdiag.ReaderTrace(ctx)
		operation := capture.Bind(ctx)
		failure := errors.New("private-reader-message")
		capture.Observe(processdiag.Admission, operation)(failure)
		capture.Observe(processdiag.CallerJoin, operation)(nil)
		capture.Finish(failure)
		return dispatch([]string{"bundle", "-dir", source, "-out", out, "-connection-socket", socket})
	})
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var packet struct {
		ConnectionState string `json:"connection_state"`
		Connection      struct {
			State       string `json:"state"`
			Outcome     string `json:"outcome"`
			Association string `json:"association"`
			Records     []struct {
				Stage string `json:"stage"`
				State string `json:"state"`
			} `json:"records"`
			Missing []string `json:"missing_stages"`
		} `json:"connection"`
	}
	if err := json.Unmarshal(body, &packet); err != nil {
		t.Fatal(err)
	}
	if packet.ConnectionState != "available" || packet.Connection.State != "joined" || packet.Connection.Outcome != "failed" || len(packet.Connection.Records) != 4 || len(packet.Connection.Missing) == 0 {
		t.Fatal("selected Reader outcome/stages/gaps missing")
	}
	if packet.Connection.Association != "selected-owner-socket; command-run-association-unproven" {
		t.Fatal("selected Reader capture became proof of command association")
	}
	if strings.Contains(string(body), "private-reader-message") {
		t.Fatal("raw Reader error leaked")
	}
}

func TestEvidencePackageRefusesCaptureMutationDuringReaderSelection(t *testing.T) {
	source := reportFixture(t, 0)
	if err := os.Chmod(source, 0700); err != nil {
		t.Fatal(err)
	}
	private := t.TempDir()
	if err := os.Chmod(private, 0700); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(private, "process.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(socket, 0600); err != nil {
		listener.Close()
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := filepath.Join(source, "manifest.json")
		body, err := os.ReadFile(name)
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		body = []byte(strings.Replace(string(body), strings.Repeat("a", 40), strings.Repeat("d", 40), 1))
		if err := os.WriteFile(name, body, 0600); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if _, err := w.Write(connectionSnapshotFixture()); err != nil {
			t.Error(err)
		}
	})}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
		if err := <-done; err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Error(err)
		}
	})
	out := filepath.Join(private, "evidence.json")
	if err := dispatch([]string{"bundle", "-dir", source, "-out", out, "-connection-socket", socket}); err == nil {
		t.Fatal("capture changed during Reader retrieval but package claimed complete")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("published mismatched report/inventory references")
	}
}

func TestEvidencePackageRetainsPartialReceiptWhenSelectedReaderIsUnavailable(t *testing.T) {
	source := reportFixture(t, 7)
	if err := os.Chmod(source, 0700); err != nil {
		t.Fatal(err)
	}
	private := t.TempDir()
	if err := os.Chmod(private, 0700); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(private, "evidence.json")
	if err := dispatch([]string{"bundle", "-dir", source, "-out", out, "-connection-socket", filepath.Join(private, "missing.sock")}); err == nil {
		t.Fatal("missing selected Reader reported as successful assembly")
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("valid command evidence lost on Reader failure: %v", err)
	}
	var packet struct {
		Assembly        string   `json:"assembly"`
		ConnectionState string   `json:"connection_state"`
		Failures        []string `json:"assembly_failures"`
		Run             struct {
			Exit     int  `json:"exit_code"`
			Complete bool `json:"complete"`
		} `json:"run"`
	}
	if err := json.Unmarshal(body, &packet); err != nil {
		t.Fatal(err)
	}
	if packet.Assembly != "incomplete" || packet.ConnectionState != "unavailable" || packet.Run.Exit != 7 || !packet.Run.Complete || len(packet.Failures) != 1 || packet.Failures[0] != "reader-selection-failed" {
		t.Fatal("Reader failure, command outcome and capture integrity were conflated")
	}
}

func TestEvidencePackageIndexesOnlyExplicitPrivateArtifacts(t *testing.T) {
	source := reportFixture(t, 7)
	if err := os.Chmod(source, 0700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"cpu.pprof": "private-profile-bytes", "command.json": "private-command-arguments", "stderr.log": "unselected-private-error"} {
		if err := os.WriteFile(filepath.Join(source, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	private := t.TempDir()
	if err := os.Chmod(private, 0700); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(private, "evidence.json")
	if err := dispatch([]string{"bundle", "-dir", source, "-out", out, "-artifact", "cpu.pprof", "-artifact", "command.json"}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var packet struct {
		Artifacts []struct {
			Name   string `json:"name"`
			Bytes  int64  `json:"bytes"`
			SHA256 string `json:"sha256"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(body, &packet); err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, artifact := range packet.Artifacts {
		found[artifact.Name] = len(artifact.SHA256) == 64
	}
	if len(packet.Artifacts) != 6 || !found["cpu.pprof"] || !found["command.json"] || found["stderr.log"] {
		t.Fatal("explicit private artifact selection not preserved")
	}
	for _, marker := range []string{"private-profile-bytes", "private-command-arguments", "unselected-private-error"} {
		if strings.Contains(string(body), marker) {
			t.Fatal("raw content copied into package")
		}
	}
}

func TestEvidencePackagePreservesFailureWhenSelectedProfileCannotBeParsed(t *testing.T) {
	source := reportFixture(t, 7)
	if err := os.Chmod(source, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "broken.pprof"), []byte("private-invalid-profile"), 0600); err != nil {
		t.Fatal(err)
	}
	private := t.TempDir()
	if err := os.Chmod(private, 0700); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(private, "evidence.json")
	if err := dispatch([]string{"bundle", "-dir", source, "-out", out, "-profile", "broken.pprof"}); err == nil {
		t.Fatal("unparseable selected profile accepted")
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("command failure lost on profile refusal: %v", err)
	}
	var packet struct {
		Assembly string `json:"assembly"`
		Run      struct {
			Exit int `json:"exit_code"`
		} `json:"run"`
		Artifacts []struct {
			Name       string `json:"name"`
			Kind       string `json:"kind"`
			Validation string `json:"validation"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(body, &packet); err != nil {
		t.Fatal(err)
	}
	if packet.Assembly != "incomplete" || packet.Run.Exit != 7 {
		t.Fatal("profile failure erased original outcome")
	}
	found := false
	for _, a := range packet.Artifacts {
		if a.Name == "broken.pprof" && a.Kind == "pprof" && a.Validation == "failed" {
			found = true
		}
	}
	if !found || strings.Contains(string(body), "private-invalid-profile") {
		t.Fatal("profile validation absent or raw profile exposed")
	}
}

func TestEvidencePackageAcceptsRealHeapProfileWithoutClaimingOperationAssociation(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "empty-build-cache"))
	source := reportFixture(t, 0)
	if err := os.Chmod(source, 0700); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(source, "heap.pprof"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(pprof.WriteHeapProfile(f), f.Close()); err != nil {
		t.Fatal(err)
	}
	private := t.TempDir()
	if err := os.Chmod(private, 0700); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(private, "evidence.json")
	if err := dispatch([]string{"bundle", "-dir", source, "-out", out, "-profile", "heap.pprof"}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var packet struct {
		Assembly    string `json:"assembly"`
		Association string `json:"association"`
		Artifacts   []struct {
			Name       string `json:"name"`
			Kind       string `json:"kind"`
			Validation string `json:"validation"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(body, &packet); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range packet.Artifacts {
		if a.Name == "heap.pprof" && a.Kind == "pprof" && a.Validation == "passed" {
			found = true
		}
	}
	if !found || packet.Assembly != "complete" || packet.Association != "explicit-input-selection; operation-association-unproven" {
		t.Fatal("real profile validation or association limit missing")
	}
}

func TestEvidencePackageRefusesAssemblyAfterSelectedTimeBudgetExpires(t *testing.T) {
	source := reportFixture(t, 7)
	if err := os.Chmod(source, 0700); err != nil {
		t.Fatal(err)
	}
	private := t.TempDir()
	if err := os.Chmod(private, 0700); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(private, "evidence.json")
	err := dispatch([]string{"bundle", "-dir", source, "-out", out, "-timeout", "1ns"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected explicit assembly deadline: %v", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("expired assembly published a complete package")
	}
	if _, err := os.Stat(filepath.Join(source, "summary.json")); err != nil {
		t.Fatal("lost original command receipt")
	}
}

func TestEvidencePackageRefusesReplacedCaptureRootDuringReaderSelection(t *testing.T) {
	source := reportFixture(t, 0)
	if err := os.Chmod(source, 0700); err != nil {
		t.Fatal(err)
	}
	private := t.TempDir()
	if err := os.Chmod(private, 0700); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(private, "process.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(socket, 0600); err != nil {
		listener.Close()
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := os.Rename(source, source+"-moved"); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if err := os.Mkdir(source, 0700); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if _, err := w.Write(connectionSnapshotFixture()); err != nil {
			t.Error(err)
		}
	})}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
		if err := <-done; err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Error(err)
		}
	})
	out := filepath.Join(private, "evidence.json")
	if err := dispatch([]string{"bundle", "-dir", source, "-out", out, "-connection-socket", socket}); err == nil {
		t.Fatal("capture root replaced during Reader retrieval but package claimed complete")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("published mismatched report/inventory references")
	}
}

func TestEvidencePackageValidatesRealAndBrokenRuntimeTrace(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "empty-build-cache"))
	source := reportFixture(t, 0)
	if err := os.Chmod(source, 0700); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(source, "runtime.trace"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := trace.Start(f); err != nil {
		f.Close()
		t.Fatal(err)
	}
	trace.Stop()
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	private := t.TempDir()
	if err := os.Chmod(private, 0700); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(private, "evidence.json")
	if err := dispatch([]string{"bundle", "-dir", source, "-out", out, "-trace", "runtime.trace"}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var packet struct {
		Assembly    string `json:"assembly"`
		Association string `json:"association"`
		Artifacts   []struct {
			Name       string `json:"name"`
			Kind       string `json:"kind"`
			Validation string `json:"validation"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(body, &packet); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range packet.Artifacts {
		if a.Name == "runtime.trace" && a.Kind == "go-trace" && a.Validation == "passed" {
			found = true
		}
	}
	if !found || packet.Assembly != "complete" || packet.Association != "explicit-input-selection; operation-association-unproven" {
		t.Fatal("real profile validation or association limit missing")
	}
	if err := os.WriteFile(filepath.Join(source, "runtime.trace"), []byte("private-broken-runtime-trace"), 0600); err != nil {
		t.Fatal(err)
	}
	refused := filepath.Join(private, "broken.json")
	if err := dispatch([]string{"bundle", "-dir", source, "-out", refused, "-trace", "runtime.trace"}); err == nil {
		t.Fatal("broken trace accepted")
	}
	body, err = os.ReadFile(refused)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &packet); err != nil {
		t.Fatal(err)
	}
	failed := false
	for _, a := range packet.Artifacts {
		if a.Name == "runtime.trace" && a.Kind == "go-trace" && a.Validation == "failed" {
			failed = true
		}
	}
	if !failed || packet.Assembly != "incomplete" || strings.Contains(string(body), "private-broken-runtime-trace") {
		t.Fatal("trace failure lost or exposed raw contents")
	}
}

func TestEvidencePackageDoesNotClaimCompleteAssemblyForIncompatibleCaptureSchema(t *testing.T) {
	source := reportFixture(t, 7)
	if err := os.Chmod(source, 0700); err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(source, "manifest.json")
	body, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	body = []byte(strings.Replace(string(body), "ardents-local-diagnostics-v1", "unsupported-capture-v9", 1))
	if err := os.WriteFile(name, body, 0600); err != nil {
		t.Fatal(err)
	}
	private := t.TempDir()
	if err := os.Chmod(private, 0700); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(private, "evidence.json")
	if err := dispatch([]string{"bundle", "-dir", source, "-out", out}); err == nil {
		t.Fatal("unsupported schema reported complete assembly")
	}
	receipt, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var packet struct {
		Assembly string `json:"assembly"`
		Run      struct {
			Exit int      `json:"exit_code"`
			Gaps []string `json:"gaps"`
		} `json:"run"`
	}
	if err := json.Unmarshal(receipt, &packet); err != nil {
		t.Fatal(err)
	}
	if packet.Assembly != "incomplete" || packet.Run.Exit != 7 || !strings.Contains(strings.Join(packet.Run.Gaps, " "), "manifest-unavailable-or-invalid") {
		t.Fatal("schema refusal or original failure missing")
	}
}

func TestEvidencePackageSelectsCommandReceiptWithoutExportingArguments(t *testing.T) {
	source := reportFixture(t, 7)
	if err := os.Chmod(source, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "command.json"), []byte(`["go","test","private-command-argument"]`), 0600); err != nil {
		t.Fatal(err)
	}
	private := t.TempDir()
	if err := os.Chmod(private, 0700); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(private, "evidence.json")
	if err := dispatch([]string{"bundle", "-dir", source, "-out", out, "-command", "command.json"}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var packet struct {
		Reproduction struct {
			State       string `json:"state"`
			Artifact    string `json:"command_artifact"`
			Count       int    `json:"argument_count"`
			Environment string `json:"environment"`
			Association string `json:"association"`
		} `json:"reproduction"`
	}
	if err := json.Unmarshal(body, &packet); err != nil {
		t.Fatal(err)
	}
	r := packet.Reproduction
	if r.State != "available" || r.Artifact != "command.json" || r.Count != 3 || r.Environment != "not-fully-declared" || r.Association != "selected-command-receipt; run-association-unproven" {
		t.Fatal("command recipe availability or evidence limits missing")
	}
	if strings.Contains(string(body), "private-command-argument") {
		t.Fatal("raw command arguments exported")
	}
}

func TestEvidencePackageRetainsFailedOutputWithoutOverwriting(t *testing.T) {
	if os.Getenv("ARDENTS_EVIDENCE_WRITE_LIMIT_CHILD") == "1" {
		var limit syscall.Rlimit
		if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &limit); err != nil {
			t.Fatal(err)
		}
		limit.Cur = 0
		if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &limit); err != nil {
			t.Fatal(err)
		}
		err := dispatch([]string{"bundle", "-dir", os.Getenv("ARDENTS_EVIDENCE_WRITE_SOURCE"), "-out", os.Getenv("ARDENTS_EVIDENCE_WRITE_OUTPUT")})
		if !errors.Is(err, syscall.EFBIG) {
			t.Fatalf("expected actual kernel file-size refusal: %v", err)
		}
		return
	}
	source := reportFixture(t, 7)
	if err := os.Chmod(source, 0700); err != nil {
		t.Fatal(err)
	}
	private := t.TempDir()
	if err := os.Chmod(private, 0700); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(private, "failed.json")
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestEvidencePackageRetainsFailedOutputWithoutOverwriting$")
	cmd.Env = append(os.Environ(), "ARDENTS_EVIDENCE_WRITE_LIMIT_CHILD=1", "ARDENTS_EVIDENCE_WRITE_SOURCE="+source, "ARDENTS_EVIDENCE_WRITE_OUTPUT="+out)
	if body, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("real write refusal: %v %s", err, body)
	}
	info, err := os.Stat(out)
	if err != nil || info.Size() != 0 {
		t.Fatalf("failed output not retained: %v", err)
	}
	if err := dispatch([]string{"bundle", "-dir", source, "-out", out}); err == nil {
		t.Fatal("silently overwrote failed output")
	}
	info, err = os.Stat(out)
	if err != nil || info.Size() != 0 {
		t.Fatal("failed output changed on retry")
	}
	if _, err := os.Stat(filepath.Join(source, "summary.json")); err != nil {
		t.Fatal("lost original command failure")
	}
}

func TestEvidencePackageRejectsNullCommandArguments(t *testing.T) {
	source := reportFixture(t, 7)
	if err := os.Chmod(source, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "command.json"), []byte(`["go",null]`), 0600); err != nil {
		t.Fatal(err)
	}
	private := t.TempDir()
	if err := os.Chmod(private, 0700); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(private, "evidence.json")
	if err := dispatch([]string{"bundle", "-dir", source, "-out", out, "-command", "command.json"}); err == nil {
		t.Fatal("null argv silently became an empty string")
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var packet struct {
		Assembly     string `json:"assembly"`
		Reproduction struct {
			State string `json:"state"`
		} `json:"reproduction"`
		Run struct {
			Exit int `json:"exit_code"`
		} `json:"run"`
	}
	if err := json.Unmarshal(body, &packet); err != nil {
		t.Fatal(err)
	}
	if packet.Assembly != "incomplete" || packet.Reproduction.State != "unavailable" || packet.Run.Exit != 7 {
		t.Fatal("corrupt command receipt lost original failure")
	}
}

func TestEvidencePackageRefusesUnsafeAndOverBudgetInputs(t *testing.T) {
	cases := []struct {
		name  string
		alter func(string) error
		args  []string
	}{
		{"symlink", func(dir string) error {
			old := filepath.Join(dir, "summary.json")
			if err := os.Rename(old, old+".original"); err != nil {
				return err
			}
			return os.Symlink("summary.json.original", old)
		}, nil},
		{"hardlink", func(dir string) error {
			return os.Link(filepath.Join(dir, "summary.json"), filepath.Join(dir, "linked-summary"))
		}, nil},
		{"public-file", func(dir string) error { return os.Chmod(filepath.Join(dir, "summary.json"), 0644) }, nil},
		{"fifo", func(dir string) error {
			old := filepath.Join(dir, "summary.json")
			if err := os.Rename(old, old+".original"); err != nil {
				return err
			}
			return syscall.Mkfifo(old, 0600)
		}, nil},
		{"path-traversal", nil, []string{"-artifact", "../summary.json"}},
		{"duplicate-kinds", nil, []string{"-artifact", "cpu.pprof", "-profile", "cpu.pprof"}},
		{"required-input-selected-again", nil, []string{"-artifact", "manifest.json"}},
		{"artifact-byte-limit", func(dir string) error {
			f, err := os.OpenFile(filepath.Join(dir, "large"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				return err
			}
			return errors.Join(f.Truncate((64<<20)+1), f.Close())
		}, []string{"-artifact", "large"}},
		{"total-byte-limit", func(dir string) error {
			for _, name := range []string{"large-a", "large-b"} {
				f, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
				if err != nil {
					return err
				}
				if err := errors.Join(f.Truncate(64<<20), f.Close()); err != nil {
					return err
				}
			}
			return nil
		}, []string{"-artifact", "large-a", "-artifact", "large-b"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			source := reportFixture(t, 7)
			if err := os.Chmod(source, 0700); err != nil {
				t.Fatal(err)
			}
			if c.alter != nil {
				if err := c.alter(source); err != nil {
					t.Fatal(err)
				}
			}
			private := t.TempDir()
			if err := os.Chmod(private, 0700); err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(private, "evidence.json")
			args := append([]string{"bundle", "-dir", source, "-out", out}, c.args...)
			if err := dispatch(args); err == nil {
				t.Fatal("unsafe or over-budget input admitted")
			}
			if _, err := os.Stat(out); !os.IsNotExist(err) {
				t.Fatal("published a complete package from refused input")
			}
			if _, err := os.Stat(filepath.Join(source, "manifest.json")); err != nil {
				t.Fatal("removed original capture")
			}
		})
	}
}

func TestEvidencePackageRejectsInvalidUTF8CommandArguments(t *testing.T) {
	source := reportFixture(t, 7)
	if err := os.Chmod(source, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "command.json"), append(append([]byte(`["go","`), 0xff), []byte(`"]`)...), 0600); err != nil {
		t.Fatal(err)
	}
	private := t.TempDir()
	if err := os.Chmod(private, 0700); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(private, "evidence.json")
	if err := dispatch([]string{"bundle", "-dir", source, "-out", out, "-command", "command.json"}); err == nil {
		t.Fatal("invalid UTF8 argv silently became replacement characters")
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var packet struct {
		Assembly     string `json:"assembly"`
		Reproduction struct {
			State string `json:"state"`
		} `json:"reproduction"`
		Run struct {
			Exit int `json:"exit_code"`
		} `json:"run"`
	}
	if err := json.Unmarshal(body, &packet); err != nil {
		t.Fatal(err)
	}
	if packet.Assembly != "incomplete" || packet.Reproduction.State != "unavailable" || packet.Run.Exit != 7 {
		t.Fatal("corrupt command receipt lost original failure")
	}
}

func TestEvidencePackageCancelsInFlightReaderAtAssemblyDeadline(t *testing.T) {
	source := reportFixture(t, 0)
	if err := os.Chmod(source, 0700); err != nil {
		t.Fatal(err)
	}
	private := t.TempDir()
	if err := os.Chmod(private, 0700); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(private, "process.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(socket, 0600); err != nil {
		listener.Close()
		t.Fatal(err)
	}
	requested := make(chan struct{})
	canceled := make(chan struct{})
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requested)
		<-r.Context().Done()
		close(canceled)
	})}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
		if err := <-done; err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Error(err)
		}
	})
	out := filepath.Join(private, "evidence.json")
	err = dispatch([]string{"bundle", "-dir", source, "-out", out, "-connection-socket", socket, "-timeout", "200ms"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected actual Reader cancellation at assembly deadline: %v", err)
	}
	select {
	case <-requested:
	default:
		t.Fatal("Reader boundary was not reached")
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("Reader HTTP request was not canceled")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("timed-out Reader assembly published complete output")
	}
	if _, err := os.Stat(filepath.Join(source, "summary.json")); err != nil {
		t.Fatal("lost original capture")
	}
}
