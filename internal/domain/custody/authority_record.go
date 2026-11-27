package custody

import (
	"context"
	"crypto/subtle"
	"fmt"
	"os"
)

func (vault *Vault) createAuthorityRecord(ctx context.Context, operation Operation, secrets SecretInput) (Receipt, error) {
	if secrets == nil || operation.RecordID != "" || operation.Path != "" || operation.Expected != (AuthorityBinding{}) {
		return Receipt{}, ErrInvalid
	}
	if err := vault.prepareFloor(operation.authority); err != nil {
		return Receipt{}, err
	}
	password, err := readPassword(ctx, secrets, SecretPromptVaultCreate)
	if err != nil {
		return Receipt{}, err
	}
	defer clear(password)
	confirmation, err := readPassword(ctx, secrets, SecretPromptVaultCreateConfirm)
	if err != nil {
		return Receipt{}, err
	}
	defer clear(confirmation)
	if subtle.ConstantTimeCompare(password, confirmation) != 1 {
		return Receipt{}, ErrInvalid
	}
	plaintext, err := encodeAuthorityState(PurposeVault, operation.authority)
	if err != nil {
		return Receipt{}, err
	}
	defer clear(plaintext)
	envelopeBytes, err := sealEnvelope(PurposeVault, plaintext, password)
	if err != nil {
		return Receipt{}, err
	}
	defer clear(envelopeBytes)
	recordID, err := freshRecordID()
	if err != nil {
		return Receipt{}, err
	}
	if err := vault.publishActiveRecord(recordID, envelopeBytes); err != nil {
		return Receipt{}, err
	}
	if err := vault.advanceFloor(operation.authority); err != nil {
		return Receipt{}, err
	}
	info, err := inspectEnvelope(envelopeBytes)
	if err != nil {
		return Receipt{}, err
	}
	return Receipt{Operation: operationCreateVaultRecord, RecordID: recordID, Envelope: info, Authority: authorityReceipt(operation.authority), State: RecordActive}, nil
}

func (vault *Vault) verifyAuthorityRecord(ctx context.Context, operation Operation, secrets SecretInput) (Receipt, error) {
	if secrets == nil || operation.Path != "" || !isZeroAuthorityState(operation.authority) || !validRecordID(operation.RecordID) {
		return Receipt{}, ErrInvalid
	}
	raw, recordState, _, err := vault.readRetainedRecord(operation.RecordID)
	if err != nil {
		return Receipt{}, fmt.Errorf("read vault record: %w", err)
	}
	password, err := readPassword(ctx, secrets, SecretPromptVaultUnlock)
	if err != nil {
		return Receipt{}, err
	}
	defer clear(password)
	purpose, plaintext, info, err := openEnvelope(raw, password)
	if err != nil {
		return Receipt{}, err
	}
	defer clear(plaintext)
	if purpose != PurposeVault {
		return Receipt{}, ErrInvalid
	}
	state, err := decodeAuthorityState(plaintext, PurposeVault)
	if err != nil {
		return Receipt{}, err
	}
	defer func() { clear(state.RootMaterial); clear(state.AdmissionJournal); clear(state.ServiceJournal) }()
	if state.Binding != operation.Expected {
		return Receipt{}, ErrInvalid
	}
	if state.Binding.Kind == AuthorityAdmission && recordState == RecordActive {
		clear(state.RootMaterial)
		clear(state.AdmissionJournal)
		clear(state.ServiceJournal)
		// RecordActive reads the same base record opened above; reuse its
		// plaintext instead of a second base KDF (audit DD-5). plaintext is
		// cleared by this function's defer; the returned state is owned here.
		state, info, err = vault.currentAdmissionAuthorityFromBase(plaintext, info, operation.RecordID, password, operation.Expected)
		if err != nil {
			return Receipt{}, err
		}
		return Receipt{Operation: OperationVerifyVaultRecord, RecordID: operation.RecordID, Envelope: info, Authority: authorityReceipt(state), State: RecordActive}, nil
	}
	if recordState == RecordActive {
		if err := vault.matchesFloor(state); err != nil {
			return Receipt{}, err
		}
	}
	return Receipt{Operation: OperationVerifyVaultRecord, RecordID: operation.RecordID, Envelope: info, Authority: authorityReceipt(state), State: recordState}, nil
}

func readPassword(ctx context.Context, input SecretInput, prompt SecretPrompt) ([]byte, error) {
	password, err := input.ReadSecret(ctx, prompt)
	if err != nil {
		clear(password)
		return nil, fmt.Errorf("read custody secret: %w", err)
	}
	if err := ctx.Err(); err != nil {
		clear(password)
		return nil, err
	}
	if err := validatePassword(password); err != nil {
		clear(password)
		return nil, err
	}
	return password, nil
}

// readRetainedRecord selects the active record first. A locked quarantine
// record remains available to verification, export and confirmed purge.
func (vault *Vault) readRetainedRecord(recordID string) ([]byte, RecordState, string, error) {
	for _, candidate := range []struct {
		directory string
		state     RecordState
	}{
		{vault.records, RecordActive},
		{vault.quarantine, RecordAuthorityLocked},
	} {
		path := recordPath(candidate.directory, recordID)
		raw, err := readEnvelopeFile(path)
		if err == nil {
			return raw, candidate.state, path, nil
		}
		if !os.IsNotExist(err) {
			return nil, "", "", fmt.Errorf("read vault record: %w", err)
		}
	}
	return nil, "", "", fmt.Errorf("read vault record: %w", os.ErrNotExist)
}
