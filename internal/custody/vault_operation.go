package custody

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace/authority"
)

// Execute performs exactly one bounded custody operation. It never retains a
// password, derived key, plaintext root, or signing capability after return.
func (vault *Vault) Execute(ctx context.Context, operation Operation, secrets SecretInput) (Receipt, error) {
	vault.mu.Lock()
	defer vault.mu.Unlock()
	if vault.closed {
		return Receipt{}, ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	switch operation.Kind {
	case OperationCreateVaultRecord:
		return vault.createRecord(ctx, operation, secrets)
	case OperationVerifyVaultRecord:
		return vault.verifyRecord(ctx, operation, secrets)
	case OperationExportRecoveryBundle:
		return vault.exportBundle(ctx, operation, secrets)
	case OperationRestoreRecoveryBundle:
		return vault.restoreBundle(ctx, operation, secrets)
	case OperationSignNamespaceTransition:
		return vault.signNamespaceTransition(ctx, operation, secrets)
	case OperationPrepareNamespaceSubmission:
		return vault.prepareNamespaceSubmission(ctx, operation, secrets)
	case OperationActivateRecoveredAuthority:
		return vault.activateRecoveredAuthority(ctx, operation, secrets)
	case OperationInspectEnvelope:
		return vault.inspect(operation)
	default:
		return Receipt{}, ErrInvalid
	}
}

func (vault *Vault) createRecord(ctx context.Context, operation Operation, secrets SecretInput) (Receipt, error) {
	if secrets == nil || operation.RecordID != "" || operation.Path != "" || operation.Expected != (AuthorityBinding{}) || operation.Transition != nil || operation.Preparation != nil || operation.Reconciliation != nil {
		return Receipt{}, ErrInvalid
	}
	if err := vault.prepareFloor(operation.Authority); err != nil {
		return Receipt{}, err
	}
	password, err := readPassword(ctx, secrets, SecretPromptVaultCreate)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(password)
	confirmation, err := readPassword(ctx, secrets, SecretPromptVaultCreateConfirm)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(confirmation)
	if subtle.ConstantTimeCompare(password, confirmation) != 1 {
		return Receipt{}, ErrInvalid
	}
	plaintext, err := encodeAuthorityState(PurposeVault, operation.Authority)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(plaintext)
	envelopeBytes, err := sealEnvelope(PurposeVault, plaintext, password)
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
	if err := vault.advanceFloor(operation.Authority); err != nil {
		return Receipt{}, err
	}
	info, err := inspectEnvelope(envelopeBytes)
	if err != nil {
		return Receipt{}, err
	}
	return Receipt{Operation: OperationCreateVaultRecord, RecordID: recordID, Envelope: info, Authority: authorityReceipt(operation.Authority), State: RecordActive}, nil
}

func (vault *Vault) verifyRecord(ctx context.Context, operation Operation, secrets SecretInput) (Receipt, error) {
	if secrets == nil || operation.Path != "" || operation.Transition != nil || operation.Preparation != nil || operation.Reconciliation != nil || !isZeroAuthorityState(operation.Authority) || !validRecordID(operation.RecordID) {
		return Receipt{}, ErrInvalid
	}
	raw, err := readEnvelopeFile(filepath.Join(vault.records, "record-"+operation.RecordID+".json"))
	if err != nil {
		return Receipt{}, fmt.Errorf("read vault record: %w", err)
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
	state, err := decodeAuthorityState(plaintext, PurposeVault)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(state.RootMaterial)
	if state.Binding != operation.Expected {
		return Receipt{}, ErrInvalid
	}
	if err := vault.matchesFloor(state); err != nil {
		return Receipt{}, err
	}
	return Receipt{Operation: OperationVerifyVaultRecord, RecordID: operation.RecordID, Envelope: info, Authority: authorityReceipt(state), State: RecordActive}, nil
}

func (vault *Vault) signNamespaceTransition(ctx context.Context, operation Operation, secrets SecretInput) (Receipt, error) {
	if secrets == nil || operation.Transition == nil || operation.Preparation != nil || operation.Reconciliation != nil || operation.Path != "" ||
		!isZeroAuthorityState(operation.Authority) || !validRecordID(operation.RecordID) {
		return Receipt{}, ErrInvalid
	}
	raw, err := readEnvelopeFile(filepath.Join(vault.records, "record-"+operation.RecordID+".json"))
	if err != nil {
		return Receipt{}, fmt.Errorf("read vault record: %w", err)
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
	state, err := decodeAuthorityState(plaintext, PurposeVault)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(state.RootMaterial)
	if state.Binding != operation.Expected || state.Binding.Kind != AuthorityName {
		return Receipt{}, ErrInvalid
	}
	if err := vault.matchesFloor(state); err != nil {
		return Receipt{}, err
	}
	private := ed25519.PrivateKey(append([]byte(nil), state.RootMaterial...))
	defer zero(private)
	if len(private) != ed25519.PrivateKeySize {
		return Receipt{}, ErrInvalid
	}
	public, ok := private.Public().(ed25519.PublicKey)
	if !ok || sha256.Sum256(public) != state.Binding.IDCommitment {
		return Receipt{}, ErrInvalid
	}
	signer := custodyTransitionSigner{private: private, generation: state.Generation, revision: state.Revision}
	proof, err := operation.Transition(&signer)
	defer signer.erase()
	if err != nil {
		return Receipt{}, err
	}
	if !signer.used || subtle.ConstantTimeCompare(proof, signer.signature) != 1 {
		return Receipt{}, ErrInvalid
	}
	return Receipt{Operation: OperationSignNamespaceTransition, RecordID: operation.RecordID,
		Envelope: info, Authority: authorityReceipt(state), State: RecordActive, Proof: append([]byte(nil), proof...)}, nil
}

type custodyTransitionSigner struct {
	private    ed25519.PrivateKey
	signature  []byte
	generation uint64
	revision   uint64
	used       bool
}

func (signer *custodyTransitionSigner) Sign(request authority.TransitionSigningRequest) ([]byte, error) {
	if signer == nil || signer.used || len(signer.private) != ed25519.PrivateKeySize {
		return nil, ErrInvalid
	}
	signer.used = true
	public, ok := signer.private.Public().(ed25519.PublicKey)
	generation, revision := request.Predecessor()
	if !ok || request.Authority() != [ed25519.PublicKeySize]byte(public) ||
		generation != signer.generation || revision != signer.revision {
		return nil, ErrInvalid
	}
	signer.signature = ed25519.Sign(signer.private, request.Transcript())
	return append([]byte(nil), signer.signature...), nil
}

func (signer *custodyTransitionSigner) erase() {
	if signer == nil {
		return
	}
	zero(signer.private)
	zero(signer.signature)
	signer.private, signer.signature = nil, nil
}

func (vault *Vault) inspect(operation Operation) (Receipt, error) {
	if operation.RecordID != "" || !isZeroAuthorityState(operation.Authority) || operation.Expected != (AuthorityBinding{}) || operation.Path == "" || operation.Transition != nil || operation.Preparation != nil || operation.Reconciliation != nil {
		return Receipt{}, ErrInvalid
	}
	raw, err := readEnvelopeFile(operation.Path)
	if err != nil {
		return Receipt{}, fmt.Errorf("read custody envelope: %w", err)
	}
	info, err := inspectEnvelope(raw)
	if err != nil {
		return Receipt{}, err
	}
	return Receipt{Operation: OperationInspectEnvelope, Envelope: info}, nil
}

func readPassword(ctx context.Context, input SecretInput, prompt SecretPrompt) ([]byte, error) {
	password, err := input.ReadSecret(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("read custody secret: %w", err)
	}
	if err := validatePassword(password); err != nil {
		zero(password)
		return nil, err
	}
	return password, nil
}

func (vault *Vault) writeRecord(recordID string, body []byte) error {
	return vault.writeRecordIn(vault.records, recordID, body)
}

func (vault *Vault) writeQuarantineRecord(recordID string, body []byte) error {
	return vault.writeRecordIn(vault.quarantine, recordID, body)
}

func (vault *Vault) writeRecordIn(directory, recordID string, body []byte) error {
	path := filepath.Join(directory, "record-"+recordID+".json")
	if err := vault.reserveRecordSpace(int64(len(body))); err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".record-")
	if err != nil {
		return fmt.Errorf("create encrypted vault temporary: %w", err)
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return fmt.Errorf("protect vault temporary: %w", err)
	}
	if _, err := file.Write(body); err != nil {
		_ = file.Close()
		return fmt.Errorf("write vault record: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("flush vault record: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close vault record: %w", err)
	}
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("vault record collision: %w", ErrInvalid)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect vault record destination: %w", err)
	}
	if err := durableRename(temporary, path); err != nil {
		return fmt.Errorf("publish vault record: %w", err)
	}
	if err := syncDirectory(directory); err != nil {
		return fmt.Errorf("flush vault record directory: %w", err)
	}
	if err := verifyPersistedEnvelope(path, body); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

// verifyPersistedEnvelope confirms that the completed encrypted bytes were
// published unchanged and still meet the canonical envelope profile. It does
// not run another KDF: one custody operation has one explicit password attempt.
func verifyPersistedEnvelope(path string, expected []byte) error {
	persisted, err := readEnvelopeFile(path)
	if err != nil {
		return fmt.Errorf("reopen encrypted vault record: %w", err)
	}
	defer zero(persisted)
	if !bytes.Equal(persisted, expected) {
		return ErrInvalid
	}
	if _, err := inspectEnvelope(persisted); err != nil {
		return err
	}
	return nil
}

func (vault *Vault) reserveRecordSpace(incoming int64) error {
	var count int
	var total int64
	for _, directory := range []string{vault.records, vault.quarantine} {
		entries, err := os.ReadDir(directory)
		if err != nil {
			return fmt.Errorf("list vault records: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !validRecordFilename(entry.Name()) {
				return fmt.Errorf("vault record root: %w", ErrInvalid)
			}
			info, err := entry.Info()
			if err != nil {
				return fmt.Errorf("inspect vault record: %w", err)
			}
			count++
			total += info.Size()
		}
	}
	if count >= maximumVaultRecords || incoming > maximumVaultBytes || total > maximumVaultBytes-incoming {
		return ErrInvalid
	}
	return nil
}

func (vault *Vault) isEmpty() (bool, error) {
	for _, directory := range []string{vault.records, vault.quarantine} {
		entries, err := os.ReadDir(directory)
		if err != nil {
			return false, fmt.Errorf("list vault records: %w", err)
		}
		if len(entries) != 0 {
			return false, nil
		}
	}
	return true, nil
}

func validRecordFilename(value string) bool {
	if len(value) != len("record-")+32+len(".json") || value[:len("record-")] != "record-" || value[len(value)-len(".json"):] != ".json" {
		return false
	}
	return validRecordID(value[len("record-") : len(value)-len(".json")])
}

func readEnvelopeFile(path string) ([]byte, error) {
	return readRegularFile(path, maximumEnvelopeBytes)
}

func readSmallFile(path string) ([]byte, error) {
	return readRegularFile(path, maximumFloorBytes)
}

// readRegularFile excludes directories, symlinks, devices, and other special
// objects before bounded reading. Platform handle/reparse resistance remains a
// separate qualification obligation; this is the portable lexical admission
// guard used by every custody file reader.
func readRegularFile(path string, maximum uint64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, ErrInvalid
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, int64(maximum)+1))
	if err != nil {
		return nil, err
	}
	if uint64(len(body)) > maximum {
		zero(body)
		return nil, ErrInvalid
	}
	return body, nil
}

func freshRecordID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		return "", fmt.Errorf("record identifier: %w", err)
	}
	defer zero(bytes)
	return hex.EncodeToString(bytes), nil
}

func validRecordID(value string) bool {
	if len(value) != 32 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && hex.EncodeToString(decoded) == value
}

func authorityReceipt(state AuthorityState) AuthorityReceipt {
	return AuthorityReceipt{Binding: state.Binding, Generation: state.Generation, Revision: state.Revision, Watermarks: append([]Watermark(nil), state.Watermarks...)}
}
