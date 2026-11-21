//go:build linux

package main

import (
	"encoding/json"
	"testing"
)

func TestRecordedProcessSampleRetainsOriginalFieldTypes(t *testing.T) {
	const sample = `{"rss_bytes":4194304,"cpu_pressure":0.25,"emergency_events":2,"future_field":"ignored"}`
	var record recordedProcessSample
	if err := json.Unmarshal([]byte(sample), &record); err != nil {
		t.Fatal(err)
	}
	if record.RSSBytes != 4194304 || record.CPUPressure != 0.25 || record.EmergencyEvents != 2 {
		t.Fatalf("retained sample differs: %+v", record)
	}
	for _, malformed := range []string{
		`{"rss_bytes":4194304,"cpu_pressure":"0.25"}`,
		`{"rss_bytes":4194304,"emergency_events":-1}`,
		`{"rss_bytes":4194304,"storage_files":false}`,
	} {
		if err := json.Unmarshal([]byte(malformed), &record); err == nil {
			t.Fatalf("accepted malformed known sample field: %s", malformed)
		}
	}
}
