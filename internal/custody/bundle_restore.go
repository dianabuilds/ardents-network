package custody

import (
	"context"
	"crypto/subtle"
	"fmt"
)

func (vault *Vault) restoreBundle(ctx context.Context, operation Operation, secrets SecretInput) (Receipt, error) {
	if secrets == nil || operation.Transition != nil || operation.Preparation != nil || operation.Reconciliation != nil || operation.RecordID != "" || !isZeroAuthorityState(operation.Authority) || operation.Path == "" {
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
	defer zero(bundlePassword)
	purpose, plaintext, _, err := openEnvelope(raw, bundlePassword)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(plaintext)
	if purpose != PurposeBundle {
		return Receipt{}, ErrInvalid
	}
	state, err := decodeAuthorityState(plaintext, PurposeBundle)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(state.RootMaterial)
	if state.Binding != operation.Expected {
		return Receipt{}, ErrInvalid
	}
	vaultPassword, err := readPassword(ctx, secrets, SecretPromptVaultCreate)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(vaultPassword)
	confirmation, err := readPassword(ctx, secrets, SecretPromptVaultCreateConfirm)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(confirmation)
	if subtle.ConstantTimeCompare(vaultPassword, confirmation) != 1 {
		return Receipt{}, ErrInvalid
	}
	sealed, err := encodeAuthorityState(PurposeVault, state)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(sealed)
	envelopeBytes, err := sealEnvelope(PurposeVault, sealed, vaultPassword)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(envelopeBytes)
	recordID, err := freshRecordID()
	if err != nil {
		return Receipt{}, err
	}
	if err := vault.writeQuarantineRecord(recordID, envelopeBytes); err != nil {
		return Receipt{}, err
	}
	info, err := inspectEnvelope(envelopeBytes)
	if err != nil {
		return Receipt{}, err
	}
	return Receipt{Operation: OperationRestoreRecoveryBundle, RecordID: recordID, Envelope: info, Authority: authorityReceipt(state), State: RecordAuthorityLocked}, nil
}
