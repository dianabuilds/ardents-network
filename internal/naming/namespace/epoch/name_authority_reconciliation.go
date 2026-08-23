package epoch

import (
	"crypto/ed25519"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace/record"
)

// NameAuthorityReconciliation is an opaque witness that one verified current
// Namespace materialization contains exactly one active Name Authority.
// Callers cannot construct a witness from an arbitrary Record.
type NameAuthorityReconciliation struct {
	store      *Store
	network    [32]byte
	authority  [ed25519.PublicKeySize]byte
	generation uint64
	revision   uint64
}

// CurrentNameAuthority obtains the active current Namespace record for one
// Name Authority public key. Ambiguous, absent, inactive, or invalid records
// do not produce a reconciliation witness.
func (store *Store) CurrentNameAuthority(authority [ed25519.PublicKeySize]byte) (NameAuthorityReconciliation, error) {
	if store == nil || store.root == nil || authority == [ed25519.PublicKeySize]byte{} {
		return NameAuthorityReconciliation{}, errors.New("name Authority reconciliation source is unavailable")
	}
	records, _, err := store.CurrentRecords()
	if err != nil {
		return NameAuthorityReconciliation{}, err
	}
	var found record.Record
	for _, value := range records {
		public, keyErr := record.AuthorityKey(value.Authority)
		if keyErr != nil || [ed25519.PublicKeySize]byte(public) != authority {
			continue
		}
		if value.Lease != "active" || found.Name != "" {
			return NameAuthorityReconciliation{}, errors.New("name Authority reconciliation source is ambiguous")
		}
		found = value
	}
	if found.Name == "" || found.Generation == 0 || found.Revision == 0 {
		return NameAuthorityReconciliation{}, errors.New("name Authority reconciliation source is unavailable")
	}
	return NameAuthorityReconciliation{store: store, network: store.policy.Network, authority: authority,
		generation: found.Generation, revision: found.Revision}, nil
}

// Match verifies that this witness remains scoped to the expected Network and
// Authority and returns its authenticated current generation and revision.
func (source NameAuthorityReconciliation) Match(network [32]byte, authority [ed25519.PublicKeySize]byte) (uint64, uint64, error) {
	if source.store == nil || source.network != network || source.authority != authority || source.generation == 0 || source.revision == 0 {
		return 0, 0, errors.New("name Authority reconciliation source is invalid")
	}
	current, err := source.store.CurrentNameAuthority(authority)
	if err != nil || current.network != source.network || current.generation != source.generation || current.revision != source.revision {
		return 0, 0, errors.New("name Authority reconciliation source is stale")
	}
	return source.generation, source.revision, nil
}
