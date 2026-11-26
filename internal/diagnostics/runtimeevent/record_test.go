package runtimeevent

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestJournalEnvelopeCannotSupplyEventFields(t *testing.T) {
	for _, input := range []string{
		`{"schema":"ardents-source-event-v1","kind":"source-ready","MESSAGE":"{}"}`,
		`{"kind":"source-ready","MESSAGE":"{\"schema\":\"ardents-source-event-v1\"}"}`,
		`{"schema":"ardents-node-event-v1","kind":"lifecycle","MESSAGE":"ordinary journal text"}`,
	} {
		if record, present, err := Decode([]byte(input)); err != nil || present {
			t.Fatalf("envelope declared an event: record=%+v present=%t err=%v", record, present, err)
		}
	}
	input := `{"reason":"private-outer-reason","_PID":"42","__REALTIME_TIMESTAMP":"1795701600000000","MESSAGE":"{\"schema\":\"ardents-source-event-v1\",\"kind\":\"source-ready\"}"}`
	record, present, err := Decode([]byte(input))
	if err != nil || !present || record.Owner != "source" || record.PID != "42" || record.Reason != "" || !record.At.IsZero() || record.JournalAt != time.UnixMicro(1795701600000000).UTC() {
		t.Fatalf("independent journal/event facts: %+v %t %v", record, present, err)
	}
}

func TestKnownFieldsRejectWrongTypesAndInvalidSourceReason(t *testing.T) {
	for _, schema := range []string{"ardents-node-event-v1", "ardents-source-event-v1", "ardents-headless-runtime-event-v1"} {
		for _, field := range []string{"kind", "at", "state", "carrier_profile"} {
			for _, value := range []any{42, false, nil, []any{"private-marker"}, map[string]any{"private-marker": true}} {
				raw, _ := json.Marshal(map[string]any{"schema": schema, "kind": "ignored-kind", field: value})
				if _, _, err := Decode(raw); err == nil || strings.Contains(err.Error(), "private-marker") {
					t.Fatalf("schema=%s field=%s value=%T: %v", schema, field, value, err)
				}
			}
		}
	}
	for _, input := range []string{
		`{"schema":"ardents-source-event-v1","kind":"source-ready","reason":"cleanup"}`,
		`{"schema":"ardents-source-event-v1","kind":"source-failed"}`,
		`{"schema":"ardents-source-event-v1","kind":"source-failed","reason":"private-marker"}`,
		`{"schema":"ardents-node-event-v1","kind":"lifecycle","at":"private-marker"}`,
		`{"schema":"ardents-node-event-v1","kind":"lifecycle","assignment":"private\nmarker"}`,
	} {
		if _, _, err := Decode([]byte(input)); err == nil || strings.Contains(err.Error(), "private-marker") {
			t.Fatalf("unsafe known record: %v", err)
		}
	}
}

func TestRecordedCategoriesAndDetachedNumericFacts(t *testing.T) {
	for _, reason := range []string{"background-work", "cleanup"} {
		raw := `{"schema":"ardents-source-event-v1","kind":"source-failed","reason":"` + reason + `"}`
		record, present, err := Decode([]byte(raw))
		if err != nil || !present || record.Reason != reason {
			t.Fatalf("source failure: %+v %t %v", record, present, err)
		}
	}
	record, present, err := Decode([]byte(`{"schema":"ardents-node-event-v1","kind":"resource-sample","state":"OBSERVED","resource":{"queue_items":0,"memory_bytes":128,"private-marker":1,"fds":null,"threads":"unmeasured"},"hosting":{"Observation":{"UsedBytes":0,"private-marker":9}}}`))
	if err != nil || !present || len(record.Resource) != 2 || len(record.Hosting) != 1 || record.Resource["memory_bytes"] != 128 {
		t.Fatalf("bounded numeric facts: %+v %t %v", record, present, err)
	}
	if value, present := record.Resource["queue_items"]; !present || value != 0 {
		t.Fatal("measured zero lost")
	}
	if _, present := record.Resource["fds"]; present {
		t.Fatal("null became measured zero")
	}
	kinds := Categories()
	kinds[0].Kind = "private-marker"
	resources := ResourceNames()
	resources[0] = "private-marker"
	hosting := HostingNames()
	hosting[0] = "private-marker"
	if !KnownKind(NodeSchema, "lifecycle") || ResourceNames()[0] != "rss_bytes" || HostingNames()[0] != "UsedBytes" {
		t.Fatal("caller mutated catalogue")
	}
	if len(Categories()) != 12 || KnownKind(SourceSchema, "lifecycle") || KnownFailure(NodeSchema, "private-marker") {
		t.Fatal("foreign category admitted")
	}
}

func TestUnknownAndOversizedInput(t *testing.T) {
	if _, present, err := Decode([]byte(`{"schema":"unknown","kind":42}`)); err != nil || present {
		t.Fatalf("unknown schema: %t %v", present, err)
	}
	for _, raw := range []string{"null", "{", strings.Repeat("x", (1<<20)+1)} {
		if _, _, err := Decode([]byte(raw)); err == nil {
			t.Fatal("invalid input accepted")
		}
	}
}
