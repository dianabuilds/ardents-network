//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/hosting"
	metrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	traces "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"
)

func hostingConfig(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func hostingInitializePlan(t *testing.T) hostingPlan {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	return hostingPlan{Root: filepath.Join(t.TempDir(), "budget"), Policy: hosting.Policy{Provider: "private-provider", Start: now.Add(-time.Minute), End: now.Add(time.Hour), Unit: "B", Quantity: 1000000000, Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1000}}
}

func TestHostingCompiledCLISharedBudgetCancelAndCrash(t *testing.T) {
	binary := compiledCommand(t)
	command := func(operation, config string) (hostingResult, error) {
		cmd := exec.CommandContext(t.Context(), binary, "hosting", operation, "--config", config)
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		var result hostingResult
		if e := json.Unmarshal(out.Bytes(), &result); e != nil {
			t.Fatalf("invalid CLI result %s %v", &out, e)
		}
		return result, err
	}
	for _, episode := range []string{"complete", "cancel", "crash", "cleanup-failure"} {
		t.Run(episode, func(t *testing.T) {
			p := hostingInitializePlan(t)
			if result, err := command("initialize", hostingConfig(t, map[string]any{"root": p.Root, "policy": p.Policy})); err != nil || result.Outcome != "completed" {
				t.Fatalf("initialize %+v %v", result, err)
			}
			hold := map[string]any{"root": p.Root, "work": hosting.Traffic{Tx: 700000000}, "termination": hosting.Traffic{Rx: 100000000}, "hold_ms": uint64(5000)}
			if episode == "complete" {
				// This is a successful lifecycle oracle, not a 20ms storage
				// latency promise. Reserve still enforces the original deadline.
				hold["hold_ms"] = uint64(1000)
				result, err := command("hold", hostingConfig(t, hold))
				if err != nil || result.Outcome != "completed" {
					t.Fatalf("complete %+v %v", result, err)
				}
			} else {
				cmd := exec.CommandContext(t.Context(), binary, "hosting", "hold", "--config", hostingConfig(t, hold))
				stderr, err := cmd.StderrPipe()
				if err != nil {
					t.Fatal(err)
				}
				var stdout bytes.Buffer
				cmd.Stdout = &stdout
				if err = cmd.Start(); err != nil {
					t.Fatal(err)
				}
				joined := false
				defer func() {
					if !joined {
						cmd.Process.Kill()
						cmd.Wait()
					}
				}()
				ready := make(chan error, 1)
				go func() {
					var event struct{ Phase, Outcome string }
					e := json.NewDecoder(stderr).Decode(&event)
					if e == nil && (event.Phase != "hold" || event.Outcome != "reserved") {
						e = io.ErrUnexpectedEOF
					}
					ready <- e
					io.Copy(io.Discard, stderr)
				}()
				select {
				case e := <-ready:
					if e != nil {
						t.Fatal(e)
					}
				case <-time.After(3 * time.Second):
					t.Fatal("hold readiness missing")
				}
				view, err := command("observe", hostingConfig(t, map[string]any{"root": p.Root}))
				if err != nil || view.Observation == nil || view.Observation.ReservedBytes != 800000000 {
					t.Fatalf("shared %+v %v", view, err)
				}
				refused, err := command("hold", hostingConfig(t, hold))
				if err == nil || refused.Outcome != "budget-exhausted" {
					t.Fatalf("competing reserve %+v %v", refused, err)
				}
				if episode == "cleanup-failure" {
					if err := os.WriteFile(filepath.Join(p.Root, "budget.pending"), []byte("interrupted"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				if episode == "cancel" || episode == "cleanup-failure" {
					err = cmd.Process.Signal(os.Interrupt)
				} else {
					err = cmd.Process.Kill()
				}
				if err != nil {
					t.Fatal(err)
				}
				err = cmd.Wait()
				joined = true
				if err == nil {
					t.Fatal("interrupted process succeeded")
				}
				if episode == "cancel" {
					var result hostingResult
					if json.Unmarshal(stdout.Bytes(), &result) != nil || result.Outcome != "canceled" || cmd.ProcessState.ExitCode() != 130 {
						t.Fatalf("cancel %s %v", &stdout, err)
					}
				}
				if episode == "cleanup-failure" {
					var result hostingResult
					if json.Unmarshal(stdout.Bytes(), &result) != nil || result.Outcome != "storage-uncertain" || cmd.ProcessState.ExitCode() != 1 {
						t.Fatalf("cleanup masked by cancel: %s %v", &stdout, err)
					}
					return
				}
			}
			view, err := command("observe", hostingConfig(t, map[string]any{"root": p.Root}))
			want := uint64(0)
			if episode == "crash" {
				want = 800000000
			}
			if err != nil || view.Observation == nil || view.Observation.ReservedBytes != want {
				t.Fatalf("reopen %+v want %d %v", view, want, err)
			}
		})
	}
}

func TestHostingOTLPContainsOnlyFiniteAttributes(t *testing.T) {
	var mu sync.Mutex
	seen := map[string][]byte{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 32769))
		if err != nil || len(body) > 32768 {
			t.Error("unreadable or oversized OTLP export", err)
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("ambient authorization escaped into the export")
		}
		mu.Lock()
		seen[r.URL.Path] = body
		mu.Unlock()
		w.Header().Set("Content-Type", "application/x-protobuf")
	}))
	defer server.Close()
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "Authorization=private-header")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http/json")
	t.Setenv("OTEL_TRACES_SAMPLER", "always_off")
	p := hostingInitializePlan(t)
	config := hostingConfig(t, map[string]any{"root": p.Root, "policy": p.Policy})
	var out, log bytes.Buffer
	if code := run(context.Background(), []string{"hosting", "initialize", "--config", config, server.URL + "/"}, &out, &log); code != 0 {
		t.Fatalf("%d %s %s", code, &out, &log)
	}
	mu.Lock()
	defer mu.Unlock()
	var tr traces.ExportTraceServiceRequest
	var mr metrics.ExportMetricsServiceRequest
	if proto.Unmarshal(seen["/v1/traces"], &tr) != nil || len(tr.ResourceSpans) != 1 {
		t.Fatal("trace missing")
	}
	span := tr.ResourceSpans[0].ScopeSpans[0].Spans[0]
	if span.Name != "hosting.initialize" || len(span.Attributes) != 3 || len(span.ParentSpanId) != 0 || len(span.Events) != 0 || len(span.Links) != 0 {
		t.Fatalf("unsafe span %v", span)
	}
	for _, a := range span.Attributes {
		switch a.Key {
		case "operation":
			if a.Value.GetStringValue() != "hosting.initialize" {
				t.Fatal(a)
			}
		case "phase":
			if a.Value.GetStringValue() != "execute" {
				t.Fatal(a)
			}
		case "outcome":
			if a.Value.GetStringValue() != "completed" {
				t.Fatal(a)
			}
		default:
			t.Fatal(a)
		}
	}
	if proto.Unmarshal(seen["/v1/metrics"], &mr) != nil || len(mr.ResourceMetrics) != 1 {
		t.Fatal("metrics missing")
	}
	resource := tr.ResourceSpans[0].Resource
	if len(resource.Attributes) != 1 || resource.Attributes[0].Key != "service.name" ||
		resource.Attributes[0].Value.GetStringValue() != "ardents-next" ||
		!proto.Equal(resource, mr.ResourceMetrics[0].Resource) {
		t.Fatal("unsafe resource metadata")
	}
	for _, scope := range mr.ResourceMetrics[0].ScopeMetrics {
		if len(scope.Metrics) != 2 {
			t.Fatal("unexpected instruments")
		}
		for _, m := range scope.Metrics {
			switch m.Name {
			case "ardents.hosting.operations":
				points := m.GetSum().DataPoints
				if len(points) != 1 || points[0].GetAsInt() != 1 || len(points[0].Attributes) != 3 || len(points[0].Exemplars) != 0 {
					t.Fatal("unsafe counter")
				}
			case "ardents.hosting.duration":
				points := m.GetHistogram().DataPoints
				if m.Unit != "s" || len(points) != 1 || points[0].Count != 1 || len(points[0].Attributes) != 3 || len(points[0].Exemplars) != 0 {
					t.Fatal("unsafe duration")
				}
			default:
				t.Fatal("unexpected metric", m.Name)
			}
		}
	}
	for _, observed := range [][]byte{out.Bytes(), log.Bytes(), seen["/v1/traces"], seen["/v1/metrics"]} {
		for _, private := range []string{p.Root, config, p.Policy.Provider, "private-header"} {
			if bytes.Contains(observed, []byte(private)) {
				t.Fatal("private metadata escaped")
			}
		}
	}
	var event struct{ Telemetry string }
	if err := json.Unmarshal(log.Bytes(), &event); err != nil || event.Telemetry != "completed" {
		t.Fatalf("unexpected telemetry status %q: %v", log.String(), err)
	}
}

func TestHostingCollectorFailurePreservesCommittedResult(t *testing.T) {
	for _, failure := range []string{"unreachable", "timeout", "oversized-response"} {
		t.Run(failure, func(t *testing.T) {
			endpoint := "http://127.0.0.1:1"
			if failure != "unreachable" {
				release := make(chan struct{})
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if failure == "timeout" {
						select {
						case <-r.Context().Done():
						case <-release:
						}
						return
					}
					w.Header().Set("Content-Type", "application/x-protobuf")
					_, _ = w.Write(bytes.Repeat([]byte("private-response"), 1000))
				}))
				defer server.Close()
				defer close(release)
				endpoint = server.URL
			}
			p := hostingInitializePlan(t)
			config := hostingConfig(t, map[string]any{"root": p.Root, "policy": p.Policy})
			var out, diagnostic bytes.Buffer
			start := time.Now()
			if code := run(t.Context(), []string{"hosting", "initialize", "--config", config, endpoint}, &out, &diagnostic); code != 0 {
				t.Fatalf("collector changed result: %d, %s, %s", code, out.String(), diagnostic.String())
			}
			if time.Since(start) > 2*time.Second {
				t.Fatal("collector failure exceeded the operation test bound")
			}
			var event struct{ Telemetry string }
			if err := json.Unmarshal(diagnostic.Bytes(), &event); err != nil || event.Telemetry != "unavailable" ||
				bytes.Contains(diagnostic.Bytes(), []byte("private-response")) {
				t.Fatalf("unsafe telemetry refusal %q: %v", diagnostic.String(), err)
			}
			budget, err := hosting.Open(p.Root)
			if err != nil {
				t.Fatal("collector failure changed retained state", err)
			}
			if err := budget.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHostingInputAndOutputRefusals(t *testing.T) {
	p := hostingInitializePlan(t)
	config := hostingConfig(t, map[string]any{"root": p.Root, "policy": p.Policy})
	args := []string{"hosting", "initialize", "--config", config}
	var out, diagnostic bytes.Buffer
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if code := run(canceled, args, &out, &diagnostic); code != 130 || diagnostic.Len() != 0 {
		t.Fatalf("canceled hosting initialization returned %d, stderr %q", code, diagnostic.String())
	}
	if _, err := os.Stat(p.Root); !os.IsNotExist(err) {
		t.Fatalf("canceled hosting initialization touched its root: %v", err)
	}
	invalid := filepath.Join(t.TempDir(), "private-config")
	if err := os.WriteFile(invalid, []byte("private-input"), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := run(t.Context(), []string{"hosting", "initialize", "--config", invalid}, &out, &diagnostic); code != 2 ||
		out.String() != "{\"operation\":\"hosting.initialize\",\"phase\":\"input\",\"outcome\":\"invalid-input\"}\n" || diagnostic.Len() != 0 {
		t.Fatalf("invalid input: code %d, stdout %q, stderr %q", code, out.String(), diagnostic.String())
	}
	if code := run(t.Context(), args, unavailableOutput{}, &diagnostic); code != 2 ||
		bytes.Contains(diagnostic.Bytes(), []byte("private-output-error")) {
		t.Fatalf("unsafe output refusal: code %d, stderr %q", code, diagnostic.String())
	}
}
