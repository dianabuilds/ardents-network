package reachability

import (
	"crypto/sha256"
	"errors"
	"time"
)

// PublishPrivate verifies an exact generation-3 proof against the receiving
// duty's current authenticated profile before updating the existing durable
// publication and revision floors. It cannot authorize a State duty or slot.
func (store *Store) PublishPrivate(raw []byte, profile [32]byte, at time.Time) (StoreResult, error) {
	if store == nil || at.IsZero() || profile == [32]byte{} {
		return StoreResult{Class: StoreInvalid}, errors.New("private reachability publication input is invalid")
	}
	value, err := decodePrivateDescriptor(raw)
	if err != nil {
		return StoreResult{Class: StoreInvalid}, err
	}
	verified, err := VerifyPrivate(raw, value.Target, store.network, profile, at)
	if err != nil {
		return StoreResult{Class: StoreInvalid}, err
	}
	return store.publishVerified(storedDescriptor{raw: append([]byte(nil), raw...), verified: verified, digest: sha256.Sum256(raw)})
}

// LookupPrivate returns only the highest unconflicted live proof under the
// caller's current authenticated profile. Legacy lookup never exposes it.
func (store *Store) LookupPrivate(target, profile [32]byte, at time.Time) ([]byte, StoreClass, error) {
	if profile == [32]byte{} {
		return nil, StoreInvalid, errors.New("private reachability lookup profile is missing")
	}
	return store.lookup(target, profile, at)
}

func comparePrivateRevision(prior, candidate storedDescriptor) (StoreClass, *storedDescriptor, error) {
	oldRevision, newRevision := prior.verified.Descriptor.Private.Revision, candidate.verified.Descriptor.Private.Revision
	if newRevision < oldRevision {
		return StoreStale, nil, errors.New("private reachability revision is stale")
	}
	if newRevision > oldRevision {
		return StoreAccepted, &candidate, nil
	}
	if prior.revisionConflicting {
		return StoreConflicting, nil, errors.New("private reachability revision remains conflicting")
	}
	if candidate.digest == prior.digest {
		return StoreAlreadyCurrent, nil, nil
	}
	prior.revisionConflicting = true
	return StoreConflicting, &prior, errors.New("private reachability revision conflicts")
}
