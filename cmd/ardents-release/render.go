package main

import (
	"encoding/hex"

	"github.com/dianabuilds/ardents-network/internal/releasedecision"
)

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

func floorToJSON(floors releasedecision.FloorSet) floorOut {
	return floorOut{
		RootVersion:      floors.RootVersion,
		RootDigest:       hex.EncodeToString(floors.RootDigest),
		TimestampVersion: floors.TimestampVersion,
		TimestampDigest:  hex.EncodeToString(floors.TimestampDigest),
		SnapshotVersion:  floors.SnapshotVersion,
		SnapshotDigest:   hex.EncodeToString(floors.SnapshotDigest),
		TargetsVersion:   floors.TargetsVersion,
		TargetsDigest:    hex.EncodeToString(floors.TargetsDigest),
	}
}
