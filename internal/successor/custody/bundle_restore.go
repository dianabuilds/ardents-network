package custody

import (
	"context"
	"crypto/subtle"
	"fmt"
)

func (vault *Vault) restoreBundle(ctx context.Context, operation Operation, secrets SecretInput) (Receipt, error) {
	if secrets == nil || operation.RecordID != "" || !isZeroAuthorityState(operation.authority) || operation.Path == "" {
		return Receipt{}, ErrInvalid
	}
	empty, err := vault.isEmpty()
	if err != nil {
		return Receipt{}, err
	}
	if !empty {
		return Receipt{}, ErrInvalid
	}
	raw, err := readEnvelopeFile(operation.Path)
	if err != nil {
		return Receipt{}, fmt.Errorf("read recovery bundle: %w", err)
	}
	bundlePassword, err := readPassword(ctx, secrets, SecretPromptBundleRestore)
	if err != nil {
		return Receipt{}, err
	}
	defer clear(bundlePassword)
	purpose, plaintext, _, err := openEnvelope(raw, bundlePassword)
	if err != nil {
		return Receipt{}, err
	}
	defer clear(plaintext)
	if purpose != PurposeBundle {
		return Receipt{}, ErrInvalid
	}
	state, err := decodeAuthorityState(plaintext, PurposeBundle)
	if err != nil {
		return Receipt{}, err
	}
	defer func() { clear(state.RootMaterial); clear(state.AdmissionJournal); clear(state.ServiceJournal) }()
	if state.Binding != operation.Expected {
		return Receipt{}, ErrInvalid
	}
	floors, err := vault.readFloors()
	if err != nil {
		return Receipt{}, err
	}
	retained, found := floorFor(floors, state.Binding)
	if found && !floorEqualsState(retained, state) && !strictlyHigher(state, retained) {
		return Receipt{}, ErrInvalid
	}
	vaultPassword, err := readPassword(ctx, secrets, SecretPromptVaultCreate)
	if err != nil {
		return Receipt{}, err
	}
	defer clear(vaultPassword)
	confirmation, err := readPassword(ctx, secrets, SecretPromptVaultCreateConfirm)
	if err != nil {
		return Receipt{}, err
	}
	defer clear(confirmation)
	if subtle.ConstantTimeCompare(vaultPassword, confirmation) != 1 {
		return Receipt{}, ErrInvalid
	}
	sealed, err := encodeAuthorityState(PurposeVault, state)
	if err != nil {
		return Receipt{}, err
	}
	defer clear(sealed)
	envelopeBytes, err := sealEnvelope(PurposeVault, sealed, vaultPassword)
	if err != nil {
		return Receipt{}, err
	}
	defer clear(envelopeBytes)
	recordID, err := freshRecordID()
	if err != nil {
		return Receipt{}, err
	}
	if err := vault.writeQuarantineRecord(recordID, envelopeBytes); err != nil {
		return Receipt{}, err
	}
	if !found || !floorEqualsState(retained, state) {
		if err := vault.advanceFloor(state); err != nil {
			return Receipt{}, err
		}
	}
	info, err := inspectEnvelope(envelopeBytes)
	if err != nil {
		return Receipt{}, err
	}
	return Receipt{Operation: OperationRestoreRecoveryBundle, RecordID: recordID, Envelope: info, Authority: authorityReceipt(state), State: RecordAuthorityLocked}, nil
}
