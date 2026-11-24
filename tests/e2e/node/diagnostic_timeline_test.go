package node_test

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

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
