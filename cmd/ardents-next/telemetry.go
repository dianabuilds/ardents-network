package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/exemplar"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
)

// Telemetry is command composition, not part of admission authority.
type observation struct {
	traces    *sdktrace.TracerProvider
	meters    *sdkmetric.MeterProvider
	reader    *sdkmetric.ManualReader
	exporter  *otlpmetrichttp.Exporter
	transport *http.Transport
	failure   atomic.Bool
}

func collectorEndpoint(value string) error {
	if value == "" {
		return nil
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "http" || u.User != nil || u.ForceQuery || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return errors.New("invalid collector")
	}
	ip := net.ParseIP(u.Hostname())
	port, portError := strconv.ParseUint(u.Port(), 10, 16)
	if ip == nil || !ip.IsLoopback() || portError != nil || port == 0 || u.RawPath != "" {
		return errors.New("invalid collector")
	}
	return nil
}

func newObservation(endpoint string) (*observation, error) {
	o := &observation{}
	r := resource.NewSchemaless(attribute.String("service.name", "ardents-next"))
	options := []sdktrace.TracerProviderOption{sdktrace.WithResource(r), sdktrace.WithSampler(sdktrace.AlwaysSample()), sdktrace.WithoutPanicRecording()}
	if endpoint != "" {
		endpoint = strings.TrimSuffix(endpoint, "/")
		o.transport = &http.Transport{Proxy: nil, DisableKeepAlives: true, MaxConnsPerHost: 1, MaxResponseHeaderBytes: 4096, ResponseHeaderTimeout: 250 * time.Millisecond}
		client := &http.Client{Transport: o.transport, Timeout: 250 * time.Millisecond, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		e, err := otlptracehttp.New(context.Background(), otlptracehttp.WithEndpointURL(endpoint+"/v1/traces"), otlptracehttp.WithHTTPClient(client), otlptracehttp.WithEncoding(otlptracehttp.EncodingProtobuf), otlptracehttp.WithMaxRequestSize(32768), otlptracehttp.WithMaxResponseSize(4096), otlptracehttp.WithHeaders(map[string]string{}), otlptracehttp.WithCompression(otlptracehttp.NoCompression), otlptracehttp.WithRetry(otlptracehttp.RetryConfig{Enabled: false}), otlptracehttp.WithTimeout(250*time.Millisecond))
		if err != nil {
			return nil, errors.New("telemetry unavailable")
		}
		options = append(options, sdktrace.WithBatcher(&quietTraceExporter{SpanExporter: e, failure: &o.failure}, sdktrace.WithMaxQueueSize(1), sdktrace.WithMaxExportBatchSize(1), sdktrace.WithExportTimeout(250*time.Millisecond)))
		m, err := otlpmetrichttp.New(context.Background(), otlpmetrichttp.WithEndpointURL(endpoint+"/v1/metrics"), otlpmetrichttp.WithHTTPClient(client), otlpmetrichttp.WithMaxRequestSize(32768), otlpmetrichttp.WithMaxResponseSize(4096), otlpmetrichttp.WithHeaders(map[string]string{}), otlpmetrichttp.WithCompression(otlpmetrichttp.NoCompression), otlpmetrichttp.WithRetry(otlpmetrichttp.RetryConfig{Enabled: false}), otlpmetrichttp.WithTimeout(250*time.Millisecond))
		if err != nil {
			_ = e.Shutdown(context.Background())
			return nil, errors.New("telemetry unavailable")
		}
		o.exporter = m
	}
	o.traces = sdktrace.NewTracerProvider(options...)
	o.reader = sdkmetric.NewManualReader()
	o.meters = sdkmetric.NewMeterProvider(sdkmetric.WithResource(r), sdkmetric.WithReader(o.reader), sdkmetric.WithExemplarFilter(exemplar.AlwaysOffFilter), sdkmetric.WithCardinalityLimit(16))
	return o, nil
}

type quietTraceExporter struct {
	sdktrace.SpanExporter
	failure *atomic.Bool
}

func (e *quietTraceExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	if err := e.SpanExporter.ExportSpans(ctx, spans); err != nil {
		e.failure.Store(true)
	}
	// Raw exporter errors can contain endpoint and response data. Retain only
	// a finite failure indication instead of forwarding to the global handler.
	return nil
}

func (o *observation) inspect(ctx context.Context, raw []byte, f admission.Facts) admission.Outcome {
	if o == nil {
		return admission.Inspect(ctx, raw, f)
	}
	// No external parent context or propagation is accepted.
	spanContext, span := o.traces.Tracer("ardents.permission.inspection").Start(context.Background(), "permission.inspect")
	start := time.Now()
	result := admission.Inspect(ctx, raw, f)
	attr := attribute.String("outcome", string(result))
	span.SetAttributes(attr)
	span.End()
	meter := o.meters.Meter("ardents.permission.inspection")
	count, err := meter.Int64Counter("ardents.permission.inspections")
	if err == nil {
		count.Add(spanContext, 1, metric.WithAttributes(attr))
	}
	duration, err := meter.Float64Histogram("ardents.permission.inspection.duration", metric.WithUnit("s"))
	if err == nil {
		duration.Record(spanContext, time.Since(start).Seconds(), metric.WithAttributes(attr))
	}
	return result
}

func (o *observation) close() bool {
	if o == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()
	if o.exporter != nil {
		var data metricdata.ResourceMetrics
		if o.reader.Collect(ctx, &data) != nil || o.exporter.Export(ctx, &data) != nil {
			o.failure.Store(true)
		}
		if o.exporter.Shutdown(ctx) != nil {
			o.failure.Store(true)
		}
	}
	if o.traces.Shutdown(ctx) != nil {
		o.failure.Store(true)
	}
	if o.meters.Shutdown(ctx) != nil {
		o.failure.Store(true)
	}
	if o.transport != nil {
		o.transport.CloseIdleConnections()
	}
	return !o.failure.Load()
}
