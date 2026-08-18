package blockedverify

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
)

const maximumFinalTelemetryBytes = 2 << 20

type finalRawTelemetry struct {
	Root uint16 `json:"root"`
	Role string `json:"role"`
	Kind string `json:"kind"`
	Data []byte `json:"data"`
}

type finalRawTelemetryEvidence struct {
	Schema string              `json:"schema"`
	CellID string              `json:"cell_id"`
	Files  []finalRawTelemetry `json:"files"`
}

func verifyFinalTelemetryEvidence(root string, cells []finalCellObservation) []string {
	for _, cell := range cells {
		raw, reason := loadFinalRawTelemetry(root, cell)
		if reason != "" || !validFinalRawTelemetry(raw.Files) || !validFinalTelemetryStreams(raw.Files) {
			return []string{"final cell raw telemetry is incomplete or invalid"}
		}
	}
	return nil
}

func verifyFinalTelemetryAggregates(root string, summary *finalSummary) []string {
	if summary == nil {
		return []string{"final telemetry aggregate summary is missing"}
	}
	byID := make(map[string]finalCellObservation, len(summary.Cells))
	for _, cell := range summary.Cells {
		byID[cell.ID] = cell
	}
	for _, sustained := range summary.Sustained {
		var endpoint, publisher uint64
		for run := range sustained.Runs {
			cell, ok := byID[fmt.Sprintf("sustained/%s/run-%d", sustained.Direction, run)]
			if !ok {
				return []string{"final sustained telemetry cell is missing"}
			}
			raw, reason := loadFinalRawTelemetry(root, cell)
			if reason != "" {
				return []string{"final sustained telemetry is unavailable"}
			}
			endpointDelta, endpointOK := finalCarrierDelta(raw.Files, "endpoint")
			publisherDelta, publisherOK := finalCarrierDelta(raw.Files, "publisher")
			if !endpointOK || !publisherOK || ^uint64(0)-endpoint < endpointDelta || ^uint64(0)-publisher < publisherDelta {
				return []string{"final sustained carrier telemetry is incomplete"}
			}
			endpoint += endpointDelta
			publisher += publisherDelta
		}
		if sustained.DeliveredBytes == 0 || endpoint != sustained.EndpointCarrierBytes ||
			publisher != sustained.PublisherCarrierBytes ||
			float64(endpoint)/float64(sustained.DeliveredBytes) != sustained.EndpointCarrierRatio ||
			float64(publisher)/float64(sustained.DeliveredBytes) != sustained.PublisherCarrierRatio {
			return []string{"final sustained carrier ratios do not reproduce raw telemetry"}
		}
	}
	return nil
}

func loadFinalRawTelemetry(root string, cell finalCellObservation) (finalRawTelemetryEvidence, string) {
	expected := finalTelemetryEvidencePath(cell.ID)
	artifact := cell.TelemetryEvidence
	path, safe := safeArtifactPath(root, expected)
	if !safe || artifact.Path != expected || artifact.Bytes < 1 || !isHexDigest(artifact.SHA256, 32) {
		return finalRawTelemetryEvidence{}, "commitment"
	}
	hash, size, err := hashFile(path)
	if err != nil || hash != artifact.SHA256 || size != artifact.Bytes {
		return finalRawTelemetryEvidence{}, "hash"
	}
	var raw finalRawTelemetryEvidence
	input, err := readStableFile(path)
	if err != nil || decodeCanonicalSnapshot(input, &raw) != nil ||
		raw.Schema != "ardents-h3-final-raw-telemetry-v1" || raw.CellID != cell.ID {
		return finalRawTelemetryEvidence{}, "contents"
	}
	return raw, ""
}

func finalCarrierDelta(files []finalRawTelemetry, role string) (uint64, bool) {
	for _, file := range files {
		if file.Role != role || file.Kind != "carrier.jsonl" {
			continue
		}
		samples, ok := decodeFinalCarrierStream(file.Data)
		if !ok || !validFinalTelemetryCadence(carrierOffsets(samples[1:len(samples)-1])) {
			return 0, false
		}
		return samples[len(samples)-1].Bytes - samples[0].Bytes, true
	}
	return 0, false
}

func finalTelemetryEvidencePath(cell string) string {
	digest := sha256.Sum256([]byte(cell))
	return filepath.ToSlash(filepath.Join("final-telemetry", hex.EncodeToString(digest[:])+".json"))
}

func validFinalRawTelemetry(files []finalRawTelemetry) bool {
	for _, file := range files {
		if file.Role != "endpoint" && file.Role != "bridge" && file.Role != "publisher" ||
			file.Kind != "resource.jsonl" && file.Kind != "carrier.jsonl" ||
			len(file.Data) == 0 || len(file.Data) > maximumFinalTelemetryBytes {
			return false
		}
	}
	return true
}
