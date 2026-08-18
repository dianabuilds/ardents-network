package blockedverify

import (
	"encoding/json"
	"testing"
)

func TestFinalTelemetryRejectsUnknownOrEmptyFiles(t *testing.T) {
	valid := []finalRawTelemetry{{Root: 0, Role: "bridge", Kind: "carrier.jsonl", Data: []byte("sample\n")}}
	if !validFinalRawTelemetry(valid) {
		t.Fatal("known bounded telemetry was rejected")
	}
	valid[0].Data = nil
	if validFinalRawTelemetry(valid) {
		t.Fatal("empty telemetry was accepted")
	}
}

func TestFinalTelemetryRejectsMissingOrCoalescedStreamRecords(t *testing.T) {
	resource := []finalResourceSample{{Schema: "ardents-h3-process-resource-v1", Ordinal: 0},
		{Schema: "ardents-h3-process-resource-v1", Ordinal: 1, OffsetMillis: 1_000}}
	carrier := []finalCarrierSample{{Schema: "ardents-h3-carrier-counter-v1", Ordinal: 0, Boundary: "before"},
		{Schema: "ardents-h3-carrier-counter-v1", Ordinal: 1, OffsetMillis: 1_000, Bytes: 10},
		{Schema: "ardents-h3-carrier-counter-v1", Ordinal: 2, OffsetMillis: 2_000, Bytes: 20, Boundary: "after"}}
	files := []finalRawTelemetry{{Role: "bridge", Kind: "resource.jsonl", Data: telemetryLines(t, resource)},
		{Role: "bridge", Kind: "carrier.jsonl", Data: telemetryLines(t, carrier)}}
	if !validFinalTelemetryStreams(files) {
		t.Fatal("valid raw telemetry streams were rejected")
	}
	resource[1].OffsetMillis = 10
	files[0].Data = telemetryLines(t, resource)
	if validFinalTelemetryStreams(files) {
		t.Fatal("coalesced resource stream was accepted")
	}
}

func TestFinalCarrierDeltaUsesBeforeAndAfterCounters(t *testing.T) {
	carrier := []finalCarrierSample{{Schema: "ardents-h3-carrier-counter-v1", Ordinal: 0, Bytes: 7, Boundary: "before"},
		{Schema: "ardents-h3-carrier-counter-v1", Ordinal: 1, OffsetMillis: 1_000, Bytes: 11},
		{Schema: "ardents-h3-carrier-counter-v1", Ordinal: 2, OffsetMillis: 2_000, Bytes: 23, Boundary: "after"}}
	delta, ok := finalCarrierDelta([]finalRawTelemetry{{Role: "endpoint", Kind: "carrier.jsonl",
		Data: telemetryLines(t, carrier)}}, "endpoint")
	if !ok || delta != 16 {
		t.Fatalf("delta=%d ok=%t", delta, ok)
	}
}

func telemetryLines[T any](t *testing.T, values []T) []byte {
	t.Helper()
	var result []byte
	for _, value := range values {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, raw...)
		result = append(result, '\n')
	}
	return result
}
