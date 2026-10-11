package custody

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

func (vault *Vault) purgeRecord(ctx context.Context, operation Operation, secrets SecretInput) (Receipt, error) {
	if ctx == nil || secrets == nil || !isZeroAuthorityState(operation.authority) || operation.Path != "" || !validRecordID(operation.RecordID) {
		return Receipt{}, ErrInvalid
	}
	raw, state, path, err := vault.exportableRecord(operation.RecordID)
	if err != nil {
		return Receipt{}, err
	}
	password, err := readPassword(ctx, secrets, SecretPromptVaultUnlock)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(password)
	purpose, plaintext, info, err := openEnvelope(raw, password)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(plaintext)
	if purpose != PurposeVault {
		return Receipt{}, ErrInvalid
	}
	authority, err := decodeAuthorityState(plaintext, PurposeVault)
	if err != nil {
		return Receipt{}, err
	}
	defer func() { zero(authority.RootMaterial); zero(authority.AdmissionJournal); zero(authority.ServiceJournal) }()
	if authority.Binding != operation.Expected {
		return Receipt{}, ErrInvalid
	}
	if authority.Binding.Kind == AuthorityAdmission && state == RecordActive {
		zero(authority.RootMaterial)
		zero(authority.AdmissionJournal)
		zero(authority.ServiceJournal)
		authority, info, err = vault.openCurrentAdmissionAuthority(operation.RecordID, password, operation.Expected)
		if err != nil {
			return Receipt{}, err
		}
	}
	confirmed, err := secrets.Confirm(ctx, ConfirmationPromptVaultPurge)
	if err != nil || !confirmed {
		if err != nil {
			return Receipt{}, err
		}
		return Receipt{}, ErrInvalid
	}
	if err := vault.check(); err != nil {
		return Receipt{}, err
	}
	infoPath, err := os.Lstat(path)
	if err != nil || !infoPath.Mode().IsRegular() || infoPath.Mode()&os.ModeSymlink != 0 {
		return Receipt{}, fmt.Errorf("purge vault record: %w", ErrInvalid)
	}
	if authority.Binding.Kind == AuthorityAdmission && state == RecordActive {
		ledger, err := admissionLedgerPath(vault.root, operation.RecordID)
		if err != nil {
			return Receipt{}, err
		}
		if err := os.Remove(ledger); err != nil && !os.IsNotExist(err) {
			return Receipt{}, fmt.Errorf("purge admission ledger: %w", err)
		}
		if err := syncDirectory(vault.root); err != nil {
			return Receipt{}, fmt.Errorf("flush purged admission ledger: %w", err)
		}
	}
	if err := os.Remove(path); err != nil {
		return Receipt{}, fmt.Errorf("purge vault record: %w", err)
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		return Receipt{}, fmt.Errorf("purge vault record: %w", err)
	}
	return Receipt{Operation: OperationPurgeVaultRecord, RecordID: operation.RecordID, Envelope: info,
		Authority: authorityReceipt(authority), State: state}, nil
}
