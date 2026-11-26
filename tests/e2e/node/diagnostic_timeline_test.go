package node_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCurrentSourceCommandReachesCompiledDiagnosticTimeline(t *testing.T) {
	binary := buildNodeCommand(t)
	privatePath := filepath.Join(t.TempDir(), "PRIVATE-plan-does-not-exist")
	command := exec.CommandContext(t.Context(), binary, "source", "serve", privatePath)
	var out, diagnostic bytes.Buffer
	command.Stdout, command.Stderr = &out, &diagnostic
	err := command.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("source refusal: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("refused source announced stdout: %q", out.String())
	}
	var report map[string]any
	if err := json.Unmarshal(diagnostic.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	wantOperation, wantOutcome := "source", "unsupported-platform"
	if runtime.GOOS == "linux" {
		wantOperation, wantOutcome = "source.serve", "failed"
	}
	if report["operation"] != wantOperation || report["phase"] != "refused" || report["outcome"] != wantOutcome {
		t.Fatalf("command report: %v", report)
	}
	result := projectCommandTimeline(t, binary, diagnostic.Bytes())
	want := "-\tunavailable\tsource\t-\t-\t-\t" + wantOperation + "\trefused\t\"" + wantOutcome + "\"\n"
	if string(result) != want || strings.Contains(string(result), "PRIVATE") {
		t.Fatalf("command was filtered or leaked private detail: %q", result)
	}
	if _, err := os.Stat(privatePath); !os.IsNotExist(err) {
		t.Fatal("refusal created input")
	}
}

func TestDiagnosticTimelineCommandProjectsStdin(t *testing.T) {
	input := `{"schema":"ardents-source-event-v1","kind":"source-ready","at":"2026-09-25T12:30:00Z"}` + "\n"
	command := exec.CommandContext(t.Context(), buildNodeCommand(t), "diagnostics", "timeline")
	command.Stdin = strings.NewReader(input)
	var output, diagnostic bytes.Buffer
	command.Stdout, command.Stderr = &output, &diagnostic
	if err := command.Run(); err != nil || diagnostic.Len() != 0 {
		t.Fatalf("timeline: %v, stderr %q", err, &diagnostic)
	}
	if want := "2026-09-25T12:30:00Z\tevent\tsource\t-\t-\t-\tsource-ready\t-\t\"\"\n"; output.String() != want {
		t.Fatalf("timeline = %q, want %q", output.String(), want)
	}
}

func projectCommandTimeline(t *testing.T, binary string, report []byte) []byte {
	t.Helper()
	command := exec.CommandContext(t.Context(), binary, "diagnostics", "timeline")
	command.Stdin = bytes.NewReader(report)
	result, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("timeline: %v %s", err, result)
	}
	return result
}
