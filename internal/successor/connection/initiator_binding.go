package connection

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/publication"
	"github.com/dianabuilds/ardents-network/internal/successor/route/capsule"
)

// InitiatorBinding retains the independently verified destination and original
// Reader context. The local secret context/salt are not capsule fields.
type InitiatorBinding struct {
	self                          *InitiatorBinding
	operation                     *executionruntime.Operation
	setup                         context.Context
	identity                      wire.LogicalIdentity
	logical, attachment           [32]byte
	localContext                  [32]byte
	joinSecretDigest, joinContext [32]byte
	expires                       time.Time
	revision                      uint64
	recipientEnd                  time.Time
	proof                         publication.Proof
	source                        *InitiatorSource
}

// prepareInitiator fills only fresh logical fields under the already claimed
// original Source. It cannot claim a second operation or accept raw copied
// request facts as Source/JOIN authority.
func prepareInitiator(ctx context.Context, source *InitiatorSource, proof publication.Proof, profile [32]byte,
	request capsule.Request, localEnd time.Time) (*InitiatorBinding, capsule.Request, error) {
	if ctx == nil || source == nil || request.ConnectionNonce != [32]byte{} || request.InitiatorBinding != [32]byte{} {
		return nil, capsule.Request{}, errIntegrity
	}
	if err := ctx.Err(); err != nil {
		return nil, capsule.Request{}, err
	}
	if err := source.check(); err != nil {
		return nil, capsule.Request{}, err
	}
	operation, localContext := source.config.Operation, source.localContext
	var salt [32]byte
	defer clear(salt[:])
	if _, err := rand.Read(salt[:]); err != nil || salt == [32]byte{} {
		return nil, capsule.Request{}, errors.New("initiator binding salt is unavailable")
	}
	if _, err := rand.Read(request.ConnectionNonce[:]); err != nil || request.ConnectionNonce == [32]byte{} {
		return nil, capsule.Request{}, errors.New("logical Connection nonce is unavailable")
	}
	transcript := append([]byte("ardents-initiator-binding-v3\x00"), salt[:]...)
	transcript = append(transcript, localContext[:]...)
	request.InitiatorBinding = sha256.Sum256(transcript)
	clear(transcript)
	digest, err := request.Digest()
	if err != nil {
		return nil, capsule.Request{}, err
	}
	derived, err := deriveRecipientBinding(proof, profile, request, digest, localEnd, time.Now())
	if err != nil {
		return nil, capsule.Request{}, err
	}
	if end, exists := operation.Context().Deadline(); exists && time.Unix(request.WorkSafetyMaximum, 0).After(end) {
		return nil, capsule.Request{}, errIntegrity
	}
	if err := ctx.Err(); err != nil {
		return nil, capsule.Request{}, err
	}
	if err := source.check(); err != nil {
		return nil, capsule.Request{}, err
	}
	binding := &InitiatorBinding{operation: operation, setup: ctx, identity: derived.identity, logical: derived.logical,
		attachment: derived.attachment, localContext: localContext, expires: request.Deadline, revision: request.Revision, proof: proof,
		joinSecretDigest: sha256.Sum256(request.JoinSecret[:]), joinContext: request.HandshakeContext, source: source}
	binding.self = binding
	if err := binding.check(true); err != nil {
		return nil, capsule.Request{}, err
	}
	return binding, request, nil
}

func (binding *InitiatorBinding) check(setup bool) error {
	if binding == nil || binding.self != binding || binding.operation == nil || binding.setup == nil || binding.source == nil {
		return errIntegrity
	}
	if err := binding.source.check(); err != nil {
		return err
	}
	localContext, err := binding.operation.ConnectionContext()
	if err != nil || localContext != binding.localContext {
		return errors.Join(errIntegrity, err)
	}
	now := time.Now()
	value := binding.proof.Delegation()
	if now.Before(value.NotBefore) || !now.Before(value.NotAfter) || !now.Before(time.Unix(binding.identity.WorkSafetyNotAfter, 0)) {
		return errIntegrity
	}
	if setup && (binding.setup.Err() != nil || !now.Before(binding.expires)) {
		return errors.Join(errIntegrity, binding.setup.Err())
	}
	if setup && !binding.recipientEnd.IsZero() && !now.Before(binding.recipientEnd) {
		return errIntegrity
	}
	return nil
}
