package custody

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"

	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

const serviceCredentialWatermark = "credential-generation"
const serviceCredentialNotAfterWatermark = "credential-not-after"

func (vault *Vault) createServiceAuthority(ctx context.Context, operation Operation, secrets SecretInput) (Receipt, error) {
	if secrets == nil || !validServiceAuthorityCreation(operation) {
		return Receipt{}, ErrInvalid
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Receipt{}, err
	}
	defer zero(private)
	state := AuthorityState{
		Binding:    operation.Authority.Binding,
		Generation: 1,
		Watermarks: []Watermark{{Domain: serviceCredentialWatermark, Value: 0},
			{Domain: serviceCredentialNotAfterWatermark, Value: 0}},
	}
	state.Binding.IDCommitment = sha256.Sum256(public)
	state.RootMaterial = private
	receipt, err := vault.createRecord(ctx, Operation{Kind: OperationCreateVaultRecord, Authority: state}, secrets)
	if err != nil {
		return Receipt{}, err
	}
	var authorityPublic [32]byte
	copy(authorityPublic[:], public)
	receipt.Operation = OperationCreateServiceAuthority
	receipt.ServiceAuthority = ServiceAuthorityReceipt{Public: authorityPublic, Target: publication.Target(authorityPublic)}
	return receipt, nil
}

func validServiceAuthorityCreation(operation Operation) bool {
	binding := operation.Authority.Binding
	return operation.RecordID == "" && operation.Path == "" && operation.Expected == (AuthorityBinding{}) &&
		operation.Transition == nil && operation.Preparation == nil && operation.Reconciliation == nil &&
		binding.Kind == AuthorityService && binding.Environment != [32]byte{} && binding.Network != [32]byte{} &&
		binding.Root != [32]byte{} && binding.IDCommitment == [32]byte{} && len(operation.Authority.RootMaterial) == 0 &&
		operation.Authority.Generation == 0 && operation.Authority.Revision == 0 && len(operation.Authority.Watermarks) == 0
}
