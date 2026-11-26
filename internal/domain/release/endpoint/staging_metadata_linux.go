package endpoint

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"strconv"

	"github.com/dianabuilds/ardents-network/internal/domain/release"
)

// VerifyStagingMetadata requires the exact retained metadata before the
// initial generation is staged. It never advances the Release floors.
func VerifyStagingMetadata(ctx context.Context, verifier *release.Verifier, input release.Inputs) error {
	floors, err := verifier.CurrentFloors(ctx)
	if err != nil || !floors.Complete() {
		return errors.Join(release.ErrTrustUnavailable, err)
	}
	versioned := func(version int64, role string) []byte {
		return input.Files[metadataRoot+strconv.FormatInt(version, 10)+"."+role+".json"]
	}
	for _, pair := range []struct{ body, digest []byte }{
		{input.RootBytes, floors.RootDigest},
		{input.Files[metadataRoot+"timestamp.json"], floors.TimestampDigest},
		{versioned(floors.SnapshotVersion, "snapshot"), floors.SnapshotDigest},
		{versioned(floors.TargetsVersion, "targets"), floors.TargetsDigest},
	} {
		sum := sha256.Sum256(pair.body)
		if len(pair.body) == 0 || !bytes.Equal(sum[:], pair.digest) {
			return release.ErrTrustUnavailable
		}
	}
	return ctx.Err()
}
