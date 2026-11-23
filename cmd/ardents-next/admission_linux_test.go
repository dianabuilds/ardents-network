//go:build linux

package main

import "github.com/dianabuilds/ardents-network/internal/admission/quota"

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	metrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	traces "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"
)

func admissionInitializationConfig(root string, binding quota.LedgerBinding) map[string]any {
	return map[string]any{"root": root, "binding": admissionBindingConfig(binding)}
}

func TestAdmissionInitializationCompiledCLI(t *testing.T) {
	raw, facts, binding := admissionCommandFixture(t, 2, 1, 2, 1)
	root := filepath.Join(t.TempDir(), "ledger")
	config := hostingConfig(t, admissionInitializationConfig(root, binding))
	command := exec.CommandContext(t.Context(), compiledCommand(t), "admission", "initialize", "--config", config)
	var out, diagnostic bytes.Buffer
	command.Stdout, command.Stderr = &out, &diagnostic
	if err := command.Run(); err != nil {
		t.Fatal(err, &out, &diagnostic)
	}
	var result admissionResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.Outcome != quota.Initialized {
		t.Fatal(result, err)
	}
	if bytes.Contains(diagnostic.Bytes(), []byte(root)) {
		t.Fatal("path leaked")
	}

	before := snapshotAdmissionRoot(t, root)
	batch := filepath.Join(t.TempDir(), "batch")
	if err := os.WriteFile(batch, raw, 0600); err != nil {
		t.Fatal(err)
	}
	retired := admissionInitializationConfig(root, binding)
	retired["batch_file"], retired["facts"], retired["kind"] = batch, admissionFactsConfig(facts), "bootstrap"
	command = exec.CommandContext(t.Context(), compiledCommand(t), "admission", "debit", "--config", hostingConfig(t, retired))
	output, err := command.Output()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 2 || json.Unmarshal(output, &result) != nil || result.Outcome != "invalid-input" {
		t.Fatalf("removed debit route did not refuse: %s (%v)", output, err)
	}
	if !reflect.DeepEqual(before, snapshotAdmissionRoot(t, root)) {
		t.Fatal("removed debit route modified quota")
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
	_, _, binding := admissionCommandFixture(t, 2, 1, 2, 1)
	root := filepath.Join(t.TempDir(), "ledger")
	init := admissionInitializationConfig(root, binding)
	var out, log bytes.Buffer
	if code := run(t.Context(), []string{"admission", "initialize", "--config", hostingConfig(t, init), server.URL}, &out, &log); code != 0 {
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
	if span.Name != "admission.initialize" || len(span.Attributes) != 3 || len(span.Events) != 0 || len(span.ParentSpanId) != 0 {
		t.Fatal("unsafe span")
	}
	for _, attr := range span.Attributes {
		want := map[string]string{"operation": "admission.initialize", "phase": "execute", "outcome": "initialized-offline"}
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
