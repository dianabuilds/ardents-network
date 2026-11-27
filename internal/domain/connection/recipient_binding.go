package connection

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/recipient"
	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
	executionruntime "github.com/dianabuilds/ardents-network/internal/domain/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/domain/publication"
	"github.com/dianabuilds/ardents-network/internal/domain/route/capsule"
)

// RecipientBinding seals an initial logical tuple and its fresh Attachment
// transcript. It retains the exact qualified operation, never a supplied
// permission flag. Service authentication and replay commit remain separate.
type RecipientBinding struct {
	ctx       context.Context
	operation *executionruntime.Operation
	facts     recipient.Facts
}

// BindRecipient checks independently verified Publication facts against one
// decrypted candidate. localEnd comes from the live composition's original
// local/Network bounds; it cannot be extended by candidate Work Safety fields.
// Recovery requires retained continuity and is unavailable at this boundary.
func BindRecipient(ctx context.Context, operation *executionruntime.Operation, proof publication.Proof, profile [32]byte, request capsule.Request, plaintextDigest [32]byte, localEnd time.Time) (*RecipientBinding, error) {
	if ctx == nil || operation == nil {
		return nil, errors.New("recipient binding original operation absent")
	}
	if err := errors.Join(ctx.Err(), operation.CheckPublisher()); err != nil {
		return nil, err
	}
	facts, err := deriveRecipientGeneration(proof, profile, request, plaintextDigest, localEnd, time.Now(), 1, wire.ProfileForOperation(operation.UsesByteStream()))
	if err != nil {
		return nil, err
	}
	binding := &RecipientBinding{ctx: ctx, operation: operation, facts: facts}
	if err := binding.Check(ctx, operation); err != nil {
		return nil, err
	}
	return binding, nil
}

// deriveRecipientGeneration checks the original capsule grammar and passes the
// existing logical identity shape to the shared Publication/transcript rule.
func deriveRecipientGeneration(proof publication.Proof, profile [32]byte, request capsule.Request,
	plaintextDigest [32]byte, localEnd, now time.Time, generation uint64, application wire.ApplicationProfile) (recipient.Facts, error) {
	actualDigest, err := request.Digest()
	if err != nil || actualDigest != plaintextDigest {
		return recipient.Facts{}, errors.Join(errors.New("recipient plaintext commitment differs"), err)
	}
	if request.ProfileDigest != profile || request.JoinSecret == [32]byte{} || request.HandshakeContext == [32]byte{} ||
		request.RendezvousNode == [32]byte{} || request.RendezvousDutyGeneration == 0 || request.AttachmentGeneration != generation {
		return recipient.Facts{}, recipient.ErrBinding
	}
	identity := wire.LogicalIdentity{Network: request.Network, Target: request.Target, Application: application,
		Publication: request.PublicationDigest, Profile: profile, ConnectionNonce: request.ConnectionNonce,
		InitiatorBinding: request.InitiatorBinding, WorkSafetyNotAfter: request.WorkSafetyNotAfter,
		WorkSafetyMaximum: request.WorkSafetyMaximum, NoNewRecoveryAfter: request.NoNewRecoveryAfter}
	return recipient.Derive(proof, identity, plaintextDigest, request.Deadline, localEnd, now, generation, request.Revision)
}

// Check is an initial recipient handoff check under the original capsule expiry
// and operation. It cannot rebind this tuple to another Job or accept recovery.
func (binding *RecipientBinding) Check(ctx context.Context, operation *executionruntime.Operation) error {
	if binding == nil || ctx == nil || binding.ctx == nil || operation == nil || binding.operation != operation || binding.facts.Logical == [32]byte{} || !time.Now().Before(binding.facts.Deadline) {
		return errors.New("recipient binding original lifetime unavailable")
	}
	return errors.Join(ctx.Err(), binding.ctx.Err(), operation.CheckPublisher())
}

// Contexts are copied transcript commitments, not authenticated stream handles.
func (binding *RecipientBinding) Contexts() ([32]byte, [32]byte) {
	if binding == nil {
		return [32]byte{}, [32]byte{}
	}
	return binding.facts.Logical, binding.facts.Attachment
}
