package blockedverify

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
)

const maximumFinalTelemetryBytes = 2 << 20

type finalRawTelemetry struct {
	Root     uint16             `json:"root"`
	Role     string             `json:"role"`
	Kind     string             `json:"kind"`
	Artifact artifactCommitment `json:"artifact"`
	Data     []byte             `json:"-"`
}

type finalRawTelemetryEvidence struct {
	Schema string              `json:"schema"`
	CellID string              `json:"cell_id"`
	Files  []finalRawTelemetry `json:"files"`
}

func verifyFinalTelemetryEvidence(root string, cells []finalCellObservation) []string {
	for _, cell := range cells {
		raw, reason := loadFinalRawTelemetry(root, cell)
		if reason != "" || !validFinalRawTelemetry(raw.Files, cell.ID) || !validFinalTelemetryStreams(raw.Files) {
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
			observation := sustained.Runs[run]
			if len(observation.WindowEndsMillis) != 10 {
				return []string{"final sustained telemetry window schedule is incomplete"}
			}
			windowFinished := observation.WindowEndsMillis[len(observation.WindowEndsMillis)-1]
			cell, ok := byID[fmt.Sprintf("sustained/%s/run-%d", sustained.Direction, run)]
			if !ok {
				return []string{"final sustained telemetry cell is missing"}
			}
			raw, reason := loadFinalRawTelemetry(root, cell)
			if reason != "" {
				return []string{"final sustained telemetry is unavailable"}
			}
			if !reproducesFinalRoleResources(raw.Files, observation.Resources,
				observation.StartedOffsetMillis, windowFinished) {
				return []string{"final sustained resource telemetry does not reproduce its aggregate"}
			}
			endpointDelta, endpointOK := finalCarrierDelta(raw.Files, "endpoint",
				observation.StartedOffsetMillis, windowFinished)
			publisherDelta, publisherOK := finalCarrierDelta(raw.Files, "publisher",
				observation.StartedOffsetMillis, windowFinished)
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
		raw.Schema != "ardents-h3-final-raw-telemetry-v2" || raw.CellID != cell.ID ||
		!validFinalRawTelemetry(raw.Files, cell.ID) {
		return finalRawTelemetryEvidence{}, "contents"
	}
	for index := range raw.Files {
		stream, streamReason := loadFinalTelemetryStream(root, cell.ID, index, raw.Files[index].Artifact)
		if streamReason != "" {
			return finalRawTelemetryEvidence{}, streamReason
		}
		raw.Files[index].Data = stream
	}
	return raw, ""
}

func finalCarrierDelta(files []finalRawTelemetry, role string, started, finished uint64) (uint64, bool) {
	for _, file := range files {
		if file.Role != role || file.Kind != "carrier.jsonl" {
			continue
		}
		samples, ok := decodeFinalCarrierStream(file.Data)
		if !ok || !completeFinalSustainedCarrier(samples, started, finished) {
			return 0, false
		}
		return samples[len(samples)-1].Bytes - samples[0].Bytes, true
	}
	return 0, false
}

func completeFinalSustainedCarrier(samples []finalCarrierSample, started, finished uint64) bool {
	if len(samples) < 602 || started >= finished || finished-started < 10*60*1_000 ||
		samples[0].OffsetMillis > started || samples[len(samples)-1].OffsetMillis < finished {
		return false
	}
	active := 0
	for index := 1; index < len(samples)-1; index++ {
		gap := samples[index].OffsetMillis - samples[index-1].OffsetMillis
		if gap < 750 || gap > 1_250 {
			return false
		}
		if samples[index].OffsetMillis > started && samples[index].OffsetMillis <= finished {
			active++
		}
	}
	last, prior := samples[len(samples)-1], samples[len(samples)-2]
	return active >= 600 && last.OffsetMillis >= prior.OffsetMillis && last.OffsetMillis-prior.OffsetMillis <= 1_250
}

func finalTelemetryEvidencePath(cell string) string {
	digest := sha256.Sum256([]byte(cell))
	return filepath.ToSlash(filepath.Join("final-telemetry", hex.EncodeToString(digest[:])+".json"))
}

func validFinalRawTelemetry(files []finalRawTelemetry, cell string) bool {
	roots := 1
	if cell == "pressure/P4" {
		roots = 10
	}
	if len(files) != roots*6 {
		return false
	}
	for index, file := range files {
		root := index / 6
		role := []string{"endpoint", "bridge", "publisher"}[(index%6)/2]
		kind := []string{"resource.jsonl", "carrier.jsonl"}[index%2]
		if file.Root != uint16(root) || file.Role != role || file.Kind != kind ||
			file.Artifact.Path != finalTelemetryStreamPath(cell, index) || file.Artifact.Bytes < 1 ||
			file.Artifact.Bytes > maximumFinalTelemetryBytes || !isHexDigest(file.Artifact.SHA256, 32) {
			return false
		}
	}
	return true
}

func loadFinalTelemetryStream(root, cell string, index int, artifact artifactCommitment) ([]byte, string) {
	expected := finalTelemetryStreamPath(cell, index)
	path, safe := safeArtifactPath(root, expected)
	if !safe || artifact.Path != expected || artifact.Bytes < 1 ||
		artifact.Bytes > maximumFinalTelemetryBytes || !isHexDigest(artifact.SHA256, 32) {
		return nil, "commitment"
	}
	raw, err := readStableFile(path)
	if err != nil || len(raw) > maximumFinalTelemetryBytes || int64(len(raw)) != artifact.Bytes {
		return nil, "contents"
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != artifact.SHA256 {
		return nil, "hash"
	}
	return raw, ""
}

func finalTelemetryStreamPath(cell string, index int) string {
	digest := sha256.Sum256([]byte(cell))
	return filepath.ToSlash(filepath.Join("final-telemetry", hex.EncodeToString(digest[:]),
		fmt.Sprintf("%03d.jsonl", index)))
}
