//go:build live

package network_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFinalWorkerPublishesFileBackedEvidenceWithoutStreamingPayload(t *testing.T) {
	secret := t.TempDir()
	t.Setenv("ARDENTS_BLOCKED_STAGING_ROOT", t.TempDir())
	root, err := prepareFinalWorkerRoot(strings.Repeat("a", 24))
	if err != nil {
		t.Fatal(err)
	}
	cellID := "profile/C1/00"
	observers := []finalRawObserverSet{{Observers: []finalRawObserver{{Boundary: "endpoint-adapter", Role: "endpoint",
		Path: finalPathObservation{Phase: "measured", Counts: map[string]int64{}, Passed: true},
		DNS:  finalDNSObservation{BoundaryControls: map[string]finalDNSControl{}}}}}}
	payload := bytes.Repeat([]byte("raw-sample-must-not-enter-control-stream\n"), 24_000)
	telemetry := fixtureFinalRawTelemetry(cellID, payload)
	if err := writeFinalWorkerHandoff(root, cellID, observers, telemetry); err != nil {
		t.Fatal(err)
	}
	observerArtifact, telemetryArtifact, err := publishFinalWorkerHandoff(root, secret, cellID)
	if err != nil {
		t.Fatal(err)
	}
	value := finalWorkerResult{Schema: "ardents-h3-final-worker-cell-v1", CellID: cellID,
		ObserverEvidence: observerArtifact, TelemetryEvidence: telemetryArtifact}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("raw-sample")) || len(raw) > 4<<10 {
		t.Fatalf("worker control record retained raw evidence: bytes=%d", len(raw))
	}
	for _, artifact := range []finalRunnerArtifact{observerArtifact, telemetryArtifact} {
		if artifact.Path == "" || artifact.SHA256 == "" || artifact.Bytes < 1 {
			t.Fatalf("incomplete artifact commitment: %+v", artifact)
		}
		if _, err := os.Stat(filepath.Join(secret, filepath.FromSlash(artifact.Path))); err != nil {
			t.Fatalf("published artifact %s: %v", artifact.Path, err)
		}
	}
	if err := releaseFinalWorkerRoot(root); err != nil {
		t.Fatal(err)
	}
}

func TestFinalWorkerHandoffPublishesTelemetryBeyondAggregateControlBound(t *testing.T) {
	secret := t.TempDir()
	t.Setenv("ARDENTS_BLOCKED_STAGING_ROOT", t.TempDir())
	root, err := prepareFinalWorkerRoot(strings.Repeat("d", 24))
	if err != nil {
		t.Fatal(err)
	}
	payload := bytes.Repeat([]byte("x"), 300<<10)
	cellID := "pressure/P4"
	telemetry := fixtureFinalRawTelemetry(cellID, payload)
	if err := writeFinalWorkerHandoff(root, cellID, []finalRawObserverSet{{}}, telemetry); err != nil {
		t.Fatal(err)
	}
	_, artifact, err := publishFinalWorkerHandoff(root, secret, cellID)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Bytes >= maximumFinalHandoffArtifact {
		t.Fatalf("telemetry index retained aggregate payload: %d", artifact.Bytes)
	}
}

func TestFinalCampaignControlStreamFitsAllCellsWithoutRawEvidence(t *testing.T) {
	var stream bytes.Buffer
	encoder := json.NewEncoder(&stream)
	for index := range 594 {
		cellID := fmt.Sprintf("cell/%03d", index)
		artifact := finalRunnerArtifact{Path: finalRunnerArtifactPath("final-observers", cellID),
			SHA256: strings.Repeat("a", 64), Bytes: 1 << 20}
		observation := finalRunnerObservation{Schema: "ardents-h3-blocked-cell-observation-v1",
			CellID: cellID, Seed: strings.Repeat("b", 64), ObservedTerminal: "success",
			ProductStarted: true, FaultInjected: true, Attribution: "exact",
			Observers: fixtureFinalRunnerObservers(), Residuals: fixtureFinalRunnerResiduals(),
			ObserverEvidence: artifact, TelemetryEvidence: finalRunnerArtifact{
				Path: finalRunnerArtifactPath("final-telemetry", cellID), SHA256: strings.Repeat("c", 64), Bytes: 8 << 20}}
		if err := encoder.Encode(observation); err != nil {
			t.Fatal(err)
		}
	}
	if stream.Len() >= 16<<20 {
		t.Fatalf("594-cell control stream exceeds campaign bound: %d", stream.Len())
	}
}

func TestFinalWorkerHandoffRejectsUnownedAndDuplicatePublication(t *testing.T) {
	secret := t.TempDir()
	t.Setenv("ARDENTS_BLOCKED_STAGING_ROOT", t.TempDir())
	cellID := "profile/C1/00"
	write := func(token string) string {
		root, err := prepareFinalWorkerRoot(token)
		if err != nil {
			t.Fatal(err)
		}
		if err := writeFinalWorkerHandoff(root, cellID, []finalRawObserverSet{{}},
			fixtureFinalRawTelemetry(cellID, []byte("sample\n"))); err != nil {
			t.Fatal(err)
		}
		return root
	}
	first := write(strings.Repeat("a", 24))
	if _, _, err := publishFinalWorkerHandoff(first, secret, cellID); err != nil {
		t.Fatal(err)
	}
	if err := releaseFinalWorkerRoot(first); err != nil {
		t.Fatal(err)
	}
	second := write(strings.Repeat("b", 24))
	if _, _, err := publishFinalWorkerHandoff(second, secret, cellID); err == nil {
		t.Fatal("duplicate final artifact publication was accepted")
	}
	if _, _, err := publishFinalWorkerHandoff(filepath.Join(secret, "outside"), secret, cellID); err == nil {
		t.Fatal("unowned final handoff root was accepted")
	}
}

func TestFinalWorkerHandoffRejectsSymlinkedArtifact(t *testing.T) {
	secret := t.TempDir()
	t.Setenv("ARDENTS_BLOCKED_STAGING_ROOT", t.TempDir())
	root, err := prepareFinalWorkerRoot(strings.Repeat("c", 24))
	if err != nil {
		t.Fatal(err)
	}
	cellID := "profile/C1/00"
	if err := writeFinalWorkerHandoff(root, cellID, []finalRawObserverSet{{}},
		fixtureFinalRawTelemetry(cellID, []byte("sample\n"))); err != nil {
		t.Fatal(err)
	}
	observerPath := filepath.Join(root, "handoff", "observers.json")
	outside := filepath.Join(t.TempDir(), "observers.json")
	raw, err := os.ReadFile(observerPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(observerPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, observerPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, _, err := publishFinalWorkerHandoff(root, secret, cellID); err == nil {
		t.Fatal("symlinked final worker artifact was accepted")
	}
}

func fixtureFinalRawTelemetry(cell string, payload []byte) []finalRawTelemetry {
	roots := 1
	if cell == "pressure/P4" {
		roots = 10
	}
	result := make([]finalRawTelemetry, 0, roots*6)
	for root := range roots {
		for _, role := range []string{"endpoint", "bridge", "publisher"} {
			for _, kind := range []string{"resource.jsonl", "carrier.jsonl"} {
				result = append(result, finalRawTelemetry{Root: uint16(root), Role: role,
					Kind: kind, Data: payload})
			}
		}
	}
	return result
}
