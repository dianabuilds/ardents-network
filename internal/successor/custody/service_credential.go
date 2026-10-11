package custody

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/publication"
)

const maximumServiceCredentialLifetime = 24 * time.Hour
const maximumServiceCredentialHorizon = 48 * time.Hour

func (vault *Vault) issueServiceCredential(ctx context.Context, operation Operation, secrets SecretInput) (Receipt, error) {
	if secrets == nil || !validServiceIssuance(operation) {
		return Receipt{}, ErrInvalid
	}
	if err := vault.requireIndependentRoot(); err != nil {
		return Receipt{}, err
	}
	request, err := publication.ParseServiceRequest(operation.ServiceRequest)
	if err != nil || request.NetworkID != operation.Expected.Network {
		return Receipt{}, ErrInvalid
	}
	expired, supported := serviceCredentialValidity(request, vault.now().UTC())
	if !supported {
		return Receipt{}, ErrInvalid
	}
	successorID := serviceSuccessorRecordID(operation.RecordID, request.Commitment)
	if expired {
		existing, readErr := readEnvelopeFile(filepath.Join(vault.records, "record-"+successorID+".json"))
		zero(existing)
		if readErr != nil {
			return Receipt{}, ErrInvalid
		}
	}
	sourceRaw, err := readEnvelopeFile(filepath.Join(vault.records, "record-"+operation.RecordID+".json"))
	if err != nil {
		return Receipt{}, err
	}
	defer zero(sourceRaw)
	password, err := readPassword(ctx, secrets, SecretPromptVaultUnlock)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(password)
	source, _, err := openServiceAuthority(sourceRaw, password, operation.Expected)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(source.RootMaterial)
	defer zero(source.ServiceJournal)
	successor, transcript, err := serviceSuccessor(source, request)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(successor.ServiceJournal)
	floors, err := vault.readFloors()
	if err != nil {
		return Receipt{}, err
	}
	floor, found := floorFor(floors, source.Binding)
	if !found {
		return Receipt{}, ErrInvalid
	}
	expired, supported = serviceCredentialValidity(request, vault.now().UTC())
	if !supported {
		return Receipt{}, ErrInvalid
	}
	sourceCurrent, successorCurrent := floorEqualsState(floor, source), floorEqualsState(floor, successor)
	if !sourceCurrent && !successorCurrent {
		return Receipt{}, ErrInvalid
	}
	if expired {
		// Prove the original authenticated exact successor already exists before
		// returning its deterministic response or completing its pending floor.
		raw, _, err := vault.ensureServiceSuccessor(successorID, successor, password, false, request)
		zero(raw)
		if err != nil {
			return Receipt{}, err
		}
	}
	if err := vault.check(); err != nil {
		return Receipt{}, err
	}
	credential := append(transcript, ed25519.Sign(ed25519.PrivateKey(source.RootMaterial), transcript)...)
	response, err := publication.BuildServiceResponse(operation.ServiceRequest, credential)
	if err != nil {
		return Receipt{}, err
	}
	successorRaw, info, err := vault.ensureServiceSuccessor(successorID, successor, password, sourceCurrent && !expired, request)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(successorRaw)
	if sourceCurrent {
		if err := vault.advanceFloor(successor); err != nil {
			return Receipt{}, err
		}
	} else if err := vault.matchesFloor(successor); err != nil {
		return Receipt{}, err
	}
	var public [32]byte
	copy(public[:], ed25519.PrivateKey(source.RootMaterial).Public().(ed25519.PublicKey))
	return Receipt{Operation: OperationIssueServiceCredential, RecordID: successorID, Envelope: info,
		Authority: authorityReceipt(successor), ServiceAuthority: ServiceAuthorityReceipt{Public: public,
			Target: publication.Target(public)}, ServiceResponse: response, State: RecordActive}, nil
}

func serviceCredentialValidity(request publication.RequestView, at time.Time) (expired bool, supported bool) {
	if request.NotBefore < 0 || request.NotAfter < 0 || request.NotAfter <= request.NotBefore {
		return false, false
	}
	lifetimeSeconds := request.NotAfter - request.NotBefore
	nowSeconds := at.Unix()
	if nowSeconds < 0 || lifetimeSeconds > int64(maximumServiceCredentialLifetime/time.Second) {
		return false, false
	}
	if request.NotAfter <= nowSeconds {
		return true, true
	}
	return false, request.NotAfter-nowSeconds <= int64(maximumServiceCredentialHorizon/time.Second)
}

func validServiceIssuance(operation Operation) bool {
	requestCommitment := sha256.Sum256(operation.ServiceRequest)
	return validRecordID(operation.RecordID) && operation.Expected.Kind == AuthorityService &&
		operation.Expected != (AuthorityBinding{}) && len(operation.ServiceRequest) != 0 && operation.Path == "" &&
		operation.ServiceRequestCommitment == requestCommitment &&
		isZeroAuthorityState(operation.authority)
}

func openServiceAuthority(raw, password []byte, expected AuthorityBinding) (authorityState, EnvelopeInfo, error) {
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
	if len(private) != ed25519.PrivateKeySize {
		zero(state.RootMaterial)
		zero(state.ServiceJournal)
		return authorityState{}, EnvelopeInfo{}, ErrInvalid
	}
	public, ok := private.Public().(ed25519.PublicKey)
	if state.Binding != expected || state.Binding.Kind != AuthorityService || !ok ||
		sha256.Sum256(public) != state.Binding.IDCommitment {
		zero(state.RootMaterial)
		zero(state.ServiceJournal)
		return authorityState{}, EnvelopeInfo{}, ErrInvalid
	}
	derived := ed25519.NewKeyFromSeed(private[:32])
	matches := bytes.Equal(derived, private)
	zero(derived)
	if !matches {
		zero(state.RootMaterial)
		zero(state.ServiceJournal)
		return authorityState{}, EnvelopeInfo{}, ErrInvalid
	}
	return state, info, nil
}

func serviceSuccessor(source authorityState, request publication.RequestView) (authorityState, []byte, error) {
	if source.Generation == math.MaxUint64 || source.Revision == math.MaxUint64 || len(source.Watermarks) != 2 ||
		source.Watermarks[0].Domain != serviceCredentialWatermark ||
		source.Watermarks[1].Domain != serviceCredentialNotAfterWatermark ||
		source.Watermarks[0].Value == math.MaxUint64 || source.Watermarks[1].Value > math.MaxInt64 ||
		source.Generation != source.Watermarks[0].Value+1 || source.Revision != source.Watermarks[0].Value ||
		request.NotBefore < int64(source.Watermarks[1].Value) ||
		request.NotAfter <= request.NotBefore || request.NotAfter < 0 {
		return authorityState{}, nil, ErrInvalid
	}
	if source.Generation == 1 && len(source.ServiceJournal) != 0 || source.Generation > 1 && len(source.ServiceJournal) == 0 {
		return authorityState{}, nil, ErrInvalid
	}
	if source.Generation > 1 {
		previous, err := publication.ParseServiceRequest(source.ServiceJournal)
		if err != nil || previous.NetworkID != source.Binding.Network || uint64(previous.NotAfter) != source.Watermarks[1].Value {
			return authorityState{}, nil, ErrInvalid
		}
	}
	generation := source.Watermarks[0].Value + 1
	successor := authorityState{Binding: source.Binding, RootMaterial: source.RootMaterial,
		ServiceJournal: publication.EncodeServiceRequest(request),
		Generation:     source.Generation + 1, Revision: source.Revision + 1,
		Watermarks: []Watermark{{Domain: serviceCredentialWatermark, Value: generation},
			{Domain: serviceCredentialNotAfterWatermark, Value: uint64(request.NotAfter)}}}
	private := ed25519.PrivateKey(source.RootMaterial)
	var public [32]byte
	copy(public[:], private.Public().(ed25519.PublicKey))
	transcript, err := publication.ServiceCredentialTranscript(request, public, generation)
	if err != nil {
		return authorityState{}, nil, err
	}
	return successor, transcript, nil
}

func serviceSuccessorRecordID(recordID string, commitment [32]byte) string {
	record, _ := hex.DecodeString(recordID)
	digest := sha256.Sum256(append(append([]byte("ardents-service-authority-successor-v1\x00"), record...), commitment[:]...))
	return hex.EncodeToString(digest[:16])
}

func (vault *Vault) ensureServiceSuccessor(recordID string, expected authorityState, password []byte, allowCreation bool, request publication.RequestView) ([]byte, EnvelopeInfo, error) {
	path := filepath.Join(vault.records, "record-"+recordID+".json")
	raw, err := readEnvelopeFile(path)
	if err == nil {
		state, info, openErr := openServiceAuthority(raw, password, expected.Binding)
		if openErr != nil || !sameAuthorityState(state, expected) {
			zero(state.RootMaterial)
			zero(state.ServiceJournal)
			zero(raw)
			return nil, EnvelopeInfo{}, ErrInvalid
		}
		zero(state.RootMaterial)
		zero(state.ServiceJournal)
		return raw, info, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, EnvelopeInfo{}, err
	}
	if !allowCreation {
		return nil, EnvelopeInfo{}, ErrInvalid
	}
	plaintext, err := encodeAuthorityState(PurposeVault, expected)
	if err != nil {
		return nil, EnvelopeInfo{}, err
	}
	defer zero(plaintext)
	envelope, err := sealEnvelope(PurposeVault, plaintext, password)
	if err != nil {
		return nil, EnvelopeInfo{}, err
	}
	// Sealing performs expensive password derivation. Check again at the first
	// durable effect; after it commits, floor completion must remain recoverable.
	if expired, supported := serviceCredentialValidity(request, vault.now().UTC()); expired || !supported {
		zero(envelope)
		return nil, EnvelopeInfo{}, ErrInvalid
	}
	if err := vault.writeRecord(recordID, envelope); err != nil {
		zero(envelope)
		return nil, EnvelopeInfo{}, err
	}
	info, err := inspectEnvelope(envelope)
	if err != nil {
		zero(envelope)
		return nil, EnvelopeInfo{}, err
	}
	return envelope, info, err
}
