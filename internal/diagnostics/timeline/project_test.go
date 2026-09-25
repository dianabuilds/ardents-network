package timeline

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

func TestDiagnosticTimelineCombinesRuntimeOwnersWithoutPrivateFields(t *testing.T) {
	at := time.Date(2026, time.September, 25, 12, 30, 0, 0, time.UTC)
	endpoint := diagnosticTestJSON(t, map[string]any{
		"schema": "ardents-headless-runtime-event-v1",
		"kind":   "headless-runtime-publication-withdrawal-failed",
		"at":     at.Add(time.Second), "surface": "Administration",
		"failure": "publication-state", "request_digest": "private-commitment",
	})
	input := bytes.NewBuffer(nil)
	for _, value := range []any{
		map[string]any{"schema": "ardents-node-event-v1", "kind": "lifecycle",
			"at": at, "assignment": "closed_issuer", "state": "FAILED",
			"reason": "Node role cleanup failed", "carrier_profile": "quic", "source_address": "private-address"},
		map[string]any{"MESSAGE": endpoint, "__REALTIME_TIMESTAMP": fmt.Sprint(at.Add(time.Second).UnixMicro()), "_PID": "4242"},
		map[string]any{"MESSAGE": diagnosticTestJSON(t, map[string]any{
			"schema": "ardents-source-event-v1", "kind": "source-ready",
		}), "__REALTIME_TIMESTAMP": fmt.Sprint(at.Add(2 * time.Second).UnixMicro()), "_PID": "4343"},
		map[string]any{"schema": "unknown", "kind": "secret", "content": "private-document"},
		map[string]any{"MESSAGE": "unrelated journal text", "__REALTIME_TIMESTAMP": fmt.Sprint(at.UnixMicro())},
	} {
		input.WriteString(diagnosticTestJSON(t, value))
		input.WriteByte('\n')
	}
	var output bytes.Buffer
	if err := Project(t.Context(), io.NopCloser(input), &output); err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"2026-09-25T12:30:00Z\tevent\tnode\t-\tclosed_issuer\tquic\tlifecycle\tFAILED\t\"Node role cleanup failed\"",
		"2026-09-25T12:30:01Z\tevent\tendpoint\t4242\tAdministration\t-\theadless-runtime-publication-withdrawal-failed\t-\t\"publication-state\"",
		"2026-09-25T12:30:02Z\tjournal\tsource\t4343\t-\t-\tsource-ready\t-\t\"\"",
	}, "\n") + "\n"
	if output.String() != want {
		t.Fatalf("diagnostic timeline = %q, want %q", output.String(), want)
	}
	for _, private := range []string{"private-address", "private-commitment", "private-document"} {
		if strings.Contains(output.String(), private) {
			t.Fatalf("diagnostic timeline exposed %s", private)
		}
	}
}

func TestDiagnosticTimelineRejectsMalformedKnownCategoryWithoutEcho(t *testing.T) {
	input := diagnosticTestJSON(t, map[string]any{
		"schema": "ardents-node-event-v1", "kind": "lifecycle",
		"at": "2026-09-25T12:30:00Z", "state": "BAD\nSTATE",
		"reason": "private-token",
	})
	var output bytes.Buffer
	err := Project(t.Context(), io.NopCloser(strings.NewReader(input+"\n")), &output)
	if err == nil || output.Len() != 0 || strings.Contains(err.Error(), "private-token") {
		t.Fatalf("malformed known event: output=%q err=%v", output.String(), err)
	}
}

func TestDiagnosticTimelineRejectsCorruptInputWithoutEcho(t *testing.T) {
	input := `{"schema":"ardents-node-event-v1","kind":"lifecycle","private":"sensitive` + "\n"
	var output bytes.Buffer
	err := Project(t.Context(), io.NopCloser(strings.NewReader(input)), &output)
	if err == nil || output.Len() != 0 || strings.Contains(err.Error(), "sensitive") {
		t.Fatalf("corrupt diagnostic input: output=%q err=%v", output.String(), err)
	}
}

func TestDiagnosticTimelineProjectsSourceFailureCategory(t *testing.T) {
	input := diagnosticTestJSON(t, map[string]any{
		"schema": "ardents-source-event-v1", "kind": "source-failed",
		"at": "2026-09-25T12:30:00Z", "reason": "background-work",
		"raw_error": "private source failure detail",
	})
	var output bytes.Buffer
	if err := Project(t.Context(), io.NopCloser(strings.NewReader(input+"\n")), &output); err != nil {
		t.Fatal(err)
	}
	want := "2026-09-25T12:30:00Z\tevent\tsource\t-\t-\t-\tsource-failed\t-\t\"background-work\"\n"
	if output.String() != want {
		t.Fatalf("source failure timeline = %q", output.String())
	}
}

func TestDiagnosticTimelineRejectsUnrecognizedSourceFailureReason(t *testing.T) {
	input := diagnosticTestJSON(t, map[string]any{
		"schema": "ardents-source-event-v1", "kind": "source-failed",
		"at": "2026-09-25T12:30:00Z", "reason": "private source failure detail",
	})
	var output bytes.Buffer
	err := Project(t.Context(), io.NopCloser(strings.NewReader(input+"\n")), &output)
	if err == nil || output.Len() != 0 || strings.Contains(err.Error(), "private source failure detail") {
		t.Fatalf("unsafe Source diagnostic: output=%q err=%v", output.String(), err)
	}
}

func TestDiagnosticTimelineCancellationClosesWaitingInput(t *testing.T) {
	reader, writer := io.Pipe()
	defer writer.Close()
	ctx, cancel := context.WithCancel(t.Context())
	var output bytes.Buffer
	done := make(chan error, 1)
	go func() { done <- Project(ctx, reader, &output) }()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled diagnostic timeline = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("diagnostic timeline did not join canceled input")
	}
}

func diagnosticTestJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
