package nameresolution

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/namestore"
)

func newRecordSet(store *namestore.Store, network [32]byte, minimumEpoch uint64) (recordSet, error) {
	if store == nil || network == [32]byte{} || minimumEpoch == 0 {
		return recordSet{}, errors.New("private resolution Record store is invalid")
	}
	return recordSet{network: network, store: store, minimumEpoch: minimumEpoch}, nil
}

func validMaterializationPolicy(value namestore.Policy, network [32]byte) bool {
	if value.Network != network || value.Rule != "ardents-namespace-materialization-v1" || value.Threshold < 2 ||
		value.Threshold > len(value.Authorities) || len(value.Authorities) > 16 {
		return false
	}
	for id, public := range value.Authorities {
		if len(public) != ed25519.PublicKeySize || sha256.Sum256(public) != id {
			return false
		}
	}
	return true
}

func cloneMaterializationPolicy(value namestore.Policy) namestore.Policy {
	copyValue := namestore.Policy{Network: value.Network, Rule: value.Rule,
		Authorities: make(map[[32]byte]ed25519.PublicKey, len(value.Authorities)), Threshold: value.Threshold}
	for id, public := range value.Authorities {
		copyValue.Authorities[id] = append(ed25519.PublicKey(nil), public...)
	}
	return copyValue
}

func (records recordSet) lookup(name string) ([]byte, bool) {
	proof, err := records.store.Lookup(name, records.minimumEpoch)
	return proof, err == nil
}
