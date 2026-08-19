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

func TestFinalTelemetryRejectsUnknownOrEmptyFiles(t *testing.T) {
	cell := "profile/C1/00"
	valid := fixtureFinalTelemetryIndex(cell)
	if !validFinalRawTelemetry(valid, cell) {
		t.Fatal("known bounded telemetry was rejected")
	}
	valid = valid[:5]
	if validFinalRawTelemetry(valid, cell) {
		t.Fatal("empty telemetry was accepted")
	}
}

func TestFinalTelemetryP4RequiresAllOrderedStreams(t *testing.T) {
	cell := "pressure/P4"
	valid := fixtureFinalTelemetryIndex(cell)
	if len(valid) != 60 || !validFinalRawTelemetry(valid, cell) {
		t.Fatal("complete P4 telemetry inventory was rejected")
	}
	valid[7], valid[8] = valid[8], valid[7]
	if validFinalRawTelemetry(valid, cell) {
		t.Fatal("reordered P4 telemetry inventory was accepted")
	}
}

func TestLoadFinalTelemetryV2RejectsChangedStream(t *testing.T) {
	root := t.TempDir()
	cell := finalCellObservation{ID: "profile/C1/00"}
	files := fixtureFinalTelemetryIndex(cell.ID)
	for index := range files {
		path := filepath.Join(root, filepath.FromSlash(files[index].Artifact.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		raw := []byte("sample\n")
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(raw)
		files[index].Artifact.SHA256 = hex.EncodeToString(digest[:])
		files[index].Artifact.Bytes = int64(len(raw))
	}
	index := finalRawTelemetryEvidence{Schema: "ardents-h3-final-raw-telemetry-v2", CellID: cell.ID, Files: files}
	raw, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	indexPath := filepath.Join(root, filepath.FromSlash(finalTelemetryEvidencePath(cell.ID)))
	if err := os.WriteFile(indexPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	cell.TelemetryEvidence = artifactCommitment{Path: finalTelemetryEvidencePath(cell.ID),
		SHA256: hex.EncodeToString(digest[:]), Bytes: int64(len(raw))}
	if _, reason := loadFinalRawTelemetry(root, cell); reason != "" {
		t.Fatalf("valid telemetry v2 reason=%s", reason)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(files[0].Artifact.Path)), []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, reason := loadFinalRawTelemetry(root, cell); reason == "" {
		t.Fatal("changed telemetry stream was accepted")
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

func fixtureFinalTelemetryIndex(cell string) []finalRawTelemetry {
	roots := 1
	if cell == "pressure/P4" {
		roots = 10
	}
	result := make([]finalRawTelemetry, 0, roots*6)
	for root := range roots {
		for _, role := range []string{"endpoint", "bridge", "publisher"} {
			for _, kind := range []string{"resource.jsonl", "carrier.jsonl"} {
				index := len(result)
				result = append(result, finalRawTelemetry{Root: uint16(root), Role: role, Kind: kind,
					Artifact: artifactCommitment{Path: finalTelemetryStreamPath(cell, index),
						SHA256: strings.Repeat("a", 64), Bytes: 7}})
			}
		}
	}
	return result
}
