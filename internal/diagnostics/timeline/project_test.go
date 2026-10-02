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
func TestDiagnosticTimelineRejectsWrongFieldTypes(t *testing.T) {
	for _, event := range []struct {
		schema string
		kind   string
		fields []string
	}{
		{"ardents-node-event-v1", "lifecycle", []string{"kind", "at", "assignment", "carrier_profile", "state", "reason"}},
		{"ardents-source-event-v1", "source-ready", []string{"kind", "at", "reason"}},
		{"ardents-headless-runtime-event-v1", "headless-runtime-failed", []string{"kind", "at", "surface", "failure"}},
	} {
		for _, field := range event.fields {
			for _, value := range []any{42, false, map[string]any{"private": "private-token"}, []any{"private-token"}, nil} {
				for _, journal := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%s/%T/journal=%t", event.schema, field, value, journal), func(t *testing.T) {
						record := map[string]any{"schema": event.schema, "kind": event.kind, "at": "2026-10-02T10:00:00Z"}
						record[field] = value
						input := diagnosticTestJSON(t, record)
						if journal {
							input = diagnosticTestJSON(t, map[string]any{"MESSAGE": input, "__REALTIME_TIMESTAMP": "1790935200000000"})
						}
						var output bytes.Buffer
						err := Project(t.Context(), io.NopCloser(strings.NewReader(input+"\n")), &output)
						if err == nil || output.Len() != 0 || strings.Contains(err.Error(), "private-token") {
							t.Fatalf("wrong field type: output=%q err=%v", output.String(), err)
						}
					})
				}
			}
		}
	}
}

func TestDiagnosticTimelinePreservesOptionalAbsenceAndFiltering(t *testing.T) {
	for _, record := range []map[string]any{
		{"schema": "ardents-node-event-v1", "kind": "lifecycle", "at": "2026-10-02T10:00:00Z"},
		{"schema": "ardents-source-event-v1", "kind": "source-ready"},
		{"schema": "ardents-headless-runtime-event-v1", "kind": "headless-runtime-ready", "at": "2026-10-02T10:00:00Z"},
	} {
		input := diagnosticTestJSON(t, map[string]any{"MESSAGE": diagnosticTestJSON(t, record), "__REALTIME_TIMESTAMP": "1790935200000000"})
		var output bytes.Buffer
		if err := Project(t.Context(), io.NopCloser(strings.NewReader(input+"\n")), &output); err != nil || output.Len() == 0 {
			t.Fatalf("optional absence: output=%q err=%v", output.String(), err)
		}
		if record["schema"] == "ardents-source-event-v1" && !strings.Contains(output.String(), "\tjournal\tsource\t") {
			t.Fatalf("old Source did not retain journal time: %q", output.String())
		}
	}
	for _, input := range []string{
		`{"schema":"unknown","kind":42,"state":false}`,
		`{"MESSAGE":"unrelated journal text","_PID":false}`,
	} {
		var output bytes.Buffer
		if err := Project(t.Context(), io.NopCloser(strings.NewReader(input+"\n")), &output); err != nil || output.Len() != 0 {
			t.Fatalf("filtered input: output=%q err=%v", output.String(), err)
		}
	}
}
