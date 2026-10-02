//go:build linux

package main

import (
	"bytes"
	"encoding/hex"
	metrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	traces "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"
	"testing"
)

func assertIssuanceOTLP(t *testing.T, seen map[string][]byte, p issuancePlan) {
	var tr traces.ExportTraceServiceRequest
	var mr metrics.ExportMetricsServiceRequest
	if proto.Unmarshal(seen["/v1/traces"], &tr) != nil || len(tr.ResourceSpans) != 1 {
		t.Fatal("missing trace")
	}
	span := tr.ResourceSpans[0].ScopeSpans[0].Spans[0]
	if span.Name != "issuance.initialize" || len(span.Attributes) != 3 || len(span.Events) != 0 || len(span.ParentSpanId) != 0 {
		t.Fatal("unsafe span")
	}
	for _, attr := range span.Attributes {
		want := map[string]string{"operation": "issuance.initialize", "phase": "export", "outcome": "completed"}
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
			case "ardents.issuance.operations":
				points := m.GetSum().DataPoints
				if len(points) != 1 || points[0].GetAsInt() != 1 || len(points[0].Attributes) != 3 || len(points[0].Exemplars) != 0 {
					t.Fatal("unsafe counter")
				}
			case "ardents.issuance.duration":
				points := m.GetHistogram().DataPoints
				if len(points) != 1 || points[0].Count != 1 || m.Unit != "s" || len(points[0].Attributes) != 3 || len(points[0].Exemplars) != 0 {
					t.Fatal("unsafe duration")
				}
			default:
				t.Fatal("unexpected metric")
			}
		}
	}
	for _, raw := range [][]byte{seen["/v1/traces"], seen["/v1/metrics"]} {
		if bytes.Contains(raw, []byte(p.Root)) || bytes.Contains(raw, []byte(hex.EncodeToString(p.Binding.Signer[:]))) {
			t.Fatal("metadata leaked")
		}
	}
}
