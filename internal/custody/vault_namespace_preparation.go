package custody

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"fmt"
	"math"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace/authority"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/record"
)

func (vault *Vault) prepareNamespaceSubmission(ctx context.Context, operation Operation, secrets SecretInput) (Receipt, error) {
	if secrets == nil || operation.Preparation == nil || operation.Transition != nil || operation.Reconciliation != nil || operation.Path != "" ||
		!isZeroAuthorityState(operation.Authority) || !validRecordID(operation.RecordID) {
		return Receipt{}, ErrInvalid
	}
	state, info, private, err := vault.activeNameKey(ctx, operation, secrets)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(state.RootMaterial)
	defer zero(private)
	signer := custodyControlSigner{private: private, generation: state.Generation, revision: state.Revision}
	submission, err := operation.Preparation(&signer)
	defer signer.erase()
	if err != nil || signer.phase != 2 || !submission.MatchesSignatures(signer.transition, signer.successor) {
		return Receipt{}, ErrInvalid
	}
	return Receipt{Operation: OperationPrepareNamespaceSubmission, RecordID: operation.RecordID,
		Envelope: info, Authority: authorityReceipt(state), State: RecordActive, Submission: submission.Canonical()}, nil
}

func (vault *Vault) activeNameKey(ctx context.Context, operation Operation, secrets SecretInput) (AuthorityState, EnvelopeInfo, ed25519.PrivateKey, error) {
	raw, err := readEnvelopeFile(filepath.Join(vault.records, "record-"+operation.RecordID+".json"))
	if err != nil {
		return AuthorityState{}, EnvelopeInfo{}, nil, fmt.Errorf("read vault record: %w", err)
	}
	password, err := readPassword(ctx, secrets, SecretPromptVaultUnlock)
	if err != nil {
		return AuthorityState{}, EnvelopeInfo{}, nil, err
	}
	defer zero(password)
	purpose, plaintext, info, err := openEnvelope(raw, password)
	if err != nil {
		return AuthorityState{}, EnvelopeInfo{}, nil, err
	}
	defer zero(plaintext)
	if purpose != PurposeVault {
		return AuthorityState{}, EnvelopeInfo{}, nil, ErrInvalid
	}
	state, err := decodeAuthorityState(plaintext, PurposeVault)
	if err != nil {
		return AuthorityState{}, EnvelopeInfo{}, nil, err
	}
	if state.Binding != operation.Expected || state.Binding.Kind != AuthorityName {
		zero(state.RootMaterial)
		return AuthorityState{}, EnvelopeInfo{}, nil, ErrInvalid
	}
	if err := vault.matchesFloor(state); err != nil {
		zero(state.RootMaterial)
		return AuthorityState{}, EnvelopeInfo{}, nil, err
	}
	private := ed25519.PrivateKey(append([]byte(nil), state.RootMaterial...))
	public, ok := private.Public().(ed25519.PublicKey)
	if len(private) != ed25519.PrivateKeySize || !ok || sha256.Sum256(public) != state.Binding.IDCommitment {
		zero(state.RootMaterial)
		zero(private)
		return AuthorityState{}, EnvelopeInfo{}, nil, ErrInvalid
	}
	return state, info, private, nil
}

type custodyControlSigner struct {
	private    ed25519.PrivateKey
	transition []byte
	successor  []byte
	generation uint64
	revision   uint64
	phase      uint8
}

func (signer *custodyControlSigner) SignTransition(request authority.TransitionSigningRequest) ([]byte, error) {
	if signer == nil || signer.phase != 0 || len(signer.private) != ed25519.PrivateKeySize || !signer.matches(request.Authority()) {
		return nil, ErrInvalid
	}
	generation, revision := request.Predecessor()
	if generation != signer.generation || revision != signer.revision {
		return nil, ErrInvalid
	}
	signer.transition = ed25519.Sign(signer.private, request.Transcript())
	signer.phase = 1
	return append([]byte(nil), signer.transition...), nil
}

func (signer *custodyControlSigner) SignRecord(request record.RecordSigningRequest) ([]byte, error) {
	if signer == nil || signer.phase != 1 || len(signer.private) != ed25519.PrivateKeySize || !signer.matches(request.Authority()) || signer.revision == math.MaxUint64 {
		return nil, ErrInvalid
	}
	generation, revision := request.Successor()
	if generation != signer.generation || revision != signer.revision+1 {
		return nil, ErrInvalid
	}
	signer.successor = ed25519.Sign(signer.private, request.Transcript())
	signer.phase = 2
	return append([]byte(nil), signer.successor...), nil
}

func (signer *custodyControlSigner) matches(authorityKey [ed25519.PublicKeySize]byte) bool {
	public, ok := signer.private.Public().(ed25519.PublicKey)
	return ok && authorityKey == [ed25519.PublicKeySize]byte(public)
}

func (signer *custodyControlSigner) erase() {
	if signer == nil {
		return
	}
	zero(signer.private)
	zero(signer.transition)
	zero(signer.successor)
	signer.private, signer.transition, signer.successor = nil, nil, nil
}
