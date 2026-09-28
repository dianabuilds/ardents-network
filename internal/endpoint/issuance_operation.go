//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// issuanceOperation owns one admitted issuance attempt through transport
// cancellation and terminal completion. The context retains only the single-
// operation admission slot and joins this owner during revocation.
type issuanceOperation struct {
	owner             *textContext
	context           context.Context
	cancelOperation   context.CancelFunc
	done              chan struct{}
	prefix            *textSourceHandle
	permission        *permission
	profile           state.ClosedProfileView
	batch             *tokenBatch
	request           []byte
	discardCanceled   bool
	discardPermission bool
}

func newIssuanceOperation(owner *textContext, permission *permission, profile state.ClosedProfileView,
	batch *tokenBatch, discardCanceled bool) *issuanceOperation {
	attempt, cancel := context.WithDeadline(owner.lease.Context(), permission.accepted.NotAfter)
	return &issuanceOperation{owner: owner, context: attempt, cancelOperation: cancel, done: make(chan struct{}),
		prefix: batch.prefix, permission: permission, profile: profile, batch: batch, request: batch.pending.Request(),
		discardCanceled: discardCanceled}
}

func (operation *issuanceOperation) cancel() {
	if operation != nil {
		operation.cancelOperation()
	}
}

func (operation *issuanceOperation) join() {
	if operation != nil {
		<-operation.done
	}
}

// retirePermissionLocked makes the permission unavailable immediately while
// leaving its operation-owned batch intact until transport and completion have
// joined. The caller holds the context lock.
func (operation *issuanceOperation) retirePermissionLocked(permission *permission) bool {
	if operation == nil || operation.permission != permission {
		return false
	}
	operation.discardPermission = true
	operation.cancel()
	return true
}

func (operation *issuanceOperation) run(caller context.Context, source client.ClosedBootstrapState,
	selection client.ClosedBootstrapSelection) error {
	interrupted := make(chan struct{})
	stop := context.AfterFunc(caller, func() {
		defer close(interrupted)
		operation.cancel()
	})
	var result client.ClosedIssuanceExchangeResult
	var exchangeErr error
	if operation.prefix == nil {
		result, exchangeErr = client.ExchangeClosedBootstrap(operation.context, source, selection, operation.request)
	} else {
		result, exchangeErr = operation.prefix.exchangeIssuer(operation.context, func(hello ardp.Hello, tokenClass uint8) ([]byte, error) {
			return operation.presentIssuerToken(selection, hello, tokenClass)
		}, operation.request)
	}
	operation.cancel()
	if !stop() {
		<-interrupted
	}
	return operation.complete(caller, result, exchangeErr)
}

func (operation *issuanceOperation) complete(caller context.Context, result client.ClosedIssuanceExchangeResult, exchangeErr error) error {
	owner := operation.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	defer operation.finishLocked()
	if owner.tokens.issuance != operation {
		clear(result.Body)
		return errors.New("text issuance completion owner changed")
	}
	owner.tokens.issuance = nil
	if errors.Is(exchangeErr, client.ErrClosedBootstrapCleanup) || errors.Is(exchangeErr, client.ErrClosedSourceCleanup) {
		owner.closeErr = errors.Join(owner.closeErr, exchangeErr)
		owner.closed = true
		owner.endpoint.failTextContexts(exchangeErr)
		return exchangeErr
	}
	current, currentTime, currentErr := owner.permissionProfileLocked()
	if currentErr != nil || owner.tokens.permission != operation.permission || current != operation.profile ||
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

func (operation *issuanceOperation) finishLocked() {
	if operation.discardPermission {
		clearPermission(operation.permission)
	}
	clear(operation.request)
	close(operation.done)
}
