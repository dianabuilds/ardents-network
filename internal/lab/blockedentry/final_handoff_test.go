package blockedentry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFinalCellAdmitsFileBackedEvidence(t *testing.T) {
	secret := t.TempDir()
	cellID := "profile/C1/00"
	observers := finalRawObserverEvidence{Schema: "ardents-h3-final-raw-observers-v1", CellID: cellID,
		Sets: []finalRawObserverSet{{Observers: fixtureFinalRawObservers()}}}
	observerPath := finalArtifactPath("final-observers", cellID)
	if err := os.MkdirAll(filepath.Dir(filepath.Join(secret, filepath.FromSlash(observerPath))), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(secret, filepath.FromSlash(observerPath)), observers); err != nil {
		t.Fatal(err)
	}
	observerCommitment, err := commitment(secret, observerPath)
	if err != nil {
		t.Fatal(err)
	}
	telemetry := finalRawTelemetryEvidence{Schema: "ardents-h3-final-raw-telemetry-v1", CellID: cellID,
		Files: []finalRawTelemetry{{Role: "bridge", Kind: "resource.jsonl", Data: []byte("sample\n")}}}
	telemetryPath := finalArtifactPath("final-telemetry", cellID)
	if err := os.MkdirAll(filepath.Dir(filepath.Join(secret, filepath.FromSlash(telemetryPath))), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(secret, filepath.FromSlash(telemetryPath)), telemetry); err != nil {
		t.Fatal(err)
	}
	telemetryCommitment, err := commitment(secret, telemetryPath)
	if err != nil {
		t.Fatal(err)
	}

	output := cellObservation{CellID: cellID, ObserverEvidence: observerCommitment,
		TelemetryEvidence: telemetryCommitment}
	cell, err := finalCellFromOutput(secret, output)
	if err != nil || cell.ObserverEvidence != observerCommitment || cell.TelemetryEvidence != telemetryCommitment {
		t.Fatalf("file-backed final cell=%+v err=%v", cell, err)
	}
	output.TelemetryEvidence.Bytes++
	if _, err := finalCellFromOutput(secret, output); err == nil {
		t.Fatal("changed telemetry commitment was accepted")
	}
	output.TelemetryEvidence = telemetryCommitment
	output.ObserverEvidence.Path = telemetryPath
	if _, err := finalCellFromOutput(secret, output); err == nil {
		t.Fatal("substituted observer artifact path was accepted")
	}
}

func fixtureFinalRawObservers() []finalRawObserver {
	result := make([]finalRawObserver, 0, len(boundaries))
	for _, boundary := range boundaries {
		result = append(result, finalRawObserver{Boundary: boundary, Role: "bridge",
			Path: finalRawPathObservation{Phase: "measured", Counts: map[string]int64{}, Passed: true},
			DNS:  finalRawDNSObservation{BoundaryControls: map[string]finalRawDNSControl{}}})
	}
	result[0].Role = "endpoint"
	return result
}
