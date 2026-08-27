package record

import (
	"crypto/sha256"
	"errors"
)

// Binding is one immutable, locally authenticated Name destination snapshot.
// Only a successful ResolveBinding result is suitable as connection provenance.
type Binding struct {
	Name             string
	Generation       uint64
	Revision         uint64
	Authority        string
	Target           [32]byte
	ParentName       string
	ParentGeneration uint64
	RecordDigest     [32]byte
	Commitment       [32]byte
}

// ResolveBindingLegacy validates a caller-built Record lineage. It remains for
// compatibility tests and evidence; runtime resolution receives Binding only
// from a threshold-authenticated Namespace proof.
func ResolveBindingLegacy(current Record, now int64, parents []Record) (Binding, string, error) {
	return resolveBinding(current, now, parents)
}

func resolveBinding(current Record, now int64, parents []Record) (Binding, string, error) {
	if err := validateRecord(current); err != nil {
		return Binding{}, "", errors.New("name record is invalid")
	}
	if ok, reason := liveLease(current, now); !ok {
		return Binding{}, "", errors.New(reason)
	}
	if current.Target == [32]byte{} {
		return Binding{}, "", errors.New("name has no current Service Target binding")
	}
	parent, err := validateParents(current.Name, parents, now)
	if err != nil || !sameParent(&current, parent) {
		return Binding{}, "", errors.New("parent lineage is missing or stale")
	}
	digest, err := recordDigest(current)
	if err != nil {
		return Binding{}, "", err
	}
	commitmentInput := append([]byte("ardents-h3-name-destination-binding-v1\x00"), digest[:]...)
	warning := leaseWarning(current, now)
	if warning == "" {
		for _, lineageRecord := range parents {
			if warning = leaseWarning(lineageRecord, now); warning != "" {
				break
			}
		}
	}
	return Binding{
		Name: current.Name, Generation: current.Generation, Revision: current.Revision,
		Authority: current.Authority, Target: current.Target,
		ParentName: current.ParentName, ParentGeneration: current.ParentGeneration,
		RecordDigest: digest, Commitment: sha256.Sum256(commitmentInput),
	}, warning, nil
}

func recordDigest(record Record) ([32]byte, error) {
	raw, err := EncodeRecord(record)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(raw), nil
}
