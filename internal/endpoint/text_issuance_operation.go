//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// textIssuanceOperation owns one admitted issuance attempt through transport
// cancellation and terminal completion. The context retains only the single-
// operation admission slot and joins this owner during revocation.
type textIssuanceOperation struct {
	owner             *textContext
	context           context.Context
	cancelOperation   context.CancelFunc
	done              chan struct{}
	prefix            *textSourceHandle
	permission        *textPermission
	profile           state.ClosedProfileView
	batch             *textTokenBatch
	request           []byte
	discardCanceled   bool
	discardPermission bool
}

func newTextIssuanceOperation(owner *textContext, permission *textPermission, profile state.ClosedProfileView,
	batch *textTokenBatch, discardCanceled bool) *textIssuanceOperation {
	attempt, cancel := context.WithDeadline(owner.lease.Context(), permission.accepted.NotAfter)
	return &textIssuanceOperation{owner: owner, context: attempt, cancelOperation: cancel, done: make(chan struct{}),
		prefix: batch.prefix, permission: permission, profile: profile, batch: batch, request: batch.pending.Request(),
		discardCanceled: discardCanceled}
}

func (operation *textIssuanceOperation) cancel() {
	if operation != nil {
		operation.cancelOperation()
	}
}

func (operation *textIssuanceOperation) join() {
	if operation != nil {
		<-operation.done
	}
}

// retirePermissionLocked makes the permission unavailable immediately while
// leaving its operation-owned batch intact until transport and completion have
// joined. The caller holds the context lock.
func (operation *textIssuanceOperation) retirePermissionLocked(permission *textPermission) bool {
	if operation == nil || operation.permission != permission {
		return false
	}
	operation.discardPermission = true
	operation.cancel()
	return true
}

func (operation *textIssuanceOperation) run(caller context.Context, source route.ClosedBootstrapState,
	selection route.ClosedBootstrapSelection) error {
	interrupted := make(chan struct{})
	stop := context.AfterFunc(caller, func() {
		defer close(interrupted)
		operation.cancel()
	})
	var result route.ClosedIssuanceExchangeResult
	var exchangeErr error
	if operation.prefix == nil {
		result, exchangeErr = route.ExchangeClosedBootstrap(operation.context, source, selection, operation.request)
	} else {
		result, exchangeErr = operation.prefix.exchangeIssuer(operation.context, func(hello route.ClosedHello, tokenClass uint8) ([]byte, error) {
			return operation.presentTextIssuerToken(selection, hello, tokenClass)
		}, operation.request)
	}
	operation.cancel()
	if !stop() {
		<-interrupted
	}
	return operation.complete(caller, result, exchangeErr)
}

func (operation *textIssuanceOperation) complete(caller context.Context, result route.ClosedIssuanceExchangeResult, exchangeErr error) error {
	owner := operation.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	defer operation.finishLocked()
	if owner.issuance != operation {
		clear(result.Body)
		return errors.New("text issuance completion owner changed")
	}
	owner.issuance = nil
	if errors.Is(exchangeErr, route.ErrClosedBootstrapCleanup) || errors.Is(exchangeErr, route.ErrClosedSourceCleanup) {
		owner.closeErr = errors.Join(owner.closeErr, exchangeErr)
		owner.closed = true
		owner.endpoint.failTextContexts(exchangeErr)
		return exchangeErr
	}
	current, currentTime, currentErr := owner.textPermissionProfileLocked()
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

func (operation *textIssuanceOperation) finishLocked() {
	if operation.discardPermission {
		clearTextPermission(operation.permission)
	}
	clear(operation.request)
	close(operation.done)
}
