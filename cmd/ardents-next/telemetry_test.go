package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"

	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	metrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	traces "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"
)

func commandFixture(t *testing.T) []string {
	t.Helper()
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, 32))
	values := map[string]string{"network": hex.EncodeToString(bytes.Repeat([]byte{1}, 32)), "issuer": hex.EncodeToString(bytes.Repeat([]byte{2}, 32)), "holder": hex.EncodeToString(bytes.Repeat([]byte{3}, 32)), "authority": hex.EncodeToString(key.Public().(ed25519.PublicKey)), "duty": "1", "class": "2", "count": "10", "now": "1970-01-01T01:00:00Z", "duty_not_before": "1970-01-01T01:00:00Z", "duty_not_after": "1970-01-01T02:00:00Z"}
	raw := make([]byte, 228)
	for i := 0; i < 32; i++ {
		raw[i] = 1
		raw[32+i] = 2
		raw[72+i] = 4
		raw[104+i] = 3
	}
	binary.BigEndian.PutUint64(raw[64:72], 1)
	binary.BigEndian.PutUint64(raw[136:144], 3600)
	binary.BigEndian.PutUint64(raw[144:152], 7200)
	binary.BigEndian.PutUint32(raw[156:160], 10)
	copy(raw[164:], ed25519.Sign(key, append([]byte("ardents-issuance-permission-v1\x00"), raw[:164]...)))
	config, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "permission")
	f := filepath.Join(dir, "facts")
	if err := os.WriteFile(p, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f, config, 0600); err != nil {
		t.Fatal(err)
	}
	return []string{"inspect-permission", p, f}
}

func TestCommandOTLPContainsOnlyDeclaredObservations(t *testing.T) {
	var mu sync.Mutex
	seen := map[string][]byte{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 32769))
		if err != nil {
			t.Error(err)
		}
		mu.Lock()
		seen[r.URL.Path] = body
		mu.Unlock()
		if r.Header.Get("Authorization") != "" {
			t.Error("ambient headers leaked")
		}
		w.Header().Set("Content-Type", "application/x-protobuf")
	}))
	defer server.Close()
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "Authorization=private-header")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http/json")
	t.Setenv("OTEL_TRACES_SAMPLER", "always_off")
	args := append(commandFixture(t), server.URL+"/")
	var out, diagnostic bytes.Buffer
	if code := run(context.Background(), args, &out, &diagnostic); code != 0 {
		t.Fatalf("exit %d: %s", code, out.String())
	}
	mu.Lock()
	defer mu.Unlock()
	var traceRequest traces.ExportTraceServiceRequest
	if proto.Unmarshal(seen["/v1/traces"], &traceRequest) != nil || len(traceRequest.ResourceSpans) != 1 {
		t.Fatal("missing trace export")
	}
	spans := traceRequest.ResourceSpans[0].ScopeSpans[0].Spans
	if len(spans) != 1 || spans[0].Name != "permission.inspect" || len(spans[0].Attributes) != 1 || spans[0].Attributes[0].Key != "outcome" {
		t.Fatal("unexpected span data")
	}
	if len(spans[0].Events) != 0 || len(spans[0].Links) != 0 || len(spans[0].ParentSpanId) != 0 {
		t.Fatal("unexpected correlation")
	}
	if spans[0].Attributes[0].Value.GetStringValue() != "accepted-offline" {
		t.Fatal("incorrect trace outcome")
	}
	resource := traceRequest.ResourceSpans[0].Resource
	if len(resource.Attributes) != 1 || resource.Attributes[0].Key != "service.name" || resource.Attributes[0].Value.GetStringValue() != "ardents-next" {
		t.Fatal("unexpected trace resource data")
	}
	var metricRequest metrics.ExportMetricsServiceRequest
	if proto.Unmarshal(seen["/v1/metrics"], &metricRequest) != nil || len(metricRequest.ResourceMetrics) != 1 {
		t.Fatal("missing metric export")
	}
	if len(metricRequest.ResourceMetrics[0].ScopeMetrics[0].Metrics) != 2 {
		t.Fatal("missing instruments")
	}
	metricResource := metricRequest.ResourceMetrics[0].Resource
	if !proto.Equal(resource, metricResource) {
		t.Fatal("unexpected metric resource data")
	}
	for _, instrument := range metricRequest.ResourceMetrics[0].ScopeMetrics[0].Metrics {
		switch instrument.Name {
		case "ardents.permission.inspections":
			points := instrument.GetSum().GetDataPoints()
			if len(points) != 1 || points[0].GetAsInt() != 1 || len(points[0].Attributes) != 1 || points[0].Attributes[0].Key != "outcome" || points[0].Attributes[0].Value.GetStringValue() != "accepted-offline" || len(points[0].Exemplars) != 0 {
				t.Fatal("incorrect inspection counter")
			}
		case "ardents.permission.inspection.duration":
			points := instrument.GetHistogram().GetDataPoints()
			if instrument.Unit != "s" || len(points) != 1 || points[0].Count != 1 || points[0].GetSum() < 0 || len(points[0].Attributes) != 1 || points[0].Attributes[0].Key != "outcome" || points[0].Attributes[0].Value.GetStringValue() != "accepted-offline" || len(points[0].Exemplars) != 0 {
				t.Fatal("incorrect inspection histogram")
			}
		default:
			t.Fatalf("unexpected metric %s", instrument.Name)
		}
	}
	for _, body := range seen {
		for _, private := range []string{args[1], args[2], "private-header"} {
			if bytes.Contains(body, []byte(private)) {
				t.Fatal("private data exported")
			}
		}
	}
	if !bytes.Contains(diagnostic.Bytes(), []byte(`"telemetry":"completed"`)) {
		t.Fatal(diagnostic.String())
	}
}

func TestCollectorFailureDoesNotChangeOutcome(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	defer close(release)
	args := append(commandFixture(t), server.URL)
	var out, diagnostic bytes.Buffer
	start := time.Now()
	if code := run(context.Background(), args, &out, &diagnostic); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("unbounded export")
	}
	if !bytes.Contains(diagnostic.Bytes(), []byte(`"telemetry":"unavailable"`)) {
		t.Fatal(diagnostic.String())
	}
}

func TestCanceledCommandAndCollectorValidation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, diagnostic bytes.Buffer
	if code := run(ctx, commandFixture(t), &out, &diagnostic); code != 130 {
		t.Fatalf("exit %d", code)
	}
	for _, endpoint := range []string{"http://127.0.0.1:0", "http://127.0.0.1:65536", "http://example.com:4318", "http://localhost:4318", "http://127.0.0.1:4318/?token=secret", "http://user:pass@127.0.0.1:4318", "https://127.0.0.1:4318"} {
		if collectorEndpoint(endpoint) == nil {
			t.Fatalf("accepted endpoint %s", endpoint)
		}
	}
}

func TestCollectorResponseIsBounded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-protobuf")
		_, _ = w.Write(bytes.Repeat([]byte("private-response"), 1000))
	}))
	defer server.Close()
	var out, diagnostic bytes.Buffer
	if code := run(context.Background(), append(commandFixture(t), server.URL), &out, &diagnostic); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if bytes.Contains(diagnostic.Bytes(), []byte("private-response")) || !bytes.Contains(diagnostic.Bytes(), []byte(`"telemetry":"unavailable"`)) {
		t.Fatal("unsafe exporter diagnosis")
	}
}
