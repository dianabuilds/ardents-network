package telemetry

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
)

// ExportMeasurements uses the same loopback-only bounded OTLP transport as
// command observations. It exports only validated catalogue quantities. The
// snapshot's capture start is the cumulative counter/reset boundary. No
// correlation identity, dynamic attributes, errors or owner object enters it.
func (o *Observation) ExportMeasurements(point runtimeevent.MeasurementSnapshot) bool {
	if o == nil || o.exporter == nil {
		return false
	}
	if !runtimeevent.ValidMeasurementSnapshot(point) {
		o.failure.Store(true)
		return false
	}
	scope := metricdata.ScopeMetrics{Scope: instrumentation.Scope{Name: runtimeevent.MeasurementScope}}
	add := func(name, unit string, value float64, counter bool) {
		data := metricdata.DataPoint[float64]{StartTime: point.Started, Time: point.At, Value: value}
		var aggregation metricdata.Aggregation = metricdata.Gauge[float64]{DataPoints: []metricdata.DataPoint[float64]{data}}
		if counter {
			aggregation = metricdata.Sum[float64]{DataPoints: []metricdata.DataPoint[float64]{data}, Temporality: metricdata.CumulativeTemporality, IsMonotonic: true}
		}
		scope.Metrics = append(scope.Metrics, metricdata.Metrics{Name: runtimeevent.MeasurementScope + "." + name, Unit: unit, Data: aggregation})
	}
	series, err := runtimeevent.MeasurementSeriesFor(point)
	if err != nil {
		o.failure.Store(true)
		return false
	}
	for _, value := range series {
		unit := value.Unit.OTLP()
		add(value.Name, unit, value.Value, value.Kind == runtimeevent.CounterSeries)
	}

	data := metricdata.ResourceMetrics{Resource: resource.NewSchemaless(attribute.String("service.name", "ardents-next")), ScopeMetrics: []metricdata.ScopeMetrics{scope}}
	ctx, cancel := context.WithTimeout(context.Background(), exportTimeout)
	defer cancel()
	if o.exporter.Export(ctx, &data) != nil {
		o.failure.Store(true)
		return false
	}
	return true
}
