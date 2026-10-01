//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/endpoint/source"
	"github.com/dianabuilds/ardents-network/internal/endpoint/tokenjournal"
	"github.com/dianabuilds/ardents-network/internal/endpoint/tokens"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// openPrefix uses only the context's retained members and finalized stock.
// It never upgrades an issuance-bootstrap lane or accepts a worker peer list.
func (owner *dutyContext) openPrefix(ctx context.Context) (*source.Handle, error) {
	if owner == nil || ctx == nil || ctx.Err() != nil {
		return nil, source.PrefixFailureAt("context", errors.New("text prefix context unavailable"))
	}
	owner.mu.Lock()
	_, _, err := owner.permissionProfileLocked()
	if err != nil || !owner.tokens.Permission.HasAccepted() || owner.source.CurrentLocked() != nil || owner.source.OpeningInProgressLocked() || owner.tokens.Issuance != nil {
		owner.mu.Unlock()
		return nil, source.PrefixFailureAt("authority", errors.Join(err, errors.New("text prefix owner unavailable")))
	}
	bootstrapState, ok := owner.endpoint.closedState.(client.ClosedBootstrapState)
	if !ok {
		owner.mu.Unlock()
		return nil, source.PrefixFailureAt("state", errors.New("text prefix State unavailable"))
	}
	operation := newOperationFlight(owner)
	// Reserve the whole stock -> opening transition. Concurrent opens cannot
	// spend a second bootstrap batch from an obsolete missing-stock snapshot.
	if !owner.source.ReserveOpeningLocked(operation) {
		owner.mu.Unlock()
		operation.cancel()
		close(operation.done)
		return nil, source.PrefixFailureAt("authority", errors.New("text prefix reservation unavailable"))
	}
	owner.mu.Unlock()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); operation.cancel() })
	selection, openErr := owner.ensurePrefixStock(operation.context, operation)
	var prefix *client.ClosedSourcePrefix
	if openErr == nil {
		prefix, openErr = client.OpenClosedSourcePrefix(operation.context, bootstrapState, selection, func(hello ardp.Hello, class uint8) ([]byte, error) {
			return operation.presentToken(selection, hello, class)
		})
		if openErr != nil {
			stage := client.ClosedSourceOpenFailureStage(openErr)
			if presentation := source.TokenPresentationFailureStage(openErr); presentation != "unknown" {
				stage += "-" + presentation
			}
			openErr = source.PrefixFailureAt("opening-"+stage, openErr)
		}
	}
	if !stop() {
		<-interrupted
	}
	return operation.complete(ctx, prefix, openErr)
}
func (operation *operationFlight) presentToken(selection client.ClosedBootstrapSelection, hello ardp.Hello, class uint8) ([]byte, error) {
	owner := operation.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	profile, now, err := owner.permissionProfileLocked()
	if err != nil || !owner.tokens.Permission.HasAccepted() || !operation.admittedLocked(owner) ||
		hello.NetworkID != profile.NetworkID || hello.StateGeneration != profile.StateGeneration || hello.StateDigest != profile.StateDigest ||
		hello.ProfileDigest != profile.Digest || hello.Purpose != ardp.PurposeForwarding || class != 2 ||
		hello.ChannelNonce == [32]byte{} || !now.Before(hello.Deadline) || hello.Deadline.After(profile.NotAfter) {
		return nil, source.TokenPresentationFailureAt("authority", errors.Join(err, errors.New("text token presentation authority unavailable")))
	}
	current, err := owner.selectBootstrapLocked()
	if err != nil || current != selection || (hello.RecipientNodeID != current.EntryNodeID && hello.RecipientNodeID != current.InteriorNodeID) {
		return nil, source.TokenPresentationFailureAt("selection-"+interiorSelectionFailureStage(err), errors.Join(err, errors.New("text token presentation source changed")))
	}
	token, err := owner.tokens.TakeTokenLocked(profile, now, hello, class, operation.context)
	if err != nil {
		return nil, source.TokenPresentationFailureAt("take-"+tokens.TransferFailureStage(err), err)
	}
	return token, nil
}

func (endpoint *endpoint) tokenJournal() (*tokenjournal.Journal, error) {
	endpoint.dutyMu.Lock()
	defer endpoint.dutyMu.Unlock()
	if endpoint.dutyClosed || endpoint.closedTokenRoot == "" {
		return nil, errors.New("text token journal root unavailable")
	}
	if endpoint.closedTokenJournal == nil {
		journal, err := tokenjournal.Open(endpoint.closedTokenRoot, endpoint.network, endpoint.clock)
		if err != nil {
			return nil, err
		}
		endpoint.closedTokenJournal = journal
	}
	return endpoint.closedTokenJournal, nil
}

func (owner *dutyContext) ensurePrefixStock(ctx context.Context, opening *operationFlight) (client.ClosedBootstrapSelection, error) {
	owner.mu.Lock()
	_, _, err := owner.permissionProfileLocked()
	if err != nil || ctx.Err() != nil || !owner.tokens.Permission.HasAccepted() ||
		owner.source.CurrentLocked() != nil || !opening.admittedLocked(owner) || owner.tokens.Issuance != nil {
		owner.mu.Unlock()
		return client.ClosedBootstrapSelection{}, source.PrefixFailureAt("stock-authority", errors.Join(err, ctx.Err(), errors.New("text prefix stock owner unavailable")))
	}
	selection, err := owner.selectBootstrapLocked()
	if err != nil {
		owner.mu.Unlock()
		return client.ClosedBootstrapSelection{}, source.PrefixFailureAt("stock-selection-"+interiorSelectionFailureStage(err), err)
	}
	missing := owner.tokens.Permission.MissingStockFor(selection.ProfileDigest, [][32]byte{selection.EntryNodeID, selection.InteriorNodeID}, 2)
	owner.mu.Unlock()
	if len(missing) != 0 {
		// Independent receiver inputs share one common class/window key.
		if err := owner.issueTokensForOpening(ctx, missing, 2, opening, false); err != nil {
			return client.ClosedBootstrapSelection{}, source.PrefixFailureAt("stock-issuance", err)
		}
	}
	if err := owner.prepareIssuerStock(ctx, nil, 0, opening, nil, nil); err != nil {
		return client.ClosedBootstrapSelection{}, source.PrefixFailureAt("stock-issuer", err)
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	current, err := owner.selectBootstrapLocked()
	if err != nil || current != selection || ctx.Err() != nil || !opening.admittedLocked(owner) {
		return client.ClosedBootstrapSelection{}, source.PrefixFailureAt("stock-stability", errors.Join(err, ctx.Err(), errors.New("text prefix selection changed during issuance")))
	}
	return selection, nil
}
