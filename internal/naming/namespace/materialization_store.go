package namespace

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

// Open claims a naming-state root for exactly one Network Epoch policy.
func Open(path string, input MaterializationPolicy) (*Store, error) {
	policy, err := validMaterializationPolicy(input)
	if err != nil {
		return nil, err
	}
	root, err := openNamespaceRoot(path)
	if err != nil {
		return nil, err
	}
	return &Store{root: root, policy: policy}, nil
}

// Commit threshold-attests and atomically publishes one strictly newer current
// Namespace. The attester receives only the canonical statement transcript.
func (store *Store) Commit(epoch Epoch, signed [][]byte,
	attest func([]byte) ([][32]byte, [][]byte, error),
) error {
	if store == nil || store.root == nil || attest == nil || !validEpoch(epoch) {
		return errors.New("naming materialization input is invalid")
	}
	records, leaves, err := materializeRecords(store.policy.Network, signed)
	if err != nil {
		return err
	}
	pending, err := store.pendingCursorFor(records)
	if err != nil {
		return err
	}
	value := statement{network: store.policy.Network, epoch: epoch.Number, epochDigest: epoch.Digest, rule: store.policy.Rule,
		cutoff: epoch.CutoffOffset, recordRoot: recordRoot(leaves), recordLength: uint32(len(leaves)),
		transitionRoot: epoch.TransitionRoot, transitionLength: epoch.TransitionLength,
		rejectionRoot: epoch.RejectionRoot, rejectionLength: epoch.RejectionLength}
	transcript := statementTranscript(value)
	ids, signatures, err := attest(append([]byte(nil), transcript...))
	attested := attestedStatement{statement: value, signerIDs: ids, signatures: signatures}
	if err != nil || !verifyAttestation(store.policy, attested) {
		return errors.New("naming materialization threshold is invalid")
	}
	current, _, loadErr := store.root.load()
	if loadErr != nil {
		return errors.New("naming state is tampered")
	}
	if current != "" {
		previous, currentErr := store.load(0)
		if currentErr != nil {
			return currentErr
		}
		if value.epoch <= previous.attested.statement.epoch {
			return errors.New("naming state epoch is not monotonic")
		}
	}
	metadata := encodeAttested(attested)
	chunks, err := encodeRecordChunks(records)
	if err != nil {
		return err
	}
	name := snapshotGenerationDigest(metadata, chunks, pending)
	return store.root.commit(namespaceGeneration{Name: hex.EncodeToString(name[:]),
		Epoch: metadata, Inputs: chunks, Pending: pending, Activate: true})
}

// Close releases the exclusive root lease.
func (store *Store) Close() error {
	if store == nil || store.root == nil {
		return nil
	}
	return store.root.close()
}

func (store *Store) load(minimumEpoch uint64) (snapshot, error) {
	if store == nil || store.root == nil {
		return snapshot{}, errors.New("naming state store is unavailable")
	}
	current, generations, err := store.root.load()
	if err != nil || current == "" {
		return snapshot{}, errors.New("naming state is tampered")
	}
	for _, generation := range generations {
		if generation.Name != current {
			continue
		}
		digest := snapshotGenerationDigest(generation.Epoch, generation.Inputs, generation.Pending)
		attested, decodeErr := decodeAttested(generation.Epoch)
		persistedRecords, persistenceErr := decodeRecordChunks(generation.Inputs)
		records, leaves, materialErr := materializeRecords(store.policy.Network, persistedRecords)
		canonicalChunks, canonicalErr := encodeRecordChunks(records)
		if decodeErr != nil || materialErr != nil || hex.EncodeToString(digest[:]) != current ||
			!verifyAttestation(store.policy, attested) || attested.statement.recordRoot != recordRoot(leaves) ||
			attested.statement.recordLength != uint32(len(leaves)) || persistenceErr != nil || canonicalErr != nil ||
			!sameInputs(canonicalChunks, generation.Inputs) {
			return snapshot{}, errors.New("naming state is tampered")
		}
		if attested.statement.epoch < minimumEpoch {
			return snapshot{}, errors.New("naming state is stale")
		}
		return snapshot{attested: attested, records: records, leaves: leaves, pending: generation.Pending}, nil
	}
	return snapshot{}, errors.New("naming state is tampered")
}

func validMaterializationPolicy(input MaterializationPolicy) (MaterializationPolicy, error) {
	if input.Network == [32]byte{} || input.Rule != materializationRule || input.Threshold < 2 ||
		input.Threshold > len(input.Authorities) || len(input.Authorities) > 16 {
		return MaterializationPolicy{}, errors.New("naming materialization policy is invalid")
	}
	policy := MaterializationPolicy{Network: input.Network, Rule: input.Rule,
		Authorities: make(map[[32]byte]ed25519.PublicKey, len(input.Authorities)), Threshold: input.Threshold}
	for id, public := range input.Authorities {
		if len(public) != ed25519.PublicKeySize || sha256.Sum256(public) != id {
			return MaterializationPolicy{}, errors.New("naming materialization authority is invalid")
		}
		policy.Authorities[id] = append(ed25519.PublicKey(nil), public...)
	}
	return policy, nil
}

func validEpoch(value Epoch) bool {
	return value.Number > 0 && value.Digest != [32]byte{} && value.CutoffOffset >= 0 && value.TransitionRoot != [32]byte{} &&
		value.RejectionRoot != [32]byte{}
}

func snapshotDigest(metadata []byte, inputs [][]byte) [32]byte {
	return snapshotGenerationDigest(metadata, inputs, 0)
}

func snapshotGenerationDigest(metadata []byte, inputs [][]byte, pending uint64) [32]byte {
	out := append([]byte("ardents-naming-state-snapshot-v2\x00"), metadata...)
	for _, input := range inputs {
		out = appendUint32(out, uint32(len(input)))
		out = append(out, input...)
	}
	if pending != 0 {
		out = append(out, "pending-v1\x00"...)
		out = appendUint64(out, pending)
	}
	return sha256.Sum256(out)
}
