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
	layout := finalTelemetryLayout(cell)
	result := make([]finalRawTelemetry, 0, len(layout))
	for index, slot := range layout {
		result = append(result, finalRawTelemetry{Root: slot.root, Role: slot.role, Kind: slot.kind,
			Artifact: artifactCommitment{Path: finalTelemetryStreamPath(cell, index),
				SHA256: "f0a1c55a617b39c8c209aaad1cba3c8e11b4ba87d2d57ca15382641d1a62d4f7", Bytes: 7}})
	}
	return result
}

func TestFinalTelemetryCapacityInventoryRetainsEachEndpoint(t *testing.T) {
	for cell, count := range map[string]int{
		"capacity/h3-s5-b1-v1/0":        19,
		"capacity/h3-s5-b1-v1-strong/0": 55,
	} {
		if files := fixtureFinalTelemetryIndex(cell); len(files) != count || !validFinalRawTelemetry(files, cell) {
			t.Fatalf("%s inventory count=%d valid=%t", cell, len(files), validFinalRawTelemetry(files, cell))
		}
	}
}
