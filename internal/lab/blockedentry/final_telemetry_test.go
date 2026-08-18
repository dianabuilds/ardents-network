package blockedentry

import "testing"

func TestFinalTelemetryRequiresBoundedKnownFiles(t *testing.T) {
	valid := []finalRawTelemetry{{Root: 0, Role: "bridge", Kind: "resource.jsonl", Data: []byte("sample\n")}}
	if !validFinalRawTelemetry(valid) {
		t.Fatal("known bounded telemetry was rejected")
	}
	valid[0].Kind = "unknown"
	if validFinalRawTelemetry(valid) {
		t.Fatal("unknown telemetry kind was accepted")
	}
}
