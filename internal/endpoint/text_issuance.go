//go:build linux

package endpoint

import (
	"context"
	"errors"
	"slices"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

// Blinding state and finalized stock never leave this context. The network
// attempt owns copied request bytes only, so revocation can erase secrets
// under owner.mu while cancellation interrupts and joins the transport tree.
type textTokenBatch struct {
	refill     bool // Retained internal stock work; never receiver admission authority.
	prefix     *route.ClosedSourcePrefix
	challenges []credential.ClosedTokenContext
	selection  route.ClosedBootstrapSelection
	pending    *credential.PendingClosedTokenBatch
}

type textTokenStock struct {
	challenge credential.ClosedTokenContext
	tokens    [][]byte
}

type textIssuanceFlight struct {
	context context.Context
	prefix  *route.ClosedSourcePrefix
	cancel  context.CancelFunc
	done    chan struct{}
}

// issueTextTokens is the trusted context owner's issuance operation. The
// retained Route members and intended receiver originate in Endpoint, never
// on a worker attachment. There is at most one live exchange per context.
func (owner *textContext) issueTextTokens(ctx context.Context, receivers [][32]byte, class uint8) error {
	return owner.issueTextTokensForOpening(ctx, receivers, class, nil, false)
}

// A non-nil opening must be the exact retained prefix transition. Keeping it
// across both bootstrap flights prevents unrelated issuance stealing its slot.
func (owner *textContext) issueTextTokensForOpening(ctx context.Context, receivers [][32]byte, class uint8, opening *textSourceFlight, refill bool) error {
	if owner == nil || ctx == nil || ctx.Err() != nil || class < 1 || class > 3 || len(receivers) == 0 || len(receivers) > 32 {
		return errors.New("text issuance context is unavailable")
	}
	owner.mu.Lock()
	if owner.prefixOpening != opening || opening != nil && opening.context.Err() != nil {
		owner.mu.Unlock()
		return errors.New("text issuance prefix reservation unavailable")
	}
	hasPrefix := owner.prefix != nil
	owner.mu.Unlock()
	if hasPrefix && !refill {
		if err := owner.prepareTextIssuerStock(ctx, receivers, class, opening); err != nil {
			return err
		}
	}
	owner.mu.Lock()
	profile, now, err := owner.textPermissionProfileLocked()
	if err != nil {
		owner.mu.Unlock()
		return err
	}
	permission := owner.permission
	source, ok := owner.endpoint.closedState.(route.ClosedBootstrapState)
	if !ok || permission == nil || permission.accepted == (credential.Permission{}) || permission.profile != profile ||
		owner.issuance != nil || owner.prefixOpening != opening || opening != nil && opening.context.Err() != nil {
		owner.mu.Unlock()
		return errors.New("text issuance owner is unavailable")
	}
	selection, err := owner.selectTextBootstrapLocked()
	if err != nil || selection.ProfileDigest != profile.Digest {
		owner.mu.Unlock()
		return errors.New("text issuance source selection unavailable")
	}
	view, err := source.CurrentClosedRoute()
	if err != nil || view.Profile != profile || int(view.NodeCount) > len(view.Nodes) {
		owner.mu.Unlock()
		return errors.New("text issuance recipients are unavailable")
	}
	challenges := make([]credential.ClosedTokenContext, len(receivers))
	for index, receiver := range receivers {
		challenge := credential.ClosedTokenContext{NetworkID: profile.NetworkID, ProfileDigest: profile.Digest, IssuerNodeID: profile.IssuerNodeID,
			ReceiverNodeID: receiver, Class: class, WindowStart: permission.accepted.NotBefore}
		for _, node := range view.Nodes[:view.NodeCount] {
			if node.NodeID == receiver {
				if challenge.ReceiverDutyGeneration != 0 {
					owner.mu.Unlock()
					return errors.New("text issuance receiver is ambiguous")
				}
				challenge.ReceiverDutyGeneration = node.DutyGeneration
			}
		}
		if challenge.ReceiverDutyGeneration == 0 {
			owner.mu.Unlock()
			return errors.New("text issuance receiver is absent from State")
		}
		challenges[index] = challenge
	}
	batch := permission.pending
	if batch != nil {
		if batch.refill != refill || !slices.Equal(batch.challenges, challenges) || batch.selection != selection ||
			batch.prefix != nil && batch.prefix != owner.prefix {
			owner.mu.Unlock()
			return errors.New("text issuance retry must retain the original batch")
		}
	} else {
		if owner.prefix == nil && permission.batches >= 2 {
			owner.mu.Unlock()
			return errors.New("text bootstrap batch allowance is exhausted")
		}
		if uint32(len(challenges)) > permission.accepted.Maxima[class-1]-permission.reserved[class-1] {
			owner.mu.Unlock()
			return errors.New("text issuance allocation is exhausted")
		}
		pending, err := credential.PrepareClosedTokenBatch(credential.ClosedTokenBatchConfig{Profile: profile, Contexts: challenges,
			Permission: permission.accepted, HolderKey: permission.holder, Now: now})
		if err != nil {
			owner.mu.Unlock()
			return err
		}
		batch = &textTokenBatch{refill: refill, prefix: owner.prefix, challenges: challenges, selection: selection, pending: pending}
		permission.pending = batch
		permission.reserved[class-1] += uint32(len(challenges))
		if batch.prefix == nil {
			permission.batches++
		}
	}
	request := batch.pending.Request()
	attempt, cancel := context.WithDeadline(owner.lease.Context(), permission.accepted.NotAfter)
	flight := &textIssuanceFlight{context: attempt, prefix: batch.prefix, cancel: cancel, done: make(chan struct{})}
	owner.issuance = flight
	owner.mu.Unlock()
	defer clear(request)
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); cancel() })
	var result route.ClosedIssuanceExchangeResult
	var exchangeErr error
	if batch.prefix == nil {
		result, exchangeErr = route.ExchangeClosedBootstrap(attempt, source, selection, request)
	} else {
		result, exchangeErr = batch.prefix.ExchangeIssuer(attempt, func(hello route.ClosedHello, tokenClass uint8) ([]byte, error) {
			return owner.presentTextIssuerToken(flight, selection, hello, tokenClass)
		}, request)
	}
	cancel()
	if !stop() {
		<-interrupted
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	defer close(flight.done)
	owner.issuance = nil
	if errors.Is(exchangeErr, route.ErrClosedBootstrapCleanup) || errors.Is(exchangeErr, route.ErrClosedSourceCleanup) {
		owner.closeErr = errors.Join(owner.closeErr, exchangeErr)
		owner.closed = true
		owner.endpoint.failTextContexts(exchangeErr)
		return exchangeErr
	}
	current, currentTime, currentErr := owner.textPermissionProfileLocked()
	if currentErr != nil || owner.permission != permission || current != profile || !currentTime.Before(permission.accepted.NotAfter) {
		clear(result.Body)
		return errors.New("text issuance owner changed before completion")
	}

	if err := ctx.Err(); err != nil {
		clear(result.Body)
		return err
	}
	if exchangeErr != nil {
		// Preserve the original opaque blinding state and request ID for an explicit
		// same-process retry. No new request, refund, fallback or resampling occurs.
		return exchangeErr
	}
	tokens, err := batch.pending.FinalizeTerminalOperation(result.Nonce, result.Body)
	clear(result.Body)
	permission.pending = nil
	if err != nil {
		return err
	}
	for index, token := range tokens {
		challenge := batch.challenges[index]
		found := false
		for slot := range permission.stock {
			if permission.stock[slot].challenge == challenge {
				permission.stock[slot].tokens = append(permission.stock[slot].tokens, token)
				found = true
				break
			}
		}
		if !found {
			permission.stock = append(permission.stock, textTokenStock{challenge: challenge, tokens: [][]byte{token}})
		}
	}
	return nil
}
