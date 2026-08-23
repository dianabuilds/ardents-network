package namespace

import (
	"errors"
	"sort"
	"time"
)

// EpochInstallation is one opaque Namespace-owned candidate for an Epoch
// materialization. It begins from the verified current snapshot and can add
// only selected pending successors or a verified ClaimWinner. Callers never
// construct its Record corpus directly.
type EpochInstallation struct {
	store   *Store
	epoch   Epoch
	at      time.Time
	policy  Policy
	base    string
	records map[string][]byte
	cursor  uint64
}

// BeginEpochInstallation prepares one non-current installation candidate from
// the Store's verified current snapshot. Its result becomes current only after
// Commit obtains the existing threshold attestation.
func (store *Store) BeginEpochInstallation(epoch Epoch, materializedAt time.Time,
	policy Policy,
) (*EpochInstallation, error) {
	if store == nil || store.root == nil || !validEpoch(epoch) || materializedAt.IsZero() || materializedAt.Unix() <= 0 {
		return nil, errors.New("naming Epoch installation input is invalid")
	}
	current, _, err := store.root.load()
	if err != nil {
		return nil, errors.New("naming state is tampered")
	}
	installation := &EpochInstallation{store: store, epoch: epoch, at: materializedAt, policy: policy,
		base: current, records: make(map[string][]byte)}
	if current == "" {
		return installation, nil
	}
	snapshot, err := store.load(0)
	if err != nil {
		return nil, err
	}
	installation.cursor = snapshot.pending
	for _, signed := range snapshot.records {
		record, verifyErr := VerifyRecord(store.policy.Network, signed)
		if verifyErr != nil || installation.records[record.Name] != nil {
			return nil, errors.New("naming current installation state is invalid")
		}
		installation.records[record.Name] = append([]byte(nil), signed...)
	}
	return installation, nil
}

// IncludePendingThrough selects the next immutable pending-journal prefix.
// The selected transitions remain checked again by Commit against the durable
// cursor, so a stale candidate cannot splice a Record into current state.
func (installation *EpochInstallation) IncludePendingThrough(sequence uint64) error {
	if installation == nil || installation.store == nil || sequence < installation.cursor {
		return errors.New("naming Epoch pending selection is invalid")
	}
	entries, err := installation.store.pending()
	if err != nil || sequence > uint64(len(entries)) {
		return errors.New("naming Epoch pending selection is unavailable")
	}
	for index := installation.cursor; index < sequence; index++ {
		record, verifyErr := VerifyRecord(installation.store.policy.Network, entries[index].successor)
		if verifyErr != nil {
			return errors.New("naming Epoch pending successor is invalid")
		}
		installation.records[record.Name] = append([]byte(nil), entries[index].successor...)
	}
	installation.cursor = sequence
	return nil
}

// MaterializeClaim derives exactly the verified winner's root Record and asks
// the claimant's signing port to sign its sealed transcript. A substituted
// signature is denied before it changes this installation or consumes the winner.
func (installation *EpochInstallation) MaterializeClaim(winner *ClaimWinner,
	signer RecordSigner,
) error {
	if installation == nil || installation.store == nil || winner == nil || winner.value == nil || signer == nil {
		return errors.New("naming Epoch claim installation is invalid")
	}
	if winner.value.network != installation.store.policy.Network || winner.value.epoch != installation.epoch.Number {
		return errors.New("root claim winner does not belong to this Namespace Epoch")
	}
	name := winner.value.name
	var current *Record
	if signed := installation.records[name]; signed != nil {
		decoded, err := VerifyRecord(installation.store.policy.Network, signed)
		if err != nil {
			return errors.New("naming Epoch claim predecessor is invalid")
		}
		current = &decoded
	}
	record, err := winner.prepare(current, installation.at, installation.policy)
	if err != nil {
		return err
	}
	request, err := newRecordSigningRequest(installation.store.policy.Network, record)
	if err != nil {
		return errors.New("naming Epoch claim signing request is invalid")
	}
	signature, err := signer.Sign(request)
	if err != nil {
		return errors.New("naming Epoch claim signer denied the derived Record")
	}
	signed, err := request.seal(signature)
	if err != nil {
		return errors.New("naming Epoch claim signer substituted the derived Record")
	}
	verified, err := VerifyRecord(installation.store.policy.Network, signed)
	if err != nil || !sameRecord(record, verified) {
		return errors.New("naming Epoch claim signer substituted the derived Record")
	}
	if !winner.consume() {
		return errors.New("root claim winner was already materialized")
	}
	installation.records[record.Name] = append([]byte(nil), signed...)
	return nil
}

// Commit atomically publishes this candidate through the existing
// threshold-attested Store statement.
func (installation *EpochInstallation) Commit(
	attest func([]byte) ([][32]byte, [][]byte, error),
) error {
	if installation == nil || installation.store == nil {
		return errors.New("naming Epoch installation is unavailable")
	}
	names := make([]string, 0, len(installation.records))
	for name := range installation.records {
		names = append(names, name)
	}
	sort.Strings(names)
	signed := make([][]byte, 0, len(names))
	for _, name := range names {
		signed = append(signed, append([]byte(nil), installation.records[name]...))
	}
	return installation.store.commitInstallation(installation, signed, attest)
}
