package main

import (
	"bytes"
	"context"
	"crypto/sha256"

	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// A detached first inspection can initialize an empty Release store;
// a retained or interrupted store must not advance without a live transition.
func initialInspectionMetadata(ctx context.Context, v *release.Verifier, input release.Inputs) error {
	floors, err := v.CurrentFloors(ctx)
	if err != nil {
		return err
	}
	if floors.RootVersion == 0 && floors.TimestampVersion == 0 && floors.SnapshotVersion == 0 && floors.TargetsVersion == 0 &&
		len(floors.RootDigest) == 0 && len(floors.TimestampDigest) == 0 && len(floors.SnapshotDigest) == 0 && len(floors.TargetsDigest) == 0 {
		return nil
	}
	return sameReleaseMetadata(ctx, v, input)
}

// Detached inspection may only reverify the exact already retained metadata.
// The verifier still authenticates signatures and target bytes independently.
func sameReleaseMetadata(ctx context.Context, v *release.Verifier, input release.Inputs) error {
	floors, err := v.CurrentFloors(ctx)
	if err != nil {
		return err
	}
	root := sha256.Sum256(input.RootBytes)
	if !floors.Complete() || !bytes.Equal(root[:], floors.RootDigest) {
		return release.ErrTrustUnavailable
	}
	expected := [][]byte{floors.TimestampDigest, floors.SnapshotDigest, floors.TargetsDigest}
	seen := [3]bool{}
	for _, raw := range input.Files {
		digest := sha256.Sum256(raw)
		matched := false
		for i, retained := range expected {
			if bytes.Equal(digest[:], retained) {
				seen[i], matched = true, true
			}
		}
		if !matched {
			return release.ErrTrustUnavailable
		}
	}
	for _, present := range seen {
		if !present {
			return release.ErrTrustUnavailable
		}
	}
	return nil
}
