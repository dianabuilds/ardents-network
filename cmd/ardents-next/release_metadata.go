package main

import (
	"bytes"
	"context"
	"crypto/sha256"

	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

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
