package network

import "errors"

// ProfileHistory retains the sole profile identity and any conflicting signed
// identity for one accepted Epoch. A repeat cannot clear a retained conflict.
type ProfileHistory struct {
	generation, accepted, conflict [32]byte
	epoch                          uint64
}

func RestoreProfileHistory(generation [32]byte, epoch uint64, accepted, conflict [32]byte) (ProfileHistory, error) {
	if generation == [32]byte{} || epoch == 0 || conflict != [32]byte{} && (accepted == [32]byte{} || accepted == conflict) {
		return ProfileHistory{}, errors.New("profile history is invalid")
	}
	return ProfileHistory{generation: generation, epoch: epoch, accepted: accepted, conflict: conflict}, nil
}

// ProfileDecision must be committed before it changes accepted network facts.
// A conflicting decision preserves the first digest as well as the conflict.
type ProfileDecision struct {
	next    ProfileHistory
	changed bool
}

func (decision ProfileDecision) NeedsCommit() bool        { return decision.changed }
func (decision ProfileDecision) Conflicted() bool         { return decision.next.conflict != [32]byte{} }
func (decision ProfileDecision) AcceptedDigest() [32]byte { return decision.next.accepted }
func (decision ProfileDecision) ConflictDigest() [32]byte { return decision.next.conflict }

func (history ProfileHistory) Consider(profile ProfileBinding) (ProfileDecision, error) {
	if history.generation == [32]byte{} || profile.Generation != history.generation || profile.Epoch != history.epoch || profile.Digest == [32]byte{} {
		return ProfileDecision{}, errors.New("profile does not belong to this history")
	}
	next := history
	if next.conflict == [32]byte{} {
		if next.accepted == [32]byte{} {
			next.accepted = profile.Digest
		} else if next.accepted != profile.Digest {
			next.conflict = profile.Digest
		}
	}
	return ProfileDecision{next: next, changed: next != history}, nil
}
