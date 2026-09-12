package reachability

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"
)

const (
	storeMarkerName = ".ardents-reachability-store-v1"
	storeMarker     = "ardents-reachability-store-v1\n"
	storeLockName   = ".ardents-reachability-store-lock"
	storeRecords    = "records"
	maximumTargets  = 128
)

// StoreConfig owns one Gateway-local Target generation/conflict root.
type StoreConfig struct {
	Root      string
	NetworkID [32]byte
}

// Store owns one exclusive durable Target floor. It is deliberately only the
// Gateway's currentness state: OHTTP, State selection, and Relay forwarding
// are separate adapters.
type Store struct {
	network [32]byte
	path    string
	lease   storeLease

	mu      sync.Mutex
	closed  bool
	failure error
	records map[[32]byte]storedDescriptor
}

// StoreClass is the closed result of descriptor publication or lookup.
type StoreClass string

const (
	StoreAccepted       StoreClass = "accepted"
	StoreAlreadyCurrent StoreClass = "already-current"
	StoreStale          StoreClass = "stale"
	StoreConflicting    StoreClass = "conflicting"
	StoreInvalid        StoreClass = "invalid"
)

// StoreResult reveals only the exact Target and bounded result class.
type StoreResult struct {
	Class  StoreClass
	Target [32]byte
}

type storedDescriptor struct {
	raw                 []byte
	verified            Verified
	digest              [32]byte
	conflicting         bool
	revisionConflicting bool
}

// OpenStore reconstructs one Gateway's accepted generation/conflict state and
// holds an exclusive root lease until Close.
func OpenStore(config StoreConfig) (*Store, error) {
	if config.Root == "" || config.NetworkID == [32]byte{} {
		return nil, errors.New("reachability store configuration is incomplete")
	}
	path, err := filepath.Abs(config.Root)
	if err != nil {
		return nil, fmt.Errorf("resolve reachability store root: %w", err)
	}
	if err := prepareStoreRoot(path); err != nil {
		return nil, err
	}
	lease, err := acquireStoreLease(path)
	if err != nil {
		return nil, err
	}
	if err := initializeStoreRoot(path); err != nil {
		return nil, errors.Join(err, lease.release())
	}
	store := &Store{path: path, network: config.NetworkID, lease: lease, records: make(map[[32]byte]storedDescriptor)}
	if err := store.restore(); err != nil {
		_ = store.lease.release()
		return nil, err
	}
	return store, nil
}

// Close releases the root lease but retains every accepted floor and conflict.
func (store *Store) Close() error {
	if store == nil {
		return nil
	}
	store.mu.Lock()
	if store.closed {
		store.mu.Unlock()
		return nil
	}
	store.closed, store.records = true, nil
	store.mu.Unlock()
	return store.lease.release()
}

// Publish accepts a fresh exact descriptor only when it cannot roll a Target
// backward. A differing publication at one generation creates a persistent
// explicit conflict; a later, non-overlapping generation is the only repair.
func (store *Store) Publish(raw []byte, at time.Time) (StoreResult, error) {
	if store == nil || at.IsZero() {
		return StoreResult{Class: StoreInvalid}, errors.New("reachability store publication input is incomplete")
	}
	candidate, err := verifyStored(raw, store.network, at)
	if err != nil {
		return StoreResult{Class: StoreInvalid}, errors.New("reachability store descriptor is invalid")
	}
	return store.publishVerified(candidate)
}

func (store *Store) publishVerified(candidate storedDescriptor) (StoreResult, error) {
	target := candidate.verified.Descriptor.Target
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.failure != nil {
		return StoreResult{Class: StoreInvalid, Target: target}, errors.Join(errors.New("reachability store is unavailable"), store.failure)
	}
	prior, exists := store.records[target]
	if !exists {
		if len(store.records) >= maximumTargets {
			return StoreResult{Class: StoreInvalid, Target: target}, errors.New("reachability store Target capacity is exhausted")
		}
		if err := store.write(candidate); err != nil {
			store.failure = err
			return StoreResult{Class: StoreInvalid, Target: target}, err
		}
		store.records[target] = candidate
		return StoreResult{Class: StoreAccepted, Target: target}, nil
	}
	result, next, err := compareStored(prior, candidate)
	if next == nil {
		return StoreResult{Class: result, Target: target}, err
	}
	if err := store.write(*next); err != nil {
		store.failure = err
		return StoreResult{Class: StoreInvalid, Target: target}, err
	}
	store.records[target] = *next
	if err != nil {
		return StoreResult{Class: result, Target: target}, err
	}
	return StoreResult{Class: result, Target: target}, nil
}

// Lookup returns one exact currently verifiable descriptor. Expiry, absence,
// and conflict become classified failures; no alternate Target is considered.
func (store *Store) Lookup(target [32]byte, at time.Time) ([]byte, StoreClass, error) {
	return store.lookup(target, [32]byte{}, at)
}

func (store *Store) lookup(target, profile [32]byte, at time.Time) ([]byte, StoreClass, error) {
	if store == nil || target == [32]byte{} || at.IsZero() {
		return nil, StoreInvalid, errors.New("reachability store lookup input is incomplete")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.failure != nil {
		return nil, StoreInvalid, errors.Join(errors.New("reachability store is unavailable"), store.failure)
	}
	record, exists := store.records[target]
	if !exists || record.conflicting || record.revisionConflicting {
		class := StoreStale
		if exists {
			class = StoreConflicting
		}
		return nil, class, errors.New("reachability descriptor is unavailable")
	}
	var verifyErr error
	if profile == [32]byte{} {
		_, verifyErr = Verify(record.raw, target, store.network, at)
	} else {
		_, verifyErr = VerifyPrivate(record.raw, target, store.network, profile, at)
	}
	if verifyErr != nil {
		return nil, StoreStale, errors.New("reachability descriptor is unavailable")
	}
	return append([]byte(nil), record.raw...), StoreAlreadyCurrent, nil
}

func verifyStored(raw []byte, network [32]byte, at time.Time) (storedDescriptor, error) {
	descriptor, _, err := decode(raw)
	if err != nil {
		return storedDescriptor{}, err
	}
	verified, err := Verify(raw, descriptor.Target, network, at)
	if err != nil {
		return storedDescriptor{}, err
	}
	return storedDescriptor{raw: append([]byte(nil), raw...), verified: verified, digest: sha256.Sum256(raw)}, nil
}

func compareStored(prior, candidate storedDescriptor) (StoreClass, *storedDescriptor, error) {
	oldCredential, newCredential := prior.verified.Current.Credential, candidate.verified.Current.Credential
	if candidate.verified.Descriptor.Target != prior.verified.Descriptor.Target {
		return StoreInvalid, nil, errors.New("reachability store compared different Targets")
	}
	private := candidate.verified.Descriptor.Version == privateDescriptorVersion
	if private != (prior.verified.Descriptor.Version == privateDescriptorVersion) {
		return StoreInvalid, nil, errors.New("reachability format change requires floor adoption")
	}
	if !private && candidate.verified.Descriptor.Introduction.Epoch == 0 {
		return StoreInvalid, nil, errors.New("reachability descriptor lacks State epoch")
	}
	if newCredential.Generation < oldCredential.Generation {
		return StoreStale, nil, errors.New("reachability descriptor generation is stale")
	}
	if newCredential.Generation > oldCredential.Generation {
		if newCredential.NotBefore < oldCredential.NotAfter {
			return StoreInvalid, nil, errors.New("reachability Credential generations overlap")
		}
		return StoreAccepted, &candidate, nil
	}
	if prior.conflicting {
		// Even a terminal conflict must retain a later observed expiry. Keep
		// its complete signed proof so reopening reconstructs the same floor.
		if newCredential.NotAfter > oldCredential.NotAfter {
			candidate.conflicting = true
			return StoreStale, &candidate, errors.New("reachability descriptor generation is stale")
		}
		return StoreStale, nil, errors.New("reachability descriptor generation is stale")
	}
	if candidate.verified.Current.Digest != prior.verified.Current.Digest {
		if newCredential.NotAfter > oldCredential.NotAfter {
			prior = candidate
		}
		prior.conflicting = true
		return StoreConflicting, &prior, errors.New("reachability publication generation conflicts")
	}
	if private {
		return comparePrivateRevision(prior, candidate)
	}
	if candidate.digest == prior.digest {
		return StoreAlreadyCurrent, nil, nil
	}
	if candidate.verified.Descriptor.Introduction.NotAfter.Unix() <= prior.verified.Descriptor.Introduction.NotAfter.Unix() {
		return StoreStale, nil, errors.New("reachability live slot is stale")
	}
	return StoreAccepted, &candidate, nil
}
