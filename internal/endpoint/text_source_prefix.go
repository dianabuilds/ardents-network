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

// openTextPrefix uses only the context's retained members and finalized stock.
// It never upgrades an issuance-bootstrap lane or accepts a worker peer list.
func (owner *textContext) openTextPrefix(ctx context.Context) (*route.ClosedSourcePrefix, error) {
	if owner == nil || ctx == nil || ctx.Err() != nil {
		return nil, errors.New("text prefix context unavailable")
	}
	owner.mu.Lock()
	_, _, err := owner.textPermissionProfileLocked()
	if err != nil || owner.permission == nil || owner.permission.accepted == (credential.Permission{}) || owner.prefix != nil || owner.prefixOpening != nil || owner.issuance != nil {
		owner.mu.Unlock()
		return nil, errors.New("text prefix owner unavailable")
	}
	source, ok := owner.endpoint.closedState.(route.ClosedBootstrapState)
	if !ok {
		owner.mu.Unlock()
		return nil, errors.New("text prefix State unavailable")
	}
	attempt, cancel := context.WithCancel(owner.lease.Context())
	flight := &textSourceFlight{context: attempt, cancel: cancel, done: make(chan struct{})}
	// Reserve the whole stock -> opening transition. Concurrent opens cannot
	// spend a second bootstrap batch from an obsolete missing-stock snapshot.
	owner.prefixOpening = flight
	owner.mu.Unlock()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); cancel() })
	selection, openErr := owner.ensureTextPrefixStock(attempt, flight)
	var prefix *route.ClosedSourcePrefix
	if openErr == nil {
		prefix, openErr = route.OpenClosedSourcePrefix(attempt, source, selection, func(hello route.ClosedHello, class uint8) ([]byte, error) {
			return owner.presentTextToken(selection, hello, class)
		})
	}
	if !stop() {
		<-interrupted
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	defer close(flight.done)
	owner.prefixOpening = nil
	if openErr != nil || ctx.Err() != nil || !owner.liveLocked(owner.endpoint, owner.surface) {
		cancel()
		cleanup := prefix.Close()
		if errors.Is(openErr, route.ErrClosedSourceCleanup) || cleanup != nil {
			owner.closeErr = errors.Join(owner.closeErr, openErr, cleanup)
			owner.closed = true
			owner.endpoint.failTextContexts(owner.closeErr)
		}
		return nil, errors.Join(openErr, ctx.Err(), cleanup, errors.New("text prefix unavailable"))
	}
	owner.prefix, owner.prefixCancel = prefix, cancel
	return prefix, nil
}
func (owner *textContext) presentTextToken(selection route.ClosedBootstrapSelection, hello route.ClosedHello, class uint8) ([]byte, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	profile, now, err := owner.textPermissionProfileLocked()
	if err != nil || owner.permission == nil || owner.permission.accepted == (credential.Permission{}) || owner.prefixOpening == nil || owner.prefixOpening.context.Err() != nil ||
		hello.NetworkID != profile.NetworkID || hello.StateGeneration != profile.StateGeneration || hello.StateDigest != profile.StateDigest ||
		hello.ProfileDigest != profile.Digest || hello.Purpose != route.ClosedPurposeForwarding || class != 2 ||
		hello.ChannelNonce == [32]byte{} || !now.Before(hello.Deadline) || hello.Deadline.After(profile.NotAfter) {
		return nil, errors.New("text token presentation authority unavailable")
	}
	current, err := owner.selectTextBootstrapLocked()
	if err != nil || current != selection || (hello.RecipientNodeID != current.EntryNodeID && hello.RecipientNodeID != current.InteriorNodeID) {
		return nil, errors.New("text token presentation source changed")
	}
	return owner.takeTextTokenLocked(profile, now, hello, class, owner.prefixOpening.context)
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

func (owner *textContext) ensureTextPrefixStock(ctx context.Context, flight *textSourceFlight) (route.ClosedBootstrapSelection, error) {
	owner.mu.Lock()
	_, _, err := owner.textPermissionProfileLocked()
	if err != nil || ctx.Err() != nil || owner.permission == nil || owner.permission.accepted == (credential.Permission{}) ||
		owner.prefix != nil || owner.prefixOpening != flight || owner.issuance != nil {
		owner.mu.Unlock()
		return route.ClosedBootstrapSelection{}, errors.New("text prefix stock owner unavailable")
	}
	selection, err := owner.selectTextBootstrapLocked()
	if err != nil {
		owner.mu.Unlock()
		return route.ClosedBootstrapSelection{}, err
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
		if err := owner.issueTextTokensForOpening(ctx, missing, 2, flight, false); err != nil {
			return route.ClosedBootstrapSelection{}, err
		}
	}
	if err := owner.prepareTextIssuerStock(ctx, nil, 0, flight); err != nil {
		return route.ClosedBootstrapSelection{}, err
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	current, err := owner.selectTextBootstrapLocked()
	if err != nil || current != selection || ctx.Err() != nil || owner.prefixOpening != flight {
		return route.ClosedBootstrapSelection{}, errors.New("text prefix selection changed during issuance")
	}
	return selection, nil
}
