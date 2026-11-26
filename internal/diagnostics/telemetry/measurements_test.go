package telemetry

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"

	metrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"google.golang.org/protobuf/proto"
)

func TestDomainMeasurementsReachBoundedCollectorWithoutPrivateLabels(t *testing.T) {
	var mu sync.Mutex
	var packets [][]byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/metrics" {
			t.Errorf("unexpected export %s", r.URL.Path)
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBytes+1))
		if err != nil {
			t.Error(err)
		}
		mu.Lock()
		packets = append(packets, body)
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	exporter, err := Open(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	point := runtimeevent.MeasurementSnapshot{Schema: runtimeevent.MeasurementSchema, Delivery: runtimeevent.ExportLocalOnly, At: now, Started: now.Add(-time.Second), Values: []runtimeevent.Measurement{}}
	for _, id := range runtimeevent.Instruments() {
		spec, _ := id.Spec()
		value := runtimeevent.Measurement{Name: spec.Name, Available: true, Current: 1, Peak: 2}
		if spec.Kind == runtimeevent.StageInstrument {
			value.Count = 7
			value.Seconds = 9
			value.MaximumSeconds = 4
		}
		point.Values = append(point.Values, value)
	}
	if !exporter.ExportMeasurements(point) {
		t.Fatal("complete finite catalogue did not fit bounded export")
	}
	forged := point
	forged.Values = []runtimeevent.Measurement{{Name: "private-sentinel", Available: true}}
	if exporter.ExportMeasurements(forged) {
		t.Fatal("foreign instrument admitted")
	}
	if exporter.Close() {
		t.Fatal("invalid export lost unavailable status")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(packets) != 1 || len(packets[0]) > maxRequestBytes || bytes.Contains(packets[0], []byte("private-sentinel")) {
		t.Fatal("packet bounds or validation", len(packets))
	}
	var request metrics.ExportMetricsServiceRequest
	if err := proto.Unmarshal(packets[0], &request); err != nil {
		t.Fatal(err)
	}
	var sawCounter, sawGauge bool
	for _, resource := range request.ResourceMetrics {
		for _, scope := range resource.ScopeMetrics {
			if scope.Scope.Name != "ardents.domain" {
				t.Fatal(scope.Scope)
			}
			for _, metric := range scope.Metrics {
				if !strings.HasPrefix(metric.Name, "ardents.domain.") {
					t.Fatal(metric.Name)
				}
				if sum := metric.GetSum(); sum != nil {
					sawCounter = true
					if !sum.IsMonotonic {
						t.Fatal("non-monotonic cumulative counter")
					}
					for _, p := range sum.DataPoints {
						if len(p.Attributes) != 0 || len(p.Exemplars) != 0 || p.StartTimeUnixNano != uint64(point.Started.UnixNano()) {
							t.Fatal("counter identity/reset metadata")
						}
					}
				}
				if gauge := metric.GetGauge(); gauge != nil {
					sawGauge = true
					for _, p := range gauge.DataPoints {
						if len(p.Attributes) != 0 || len(p.Exemplars) != 0 {
							t.Fatal("private dimension or exemplar")
						}
					}
				}
			}
		}
	}
	if !sawCounter || !sawGauge {
		t.Fatal("quantity types missing")
	}
}
