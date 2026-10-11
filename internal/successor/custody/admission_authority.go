package custody

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/allocation"
)

const admissionAllocationWatermark = "admission-allocation-sequence"

func (vault *Vault) createAdmissionAuthority(ctx context.Context, operation Operation, secrets SecretInput) (Receipt, error) {
	if secrets == nil || !validAdmissionAuthorityCreation(operation) {
		return Receipt{}, ErrInvalid
	}
	if err := vault.admitIndependentRoot(); err != nil {
		return Receipt{}, err
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(private)
	state := authorityState{Binding: operation.Binding, RootMaterial: private, Generation: 1,
		Watermarks: []Watermark{{Domain: admissionAllocationWatermark, Value: 0}}}
	state.Binding.IDCommitment = sha256.Sum256(public)
	receipt, err := vault.createRecord(ctx, Operation{Kind: operationCreateVaultRecord, authority: state}, secrets)
	if err != nil {
		return Receipt{}, err
	}
	var key [ed25519.PublicKeySize]byte
	copy(key[:], public)
	receipt.Operation = OperationCreateAdmissionAuthority
	receipt.AdmissionAuthority = AdmissionAuthorityReceipt{Public: key}
	return receipt, nil
}

func validAdmissionAuthorityCreation(operation Operation) bool {
	b := operation.Binding
	return operation.RecordID == "" && operation.Path == "" && operation.Expected == (AuthorityBinding{}) &&
		b.Kind == AuthorityAdmission && b.Environment != [32]byte{} && b.Network != [32]byte{} && b.Root != [32]byte{} && b.IDCommitment == [32]byte{} && isZeroAuthorityState(operation.authority)
}

func (vault *Vault) issueAdmissionPermission(ctx context.Context, operation Operation, secrets SecretInput) (Receipt, error) {
	if secrets == nil || !validAdmissionIssuance(operation) {
		return Receipt{}, ErrInvalid
	}
	if err := vault.requireIndependentRoot(); err != nil {
		return Receipt{}, err
	}
	now := vault.now().UTC().Truncate(time.Hour)
	request, err := allocation.Prepare(operation.AdmissionRequest, operation.Expected.Network, now)
	if err != nil {
		return Receipt{}, ErrInvalid
	}
	decoded, err := admission.DecodePermissionRequest(operation.AdmissionRequest)
	if err != nil || sha256.Sum256(decoded.AuthorityKey[:]) != operation.Expected.IDCommitment {
		return Receipt{}, ErrInvalid
	}
	raw, err := readEnvelopeFile(filepath.Join(vault.records, "record-"+operation.RecordID+".json"))
	if err != nil {
		return Receipt{}, err
	}
	header, err := inspectEnvelope(raw)
	zero(raw)
	if err != nil || header.Purpose != PurposeVault {
		return Receipt{}, ErrInvalid
	}
	password, err := readPassword(ctx, secrets, SecretPromptVaultUnlock)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(password)
	if !vault.now().UTC().Truncate(time.Hour).Equal(now) {
		return Receipt{}, ErrInvalid
	}
	source, _, err := vault.openCurrentAdmissionAuthority(operation.RecordID, password, operation.Expected)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(source.RootMaterial)
	defer zero(source.AdmissionJournal)
	public := ed25519.PrivateKey(source.RootMaterial).Public().(ed25519.PublicKey)
	decision, err := request.Decide(source.AdmissionJournal, [ed25519.PublicKeySize]byte(public), vault.now())
	if err != nil {
		return Receipt{}, ErrInvalid
	}
	if err := vault.check(); err != nil {
		return Receipt{}, err
	}
	if decision.Repeated() {
		permission, signErr := signAdmissionPermission(ed25519.PrivateKey(source.RootMaterial), decision.Permission())
		if signErr != nil {
			return Receipt{}, signErr
		}
		return Receipt{Operation: OperationIssueAdmissionPermission, RecordID: operation.RecordID,
			Authority: authorityReceipt(source), AdmissionPermission: permission, State: RecordActive}, nil
	}
	journal := decision.Journal()
	defer zero(journal)
	successor, err := admissionSuccessor(source, journal)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(successor.RootMaterial)
	defer zero(successor.AdmissionJournal)
	permission, err := signAdmissionPermission(ed25519.PrivateKey(source.RootMaterial), decision.Permission())
	if err != nil {
		return Receipt{}, err
	}
	info, err := vault.writeAdmissionSuccessor(operation.RecordID, source, successor, password, now)
	if err != nil {
		return Receipt{}, err
	}
	if err := vault.advanceFloor(successor); err != nil {
		return Receipt{}, err
	}
	return Receipt{Operation: OperationIssueAdmissionPermission, RecordID: operation.RecordID, Envelope: info,
		Authority: authorityReceipt(successor), AdmissionPermission: permission, State: RecordActive}, nil
}

func validAdmissionIssuance(operation Operation) bool {
	return validRecordID(operation.RecordID) && operation.Expected != (AuthorityBinding{}) && operation.Expected.Kind == AuthorityAdmission &&
		len(operation.AdmissionRequest) != 0 && operation.AdmissionRequestCommitment == sha256.Sum256(operation.AdmissionRequest) &&
		operation.Path == "" &&
		isZeroAuthorityState(operation.authority)
}

func openAdmissionAuthority(raw, password []byte, expected AuthorityBinding) (authorityState, EnvelopeInfo, error) {
	purpose, plaintext, info, err := openEnvelope(raw, password)
	if err != nil {
		return authorityState{}, EnvelopeInfo{}, err
	}
	defer zero(plaintext)
	if purpose != PurposeVault {
		return authorityState{}, EnvelopeInfo{}, ErrInvalid
	}
	state, err := decodeAuthorityState(plaintext, PurposeVault)
	if err != nil {
		return authorityState{}, EnvelopeInfo{}, err
	}
	private := ed25519.PrivateKey(state.RootMaterial)
	if state.Binding != expected || state.Binding.Kind != AuthorityAdmission || len(private) != ed25519.PrivateKeySize {
		zero(state.RootMaterial)
		zero(state.AdmissionJournal)
		return authorityState{}, EnvelopeInfo{}, ErrInvalid
	}
	public, ok := private.Public().(ed25519.PublicKey)
	if !ok || sha256.Sum256(public) != state.Binding.IDCommitment {
		zero(state.RootMaterial)
		zero(state.AdmissionJournal)
		return authorityState{}, EnvelopeInfo{}, ErrInvalid
	}
	if err := validateAdmissionAuthorityState(state); err != nil {
		zero(state.RootMaterial)
		zero(state.AdmissionJournal)
		return authorityState{}, EnvelopeInfo{}, err
	}
	return state, info, nil
}

func validateAdmissionAuthorityState(state authorityState) error {
	private := ed25519.PrivateKey(state.RootMaterial)
	if len(private) != ed25519.PrivateKeySize ||
		len(state.Watermarks) != 1 || state.Watermarks[0].Domain != admissionAllocationWatermark ||
		state.Generation == 0 || state.Generation != state.Watermarks[0].Value+1 || state.Revision != state.Watermarks[0].Value {
		return ErrInvalid
	}
	public, ok := private.Public().(ed25519.PublicKey)
	if !ok || sha256.Sum256(public) != state.Binding.IDCommitment {
		return ErrInvalid
	}
	if allocation.ValidateJournal(state.AdmissionJournal) != nil {
		return ErrInvalid
	}
	return nil
}

func admissionSuccessor(source authorityState, journal []byte) (authorityState, error) {
	if source.Generation == math.MaxUint64 || source.Revision == math.MaxUint64 || source.Watermarks[0].Value == math.MaxUint64 {
		return authorityState{}, ErrInvalid
	}
	return authorityState{Binding: source.Binding, RootMaterial: append([]byte(nil), source.RootMaterial...), AdmissionJournal: append([]byte(nil), journal...),
		Generation: source.Generation + 1, Revision: source.Revision + 1,
		Watermarks: []Watermark{{Domain: admissionAllocationWatermark, Value: source.Watermarks[0].Value + 1}}}, nil
}

func signAdmissionPermission(private ed25519.PrivateKey, permission admission.Permission) ([]byte, error) {
	if len(private) != ed25519.PrivateKeySize {
		return nil, ErrInvalid
	}
	// The signed grammar is single-sourced in internal/successor/admission: Custody
	// signs the exact transcript the authority verifies against.
	copy(permission.Signature[:], ed25519.Sign(private, admission.PermissionTranscript(permission)))
	return admission.EncodePermission(permission)
}

// Admission journal state is deliberately a single replaceable encrypted
// envelope. A successor vault record for every offline permission would turn
// the selected 1,024-record vault bound into a lower, accidental allocation
// quota. The durable floor binds this envelope's exact monotonic successor.
func admissionLedgerPath(root, recordID string) (string, error) {
	if !validRecordID(recordID) {
		return "", ErrInvalid
	}
	return filepath.Join(root, "admission-ledger-"+recordID+".json"), nil
}

func (vault *Vault) openCurrentAdmissionAuthority(recordID string, password []byte, expected AuthorityBinding) (authorityState, EnvelopeInfo, error) {
	baseRaw, err := readEnvelopeFile(filepath.Join(vault.records, "record-"+recordID+".json"))
	if err != nil {
		return authorityState{}, EnvelopeInfo{}, err
	}
	defer zero(baseRaw)
	base, baseInfo, err := openAdmissionAuthority(baseRaw, password, expected)
	if err != nil {
		return authorityState{}, EnvelopeInfo{}, err
	}
	floors, err := vault.readFloors()
	if err != nil {
		zero(base.RootMaterial)
		zero(base.AdmissionJournal)
		return authorityState{}, EnvelopeInfo{}, err
	}
	floor, found := floorFor(floors, expected)
	if !found {
		zero(base.RootMaterial)
		zero(base.AdmissionJournal)
		return authorityState{}, EnvelopeInfo{}, ErrInvalid
	}
	path, err := admissionLedgerPath(vault.root, recordID)
	if err != nil {
		zero(base.RootMaterial)
		zero(base.AdmissionJournal)
		return authorityState{}, EnvelopeInfo{}, err
	}
	raw, err := readEnvelopeFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if !floorEqualsState(floor, base) {
			zero(base.RootMaterial)
			zero(base.AdmissionJournal)
			return authorityState{}, EnvelopeInfo{}, ErrInvalid
		}
		return base, baseInfo, nil
	}
	if err != nil {
		zero(base.RootMaterial)
		zero(base.AdmissionJournal)
		return authorityState{}, EnvelopeInfo{}, err
	}
	defer zero(raw)
	current, info, err := openAdmissionAuthority(raw, password, expected)
	if err != nil {
		zero(base.RootMaterial)
		zero(base.AdmissionJournal)
		return authorityState{}, EnvelopeInfo{}, err
	}
	if floorEqualsState(floor, current) {
		zero(base.RootMaterial)
		zero(base.AdmissionJournal)
		return current, info, nil
	}
	// The immutable base is older after the first committed allocation. Recovery
	// must compare the authenticated successor to the retained floor, not the base.
	if !admissionLedgerFollowsFloor(current, floor) {
		zero(base.RootMaterial)
		zero(base.AdmissionJournal)
		zero(current.RootMaterial)
		zero(current.AdmissionJournal)
		return authorityState{}, EnvelopeInfo{}, ErrInvalid
	}
	if err := vault.advanceFloor(current); err != nil {
		zero(base.RootMaterial)
		zero(base.AdmissionJournal)
		zero(current.RootMaterial)
		zero(current.AdmissionJournal)
		return authorityState{}, EnvelopeInfo{}, err
	}
	zero(base.RootMaterial)
	zero(base.AdmissionJournal)
	return current, info, nil
}

func admissionLedgerFollowsFloor(state authorityState, floor authorityFloor) bool {
	if state.Generation != floor.Generation+1 || state.Revision != floor.Revision+1 || len(state.Watermarks) != len(floor.Watermarks) {
		return false
	}
	for index := range state.Watermarks {
		if state.Watermarks[index].Domain != floor.Watermarks[index].Domain || state.Watermarks[index].Value != floor.Watermarks[index].Value+1 {
			return false
		}
	}
	return true
}

func (vault *Vault) writeAdmissionSuccessor(recordID string, source, expected authorityState, password []byte, window time.Time) (EnvelopeInfo, error) {
	path, err := admissionLedgerPath(vault.root, recordID)
	if err != nil {
		return EnvelopeInfo{}, err
	}
	raw, err := readEnvelopeFile(path)
	defer zero(raw)
	if err == nil {
		state, info, openErr := openAdmissionAuthority(raw, password, expected.Binding)
		if openErr == nil && sameAuthorityState(state, expected) {
			zero(state.RootMaterial)
			zero(state.AdmissionJournal)
			return info, nil
		}
		if openErr != nil || !sameAuthorityState(state, source) {
			zero(state.RootMaterial)
			zero(state.AdmissionJournal)
			zero(raw)
			return EnvelopeInfo{}, ErrInvalid
		}
		zero(state.RootMaterial)
		zero(state.AdmissionJournal)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return EnvelopeInfo{}, ErrInvalid
	}
	plaintext, err := encodeAuthorityState(PurposeVault, expected)
	if err != nil {
		return EnvelopeInfo{}, err
	}
	defer zero(plaintext)
	envelope, err := sealEnvelope(PurposeVault, plaintext, password)
	if err != nil {
		return EnvelopeInfo{}, err
	}
	defer zero(envelope)
	// Do not reserve an elapsed hour after password derivation. Once written,
	// the existing successor recovery must still complete its durable floor.
	if !vault.now().UTC().Truncate(time.Hour).Equal(window) {
		return EnvelopeInfo{}, ErrInvalid
	}
	if err := vault.check(); err != nil {
		return EnvelopeInfo{}, err
	}
	if err := writeAtomicPrivate(path, envelope); err != nil {
		zero(envelope)
		return EnvelopeInfo{}, err
	}
	if err := verifyPersistedEnvelope(path, envelope); err != nil {
		return EnvelopeInfo{}, err
	}
	info, err := inspectEnvelope(envelope)
	if err != nil {
		return EnvelopeInfo{}, err
	}
	return info, nil
}
