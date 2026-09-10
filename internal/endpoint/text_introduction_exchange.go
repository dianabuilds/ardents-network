//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
)

func (owner *textContext) submitTextIntroduction(ctx context.Context, job *textJobIdentity, prepared *textIntroductionAttempt) (outcome error) {
	if owner == nil || ctx == nil || ctx.Err() != nil || prepared == nil || prepared.binding == nil ||
		prepared.binding.owner != owner || prepared.binding.job != job {
		return errors.New("text Introduction submission unavailable")
	}
	owner.mu.Lock()
	live := owner.liveTextServiceJobLocked(job, broker.Connection) && owner.prefix != nil && !prepared.submitted
	prefix := owner.prefix
	if live {
		prepared.submitted = true
	}
	owner.mu.Unlock()
	if !live {
		return errors.New("text Introduction submission owner changed")
	}
	lifetime, finish, err := owner.beginTextIntroductionExchange(ctx, job, broker.Connection)
	if err != nil {
		return err
	}
	defer func() { outcome = finish(outcome) }()
	bounded, cancel := context.WithDeadline(lifetime, prepared.plaintext.Deadline)
	defer cancel()
	receiver, profile, err := owner.prepareTextSubmissionStock(bounded, prefix)
	if err != nil {
		return err
	}
	status, err := prefix.SubmitIntroduction(bounded, func(hello route.ClosedHello, class uint8) ([]byte, error) {
		owner.mu.Lock()
		defer owner.mu.Unlock()
		current, now, err := owner.textPermissionProfileLocked()
		if err != nil || !owner.liveTextServiceJobLocked(job, broker.Connection) || bounded.Err() != nil || owner.prefix != prefix ||
			current != profile || class != 1 || hello.Purpose != route.ClosedPurposeSubmission || hello.RecipientNodeID != receiver ||
			hello.NetworkID != current.NetworkID || hello.StateGeneration != current.StateGeneration || hello.StateDigest != current.StateDigest ||
			hello.ProfileDigest != current.Digest || hello.ChannelNonce == [32]byte{} || !now.Before(hello.Deadline) ||
			hello.Deadline.After(prepared.plaintext.Deadline) {
			return nil, errors.New("text Introduction token authority changed")
		}
		selected, err := prefix.SubmissionRecipient()
		if err != nil || selected != receiver {
			return nil, errors.New("text Introduction recipient changed")
		}
		return owner.takeTextTokenLocked(current, now, hello, class, bounded)
	}, prepared.operation)
	if err != nil || status != 0 {
		return errors.Join(err, errors.New("text Introduction delivery refused"))
	}
	return prepared.binding.current()
}

// receiveTextIntroduction consumes one delivery from the actual channel owned
// by this Publisher, then acknowledges only after independent local acceptance.
func (owner *textContext) receiveTextIntroduction(ctx context.Context, job *textJobIdentity) (prepared *textIntroductionAttempt, outcome error) {
	if owner == nil || ctx == nil || ctx.Err() != nil {
		return nil, errors.New("text Introduction receiver unavailable")
	}
	owner.mu.Lock()
	if owner.publicationDraining {
		owner.mu.Unlock()
		return nil, errTextPublicationDraining
	}
	live := owner.liveTextServiceJobLocked(job, broker.Administration) && owner.registration != nil
	owner.mu.Unlock()
	if !live {
		return nil, errors.New("text Introduction Publisher job unavailable")
	}
	lifetime, finish, err := owner.beginTextIntroductionExchange(ctx, job, broker.Administration)
	if err != nil {
		return nil, err
	}
	defer func() {
		outcome = finish(outcome)
		if outcome != nil {
			prepared = nil
		}
	}()
	delivery, err := owner.nextTextIntroductionDelivery(lifetime)
	if err != nil {
		return nil, err
	}
	operation := delivery.Operation()
	defer clear(operation)
	prepared, outcome = owner.acceptTextIntroduction(lifetime, job, operation)
	if outcome == nil {
		outcome = owner.prepareTextResponder(lifetime, job, prepared)
	}
	status := uint8(0)
	if outcome != nil {
		status = 1
	}
	if err := delivery.Complete(lifetime, status); err != nil {
		return nil, errors.Join(outcome, err)
	}
	if prepared != nil && !owner.endpoint.clock().Before(prepared.plaintext.Deadline) {
		return nil, errors.New("text Introduction delivery expired before handover")
	}
	return prepared, outcome
}

func (owner *textContext) prepareTextSubmissionStock(ctx context.Context, prefix *route.ClosedSourcePrefix) ([32]byte, state.ClosedProfileView, error) {
	receiver, err := prefix.SubmissionRecipient()
	if err != nil {
		return [32]byte{}, state.ClosedProfileView{}, err
	}
	owner.mu.Lock()
	stocked := false
	profile, _, err := owner.textPermissionProfileLocked()
	if err == nil && owner.permission != nil {
		for _, stock := range owner.permission.stock {
			if stock.challenge.ReceiverNodeID == receiver && stock.challenge.ProfileDigest == profile.Digest &&
				stock.challenge.Class == 1 && stock.challenge.WindowStart == owner.permission.accepted.NotBefore && len(stock.tokens) != 0 {
				stocked = true
			}
		}
	}
	owner.mu.Unlock()
	if err != nil {
		return [32]byte{}, state.ClosedProfileView{}, err
	}
	if !stocked {
		if err := owner.issueTextTokens(ctx, [][32]byte{receiver}, 1); err != nil {
			return [32]byte{}, state.ClosedProfileView{}, err
		}
	}
	return receiver, profile, nil
}
