//go:build linux

package tokens

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// Operation owns one admitted issuance attempt through transport
// cancellation and terminal completion. The owner retains only the single-
// operation admission slot and joins this operation during revocation.
type Operation struct {
	owner           *Owner
	context         context.Context
	cancelOperation context.CancelFunc
	// Done closes exactly once, after the terminal completion has erased the
	// request bytes and any retired permission.
	Done chan struct{}
	// DiscardPermission records that revocation retired this operation's
	// permission; the terminal completion erases it after transport joins.
	DiscardPermission bool

	prefix          Prefix
	permission      *Permission
	profile         state.ClosedProfileView
	batch           *Batch
	request         []byte
	discardCanceled bool
}

func NewOperation(owner *Owner, permission *Permission, profile state.ClosedProfileView,
	batch *Batch, discardCanceled bool) *Operation {
	attempt, cancel := context.WithDeadline(owner.host.LeaseContext(), permission.Accepted.NotAfter)
	return &Operation{owner: owner, context: attempt, cancelOperation: cancel, Done: make(chan struct{}),
		prefix: batch.Prefix, permission: permission, profile: profile, batch: batch, request: batch.Pending.Request(),
		discardCanceled: discardCanceled}
}

func (operation *Operation) Cancel() {
	if operation != nil {
		operation.cancelOperation()
	}
}

func (operation *Operation) Join() {
	if operation != nil {
		<-operation.Done
	}
}

// retirePermissionLocked makes the permission unavailable immediately while
// leaving its operation-owned batch intact until transport and completion have
// joined. The caller holds the context lock.
func (operation *Operation) retirePermissionLocked(permission *Permission) bool {
	if operation == nil || operation.permission != permission {
		return false
	}
	operation.DiscardPermission = true
	operation.Cancel()
	return true
}

func (operation *Operation) Run(caller context.Context, source client.ClosedBootstrapState,
	selection client.ClosedBootstrapSelection) error {
	interrupted := make(chan struct{})
	stop := context.AfterFunc(caller, func() {
		defer close(interrupted)
		operation.Cancel()
	})
	var result client.ClosedIssuanceExchangeResult
	var exchangeErr error
	if operation.prefix == nil {
		result, exchangeErr = client.ExchangeClosedBootstrap(operation.context, source, selection, operation.request)
	} else {
		result, exchangeErr = operation.prefix.ExchangeIssuer(operation.context, func(hello ardp.Hello, tokenClass uint8) ([]byte, error) {
			return operation.presentIssuerToken(selection, hello, tokenClass)
		}, operation.request)
	}
	operation.Cancel()
	if !stop() {
		<-interrupted
	}
	return operation.Complete(caller, result, exchangeErr)
}

// Complete is the terminal admission point: it verifies the surviving owner
// and permission under the shared lock, deposits a finalized batch, and
// always finishes the operation slot exactly once.
func (operation *Operation) Complete(caller context.Context, result client.ClosedIssuanceExchangeResult, exchangeErr error) error {
	owner := operation.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	defer operation.finishLocked()
	if owner.Issuance != operation {
		clear(result.Body)
		return errors.New("text issuance completion owner changed")
	}
	owner.Issuance = nil
	if errors.Is(exchangeErr, client.ErrClosedBootstrapCleanup) || errors.Is(exchangeErr, client.ErrClosedSourceCleanup) {
		owner.host.Fail(exchangeErr)
		return exchangeErr
	}
	current, currentTime, currentErr := owner.host.ProfileLocked()
	if currentErr != nil || owner.Permission != operation.permission || current != operation.profile ||
		!currentTime.Before(operation.permission.Accepted.NotAfter) {
		clear(result.Body)
		return errors.New("text issuance owner changed before completion")
	}
	if err := caller.Err(); err != nil {
		clear(result.Body)
		if operation.discardCanceled {
			operation.permission.discardPendingBatch(operation.batch)
		}
		return err
	}
	if exchangeErr != nil {
		// Preserve the original opaque blinding state and request ID for an explicit
		// same-process retry. No new request, refund, fallback or resampling occurs.
		return exchangeErr
	}
	err := operation.permission.acceptIssuedBatch(operation.batch, result.Nonce, result.Body)
	clear(result.Body)
	return err
}

func (operation *Operation) finishLocked() {
	if operation.DiscardPermission {
		clearPermission(operation.permission)
	}
	clear(operation.request)
	close(operation.Done)
}

func (operation *Operation) presentIssuerToken(selection client.ClosedBootstrapSelection, hello ardp.Hello, class uint8) ([]byte, error) {
	owner := operation.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	profile, now, err := owner.host.ProfileLocked()
	if err != nil || owner.Issuance != operation || operation.context == nil || operation.context.Err() != nil ||
		operation.prefix == nil || !owner.host.PrefixCurrent(operation.prefix) || !owner.Permission.PendingFor(operation.prefix) ||
		hello.Purpose != ardp.PurposeIssuer || class != 1 ||
		hello.NetworkID != profile.NetworkID || hello.StateGeneration != profile.StateGeneration || hello.StateDigest != profile.StateDigest ||
		hello.ProfileDigest != profile.Digest || hello.RecipientNodeID != profile.IssuerNodeID || hello.RecipientDutyGeneration != profile.IssuerDutyGeneration ||
		hello.ChannelNonce == [32]byte{} || !now.Before(hello.Deadline) || hello.Deadline.After(profile.NotAfter) {
		return nil, errors.New("text issuer token presentation authority unavailable")
	}
	current, err := owner.host.SelectBootstrapLocked()
	if err != nil || current != selection {
		return nil, errors.New("text issuer token source changed")
	}
	return owner.TakeTokenLocked(profile, now, hello, class, operation.context)
}
