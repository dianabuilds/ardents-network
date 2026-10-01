//go:build linux

package installation

import (
	"context"
	"encoding/hex"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/enrollment"
	"github.com/dianabuilds/ardents-network/internal/release"
)

// authenticateSuccessor cannot bootstrap trust from a locally supplied bundle.
// The existing floor owner must already retain complete metadata floors at or
// above the installed binding. That binding supplies continuity constraints,
// never a serialized proof or an independently accepting authority.
func authenticateSuccessor(ctx context.Context, verifier *release.Verifier, candidate enrollment.Candidate, previous localBinding) (Authorization, error) {
	if ctx == nil || verifier == nil {
		return Authorization{}, errors.New("successor authentication context or verifier is absent")
	}
	floors, err := verifier.CurrentFloors()
	if err != nil {
		return Authorization{}, err
	}
	if floors.RootVersion <= 0 || len(floors.RootDigest) != 32 || floors.TimestampVersion <= 0 || len(floors.TimestampDigest) != 32 ||
		floors.SnapshotVersion <= 0 || len(floors.SnapshotDigest) != 32 || previous.Generation.TargetsVersion <= 0 ||
		floors.TargetsVersion < previous.Generation.TargetsVersion || len(floors.TargetsDigest) != 32 ||
		(floors.TargetsVersion == previous.Generation.TargetsVersion && hex.EncodeToString(floors.TargetsDigest) != previous.Generation.TargetsDigest) {
		return Authorization{}, errors.New("successor requires established floor-compatible Release trust")
	}
	local := candidate.Inputs.Local
	if local.Platform != previous.Generation.Platform || local.Architecture != previous.Generation.Architecture ||
		local.Environment != previous.Generation.Environment || local.Network != previous.Generation.Network {
		return Authorization{}, errors.New("successor changes the installed Release environment")
	}
	return authenticateInputs(ctx, verifier, candidate.Inputs, candidate.ProtectedDescriptor, candidate.ProtectedFiles)
}
