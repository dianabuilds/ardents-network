package custody

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"math"
	"os"
	"path/filepath"
)

func (vault *Vault) activateRecoveredAuthority(ctx context.Context, operation Operation, secrets SecretInput) (Receipt, error) {
	if secrets == nil || operation.Reconciliation == nil || operation.Transition != nil || operation.Preparation != nil ||
		!isZeroAuthorityState(operation.Authority) || operation.Path != "" || !validRecordID(operation.RecordID) {
		return Receipt{}, ErrInvalid
	}
	entries, err := os.ReadDir(vault.records)
	if err != nil || len(entries) != 0 {
		return Receipt{}, ErrInvalid
	}
	quarantineEntries, err := os.ReadDir(vault.quarantine)
	if err != nil || len(quarantineEntries) != 1 || quarantineEntries[0].Name() != "record-"+operation.RecordID+".json" || quarantineEntries[0].IsDir() {
		return Receipt{}, ErrInvalid
	}
	raw, err := readEnvelopeFile(filepath.Join(vault.quarantine, "record-"+operation.RecordID+".json"))
	if err != nil {
		return Receipt{}, err
	}
	password, err := readPassword(ctx, secrets, SecretPromptVaultUnlock)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(password)
	purpose, plaintext, _, err := openEnvelope(raw, password)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(plaintext)
	if purpose != PurposeVault {
		return Receipt{}, ErrInvalid
	}
	state, err := decodeAuthorityState(plaintext, PurposeVault)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(state.RootMaterial)
	if state.Binding != operation.Expected || state.Binding.Kind != AuthorityName {
		return Receipt{}, ErrInvalid
	}
	private := ed25519.PrivateKey(append([]byte(nil), state.RootMaterial...))
	defer zero(private)
	public, ok := private.Public().(ed25519.PublicKey)
	if len(private) != ed25519.PrivateKeySize || !ok || sha256.Sum256(public) != state.Binding.IDCommitment {
		return Receipt{}, ErrInvalid
	}
	var authorityKey [ed25519.PublicKeySize]byte
	copy(authorityKey[:], public)
	generation, revision, err := operation.Reconciliation.Match(state.Binding.Network, authorityKey)
	if err != nil || generation <= state.Generation || revision <= state.Revision {
		return Receipt{}, ErrInvalid
	}
	successor := state
	successor.Generation, successor.Revision = generation, revision
	successor.Watermarks, err = advancedWatermarks(state.Watermarks)
	if err != nil {
		return Receipt{}, err
	}
	floors, err := vault.readFloors()
	if err != nil {
		return Receipt{}, err
	}
	if floor, found := floorFor(floors, successor.Binding); found && !strictlyHigher(successor, floor) {
		return Receipt{}, ErrInvalid
	}
	plaintextSuccessor, err := encodeAuthorityState(PurposeVault, successor)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(plaintextSuccessor)
	envelopeBytes, err := sealEnvelope(PurposeVault, plaintextSuccessor, password)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(envelopeBytes)
	recordID, err := freshRecordID()
	if err != nil {
		return Receipt{}, err
	}
	if err := vault.writeRecord(recordID, envelopeBytes); err != nil {
		return Receipt{}, err
	}
	if err := vault.advanceFloor(successor); err != nil {
		return Receipt{}, err
	}
	info, err := inspectEnvelope(envelopeBytes)
	if err != nil {
		return Receipt{}, err
	}
	return Receipt{Operation: OperationActivateRecoveredAuthority, RecordID: recordID, Envelope: info,
		Authority: authorityReceipt(successor), State: RecordActive}, nil
}

func advancedWatermarks(values []Watermark) ([]Watermark, error) {
	if !validWatermarks(values) {
		return nil, ErrInvalid
	}
	next := append([]Watermark(nil), values...)
	for index := range next {
		if next[index].Value == math.MaxUint64 {
			return nil, errors.New("authority watermark cannot advance")
		}
		next[index].Value++
	}
	return next, nil
}
