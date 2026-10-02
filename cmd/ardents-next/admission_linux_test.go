//go:build linux

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	metrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	traces "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"
)

func admissionCommandPlans(t *testing.T) (string, map[string]any, map[string]any, admission.LedgerBinding) {
	t.Helper()
	raw, f, b := admissionCommandFixture(t, 2, 1, 2, 1)
	dir := t.TempDir()
	root := filepath.Join(dir, "ledger")
	batch := filepath.Join(dir, "batch")
	if err := os.WriteFile(batch, raw, 0600); err != nil {
		t.Fatal(err)
	}
	binding := admissionBindingConfig(b)
	init := map[string]any{"root": root, "binding": binding}
	debit := map[string]any{"root": root, "binding": binding, "batch_file": batch, "facts": admissionFactsConfig(f), "kind": "bootstrap"}
	return root, init, debit, b
}
func TestAdmissionCompiledCLI(t *testing.T) {
	binary := compiledCommand(t)
	root, init, debit, b := admissionCommandPlans(t)
	command := func(operation string, plan map[string]any, collector string) (admissionResult, error) {
		args := []string{"admission", operation, "--config", hostingConfig(t, plan)}
		if collector != "" {
			args = append(args, collector)
		}
		cmd := exec.CommandContext(t.Context(), binary, args...)
		var out, log bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &log
		err := cmd.Run()
		var result admissionResult
		if e := json.Unmarshal(out.Bytes(), &result); e != nil {
			t.Fatalf("invalid result %s %s %v", &out, &log, e)
		}
		if bytes.Contains(log.Bytes(), []byte(root)) {
			t.Fatal("path leaked")
		}
		return result, err
	}
	if got, err := command("initialize", init, ""); err != nil || got.Outcome != admission.Initialized {
		t.Fatal(got, err)
	}
	l, err := admission.Open(root, b)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := command("debit", debit, ""); err == nil || got.Outcome != admission.Busy {
		t.Fatal(got, err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if got, err := command("debit", debit, "http://127.0.0.1:1"); err != nil || got.Outcome != admission.Debited {
		t.Fatal(got, err)
	}
	if got, err := command("debit", debit, ""); err != nil || got.Outcome != admission.AlreadyDebited {
		t.Fatal(got, err)
	}
	raw, _, _ := admissionCommandFixture(t, 2, 1, 2, 2)
	if err := os.WriteFile(debit["batch_file"].(string), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := command("debit", debit, ""); err != nil || got.Outcome != admission.Debited {
		t.Fatal(got, err)
	}
	raw, _, _ = admissionCommandFixture(t, 2, 1, 2, 3)
	_ = os.WriteFile(debit["batch_file"].(string), raw, 0600)
	if got, err := command("debit", debit, ""); err == nil || got.Outcome != admission.Exhausted {
		t.Fatal(got, err)
	}
	debit["kind"] = "admitted"
	raw, _, _ = admissionCommandFixture(t, 2, 1, 2, 1)
	_ = os.WriteFile(debit["batch_file"].(string), raw, 0600)
	if got, err := command("debit", debit, ""); err == nil || got.Outcome != admission.Conflict {
		t.Fatal(got, err)
	}
	// Synchronize actual SIGINT/Kill after commit with the first OTLP request.
	// Collector shutdown is independent of operation cancellation.
	for _, episode := range []string{"kill", "interrupt"} {
		t.Run(episode, func(t *testing.T) {
			_, fresh, request, _ := admissionCommandPlans(t)
			if got, err := command("initialize", fresh, ""); err != nil || got.Outcome != admission.Initialized {
				t.Fatal(got, err)
			}
			entered := make(chan struct{}, 2)
			release := make(chan struct{})
			collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				entered <- struct{}{}
				select {
				case <-r.Context().Done():
				case <-release:
				}
			}))
			defer collector.Close()
			defer close(release)
			cmd := exec.CommandContext(t.Context(), binary, "admission", "debit", "--config", hostingConfig(t, request), collector.URL)
			var output bytes.Buffer
			cmd.Stdout = &output
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = cmd.Process.Kill() })
			select {
			case <-entered:
			case <-t.Context().Done():
				t.Fatal("collector not reached")
			}
			if episode == "kill" {
				if err := cmd.Process.Kill(); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := cmd.Process.Signal(os.Interrupt); err != nil {
					t.Fatal(err)
				}
			}
			err := cmd.Wait()
			if episode == "interrupt" {
				var result admissionResult
				if err != nil || json.Unmarshal(output.Bytes(), &result) != nil || result.Outcome != admission.Debited {
					t.Fatalf("committed cancellation %s %v", &output, err)
				}
			}
			if got, err := command("debit", request, ""); err != nil || got.Outcome != admission.AlreadyDebited {
				t.Fatal("process loss refunded", got, err)
			}
		})
	}
}

func TestAdmissionOTLPContainsOnlyFiniteAttributes(t *testing.T) {
	var mu sync.Mutex
	seen := map[string][]byte{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(io.LimitReader(r.Body, 32769))
		mu.Lock()
		seen[r.URL.Path] = raw
		mu.Unlock()
		w.Header().Set("Content-Type", "application/x-protobuf")
	}))
	defer server.Close()
	root, init, debit, b := admissionCommandPlans(t)
	if err := admission.Initialize(root, b); err != nil {
		t.Fatal(err)
	}
	var out, log bytes.Buffer
	if code := run(t.Context(), []string{"admission", "debit", "--config", hostingConfig(t, debit), server.URL}, &out, &log); code != 0 {
		t.Fatal(code, &out, &log)
	}
	mu.Lock()
	defer mu.Unlock()
	var tr traces.ExportTraceServiceRequest
	var mr metrics.ExportMetricsServiceRequest
	if proto.Unmarshal(seen["/v1/traces"], &tr) != nil || len(tr.ResourceSpans) != 1 {
		t.Fatal("missing trace")
	}
	span := tr.ResourceSpans[0].ScopeSpans[0].Spans[0]
	if span.Name != "admission.debit" || len(span.Attributes) != 3 || len(span.Events) != 0 || len(span.ParentSpanId) != 0 {
		t.Fatal("unsafe span")
	}
	for _, attr := range span.Attributes {
		want := map[string]string{"operation": "admission.debit", "phase": "execute", "outcome": "debited-offline"}
		if want[attr.Key] != attr.Value.GetStringValue() {
			t.Fatal("unexpected attribute", attr)
		}
	}
	if proto.Unmarshal(seen["/v1/metrics"], &mr) != nil || len(mr.ResourceMetrics) != 1 {
		t.Fatal("missing metrics")
	}
	if len(tr.ResourceSpans[0].Resource.Attributes) != 1 || len(mr.ResourceMetrics[0].Resource.Attributes) != 1 {
		t.Fatal("extra resource metadata")
	}
	for _, scope := range mr.ResourceMetrics[0].ScopeMetrics {
		if len(scope.Metrics) != 2 {
			t.Fatal("extra instruments")
		}
		for _, m := range scope.Metrics {
			switch m.Name {
			case "ardents.admission.operations":
				points := m.GetSum().DataPoints
				if len(points) != 1 || points[0].GetAsInt() != 1 || len(points[0].Attributes) != 3 || len(points[0].Exemplars) != 0 {
					t.Fatal("unsafe counter")
				}
			case "ardents.admission.duration":
				points := m.GetHistogram().DataPoints
				if len(points) != 1 || points[0].Count != 1 || m.Unit != "s" || len(points[0].Attributes) != 3 || len(points[0].Exemplars) != 0 {
					t.Fatal("unsafe duration")
				}
			default:
				t.Fatal("unexpected metric")
			}
		}
	}
	for _, raw := range [][]byte{seen["/v1/traces"], seen["/v1/metrics"], log.Bytes()} {
		if bytes.Contains(raw, []byte(root)) || bytes.Contains(raw, []byte(init["binding"].(admissionBindingInput).Authority)) {
			t.Fatal("metadata leaked")
		}
	}
}
