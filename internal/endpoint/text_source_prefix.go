//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

type textSourceFlight struct {
	context context.Context
	cancel  context.CancelFunc
	done    chan struct{}
}

// textPrefixPreparationFailure distinguishes the local stages which can stop
// an expired Source prefix from being replaced. It deliberately retains the
// original cause without exposing that cause through the headless event.
type textPrefixPreparationFailure struct {
	stage string
	cause error
}

type textTokenPresentationFailure struct {
	stage string
	cause error
}

func (failure *textTokenPresentationFailure) Error() string { return failure.cause.Error() }

func (failure *textTokenPresentationFailure) Unwrap() error { return failure.cause }

func textTokenPresentationFailureAt(stage string, cause error) error {
	return &textTokenPresentationFailure{stage: stage, cause: cause}
}

func textTokenPresentationFailureStage(cause error) string {
	var failure *textTokenPresentationFailure
	if errors.As(cause, &failure) && failure.stage != "" {
		return failure.stage
	}
	return "unknown"
}

func (failure *textPrefixPreparationFailure) Error() string { return failure.cause.Error() }

func (failure *textPrefixPreparationFailure) Unwrap() error { return failure.cause }

func textPrefixPreparationFailureAt(stage string, cause error) error {
	return &textPrefixPreparationFailure{stage: stage, cause: cause}
}

func textPrefixPreparationFailureStage(cause error) string {
	var failure *textPrefixPreparationFailure
	if errors.As(cause, &failure) && failure.stage != "" {
		return failure.stage
	}
	return "unknown"
}

// openTextPrefix uses only the context's retained members and finalized stock.
// It never upgrades an issuance-bootstrap lane or accepts a worker peer list.
func (owner *textContext) openTextPrefix(ctx context.Context) (*textSourceHandle, error) {
	if owner == nil || ctx == nil || ctx.Err() != nil {
		return nil, textPrefixPreparationFailureAt("context", errors.New("text prefix context unavailable"))
	}
	owner.mu.Lock()
	_, _, err := owner.textPermissionProfileLocked()
	if err != nil || owner.permission == nil || owner.permission.accepted == (credential.Permission{}) || owner.currentTextSourceLocked() != nil || owner.source.openingInProgressLocked() || owner.issuance != nil {
		owner.mu.Unlock()
		return nil, textPrefixPreparationFailureAt("authority", errors.Join(err, errors.New("text prefix owner unavailable")))
	}
	source, ok := owner.endpoint.closedState.(route.ClosedBootstrapState)
	if !ok {
		owner.mu.Unlock()
		return nil, textPrefixPreparationFailureAt("state", errors.New("text prefix State unavailable"))
	}
	operation := newTextPrefixOpeningOperation(owner)
	// Reserve the whole stock -> opening transition. Concurrent opens cannot
	// spend a second bootstrap batch from an obsolete missing-stock snapshot.
	if !owner.source.reserveOpeningLocked(operation) {
		owner.mu.Unlock()
		operation.cancel()
		close(operation.done)
		return nil, textPrefixPreparationFailureAt("authority", errors.New("text prefix reservation unavailable"))
	}
	owner.mu.Unlock()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); operation.cancel() })
	selection, openErr := owner.ensureTextPrefixStock(operation.context, operation)
	var prefix *route.ClosedSourcePrefix
	if openErr == nil {
		prefix, openErr = route.OpenClosedSourcePrefix(operation.context, source, selection, func(hello route.ClosedHello, class uint8) ([]byte, error) {
			return operation.presentTextToken(selection, hello, class)
		})
		if openErr != nil {
			stage := route.ClosedSourceOpenFailureStage(openErr)
			if presentation := textTokenPresentationFailureStage(openErr); presentation != "unknown" {
				stage += "-" + presentation
			}
			openErr = textPrefixPreparationFailureAt("opening-"+stage, openErr)
		}
	}
	if !stop() {
		<-interrupted
	}
	return operation.complete(ctx, prefix, openErr)
}
func (operation *textPrefixOpeningOperation) presentTextToken(selection route.ClosedBootstrapSelection, hello route.ClosedHello, class uint8) ([]byte, error) {
	owner := operation.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	profile, now, err := owner.textPermissionProfileLocked()
	if err != nil || owner.permission == nil || owner.permission.accepted == (credential.Permission{}) || !operation.admittedLocked(owner) ||
		hello.NetworkID != profile.NetworkID || hello.StateGeneration != profile.StateGeneration || hello.StateDigest != profile.StateDigest ||
		hello.ProfileDigest != profile.Digest || hello.Purpose != route.ClosedPurposeForwarding || class != 2 ||
		hello.ChannelNonce == [32]byte{} || !now.Before(hello.Deadline) || hello.Deadline.After(profile.NotAfter) {
		return nil, textTokenPresentationFailureAt("authority", errors.Join(err, errors.New("text token presentation authority unavailable")))
	}
	current, err := owner.selectTextBootstrapLocked()
	if err != nil || current != selection || (hello.RecipientNodeID != current.EntryNodeID && hello.RecipientNodeID != current.InteriorNodeID) {
		return nil, textTokenPresentationFailureAt("selection-"+textSourceSelectionFailureStage(err), errors.Join(err, errors.New("text token presentation source changed")))
	}
	token, err := owner.takeTextTokenLocked(profile, now, hello, class, operation.context)
	if err != nil {
		return nil, textTokenPresentationFailureAt("take-"+textTokenTransferFailureStage(err), err)
	}
	return token, nil
}

func (endpoint *endpoint) textTokenJournal() (*textTokenJournal, error) {
	endpoint.textMu.Lock()
	defer endpoint.textMu.Unlock()
	if endpoint.textClosed || endpoint.closedTokenRoot == "" {
		return nil, errors.New("text token journal root unavailable")
	}
	if endpoint.closedTokenJournal == nil {
		journal, err := openTextTokenJournal(endpoint.closedTokenRoot, endpoint.network, endpoint.clock)
		if err != nil {
			return nil, err
		}
		endpoint.closedTokenJournal = journal
	}
	return endpoint.closedTokenJournal, nil
}

func (owner *textContext) ensureTextPrefixStock(ctx context.Context, opening *textPrefixOpeningOperation) (route.ClosedBootstrapSelection, error) {
	owner.mu.Lock()
	_, _, err := owner.textPermissionProfileLocked()
	if err != nil || ctx.Err() != nil || owner.permission == nil || owner.permission.accepted == (credential.Permission{}) ||
		owner.currentTextSourceLocked() != nil || !opening.admittedLocked(owner) || owner.issuance != nil {
		owner.mu.Unlock()
		return route.ClosedBootstrapSelection{}, textPrefixPreparationFailureAt("stock-authority", errors.Join(err, ctx.Err(), errors.New("text prefix stock owner unavailable")))
	}
	selection, err := owner.selectTextBootstrapLocked()
	if err != nil {
		owner.mu.Unlock()
		return route.ClosedBootstrapSelection{}, textPrefixPreparationFailureAt("stock-selection-"+textSourceSelectionFailureStage(err), err)
	}
	var missing [][32]byte
	for _, receiver := range [][32]byte{selection.EntryNodeID, selection.InteriorNodeID} {
		ready := false
		for _, stock := range owner.permission.stock {
			if stock.challenge.ReceiverNodeID == receiver && stock.challenge.ProfileDigest == selection.ProfileDigest &&
				stock.challenge.Class == 2 && stock.challenge.WindowStart == owner.permission.accepted.NotBefore && len(stock.tokens) > 0 {
				ready = true
			}
		}
		if !ready {
			missing = append(missing, receiver)
		}
	}
	owner.mu.Unlock()
	if len(missing) != 0 {
		// Independent receiver inputs share one common class/window key.
		if err := owner.issueTextTokensForOpening(ctx, missing, 2, opening, false); err != nil {
			return route.ClosedBootstrapSelection{}, textPrefixPreparationFailureAt("stock-issuance", err)
		}
	}
	if err := owner.prepareTextIssuerStock(ctx, nil, 0, opening); err != nil {
		return route.ClosedBootstrapSelection{}, textPrefixPreparationFailureAt("stock-issuer", err)
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	current, err := owner.selectTextBootstrapLocked()
	if err != nil || current != selection || ctx.Err() != nil || !opening.admittedLocked(owner) {
		return route.ClosedBootstrapSelection{}, textPrefixPreparationFailureAt("stock-stability", errors.Join(err, ctx.Err(), errors.New("text prefix selection changed during issuance")))
	}
	return selection, nil
}
