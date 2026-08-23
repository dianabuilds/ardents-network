package custody

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func (vault *Vault) exportBundle(ctx context.Context, operation Operation, secrets SecretInput) (Receipt, error) {
	if secrets == nil || operation.Transition != nil || operation.Preparation != nil || operation.Reconciliation != nil || !isZeroAuthorityState(operation.Authority) || !validRecordID(operation.RecordID) || operation.Path == "" {
		return Receipt{}, ErrInvalid
	}
	raw, sourceState, err := vault.readExportableRecord(operation.RecordID)
	if err != nil {
		return Receipt{}, fmt.Errorf("read vault record: %w", err)
	}
	vaultPassword, err := readPassword(ctx, secrets, SecretPromptVaultUnlock)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(vaultPassword)
	purpose, plaintext, _, err := openEnvelope(raw, vaultPassword)
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
	if state.Binding != operation.Expected {
		return Receipt{}, ErrInvalid
	}
	bundlePassword, err := readPassword(ctx, secrets, SecretPromptBundleExport)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(bundlePassword)
	confirmation, err := readPassword(ctx, secrets, SecretPromptBundleExportConfirm)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(confirmation)
	if subtle.ConstantTimeCompare(bundlePassword, confirmation) != 1 || subtle.ConstantTimeCompare(bundlePassword, vaultPassword) == 1 {
		return Receipt{}, ErrInvalid
	}
	bundleState, err := encodeAuthorityState(PurposeBundle, state)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(bundleState)
	bundle, err := sealEnvelope(PurposeBundle, bundleState, bundlePassword)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(bundle)
	info, err := publishAndTestBundle(ctx, operation.Path, bundle, bundlePassword, operation.Expected, secrets)
	if err != nil {
		return Receipt{}, err
	}
	return Receipt{Operation: OperationExportRecoveryBundle, RecordID: operation.RecordID, Envelope: info, Authority: authorityReceipt(state), TestRestored: true, State: sourceState}, nil
}

func (vault *Vault) readExportableRecord(recordID string) ([]byte, RecordState, error) {
	for _, candidate := range []struct {
		directory string
		state     RecordState
	}{
		{vault.records, RecordActive},
		{vault.quarantine, RecordAuthorityLocked},
	} {
		raw, err := readEnvelopeFile(filepath.Join(candidate.directory, "record-"+recordID+".json"))
		if err == nil {
			return raw, candidate.state, nil
		}
		if !os.IsNotExist(err) {
			return nil, "", fmt.Errorf("read vault record: %w", err)
		}
	}
	return nil, "", fmt.Errorf("read vault record: %w", os.ErrNotExist)
}

func publishAndTestBundle(ctx context.Context, path string, body, password []byte, expected AuthorityBinding, input SecretInput) (EnvelopeInfo, error) {
	destination, err := filepath.Abs(path)
	if err != nil {
		return EnvelopeInfo{}, fmt.Errorf("bundle destination: %w", err)
	}
	parent := filepath.Dir(destination)
	parentInfo, err := os.Stat(parent)
	if err != nil || !parentInfo.IsDir() {
		return EnvelopeInfo{}, fmt.Errorf("bundle destination parent: %w", ErrInvalid)
	}
	existing := false
	if info, err := os.Lstat(destination); err == nil {
		if !info.Mode().IsRegular() {
			return EnvelopeInfo{}, ErrInvalid
		}
		existing = true
	} else if !os.IsNotExist(err) {
		return EnvelopeInfo{}, fmt.Errorf("inspect bundle destination: %w", err)
	}
	temporary, err := os.CreateTemp(parent, ".ardents-recovery-bundle-")
	if err != nil {
		return EnvelopeInfo{}, fmt.Errorf("create encrypted bundle temporary: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return EnvelopeInfo{}, fmt.Errorf("protect bundle temporary: %w", err)
	}
	if _, err := temporary.Write(body); err != nil {
		_ = temporary.Close()
		return EnvelopeInfo{}, fmt.Errorf("write bundle: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return EnvelopeInfo{}, fmt.Errorf("flush bundle: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return EnvelopeInfo{}, fmt.Errorf("close bundle: %w", err)
	}
	if _, err := testRestoreBundle(temporaryPath, password, expected); err != nil {
		return EnvelopeInfo{}, err
	}
	backupPath := ""
	removeBackup := false
	if existing {
		confirmed, err := input.Confirm(ctx, ConfirmationPromptBundleReplacement)
		if err != nil {
			return EnvelopeInfo{}, fmt.Errorf("confirm bundle replacement: %w", err)
		}
		if !confirmed {
			return EnvelopeInfo{}, ErrInvalid
		}
		backupPath, err = copyEncryptedBundle(destination, parent)
		if err != nil {
			return EnvelopeInfo{}, err
		}
		removeBackup = true
		defer func() {
			if removeBackup {
				_ = os.Remove(backupPath)
			}
		}()
	}
	if err := durableRename(temporaryPath, destination); err != nil {
		return EnvelopeInfo{}, fmt.Errorf("publish bundle: %w", err)
	}
	if err := syncDirectory(parent); err != nil {
		cause, keepBackup := restorePreviousBundle(destination, backupPath, parent, fmt.Errorf("flush bundle directory: %w", err))
		if keepBackup {
			removeBackup = false
		}
		return EnvelopeInfo{}, cause
	}
	info, err := testRestoreBundle(destination, password, expected)
	if err != nil {
		cause, keepBackup := restorePreviousBundle(destination, backupPath, parent, err)
		if keepBackup {
			removeBackup = false
		}
		return EnvelopeInfo{}, cause
	}
	if backupPath != "" {
		if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
			return EnvelopeInfo{}, fmt.Errorf("remove replaced bundle: %w", err)
		}
		removeBackup = false
	}
	return info, nil
}

// restorePreviousBundle restores an encrypted backup after a failed
// publication. If the replacement cannot be restored, it deliberately leaves
// the backup for repair instead of pretending that the previous Bundle is safe.
func restorePreviousBundle(destination, backup, parent string, cause error) (error, bool) {
	if backup == "" {
		return cause, false
	}
	if err := durableRename(backup, destination); err != nil {
		return errors.Join(cause, fmt.Errorf("restore previous bundle: %w", err)), true
	}
	if err := syncDirectory(parent); err != nil {
		return errors.Join(cause, fmt.Errorf("flush restored bundle directory: %w", err)), false
	}
	return cause, false
}

// copyEncryptedBundle preserves an existing encrypted destination before an
// atomic replacement. The destination stays in place until the replacement
// rename, so a process interruption cannot create a missing-bundle interval.
func copyEncryptedBundle(destination, parent string) (string, error) {
	source, err := os.Open(destination)
	if err != nil {
		return "", fmt.Errorf("open previous bundle: %w", err)
	}
	defer source.Close()
	backup, err := os.CreateTemp(parent, ".ardents-recovery-bundle-previous-")
	if err != nil {
		return "", fmt.Errorf("reserve bundle backup: %w", err)
	}
	backupPath := backup.Name()
	failed := true
	defer func() {
		if failed {
			_ = backup.Close()
			_ = os.Remove(backupPath)
		}
	}()
	if err := backup.Chmod(0o600); err != nil {
		return "", fmt.Errorf("protect previous bundle: %w", err)
	}
	written, err := io.Copy(backup, io.LimitReader(source, maximumEnvelopeBytes+1))
	if err != nil {
		return "", fmt.Errorf("copy previous bundle: %w", err)
	}
	if written > maximumEnvelopeBytes {
		return "", ErrInvalid
	}
	if err := backup.Sync(); err != nil {
		return "", fmt.Errorf("flush previous bundle: %w", err)
	}
	if err := backup.Close(); err != nil {
		return "", fmt.Errorf("close previous bundle: %w", err)
	}
	failed = false
	return backupPath, nil
}

func testRestoreBundle(path string, password []byte, expected AuthorityBinding) (EnvelopeInfo, error) {
	raw, err := readEnvelopeFile(path)
	if err != nil {
		return EnvelopeInfo{}, fmt.Errorf("reopen recovery bundle: %w", err)
	}
	purpose, plaintext, info, err := openEnvelope(raw, password)
	if err != nil {
		return EnvelopeInfo{}, err
	}
	defer zero(plaintext)
	if purpose != PurposeBundle {
		return EnvelopeInfo{}, ErrInvalid
	}
	state, err := decodeAuthorityState(plaintext, PurposeBundle)
	if err != nil {
		return EnvelopeInfo{}, err
	}
	defer zero(state.RootMaterial)
	if state.Binding != expected {
		return EnvelopeInfo{}, ErrInvalid
	}
	return info, nil
}
