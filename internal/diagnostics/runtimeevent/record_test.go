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
	if KnownKind(SourceSchema, "lifecycle") || KnownFailure(NodeSchema, "private-marker") {
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

func TestCurrentCommandReportTransitionAndPrivacyBoundary(t *testing.T) {
	for _, tc := range []struct{ operation, phase, outcome, owner string }{
		{"node.forwarding", "listening", "original-duty", "node"},
		{"node.join", "retiring", "completion-unavailable", "node"},
		{"node.issuer", "joined", "failed", "node"},
		{"node.descriptor", "joined", "canceled", "node"},
		{"node.introduction", "refused", "startup-unavailable", "node"},
		{"source.serve", "listening", "original-source", "source"},
		{"source.serve", "completion", "failed", "source"},
		{"source.serve", "repair", "failed", "source"},
		{"source", "refused", "unsupported-platform", "source"},
	} {
		raw, _ := json.Marshal(map[string]any{"operation": tc.operation, "phase": tc.phase, "outcome": tc.outcome,
			"at": "2030-01-01T00:00:00Z", "first_failure": "PRIVATE", "first_owner": "PRIVATE", "cleanup_owner": "PRIVATE",
			"resource": map[string]any{"fds": 77}, "request_digest": "PRIVATE", "address": "PRIVATE"})
		record, present, err := Decode(raw)
		if err != nil || !present || record.Schema != CommandReportSchema || record.Kind != tc.operation || record.Owner != tc.owner || record.Phase != tc.phase || record.Outcome != tc.outcome {
			t.Fatalf("current command: %+v %t %v", record, present, err)
		}
		projection, _ := json.Marshal(record)
		if strings.Contains(string(projection), "PRIVATE") || !record.At.IsZero() || len(record.Resource) != 0 || record.Reason != "" || record.Role != "" {
			t.Fatal("private report detail or invented measurement retained")
		}
	}
}

func TestCommandReportsRejectUnknownPairsWithoutEcho(t *testing.T) {
	for _, raw := range []string{
		`{"operation":"node.forwarding","phase":"listening","outcome":"canceled"}`,
		`{"operation":"node.join","phase":"joined","outcome":"original-duty"}`,
		`{"operation":"source.serve","phase":"retiring","outcome":"canceled"}`,
		`{"operation":"source","phase":"listening","outcome":"original-source"}`,
		`{"operation":"node.issuer","phase":"PRIVATE","outcome":"failed"}`,
		`{"operation":"source.serve","phase":"joined","outcome":"PRIVATE"}`,
		`{"operation":"source.serve","phase":null,"outcome":"failed"}`,
		`{"operation":"source.serve","phase":"joined"}`,
		`{"operation":"source.serve","phase":"joined","outcome":7}`,
		`{"operation":"node.join","phase":"joined","phase":"listening","outcome":"original-duty"}`,
		`{"operation":"source.serve","phase":"joined","outcome":"failed","outcome":"canceled"}`,
	} {
		record, present, err := Decode([]byte(raw))
		if err == nil || present || strings.Contains(err.Error(), "PRIVATE") || record.Kind != "" {
			t.Fatalf("invalid command retained: %+v %t %v", record, present, err)
		}
	}
	for _, raw := range []string{
		`{"operation":"PRIVATE","phase":"joined","outcome":"failed"}`,
		`{"schema":"foreign","operation":"node.join","phase":"joined","outcome":"failed"}`,
		`{"schema":null,"operation":"node.join","phase":"joined","outcome":"failed"}`,
		`{"operation":"node.join","phase":"joined","outcome":"failed","MESSAGE":"{}"}`,
	} {
		if _, present, err := Decode([]byte(raw)); err != nil || present {
			t.Fatalf("foreign format admitted: %t %v", present, err)
		}
	}
}

func TestExistingOTLPReportsShareOneLocalCategoryAuthority(t *testing.T) {
	for _, raw := range []string{
		`{"operation":"issuance.issue","phase":"debit","outcome":"invalid-signature","response":"PRIVATE"}`,
		`{"Operation":"hosting.observe","Phase":"execute","Outcome":"completed","Telemetry":"unavailable"}`,
		`{"operation":"admission.holder","phase":"closed","exit_code":1,"details":"PRIVATE"}`,
		`{"operation":"admission.receiver","phase":"closed","exit_code":130}`,
	} {
		record, present, err := Decode([]byte(raw))
		if err != nil || !present || record.Kind == "" || record.Outcome == "" {
			t.Fatalf("current observation filtered: %+v %t %v", record, present, err)
		}
		if strings.Contains(record.Reason, "PRIVATE") {
			t.Fatal("private receipt retained")
		}
	}
	for _, raw := range []string{
		`{"Operation":"hosting.observe","Phase":"execute","Outcome":"PRIVATE"}`,
		`{"Operation":"hosting.observe","phase":"execute","Outcome":"completed"}`,
		`{"operation":"admission.holder","phase":"closed","exit_code":99}`,
		`{"operation":"admission.holder","phase":"closed","exit_code":1,"outcome":"completed"}`,
		`{"operation":"admission.receiver","phase":"closed","exit_code":null}`,
	} {
		if _, present, err := Decode([]byte(raw)); err == nil || present {
			t.Fatalf("invalid current report accepted: %t %v", present, err)
		}
	}
}

func TestCommandCatalogueHasUniqueCompatibleProjectionKinds(t *testing.T) {
	seen := map[Category]bool{}
	for _, category := range Categories() {
		if seen[category] || !KnownKind(category.Schema, category.Kind) {
			t.Fatalf("ambiguous catalogue entry: %+v", category)
		}
		seen[category] = true
	}
	if _, ok := ClassifyCommand("custody.verify-record", "command", "completed"); !ok {
		t.Fatal("offline local observation absent")
	}
	if LookupOperation("custody.verify-record") != 0 || LookupOperation("endpoint.start-installed") != 0 {
		t.Fatal("local observations implicitly enabled OTLP export")
	}
}

func TestObservedSuccessCannotBeRelabelledAsInputOrCleanup(t *testing.T) {
	for _, tc := range []struct{ operation, phase, outcome string }{
		{"admission.initialize", "input", "initialized-offline"},
		{"hosting.observe", "close", "completed"},
		{"issuance.issue", "debit", "issued-offline"},
		{"issuance.initialize-profile", "input", "profile-initialized"},
	} {
		if _, ok := ClassifyCommand(tc.operation, tc.phase, tc.outcome); ok {
			t.Fatalf("impossible success admitted: %+v", tc)
		}
	}
	for _, operation := range []string{"admission.initialize", "hosting.initialize", "hosting.observe"} {
		if _, ok := ClassifyCommand(operation, "input", "invalid-input"); !ok {
			t.Fatal("real early refusal became corrupt observation", operation)
		}
	}
}

func TestExportDeliveryIsFiniteAndIndependentOfCommandSuccess(t *testing.T) {
	for _, delivery := range []string{"local-only", "completed", "unavailable"} {
		record, present, err := Decode([]byte(`{"operation":"issuance.inspect","phase":"export","outcome":"completed","telemetry":"` + delivery + `"}`))
		if err != nil || !present || record.Outcome != "completed" || record.Delivery != delivery {
			t.Fatal("export result erased command result", record, err)
		}
	}
	for _, value := range []string{`null`, `"PRIVATE"`, `42`} {
		if _, present, err := Decode([]byte(`{"operation":"issuance.inspect","phase":"export","outcome":"completed","telemetry":` + value + `}`)); err == nil || present {
			t.Fatal("invalid export result retained")
		}
	}
}
