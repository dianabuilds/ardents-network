//go:build linux

package main

import (
	"encoding/json"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/endpoint"
)

func TestRecordedHostingSampleRetainsJournalFieldTypes(t *testing.T) {
	const sample = `{"At":"2026-01-01T00:00:00Z","Policy":{"provider":"example","quantity":8,"directions":"tx+rx","interfaces":["eth0"]},"Boot":"boot","Interfaces":[{"Name":"eth0","Index":2,"Tx":17,"Rx":19}],"Observation":{"UsedBytes":36,"Protect":true}}`
	var record recordedHostingSample
	if err := json.Unmarshal([]byte(sample), &record); err != nil {
		t.Fatal(err)
	}
	if record.At.IsZero() || record.Policy.Quantity != 8 || record.Boot != "boot" ||
		len(record.Interfaces) != 1 || record.Interfaces[0].Tx != 17 ||
		record.Observation.UsedBytes != 36 || !record.Observation.Protect {
		t.Fatalf("retained Hosting record differs: %+v", record)
	}
	for _, invalid := range []string{
		`{"Policy":{"low_watermark_bytes":"8"}}`,
		`{"Interfaces":[{"Index":-1}]}`,
		`{"Observation":{"Protect":0}}`,
	} {
		if err := json.Unmarshal([]byte(invalid), &record); err == nil {
			t.Fatalf("accepted malformed known Hosting field: %s", invalid)
		}
	}
}

func TestResourceMeasurementsRetainIndependentHostingSnapshot(t *testing.T) {
	const input = `{"Kind":"resource-sample","Elapsed":1000000000,"Usage":{"rss_bytes":1024,"cpu_usage_usec":11},"Host":{"Policy":{"interfaces":["eth0"]},"Interfaces":[{"Name":"eth0","Index":2,"Tx":17}],"Observation":{"UsedBytes":17}}}`
	var event endpoint.StreamQualificationEvent
	if err := json.Unmarshal([]byte(input), &event); err != nil {
		t.Fatal(err)
	}
	var series resourceMeasurements
	series.observe(event)
	event.Host.Policy.Interfaces[0] = "changed"
	event.Host.Interfaces[0].Tx = 0
	if len(series.samples) != 1 {
		t.Fatalf("retained samples = %d", len(series.samples))
	}
	sample := series.samples[0]
	if sample.memory != 1024 || sample.cpu != 11 || sample.host.Policy.Interfaces[0] != "eth0" ||
		sample.host.Interfaces[0].Tx != 17 || sample.host.Observation.UsedBytes != 17 {
		t.Fatalf("retained sample changed: %+v", sample)
	}
}
