package custody

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/instance"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

const maximumServiceCredentialLifetime = 24 * time.Hour
const maximumServiceCredentialHorizon = 48 * time.Hour

func (vault *Vault) issueServiceCredential(ctx context.Context, operation Operation, secrets SecretInput) (Receipt, error) {
	if secrets == nil || !validServiceIssuance(operation) {
		return Receipt{}, ErrInvalid
	}
	request, err := instance.ParseRequest(operation.ServiceRequest)
	if err != nil || request.NetworkID != operation.Expected.Network ||
		!boundedServiceCredentialValidity(request, time.Now().UTC()) {
		return Receipt{}, ErrInvalid
	}
	sourceRaw, err := readEnvelopeFile(filepath.Join(vault.records, "record-"+operation.RecordID+".json"))
	if err != nil {
		return Receipt{}, err
	}
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
	successor, credential, err := serviceSuccessor(source, request)
	if err != nil {
		return Receipt{}, err
	}
	response, err := instance.BuildResponse(operation.ServiceRequest, credential)
	if err != nil {
		return Receipt{}, err
	}
	successorID := serviceSuccessorRecordID(operation.RecordID, request.Commitment)
	floors, err := vault.readFloors()
	if err != nil {
		return Receipt{}, err
	}
	floor, found := floorFor(floors, source.Binding)
	if !found {
		return Receipt{}, ErrInvalid
	}
	sourceCurrent, successorCurrent := floorEqualsState(floor, source), floorEqualsState(floor, successor)
	if !sourceCurrent && !successorCurrent {
		return Receipt{}, ErrInvalid
	}
	successorRaw, info, err := vault.ensureServiceSuccessor(successorID, successor, password)
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

func boundedServiceCredentialValidity(request instance.RequestView, at time.Time) bool {
	if request.NotBefore < 0 || request.NotAfter < 0 || request.NotAfter <= request.NotBefore {
		return false
	}
	lifetimeSeconds := request.NotAfter - request.NotBefore
	horizonSeconds := request.NotAfter - at.Unix()
	return lifetimeSeconds <= int64(maximumServiceCredentialLifetime/time.Second) &&
		horizonSeconds > 0 && horizonSeconds <= int64(maximumServiceCredentialHorizon/time.Second)
}

func validServiceIssuance(operation Operation) bool {
	requestCommitment := sha256.Sum256(operation.ServiceRequest)
	return validRecordID(operation.RecordID) && operation.Expected.Kind == AuthorityService &&
		operation.Expected != (AuthorityBinding{}) && len(operation.ServiceRequest) != 0 && operation.Path == "" &&
		operation.ServiceRequestCommitment == requestCommitment &&
		isZeroAuthorityState(operation.Authority) && operation.Transition == nil && operation.Preparation == nil &&
		operation.Reconciliation == nil
}

func openServiceAuthority(raw, password []byte, expected AuthorityBinding) (AuthorityState, EnvelopeInfo, error) {
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
	if len(private) != ed25519.PrivateKeySize {
		zero(state.RootMaterial)
		return AuthorityState{}, EnvelopeInfo{}, ErrInvalid
	}
	public, ok := private.Public().(ed25519.PublicKey)
	if state.Binding != expected || state.Binding.Kind != AuthorityService || !ok ||
		sha256.Sum256(public) != state.Binding.IDCommitment {
		zero(state.RootMaterial)
		return AuthorityState{}, EnvelopeInfo{}, ErrInvalid
	}
	return state, info, nil
}

func serviceSuccessor(source AuthorityState, request instance.RequestView) (AuthorityState, publication.Credential, error) {
	if source.Generation == math.MaxUint64 || source.Revision == math.MaxUint64 || len(source.Watermarks) != 2 ||
		source.Watermarks[0].Domain != serviceCredentialWatermark ||
		source.Watermarks[1].Domain != serviceCredentialNotAfterWatermark ||
		source.Watermarks[0].Value == math.MaxUint64 || source.Watermarks[1].Value > math.MaxInt64 ||
		source.Generation != source.Watermarks[0].Value+1 || source.Revision != source.Watermarks[0].Value ||
		request.NotBefore < int64(source.Watermarks[1].Value) ||
		request.NotAfter <= request.NotBefore || request.NotAfter < 0 {
		return AuthorityState{}, publication.Credential{}, ErrInvalid
	}
	generation := source.Watermarks[0].Value + 1
	successor := AuthorityState{Binding: source.Binding, RootMaterial: source.RootMaterial,
		Generation: source.Generation + 1, Revision: source.Revision + 1,
		Watermarks: []Watermark{{Domain: serviceCredentialWatermark, Value: generation},
			{Domain: serviceCredentialNotAfterWatermark, Value: uint64(request.NotAfter)}}}
	credential, err := (publication.Credential{InstancePublic: request.InstancePublic,
		IntroductionHPKEPublic: request.IntroductionPublic, Generation: generation,
		NotBefore: request.NotBefore, NotAfter: request.NotAfter, NetworkID: request.NetworkID,
		Capabilities: publication.CapabilityPublish | publication.CapabilityConnect}).Issue(ed25519.PrivateKey(source.RootMaterial))
	if err != nil {
		return AuthorityState{}, publication.Credential{}, err
	}
	return successor, credential, nil
}

func serviceSuccessorRecordID(recordID string, commitment [32]byte) string {
	record, _ := hex.DecodeString(recordID)
	digest := sha256.Sum256(append(append([]byte("ardents-service-authority-successor-v1\x00"), record...), commitment[:]...))
	return hex.EncodeToString(digest[:16])
}

func (vault *Vault) ensureServiceSuccessor(recordID string, expected AuthorityState, password []byte) ([]byte, EnvelopeInfo, error) {
	path := filepath.Join(vault.records, "record-"+recordID+".json")
	raw, err := readEnvelopeFile(path)
	if err == nil {
		state, info, openErr := openServiceAuthority(raw, password, expected.Binding)
		if openErr != nil || !sameAuthorityState(state, expected) {
			zero(state.RootMaterial)
			zero(raw)
			return nil, EnvelopeInfo{}, ErrInvalid
		}
		zero(state.RootMaterial)
		return raw, info, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, EnvelopeInfo{}, err
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

func floorEqualsState(floor authorityFloor, state AuthorityState) bool {
	return floor.Generation == state.Generation && floor.Revision == state.Revision &&
		equalWatermarks(floor.Watermarks, state.Watermarks)
}

func sameAuthorityState(left, right AuthorityState) bool {
	return left.Binding == right.Binding && left.Generation == right.Generation && left.Revision == right.Revision &&
		equalWatermarks(left.Watermarks, right.Watermarks) && string(left.RootMaterial) == string(right.RootMaterial)
}
