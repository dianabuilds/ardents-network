package runtimeevent

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"
)

// MaximumMeasurementInteger is the largest integer represented exactly by
// the numeric readers and both metric transports.
const MaximumMeasurementInteger = 1 << 53

// MeasurementHistoryLimit bounds both capture retention and report history.
const MeasurementHistoryLimit = 32

// MeasurementSnapshot is one bounded sample from an explicitly selected local
// capture. Started is the counter/reset boundary; At is sampling time, never
// the time of a domain effect. Discarded reports overwritten history samples.
type MeasurementSnapshot struct {
	Delivery  string        `json:"telemetry"`
	Schema    string        `json:"schema"`
	At        time.Time     `json:"at"`
	Started   time.Time     `json:"started_at"`
	Discarded uint64        `json:"discarded_snapshots"`
	Values    []Measurement `json:"measurements"`
}

func ValidMeasurement(value Measurement) bool {
	spec, ok := LookupInstrument(value.Name).Spec()
	if !ok {
		return false
	}
	if value.Current > MaximumMeasurementInteger || value.Peak > MaximumMeasurementInteger || value.Count > MaximumMeasurementInteger {
		return false
	}
	if !finiteNonnegative(value.Seconds) || !finiteNonnegative(value.MaximumSeconds) {
		return false
	}

	if !value.Available {
		return true
	}
	if value.Peak < value.Current {
		return false
	}
	if spec.Kind == GaugeInstrument || spec.Kind == LastSampleInstrument {
		return value.Count == 0 && value.Seconds == 0 && value.MaximumSeconds == 0
	}
	return value.MaximumSeconds <= value.Seconds && (value.Count != 0 || value.Seconds == 0 && value.MaximumSeconds == 0)
}
func ValidMeasurementSnapshot(point MeasurementSnapshot) bool {
	if !KnownExportDelivery(point.Delivery) || point.Schema != MeasurementSchema || point.At.IsZero() || point.Started.IsZero() || point.At.Before(point.Started) || point.Discarded > MaximumMeasurementInteger || len(point.Values) > len(instrumentSpecs)-1 {
		return false
	}
	seen := make(map[string]bool, len(point.Values))
	for _, value := range point.Values {
		if seen[value.Name] || !ValidMeasurement(value) {
			return false
		}
		seen[value.Name] = true
	}
	return true
}
func decodeMeasurements(raw []byte, fields map[string]json.RawMessage, record Record) (Record, bool, error) {
	invalid := errors.New("domain measurements contain invalid fields")
	if !uniqueCommandFields(raw) {
		return Record{}, false, invalid
	}
	var point MeasurementSnapshot
	if json.Unmarshal(raw, &point) != nil {
		return Record{}, false, invalid
	}
	// Null and omitted scalar fields are not numeric zero or measured absence.
	if !canonicalMeasurementFields(fields, []string{"schema", "at", "started_at", "telemetry", "discarded_snapshots", "measurements"}) {
		return Record{}, false, invalid
	}

	var rows []json.RawMessage
	if json.Unmarshal(fields["measurements"], &rows) != nil {
		return Record{}, false, invalid
	}
	for _, row := range rows {
		if !uniqueCommandFields(row) {
			return Record{}, false, invalid
		}
		var properties map[string]json.RawMessage
		if json.Unmarshal(row, &properties) != nil {
			return Record{}, false, invalid
		}
		if !canonicalMeasurementFields(properties, []string{"name", "available", "current", "peak", "count", "seconds", "maximum_seconds"}) {
			return Record{}, false, invalid
		}
	}
	point.At = point.At.UTC()
	point.Started = point.Started.UTC()
	if !ValidMeasurementSnapshot(point) {
		return Record{}, false, invalid
	}
	record.Schema, record.Kind, record.At = MeasurementSchema, MeasurementKind, point.At
	record.Measurements = &point
	return record, true, nil
}

func finiteNonnegative(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func canonicalMeasurementFields(fields map[string]json.RawMessage, names []string) bool {
	for _, name := range names {
		raw, exists := fields[name]
		if !exists || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return false
		}
	}
	for key := range fields {
		for _, name := range names {
			if key != name && strings.EqualFold(key, name) {
				return false
			}
		}
	}
	return true
}
