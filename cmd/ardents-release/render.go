package main

import "github.com/dianabuilds/ardents-network/internal/releasedecision"

// floorOut is the JSON-friendly floor shape. Each role renders its
// version and hex-encoded digest. The decoder in tests and external
// tools reads the same shape.
type floorOut struct {
	RootVersion      int64  `json:"root_version"`
	RootDigest       string `json:"root_digest"`
	TimestampVersion int64  `json:"timestamp_version"`
	TimestampDigest  string `json:"timestamp_digest"`
	SnapshotVersion  int64  `json:"snapshot_version"`
	SnapshotDigest   string `json:"snapshot_digest"`
	TargetsVersion   int64  `json:"targets_version"`
	TargetsDigest    string `json:"targets_digest"`
}

// floorToJSON renders the supplied FloorSet in the JSON-friendly
// shape. The decoder in tests and external tools reads the same
// shape.
func floorToJSON(floors releasedecision.FloorSet) floorOut {
	return floorOut{
		RootVersion:      floors.RootVersion,
		RootDigest:       hexEncodeDigest(floors.RootDigest),
		TimestampVersion: floors.TimestampVersion,
		TimestampDigest:  hexEncodeDigest(floors.TimestampDigest),
		SnapshotVersion:  floors.SnapshotVersion,
		SnapshotDigest:   hexEncodeDigest(floors.SnapshotDigest),
		TargetsVersion:   floors.TargetsVersion,
		TargetsDigest:    hexEncodeDigest(floors.TargetsDigest),
	}
}

// hexEncodeDigest encodes the supplied digest bytes as lowercase
// hex. The rendered output uses the hex form so callers can compare
// it byte-for-byte against the durable floor.
func hexEncodeDigest(digest []byte) string {
	if len(digest) == 0 {
		return ""
	}
	const hexDigits = "0123456789abcdef"
	buffer := make([]byte, 0, len(digest)*2)
	for _, b := range digest {
		buffer = append(buffer, hexDigits[b>>4], hexDigits[b&0x0f])
	}
	return string(buffer)
}
