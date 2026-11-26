package runtimeevent

import (
	"errors"
	"time"
)

type SeriesKind string

const (
	GaugeSeries   SeriesKind = "gauge"
	CounterSeries SeriesKind = "counter"
)

// MeasurementSeries is a finite derived quantity. Transport adapters share its
// name, kind and unit; no input attribute can add a dimension or rename a series.
type MeasurementSeries struct {
	Name  string
	Kind  SeriesKind
	Unit  MeasurementUnit
	Value float64
}

func MeasurementSeriesFor(point MeasurementSnapshot) ([]MeasurementSeries, error) {
	if !ValidMeasurementSnapshot(point) {
		return nil, errors.New("domain measurements unavailable")
	}
	result := []MeasurementSeries{
		{"capture_start_time_seconds", GaugeSeries, SecondsUnit, measurementUnixSeconds(point.Started)},
		{"sample_time_seconds", GaugeSeries, SecondsUnit, measurementUnixSeconds(point.At)},
		{"discarded_snapshots", CounterSeries, SnapshotsUnit, float64(point.Discarded)},
	}
	add := func(name string, kind SeriesKind, unit MeasurementUnit, value float64) {
		result = append(result, MeasurementSeries{name, kind, unit, value})
	}
	for _, v := range point.Values {
		spec, _ := LookupInstrument(v.Name).Spec()
		available := 0.0
		if v.Available {
			available = 1
		}
		add(spec.Name+".available", GaugeSeries, DimensionlessUnit, available)
		if !v.Available {
			continue
		}
		unit := spec.Unit
		if spec.Kind == StageInstrument {
			unit = OperationsUnit
		}
		add(spec.Name+".current", GaugeSeries, unit, float64(v.Current))
		add(spec.Name+".peak", GaugeSeries, unit, float64(v.Peak))
		if spec.Kind == StageInstrument {
			add(spec.Name+".calls", CounterSeries, CallsUnit, float64(v.Count))
			add(spec.Name+".seconds", CounterSeries, SecondsUnit, v.Seconds)
			add(spec.Name+".maximum_seconds", GaugeSeries, SecondsUnit, v.MaximumSeconds)
		}
	}
	return result, nil
}

func measurementUnixSeconds(at time.Time) float64 {
	return float64(at.Unix()) + float64(at.Nanosecond())/float64(time.Second)
}
