package state

import (
	"github.com/dianabuilds/ardents-network/internal/domain/network/source"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state/durable"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state/internal/distribution"
)

// CommitRetainedGenerationForTest writes one durable generation exactly as a
// historical build would have committed it, so the F-50 evidence tests can
// reconstruct a retained old-schema population that no current intake path
// would accept.
func CommitRetainedGenerationForTest(root, name string, epoch []byte, inputs [][]byte, activate bool) error {
	storage, err := openTestDurableRoot(root)
	if err != nil {
		return err
	}
	defer storage.Close()
	return storage.CommitState(durable.Generation{Name: name, Epoch: epoch, Inputs: inputs, Activate: activate})
}

// CommitRetainedControlForTest writes a distribution control floor with its
// active Epoch floor and an optional retained pending marker.
func CommitRetainedControlForTest(root string, epochFloor uint64, epochDigest, pendingDigest [32]byte, pendingValidFrom int64) error {
	storage, err := openTestDurableRoot(root)
	if err != nil {
		return err
	}
	defer storage.Close()
	state := distribution.Record{Sequence: 1, EpochFloor: epochFloor, EpochDigest: epochDigest,
		TrustedTimeFloor: pendingValidFrom, PendingDigest: pendingDigest, PendingValidFrom: pendingValidFrom}
	raw := distribution.Encode(state)
	return storage.CommitControl(distribution.Digest(raw), raw)
}

// VerifySourceCandidateForTest drives one Source bundle through the exact
// wave-intake choke point against the opened State's retained decisions.
func VerifySourceCandidateForTest(store *State, epoch []byte, inputs, materials [][]byte) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	_, err := store.verifySourceBundle(source.Bundle{Epoch: epoch, Inputs: inputs, Materials: materials},
		store.current)
	return err
}
