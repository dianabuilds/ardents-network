package telemetry

import (
	"bytes"
	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	metrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	traces "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"
)

// The collector is the oracle: rejection must happen before either SDK emits
// the observation, including when the unknown value contains private metadata.
func TestUnclassifiedObservationsNeverReachCollector(t *testing.T) {
	const private = "/private/root/person-123"
	for _, tc := range []struct {
		name       string
		operation  runtimeevent.Operation
		completion runtimeevent.Completion
	}{
		{"unknown operation", runtimeevent.LookupOperation(private), runtimeevent.Completion{}},
		{"forged operation", runtimeevent.Operation(255), runtimeevent.Completion{}},
		{"unknown phase", runtimeevent.HostingObserve, runtimeevent.HostingObserve.Classify(private, "completed")},
		{"unknown outcome", runtimeevent.HostingObserve, runtimeevent.HostingObserve.Classify("execute", private)},
		{"foreign completion", runtimeevent.AdmissionInitialize, runtimeevent.HostingObserve.Classify("execute", "completed")},
		{"foreign success", runtimeevent.AdmissionInitialize, runtimeevent.AdmissionInitialize.Classify("execute", "completed")},
		{"foreign phase", runtimeevent.IdentityImport, runtimeevent.IdentityImport.Classify("debit", "identity-imported")},
		{"zero completion", runtimeevent.HostingObserve, runtimeevent.Completion{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var requests int
			var mu sync.Mutex
			collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				requests++
				mu.Unlock()
				w.WriteHeader(http.StatusNoContent)
			}))
			defer collector.Close()
			observation, err := Open(collector.URL)
			if err != nil {
				t.Fatal(err)
			}
			observation.Begin(tc.operation)(tc.completion)
			if status := observation.CloseWithStatus(nil); status != Unavailable {
				t.Fatalf("delivery=%s", status)
			}
			mu.Lock()
			defer mu.Unlock()
			if requests != 0 {
				t.Fatalf("unclassified observation emitted %d requests", requests)
			}
		})
	}
}

func TestRegisteredCommandOTLPCompatibility(t *testing.T) {
	for _, tc := range []struct{ name, scope, phase, outcome string }{
		{"admission.initialize", "ardents.admission", "execute", "initialized-offline"},
		{"hosting.initialize", "ardents.hosting", "execute", "completed"},
		{"hosting.observe", "ardents.hosting", "close", "storage-uncertain"},
		{"issuance.initialize", "ardents.issuance", "export", "completed"},
		{"issuance.inspect", "ardents.issuance", "close", "storage-uncertain"},
		{"issuance.initialize-results", "ardents.issuance", "open-results", "initialized-results"},
		{"issuance.issue", "ardents.issuance", "debit", "invalid-signature"},
		{"issuance.issue", "ardents.issuance", "export", "already-issued"},
		{"issuance.initialize-profile", "ardents.issuance", "export", "profile-initialized"},
		{"issuance.inspect-profile", "ardents.issuance", "close-identity", "storage-uncertain"},
		{"node-identity.import", "ardents.issuance", "import", "identity-imported"},
		{"admission.prepare-binding", "ardents.issuance", "export", "binding-prepared"},
	} {
		t.Run(tc.name+"/"+tc.phase, func(t *testing.T) {
			var packets = make(map[string][]byte)
			var mu sync.Mutex
			collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				mu.Lock()
				packets[r.URL.Path] = raw
				mu.Unlock()
				w.WriteHeader(http.StatusNoContent)
			}))
			defer collector.Close()
			observation, err := Open(collector.URL)
			if err != nil {
				t.Fatal(err)
			}
			operation := runtimeevent.LookupOperation(tc.name)
			finish := observation.Begin(operation)
			started := time.Now()
			// runtimeevent.Completion remains single-use even if a caller accidentally reports twice.
			finish(operation.Classify(tc.phase, tc.outcome))
			finish(operation.Classify(tc.phase, tc.outcome))
			if status := observation.CloseWithStatus(nil); status != Unavailable {
				t.Fatalf("duplicate delivery=%s", status)
			}
			mu.Lock()
			defer mu.Unlock()
			var tr traces.ExportTraceServiceRequest
			var mr metrics.ExportMetricsServiceRequest
			if err := proto.Unmarshal(packets["/v1/traces"], &tr); err != nil {
				t.Fatal(err)
			}
			if len(tr.ResourceSpans) != 1 || len(tr.ResourceSpans[0].ScopeSpans) != 1 {
				t.Fatal("missing trace scope")
			}
			scope := tr.ResourceSpans[0].ScopeSpans[0]
			if scope.Scope.Name != tc.scope || len(scope.Spans) != 1 {
				t.Fatal("changed trace scope/count")
			}
			span := scope.Spans[0]
			if span.Name != tc.name || len(span.Attributes) != 3 || len(span.Events) != 0 || len(span.ParentSpanId) != 0 {
				t.Fatal("changed span shape")
			}
			if span.StartTimeUnixNano > uint64(started.UnixNano()) || span.EndTimeUnixNano < span.StartTimeUnixNano {
				t.Fatal("lost original timing")
			}
			want := map[string]string{"operation": tc.name, "phase": tc.phase, "outcome": tc.outcome}
			for _, attr := range span.Attributes {
				if want[attr.Key] != attr.Value.GetStringValue() {
					t.Fatal("changed span categories")
				}
			}
			if err := proto.Unmarshal(packets["/v1/metrics"], &mr); err != nil {
				t.Fatal(err)
			}
			if len(mr.ResourceMetrics) != 1 || len(mr.ResourceMetrics[0].ScopeMetrics) != 1 {
				t.Fatal("missing metric scope")
			}
			ms := mr.ResourceMetrics[0].ScopeMetrics[0]
			if ms.Scope.Name != tc.scope || len(ms.Metrics) != 2 {
				t.Fatal("changed metric scope/count")
			}
			for _, metric := range ms.Metrics {
				switch metric.Name {
				case tc.scope + ".operations":
					points := metric.GetSum().DataPoints
					if len(points) != 1 || points[0].GetAsInt() != 1 || len(points[0].Attributes) != 3 {
						t.Fatal("duplicate or changed counter")
					}
					for _, attr := range points[0].Attributes {
						if want[attr.Key] != attr.Value.GetStringValue() {
							t.Fatal("changed counter categories")
						}
					}
				case tc.scope + ".duration":
					points := metric.GetHistogram().DataPoints
					if metric.Unit != "s" || len(points) != 1 || points[0].Count != 1 || len(points[0].Attributes) != 3 {
						t.Fatal("duplicate or changed duration")
					}
					for _, attr := range points[0].Attributes {
						if want[attr.Key] != attr.Value.GetStringValue() {
							t.Fatal("changed duration categories")
						}
					}
				default:
					t.Fatal("unregistered instrument")
				}
			}
			for _, raw := range packets {
				if bytes.Contains(raw, []byte("/private/root")) {
					t.Fatal("private value exported")
				}
			}
		})
	}
}
