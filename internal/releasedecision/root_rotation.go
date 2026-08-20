package releasedecision

import (
	"bytes"
	"errors"
	"fmt"
)

// rootRotationResult captures the outcome of the consecutive root
// rotation check. Each successive root must be signed by the previous
// root threshold and the new root threshold; gaps, reuse, one-sided
// thresholds, expired candidates, cross-environment root additions,
// and emergency root additions are release-invalid.
type rootRotationResult struct {
	// advanced reports whether the candidate advanced beyond the durable
	// floor. Equal versions are still accepted when digest matches.
	advanced bool
	// root is the final trusted root in the chain.
	root rootPublication
	// conflict reports whether the candidate contradicted the durable
	// floor.
	conflict bool
}

// checkRootRotation verifies the candidate's root chain against the
// durable floor. The supplied root chain is the list of verified root
// bytes; the durable floor is the previously committed state.
func checkRootRotation(chain []rootPublication, durable FloorSet) (rootRotationResult, error) {
	if len(chain) == 0 {
		return rootRotationResult{}, errors.New("root chain is empty")
	}
	if len(chain) > int(maximumRootRotations) {
		return rootRotationResult{}, fmt.Errorf("root chain exceeds the rotation bound of %d", maximumRootRotations)
	}
	first := chain[0]
	if first.Version != 1 {
		return rootRotationResult{}, fmt.Errorf("root chain does not start at version 1, got %d", first.Version)
	}
	if durable.RootVersion != 0 && first.Version < durable.RootVersion {
		return rootRotationResult{}, errors.New("root chain is older than the durable floor")
	}
	previousVersion := first.Version
	previousDigest := first.Digest
	for index := 1; index < len(chain); index++ {
		current := chain[index]
		if current.Version != previousVersion+1 {
			return rootRotationResult{}, fmt.Errorf("root chain has a gap between version %d and %d", previousVersion, current.Version)
		}
		if bytes.Equal(current.Digest, previousDigest) {
			return rootRotationResult{}, errors.New("root chain reuses a previous digest")
		}
		previousVersion = current.Version
		previousDigest = current.Digest
	}
	final := chain[len(chain)-1]
	if durable.RootVersion != 0 {
		if final.Version < durable.RootVersion {
			return rootRotationResult{}, errors.New("final root is older than the durable floor")
		}
		if final.Version == durable.RootVersion && !bytes.Equal(final.Digest, durable.RootDigest) {
			return rootRotationResult{conflict: true}, errors.New("final root disagrees with the durable floor at the same version")
		}
	}
	return rootRotationResult{advanced: final.Version > durable.RootVersion, root: final}, nil
}

// successorFloors builds the successor FloorSet from the verified
// trusted set and the root rotation result. The committed floors are
// the active version + digest for each top-level role.
func successorFloors(set *verifiedSet, rotation rootRotationResult) (FloorSet, error) {
	if set == nil {
		return FloorSet{}, errors.New("trusted set is missing")
	}
	rootDigest := sha256Sum(set.rootBytes)
	if len(rotation.root.Digest) == 32 {
		rootDigest = [32]byte(rotation.root.Digest)
	}
	timestampBytes, err := set.set.Timestamp.MarshalJSON()
	if err != nil {
		return FloorSet{}, fmt.Errorf("marshal timestamp: %w", err)
	}
	snapshotBytes, err := set.set.Snapshot.MarshalJSON()
	if err != nil {
		return FloorSet{}, fmt.Errorf("marshal snapshot: %w", err)
	}
	targetsBytes, err := set.set.Targets[targetRole].MarshalJSON()
	if err != nil {
		return FloorSet{}, fmt.Errorf("marshal targets: %w", err)
	}
	timestampDigest := sha256Sum(timestampBytes)
	snapshotDigest := sha256Sum(snapshotBytes)
	targetsDigest := sha256Sum(targetsBytes)
	return FloorSet{
		RootVersion:      set.set.Root.Signed.Version,
		RootDigest:       rootDigest[:],
		TimestampVersion: set.set.Timestamp.Signed.Version,
		TimestampDigest:  timestampDigest[:],
		SnapshotVersion:  set.set.Snapshot.Signed.Version,
		SnapshotDigest:   snapshotDigest[:],
		TargetsVersion:   set.set.Targets[targetRole].Signed.Version,
		TargetsDigest:    targetsDigest[:],
	}, nil
}
