package blockedentry

import "testing"

func TestFinalTelemetryRequiresBoundedKnownFiles(t *testing.T) {
	cell := "profile/C1/00"
	valid := fixtureFinalTelemetryIndex(cell)
	if !validFinalRawTelemetry(valid, cell) {
		t.Fatal("known bounded telemetry was rejected")
	}
	valid[0].Kind = "unknown"
	if validFinalRawTelemetry(valid, cell) {
		t.Fatal("unknown telemetry kind was accepted")
	}
	if validFinalRawTelemetry(nil, cell) {
		t.Fatal("empty telemetry inventory was accepted")
	}
}

func fixtureFinalTelemetryIndex(cell string) []finalRawTelemetry {
	result := make([]finalRawTelemetry, 0, 6)
	for _, role := range []string{"endpoint", "bridge", "publisher"} {
		for _, kind := range []string{"resource.jsonl", "carrier.jsonl"} {
			index := len(result)
			result = append(result, finalRawTelemetry{Role: role, Kind: kind, Artifact: artifactCommitment{
				Path:   finalTelemetryStreamPath(cell, index),
				SHA256: "f0a1c55a617b39c8c209aaad1cba3c8e11b4ba87d2d57ca15382641d1a62d4f7", Bytes: 7}})
		}
	}
	return result
}
