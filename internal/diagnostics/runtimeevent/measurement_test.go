package runtimeevent

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestInstrumentCatalogueHasUniqueCompleteOwnerQuantities(t *testing.T) {
	names := map[string]bool{}
	owners := map[string]bool{}
	for _, id := range Instruments() {
		s, ok := id.Spec()
		if !ok || s.Name == "" || s.Unit == "" || s.Owner == "" || s.Kind != StageInstrument && s.Kind != GaugeInstrument && s.Kind != LastSampleInstrument || names[s.Name] || LookupInstrument(s.Name) != id {
			t.Fatal("invalid instrument", id, s)
		}
		names[s.Name] = true
		owners[s.Owner] = true
	}
	if len(owners) != 13 {
		t.Fatal("domain coverage", owners)
	}
	if _, ok := Instrument(255).Spec(); ok {
		t.Fatal("unregistered instrument admitted")
	}
}
func TestMeasurementsShareStrictSafeProjectionAndMetricSemantics(t *testing.T) {
	now := time.Now().UTC()
	point := MeasurementSnapshot{Schema: MeasurementSchema, Delivery: ExportLocalOnly, At: now, Started: now.Add(-time.Second), Values: []Measurement{{Name: "route.queue_budget_claim_bytes", Available: true, Current: 3, Peak: 9}}}
	// The public catalogue owns the spelling; derive this fixture from it.
	spec, _ := RouteReservedBytes.Spec()
	point.Values[0].Name = spec.Name
	raw, _ := json.Marshal(point)
	record, present, err := Decode(raw)
	if err != nil || !present || record.Measurements == nil || record.Measurements.Values[0].Current != 3 {
		t.Fatal(present, err)
	}
	series, err := MeasurementSeriesFor(point)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, value := range series {
		if value.Name == spec.Name+".current" {
			found = value.Kind == GaugeSeries && value.Unit == BytesUnit && value.Value == 3
		}
	}
	if !found {
		t.Fatal("shared series lost gauge type, unit or value", series)
	}
	for _, bad := range [][]byte{
		bytes.Replace(raw, []byte(`"available":true`), []byte(`"Available":false,"available":true`), 1),
		bytes.Replace(raw, []byte(`"current":3`), []byte(`"current": null `), 1),
		bytes.Replace(raw, []byte(`"current":3`), []byte(`"current":null`), 1),
		bytes.Replace(raw, []byte(`"current":3`), []byte(`"current":-1`), 1),
		bytes.Replace(raw, []byte(`"current":3`), []byte(`"current":3,"current":0`), 1),
		bytes.Replace(raw, []byte(spec.Name), []byte("private-sentinel"), 1),
		bytes.Replace(raw, []byte(`"peak":9`), []byte(`"peak":1`), 1),
	} {
		_, _, err := Decode(bad)
		if err == nil || strings.Contains(err.Error(), "private-sentinel") {
			t.Fatal("invalid record admitted or echoed", string(bad), err)
		}
	}
}
