package connection

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"time"

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
	expires             time.Time
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
	actualDigest, err := request.Digest()
	if err != nil || actualDigest != plaintextDigest {
		return nil, errors.Join(errors.New("recipient plaintext commitment differs"), err)
	}
	value := proof.Delegation()
	if proof.Digest() == [32]byte{} || value.Generation == 0 || value.Instance == [32]byte{} ||
		profile == [32]byte{} || plaintextDigest == [32]byte{} || now.Before(value.NotBefore) || !now.Before(value.NotAfter) ||
		request.Network != value.Network || request.Target != value.Target || request.PublicationDigest != proof.Digest() || request.ProfileDigest != profile ||
		request.ConnectionNonce == [32]byte{} || request.InitiatorBinding == [32]byte{} || request.JoinSecret == [32]byte{} || request.HandshakeContext == [32]byte{} ||
		request.RendezvousNode == [32]byte{} || request.RendezvousDutyGeneration == 0 || request.Revision == 0 || request.AttachmentGeneration != 1 ||
		request.Deadline != request.Deadline.UTC().Truncate(time.Second) || !now.Before(request.Deadline) || request.Deadline.After(now.Add(10*time.Second)) ||
		request.WorkSafetyNotAfter <= now.Unix() || request.WorkSafetyMaximum < request.WorkSafetyNotAfter ||
		request.NoNewRecoveryAfter <= 0 || request.NoNewRecoveryAfter > request.WorkSafetyNotAfter || request.Deadline.Unix() > request.WorkSafetyNotAfter ||
		localEnd.IsZero() || time.Unix(request.WorkSafetyMaximum, 0).After(localEnd) || time.Unix(request.WorkSafetyMaximum, 0).After(value.NotAfter) {
		return nil, errors.New("recipient immutable binding or safety bounds differ")
	}
	transcript := []byte("ardents-service-context-v3\x00")
	for _, field := range [][32]byte{value.Network, value.Target, value.Instance} {
		transcript = append(transcript, field[:]...)
	}
	transcript = binary.BigEndian.AppendUint64(transcript, value.Generation)
	for _, field := range [][32]byte{proof.Digest(), profile, request.ConnectionNonce, request.InitiatorBinding} {
		transcript = append(transcript, field[:]...)
	}
	for _, bound := range []int64{request.WorkSafetyNotAfter, request.WorkSafetyMaximum, request.NoNewRecoveryAfter} {
		transcript = binary.BigEndian.AppendUint64(transcript, uint64(bound))
	}
	logical := sha256.Sum256(transcript)
	attachmentTranscript := append([]byte("ardents-attachment-context-v3\x00"), logical[:]...)
	attachmentTranscript = append(attachmentTranscript, plaintextDigest[:]...)
	attachmentTranscript = binary.BigEndian.AppendUint64(attachmentTranscript, request.AttachmentGeneration)
	return &RecipientBinding{logical: logical, attachment: sha256.Sum256(attachmentTranscript), expires: request.Deadline}, nil
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
