package blockedverify

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func materializeFinalMutationSource(t *testing.T, root, cell string) finalCellObservation {
	t.Helper()
	observers := make([]finalRawObserver, 0, len(finalObserverBoundaries))
	for _, boundary := range finalObserverBoundaries {
		role := strings.TrimPrefix(boundary, "ordinary-")
		if boundary == "endpoint-adapter" {
			role = "endpoint"
		}
		observers = append(observers, finalRawObserver{Boundary: boundary, Role: role,
			Path: finalRawPathObservation{Phase: "mutation-source", Passed: true, Counts: map[string]int64{}},
			DNS: finalRawDNSObservation{Controls: 6, IPv4UDPControls: 2, IPv6UDPControls: 2,
				IPv4TCPControls: 2, BoundaryControls: map[string]finalRawDNSControl{
					boundary: {IPv4UDP: 2, IPv6UDP: 2, IPv4TCP: 2, IfIndex: 1, Token: strings.Repeat("t", 32)},
				}}})
	}
	observerValue := finalRawObserverEvidence{Schema: "ardents-h3-final-raw-observers-v1", CellID: cell,
		Sets: []finalRawObserverSet{{Observers: observers}}}
	observerPath := finalObserverEvidencePath(cell)
	observer := writeFinalMutationJSON(t, root, observerPath, observerValue)

	files := fixtureFinalTelemetryIndex(cell)
	for index := range files {
		stream := telemetryLines(t, []finalResourceSample{{Schema: "ardents-h3-process-resource-v1"}})
		if files[index].Kind == "carrier.jsonl" {
			stream = telemetryLines(t, []finalCarrierSample{
				{Schema: "ardents-h3-carrier-counter-v1", Boundary: "before"},
				{Schema: "ardents-h3-carrier-counter-v1", Ordinal: 1, OffsetMillis: 1},
				{Schema: "ardents-h3-carrier-counter-v1", Ordinal: 2, OffsetMillis: 2, Boundary: "after"},
			})
		}
		streamPath := filepath.Join(root, filepath.FromSlash(files[index].Artifact.Path))
		if err := os.MkdirAll(filepath.Dir(streamPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(streamPath, stream, 0o600); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(stream)
		files[index].Artifact.SHA256 = hex.EncodeToString(digest[:])
		files[index].Artifact.Bytes = int64(len(stream))
	}
	telemetryValue := finalRawTelemetryEvidence{Schema: "ardents-h3-final-raw-telemetry-v2", CellID: cell,
		Files: files}
	telemetry := writeFinalMutationJSON(t, root, finalTelemetryEvidencePath(cell), telemetryValue)
	return finalCellObservation{ID: cell, Seed: strings.Repeat("1", 64),
		ObserverEvidence: observer, TelemetryEvidence: telemetry}
}

func writeFinalMutationJSON(t *testing.T, root, relative string, value any) artifactCommitment {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	absolute := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(absolute), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(absolute, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	return artifactCommitment{Path: relative, SHA256: hex.EncodeToString(digest[:]), Bytes: int64(len(raw))}
}
