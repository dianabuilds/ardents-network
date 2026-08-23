package custody

import (
	"context"
	"crypto/subtle"
	"fmt"
	"os"
	"path/filepath"
)

func (vault *Vault) exportBundle(ctx context.Context, operation Operation, secrets SecretInput) (Receipt, error) {
	if secrets == nil || !isZeroAuthorityState(operation.Authority) || !validRecordID(operation.RecordID) || operation.Path == "" {
		return Receipt{}, ErrInvalid
	}
	raw, err := readEnvelopeFile(filepath.Join(vault.records, "record-"+operation.RecordID+".json"))
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
	if err := writeNewBundle(operation.Path, bundle); err != nil {
		return Receipt{}, err
	}
	info, err := testRestoreBundle(operation.Path, bundlePassword, operation.Expected)
	if err != nil {
		return Receipt{}, err
	}
	return Receipt{Operation: OperationExportRecoveryBundle, RecordID: operation.RecordID, Envelope: info, Authority: authorityReceipt(state), TestRestored: true}, nil
}

func writeNewBundle(path string, body []byte) error {
	destination, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("bundle destination: %w", err)
	}
	parent := filepath.Dir(destination)
	parentInfo, err := os.Stat(parent)
	if err != nil || !parentInfo.IsDir() {
		return fmt.Errorf("bundle destination parent: %w", ErrInvalid)
	}
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("bundle destination already exists: %w", ErrInvalid)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect bundle destination: %w", err)
	}
	temporary, err := os.CreateTemp(parent, ".ardents-recovery-bundle-")
	if err != nil {
		return fmt.Errorf("create encrypted bundle temporary: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("protect bundle temporary: %w", err)
	}
	if _, err := temporary.Write(body); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write bundle: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("flush bundle: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close bundle: %w", err)
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return fmt.Errorf("publish bundle: %w", err)
	}
	return nil
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
