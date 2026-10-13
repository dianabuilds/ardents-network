package connection

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/publication"
	"github.com/dianabuilds/ardents-network/internal/successor/route/capsule"
)

// RecipientBinding seals an initial logical tuple and its fresh Attachment
// transcript. It retains the exact qualified operation, never a supplied
// permission flag. Service authentication and replay commit remain separate.
type RecipientBinding struct {
	ctx                 context.Context
	operation           *executionruntime.Operation
	logical, attachment [32]byte
	identity            wire.LogicalIdentity
	expires             time.Time
	revision            uint64
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
	binding, err := deriveRecipientBinding(proof, profile, request, plaintextDigest, localEnd, time.Now())
	if err != nil {
		return nil, err
	}
	binding.operation = operation
	binding.ctx = ctx
	if err := binding.Check(ctx, operation); err != nil {
		return nil, err
	}
	return binding, nil
}

func deriveRecipientBinding(proof publication.Proof, profile [32]byte, request capsule.Request, plaintextDigest [32]byte, localEnd, now time.Time) (*RecipientBinding, error) {
	return deriveRecipientGeneration(proof, profile, request, plaintextDigest, localEnd, now, 1)
}

func deriveRecipientGeneration(proof publication.Proof, profile [32]byte, request capsule.Request, plaintextDigest [32]byte, localEnd, now time.Time, generation uint64) (*RecipientBinding, error) {
	actualDigest, err := request.Digest()
	if err != nil || actualDigest != plaintextDigest {
		return nil, errors.Join(errors.New("recipient plaintext commitment differs"), err)
	}
	value := proof.Delegation()
	if proof.Digest() == [32]byte{} || value.Generation == 0 || value.Instance == [32]byte{} ||
		profile == [32]byte{} || plaintextDigest == [32]byte{} || now.Before(value.NotBefore) || !now.Before(value.NotAfter) ||
		request.Network != value.Network || request.Target != value.Target || request.PublicationDigest != proof.Digest() || request.ProfileDigest != profile ||
		request.ConnectionNonce == [32]byte{} || request.InitiatorBinding == [32]byte{} || request.JoinSecret == [32]byte{} || request.HandshakeContext == [32]byte{} ||
		request.RendezvousNode == [32]byte{} || request.RendezvousDutyGeneration == 0 || request.Revision == 0 || generation == 0 || request.AttachmentGeneration != generation ||
		request.Deadline != request.Deadline.UTC().Truncate(time.Second) || !now.Before(request.Deadline) || request.Deadline.After(now.Add(10*time.Second)) ||
		request.WorkSafetyNotAfter <= now.Unix() || request.WorkSafetyMaximum < request.WorkSafetyNotAfter ||
		request.NoNewRecoveryAfter <= 0 || request.NoNewRecoveryAfter > request.WorkSafetyNotAfter || request.Deadline.Unix() > request.WorkSafetyNotAfter ||
		localEnd.IsZero() || time.Unix(request.WorkSafetyMaximum, 0).After(localEnd) || time.Unix(request.WorkSafetyMaximum, 0).After(value.NotAfter) {
		return nil, errors.New("recipient immutable binding or safety bounds differ")
	}
	identity := wire.LogicalIdentity{Network: value.Network, Target: value.Target, Instance: value.Instance,
		InstanceGeneration: value.Generation, Publication: proof.Digest(), Profile: profile,
		ConnectionNonce: request.ConnectionNonce, InitiatorBinding: request.InitiatorBinding,
		WorkSafetyNotAfter: request.WorkSafetyNotAfter, WorkSafetyMaximum: request.WorkSafetyMaximum, NoNewRecoveryAfter: request.NoNewRecoveryAfter}
	logical, attachment, err := wire.Contexts(identity, plaintextDigest, request.AttachmentGeneration)
	if err != nil {
		return nil, err
	}
	return &RecipientBinding{identity: identity, logical: logical, attachment: attachment, expires: request.Deadline, revision: request.Revision}, nil
}

// Check is an initial recipient handoff check under the original capsule expiry
// and operation. It cannot rebind this tuple to another Job or accept recovery.
func (binding *RecipientBinding) Check(ctx context.Context, operation *executionruntime.Operation) error {
	if binding == nil || ctx == nil || binding.ctx == nil || operation == nil || binding.operation != operation || binding.logical == [32]byte{} || !time.Now().Before(binding.expires) {
		return errors.New("recipient binding original lifetime unavailable")
	}
	return errors.Join(ctx.Err(), binding.ctx.Err(), operation.CheckPublisher())
}

// Contexts are copied transcript commitments, not authenticated stream handles.
func (binding *RecipientBinding) Contexts() ([32]byte, [32]byte) {
	if binding == nil {
		return [32]byte{}, [32]byte{}
	}
	return binding.logical, binding.attachment
}
