package custody

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

const (
	admissionAllocationWatermark = "admission-allocation-sequence"
	maximumAdmissionJournalBytes = 3 << 20
	maximumUserAllocation        = uint64(4096)
	maximumPublisherAllocation   = uint64(16384)
	maximumIssuerAllocation      = uint64(65536)
	admissionJournalEntrySize    = 77
)

type admissionAllocation struct {
	window uint64
	role   credential.AllocationRole
	tokens uint32
	id     [32]byte
	digest [32]byte
}

func (vault *Vault) createAdmissionAuthority(ctx context.Context, operation Operation, secrets SecretInput) (Receipt, error) {
	if secrets == nil || !validAdmissionAuthorityCreation(operation) {
		return Receipt{}, ErrInvalid
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(private)
	state := AuthorityState{Binding: operation.Authority.Binding, RootMaterial: private, Generation: 1,
		Watermarks: []Watermark{{Domain: admissionAllocationWatermark, Value: 0}}}
	state.Binding.IDCommitment = sha256.Sum256(public)
	receipt, err := vault.createRecord(ctx, Operation{Kind: OperationCreateVaultRecord, Authority: state}, secrets)
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
	binding := operation.Authority.Binding
	return operation.RecordID == "" && operation.Path == "" && operation.Expected == (AuthorityBinding{}) &&
		operation.Transition == nil && operation.Preparation == nil && operation.Reconciliation == nil &&
		binding.Kind == AuthorityAdmission && binding.Environment != [32]byte{} && binding.Network != [32]byte{} &&
		binding.Root != [32]byte{} && binding.IDCommitment == [32]byte{} && len(operation.Authority.RootMaterial) == 0 &&
		operation.Authority.Generation == 0 && operation.Authority.Revision == 0 && len(operation.Authority.Watermarks) == 0
}

func (vault *Vault) issueAdmissionPermission(ctx context.Context, operation Operation, secrets SecretInput) (Receipt, error) {
	if secrets == nil || !validAdmissionIssuance(operation) {
		return Receipt{}, ErrInvalid
	}
	request, err := credential.DecodePermissionRequest(operation.AdmissionRequest)
	if err != nil || request.Permission.NetworkID != operation.Expected.Network {
		return Receipt{}, ErrInvalid
	}
	now := vault.now().UTC().Truncate(time.Hour)
	if !request.Permission.NotBefore.Equal(now) || !request.Permission.NotAfter.Equal(now.Add(time.Hour)) {
		return Receipt{}, ErrInvalid
	}
	password, err := readPassword(ctx, secrets, SecretPromptVaultUnlock)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(password)
	source, _, err := vault.openCurrentAdmissionAuthority(operation.RecordID, password, operation.Expected)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(source.RootMaterial)
	defer zero(source.AdmissionJournal)
	public := ed25519.PrivateKey(source.RootMaterial).Public().(ed25519.PublicKey)
	if request.AuthorityKey != [ed25519.PublicKeySize]byte(public) {
		return Receipt{}, ErrInvalid
	}
	allocations, err := decodeAdmissionJournal(source.AdmissionJournal)
	if err != nil {
		return Receipt{}, err
	}
	window := uint64(now.Unix())
	if admissionAllocationWindowRegressed(allocations, window) {
		return Receipt{}, ErrInvalid
	}
	allocations = admissionAllocationsForWindow(allocations, window)
	digest := sha256.Sum256(operation.AdmissionRequest)
	for _, allocation := range allocations {
		if allocation.id == request.Permission.PermissionID {
			if allocation.digest != digest {
				return Receipt{}, ErrInvalid
			}
			permission, signErr := signAdmissionPermission(ed25519.PrivateKey(source.RootMaterial), request.Permission)
			if signErr != nil {
				return Receipt{}, signErr
			}
			return Receipt{Operation: OperationIssueAdmissionPermission, RecordID: operation.RecordID,
				Authority: authorityReceipt(source), AdmissionPermission: permission, State: RecordActive}, nil
		}
	}
	allocation := admissionAllocation{window: uint64(now.Unix()), role: request.Role, tokens: permissionTokens(request.Permission.Maxima),
		id: request.Permission.PermissionID, digest: digest}
	if allocation.tokens == 0 || !withinAdmissionBudget(append(allocations, allocation)) {
		return Receipt{}, ErrInvalid
	}
	successor, err := admissionSuccessor(source, append(allocations, allocation))
	if err != nil {
		return Receipt{}, err
	}
	defer zero(successor.RootMaterial)
	defer zero(successor.AdmissionJournal)
	permission, err := signAdmissionPermission(ed25519.PrivateKey(source.RootMaterial), request.Permission)
	if err != nil {
		return Receipt{}, err
	}
	info, err := vault.writeAdmissionSuccessor(operation.RecordID, source, successor, password)
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
		len(operation.ServiceRequest) == 0 && operation.ServiceRequestCommitment == ([32]byte{}) && operation.Path == "" &&
		isZeroAuthorityState(operation.Authority) && operation.Transition == nil && operation.Preparation == nil && operation.Reconciliation == nil
}

func openAdmissionAuthority(raw, password []byte, expected AuthorityBinding) (AuthorityState, EnvelopeInfo, error) {
	purpose, plaintext, info, err := openEnvelope(raw, password)
	if err != nil {
		return AuthorityState{}, EnvelopeInfo{}, err
	}
	defer zero(plaintext)
	if purpose != PurposeVault {
		return AuthorityState{}, EnvelopeInfo{}, ErrInvalid
	}
	state, err := decodeAuthorityState(plaintext, PurposeVault)
	if err != nil {
		return AuthorityState{}, EnvelopeInfo{}, err
	}
	private := ed25519.PrivateKey(state.RootMaterial)
	if state.Binding != expected || state.Binding.Kind != AuthorityAdmission || len(private) != ed25519.PrivateKeySize {
		zero(state.RootMaterial)
		zero(state.AdmissionJournal)
		return AuthorityState{}, EnvelopeInfo{}, ErrInvalid
	}
	public, ok := private.Public().(ed25519.PublicKey)
	if !ok || sha256.Sum256(public) != state.Binding.IDCommitment {
		zero(state.RootMaterial)
		zero(state.AdmissionJournal)
		return AuthorityState{}, EnvelopeInfo{}, ErrInvalid
	}
	if err := validateAdmissionAuthorityState(state); err != nil {
		zero(state.RootMaterial)
		zero(state.AdmissionJournal)
		return AuthorityState{}, EnvelopeInfo{}, err
	}
	return state, info, nil
}

func validateAdmissionAuthorityState(state AuthorityState) error {
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
	_, err := decodeAdmissionJournal(state.AdmissionJournal)
	return err
}

func admissionSuccessor(source AuthorityState, allocations []admissionAllocation) (AuthorityState, error) {
	if source.Generation == math.MaxUint64 || source.Revision == math.MaxUint64 || source.Watermarks[0].Value == math.MaxUint64 {
		return AuthorityState{}, ErrInvalid
	}
	journal, err := encodeAdmissionJournal(allocations)
	if err != nil {
		return AuthorityState{}, err
	}
	return AuthorityState{Binding: source.Binding, RootMaterial: append([]byte(nil), source.RootMaterial...), AdmissionJournal: journal,
		Generation: source.Generation + 1, Revision: source.Revision + 1,
		Watermarks: []Watermark{{Domain: admissionAllocationWatermark, Value: source.Watermarks[0].Value + 1}}}, nil
}

func signAdmissionPermission(private ed25519.PrivateKey, permission credential.Permission) ([]byte, error) {
	if len(private) != ed25519.PrivateKeySize {
		return nil, ErrInvalid
	}
	transcript := make([]byte, 0, len("ardents-issuance-permission-v1\x00")+164)
	transcript = append(transcript, "ardents-issuance-permission-v1\x00"...)
	for _, item := range [][32]byte{permission.NetworkID, permission.IssuerNodeID} {
		transcript = append(transcript, item[:]...)
	}
	transcript = binary.BigEndian.AppendUint64(transcript, permission.DutyGeneration)
	for _, item := range [][32]byte{permission.PermissionID, permission.HolderKey} {
		transcript = append(transcript, item[:]...)
	}
	transcript = binary.BigEndian.AppendUint64(transcript, uint64(permission.NotBefore.Unix()))
	transcript = binary.BigEndian.AppendUint64(transcript, uint64(permission.NotAfter.Unix()))
	for _, maximum := range permission.Maxima {
		transcript = binary.BigEndian.AppendUint32(transcript, maximum)
	}
	copy(permission.Signature[:], ed25519.Sign(private, transcript))
	return credential.EncodePermission(permission)
}

func permissionTokens(maxima [3]uint32) uint32 {
	var total uint64
	for _, maximum := range maxima {
		total += uint64(maximum)
	}
	if total > math.MaxUint32 {
		return 0
	}
	return uint32(total)
}

func withinAdmissionBudget(allocations []admissionAllocation) bool {
	var user, publisher, issuer uint64
	for _, allocation := range allocations {
		if allocation.tokens == 0 || uint64(allocation.tokens) > maximumIssuerAllocation || allocation.window == 0 ||
			allocation.role != credential.AllocationUser && allocation.role != credential.AllocationPublisher {
			return false
		}
		issuer += uint64(allocation.tokens)
		if allocation.role == credential.AllocationUser {
			user += uint64(allocation.tokens)
		} else {
			publisher += uint64(allocation.tokens)
		}
	}
	return user <= maximumUserAllocation && publisher <= maximumPublisherAllocation && issuer <= maximumIssuerAllocation
}

func admissionAllocationsForWindow(allocations []admissionAllocation, window uint64) []admissionAllocation {
	current := make([]admissionAllocation, 0, len(allocations))
	for _, allocation := range allocations {
		if allocation.window == window {
			current = append(current, allocation)
		}
	}
	return current
}

// admissionAllocationWindowRegressed rejects an issuance request behind the
// newest committed hourly reservation. Dropping that reservation on a wall
// clock rollback would make its original hour allocatable again on recovery.
func admissionAllocationWindowRegressed(allocations []admissionAllocation, window uint64) bool {
	for _, allocation := range allocations {
		if allocation.window > window {
			return true
		}
	}
	return false
}

func encodeAdmissionJournal(allocations []admissionAllocation) ([]byte, error) {
	if len(allocations) > int(maximumUserAllocation+maximumPublisherAllocation) || !withinAdmissionBudget(allocations) {
		return nil, ErrInvalid
	}
	ordered := append([]admissionAllocation(nil), allocations...)
	sort.Slice(ordered, func(i, j int) bool { return bytes.Compare(ordered[i].id[:], ordered[j].id[:]) < 0 })
	raw := make([]byte, 0, 12+len(ordered)*admissionJournalEntrySize)
	raw = append(raw, "ARDALJ01"...)
	raw = binary.BigEndian.AppendUint32(raw, uint32(len(ordered)))
	for index, allocation := range ordered {
		if index > 0 && bytes.Compare(ordered[index-1].id[:], allocation.id[:]) >= 0 {
			return nil, ErrInvalid
		}
		raw = binary.BigEndian.AppendUint64(raw, allocation.window)
		raw = append(raw, byte(allocation.role))
		raw = binary.BigEndian.AppendUint32(raw, allocation.tokens)
		raw = append(raw, allocation.id[:]...)
		raw = append(raw, allocation.digest[:]...)
	}
	if len(raw) > maximumAdmissionJournalBytes {
		return nil, ErrInvalid
	}
	return raw, nil
}

func decodeAdmissionJournal(raw []byte) ([]admissionAllocation, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if len(raw) < 12 || len(raw) > maximumAdmissionJournalBytes || string(raw[:8]) != "ARDALJ01" {
		return nil, ErrInvalid
	}
	count := int(binary.BigEndian.Uint32(raw[8:12]))
	if count > int(maximumUserAllocation+maximumPublisherAllocation) || len(raw) != 12+count*admissionJournalEntrySize {
		return nil, ErrInvalid
	}
	allocations := make([]admissionAllocation, count)
	offset := 12
	for index := range allocations {
		allocation := &allocations[index]
		allocation.window = binary.BigEndian.Uint64(raw[offset : offset+8])
		offset += 8
		allocation.role = credential.AllocationRole(raw[offset])
		offset++
		allocation.tokens = binary.BigEndian.Uint32(raw[offset : offset+4])
		offset += 4
		copy(allocation.id[:], raw[offset:offset+32])
		offset += 32
		copy(allocation.digest[:], raw[offset:offset+32])
		offset += 32
		if allocation.id == [32]byte{} || allocation.digest == [32]byte{} || (index > 0 && bytes.Compare(allocations[index-1].id[:], allocation.id[:]) >= 0) {
			return nil, ErrInvalid
		}
	}
	if !withinAdmissionBudget(allocations) {
		return nil, ErrInvalid
	}
	return allocations, nil
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

func (vault *Vault) openCurrentAdmissionAuthority(recordID string, password []byte, expected AuthorityBinding) (AuthorityState, EnvelopeInfo, error) {
	baseRaw, err := readEnvelopeFile(filepath.Join(vault.records, "record-"+recordID+".json"))
	if err != nil {
		return AuthorityState{}, EnvelopeInfo{}, err
	}
	defer zero(baseRaw)
	base, baseInfo, err := openAdmissionAuthority(baseRaw, password, expected)
	if err != nil {
		return AuthorityState{}, EnvelopeInfo{}, err
	}
	floors, err := vault.readFloors()
	if err != nil {
		zero(base.RootMaterial)
		zero(base.AdmissionJournal)
		return AuthorityState{}, EnvelopeInfo{}, err
	}
	floor, found := floorFor(floors, expected)
	if !found {
		zero(base.RootMaterial)
		zero(base.AdmissionJournal)
		return AuthorityState{}, EnvelopeInfo{}, ErrInvalid
	}
	path, err := admissionLedgerPath(vault.root, recordID)
	if err != nil {
		zero(base.RootMaterial)
		zero(base.AdmissionJournal)
		return AuthorityState{}, EnvelopeInfo{}, err
	}
	raw, err := readEnvelopeFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if !floorEqualsState(floor, base) {
			zero(base.RootMaterial)
			zero(base.AdmissionJournal)
			return AuthorityState{}, EnvelopeInfo{}, ErrInvalid
		}
		return base, baseInfo, nil
	}
	if err != nil {
		zero(base.RootMaterial)
		zero(base.AdmissionJournal)
		return AuthorityState{}, EnvelopeInfo{}, err
	}
	defer zero(raw)
	current, info, err := openAdmissionAuthority(raw, password, expected)
	if err != nil {
		zero(base.RootMaterial)
		zero(base.AdmissionJournal)
		return AuthorityState{}, EnvelopeInfo{}, err
	}
	if floorEqualsState(floor, current) {
		zero(base.RootMaterial)
		zero(base.AdmissionJournal)
		return current, info, nil
	}
	if !floorEqualsState(floor, base) || !admissionLedgerFollowsFloor(current, floor) {
		zero(base.RootMaterial)
		zero(base.AdmissionJournal)
		zero(current.RootMaterial)
		zero(current.AdmissionJournal)
		return AuthorityState{}, EnvelopeInfo{}, ErrInvalid
	}
	if err := vault.advanceFloor(current); err != nil {
		zero(base.RootMaterial)
		zero(base.AdmissionJournal)
		zero(current.RootMaterial)
		zero(current.AdmissionJournal)
		return AuthorityState{}, EnvelopeInfo{}, err
	}
	zero(base.RootMaterial)
	zero(base.AdmissionJournal)
	return current, info, nil
}

func admissionLedgerFollowsFloor(state AuthorityState, floor authorityFloor) bool {
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

func (vault *Vault) writeAdmissionSuccessor(recordID string, source, expected AuthorityState, password []byte) (EnvelopeInfo, error) {
	path, err := admissionLedgerPath(vault.root, recordID)
	if err != nil {
		return EnvelopeInfo{}, err
	}
	raw, err := readEnvelopeFile(path)
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
