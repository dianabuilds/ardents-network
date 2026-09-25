package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestDiagnosticTimelineCommandProjectsStdin(t *testing.T) {
	input := `{"schema":"ardents-source-event-v1","kind":"source-ready","at":"2026-09-25T12:30:00Z"}` + "\n"
	var output bytes.Buffer
	if err := runWithInput(t.Context(), []string{"diagnostics", "timeline"}, io.NopCloser(strings.NewReader(input)), &output); err != nil {
		t.Fatal(err)
	}
	if want := "2026-09-25T12:30:00Z\tevent\tsource\t-\t-\t-\tsource-ready\t-\t\"\"\n"; output.String() != want {
		t.Fatalf("timeline = %q, want %q", output.String(), want)
	}
}
