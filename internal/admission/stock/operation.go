//go:build linux

package stock

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// Operation owns one admitted issuance attempt through transport
// cancellation and terminal completion. The owner retains only the single-
// operation admission slot and joins this operation during revocation.
type Operation struct{ value *operation }

// operation retains all mutable state; copying a public handle cannot replace it.
type operation struct {
	started         atomic.Bool
	owner           *Owner
	context         context.Context
	cancelOperation context.CancelFunc
	// done closes exactly once, after the terminal completion has erased the
	// request bytes and any retired permission.
	done chan struct{}
	// discardPermission records that revocation retired this operation's
	// permission; the terminal completion erases it after transport joins.
	discardPermission bool

	prefix          Prefix
	permission      *permission
	profile         state.ClosedProfileView
	batch           *batch
	request         []byte
	discardCanceled bool
}

func newOperation(owner *Owner, permission *permission, profile state.ClosedProfileView,
	batch *batch, discardCanceled bool) *operation {
	attempt, cancel := context.WithDeadline(owner.host.LeaseContext(), permission.accepted.NotAfter)
	return &operation{owner: owner, context: attempt, cancelOperation: cancel, done: make(chan struct{}),
		prefix: batch.Prefix, permission: permission, profile: profile, batch: batch, request: batch.Pending.Request(),
		discardCanceled: discardCanceled}
}

// Cancel cancels the admitted exchange; the zero handle is harmless.
func (handle Operation) Cancel() { handle.value.Cancel() }

// Join waits for terminal completion; the zero handle is already joined.
func (handle Operation) Join() { handle.value.Join() }

// Run admits exactly one exchange across all copies of this handle.
func (handle Operation) Run(caller context.Context, source client.ClosedBootstrapState,
	selection client.ClosedBootstrapSelection) error {
	if handle.value == nil || !handle.value.started.CompareAndSwap(false, true) {
		return errors.New("text issuance operation already started or unavailable")
	}
	return handle.value.run(caller, source, selection)
}

func (operation *operation) Cancel() {
	if operation != nil {
		operation.cancelOperation()
	}
}

func (operation *operation) Join() {
	if operation != nil {
		<-operation.done
	}
}

// retirePermissionLocked makes the permission unavailable immediately while
// leaving its operation-owned batch intact until transport and completion have
// joined. The caller holds the context lock.
func (operation *operation) retirePermissionLocked(permission *permission) bool {
	if operation == nil || operation.permission != permission {
		return false
	}
	operation.discardPermission = true
	operation.Cancel()
	return true
}

func (operation *operation) run(caller context.Context, source client.ClosedBootstrapState,
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
	return operation.complete(caller, result, exchangeErr)
}

// complete is the terminal admission point: it verifies the surviving owner
// and permission under the shared lock, deposits a finalized batch, and
// always finishes the operation slot exactly once.
func (operation *operation) complete(caller context.Context, result client.ClosedIssuanceExchangeResult, exchangeErr error) error {
	owner := operation.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	defer operation.finishLocked()
	if owner.issuance != operation {
		clear(result.Body)
		return errors.New("text issuance completion owner changed")
	}
	owner.issuance = nil
	if errors.Is(exchangeErr, client.ErrClosedBootstrapCleanup) || errors.Is(exchangeErr, client.ErrClosedSourceCleanup) {
		owner.host.Fail(exchangeErr)
		return exchangeErr
	}
	current, currentTime, currentErr := owner.host.ProfileLocked()
	if currentErr != nil || owner.permission != operation.permission || current != operation.profile ||
		!currentTime.Before(operation.permission.accepted.NotAfter) {
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

func (operation *operation) finishLocked() {
	if operation.discardPermission {
		clearPermission(operation.permission)
	}
	clear(operation.request)
	close(operation.done)
}

func (operation *operation) presentIssuerToken(selection client.ClosedBootstrapSelection, hello ardp.Hello, class uint8) ([]byte, error) {
	owner := operation.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	profile, now, err := owner.host.ProfileLocked()
	if err != nil || owner.issuance != operation || operation.context == nil || operation.context.Err() != nil ||
		operation.prefix == nil || !owner.host.PrefixCurrent(operation.prefix) || !owner.permission.PendingFor(operation.prefix) ||
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
