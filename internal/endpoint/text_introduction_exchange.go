//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
)

func (owner *textContext) submitTextIntroduction(ctx context.Context, job *textJobIdentity, prepared *textIntroductionAttempt) (outcome error) {
	if owner == nil || ctx == nil || ctx.Err() != nil || prepared == nil || prepared.binding == nil ||
		prepared.binding.owner != owner || prepared.binding.job != job {
		return errors.New("text Introduction submission unavailable")
	}
	owner.mu.Lock()
	prefix := owner.currentTextSourceLocked()
	live := owner.liveTextServiceJobLocked(job, broker.Connection) && prefix != nil && !prepared.submitted
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
	status, err := prefix.submitIntroduction(bounded, func(hello route.ClosedHello, class uint8) ([]byte, error) {
		owner.mu.Lock()
		defer owner.mu.Unlock()
		current, now, err := owner.textPermissionProfileLocked()
		if err != nil || !owner.liveTextServiceJobLocked(job, broker.Connection) || bounded.Err() != nil || !prefix.currentLocked(owner) ||
			current != profile || class != 1 || hello.Purpose != route.ClosedPurposeSubmission || hello.RecipientNodeID != receiver ||
			hello.NetworkID != current.NetworkID || hello.StateGeneration != current.StateGeneration || hello.StateDigest != current.StateDigest ||
			hello.ProfileDigest != current.Digest || hello.ChannelNonce == [32]byte{} || !now.Before(hello.Deadline) ||
			hello.Deadline.After(prepared.plaintext.Deadline) {
			return nil, errors.New("text Introduction token authority changed")
		}
		selected, err := prefix.submissionRecipient()
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
	return owner.receiveTextIntroductionAfterDelivery(ctx, job, nil)
}

func (owner *textContext) receiveTextIntroductionAfterDelivery(ctx context.Context, job *textJobIdentity, deliveryReceived func()) (prepared *textIntroductionAttempt, outcome error) {
	return owner.receiveTextIntroductionWith(ctx, job, textIntroductionDeliveryKey{generation: 1}, nil,
		owner.acceptDispatchedTextIntroduction, deliveryReceived)
}

func (owner *textContext) receiveTextRecovery(ctx context.Context, job *textJobIdentity, binding *textServiceBinding,
	request nativeconnection.Recovery) (*textIntroductionAttempt, error) {
	if ctx == nil {
		return nil, errors.New("text recovery receiver context unavailable")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if binding == nil || binding.owner != owner || binding.job != job {
		return nil, errors.New("text recovery receiver unavailable")
	}
	if err := binding.validateTextServiceRecovery(request); err != nil {
		return nil, err
	}
	want := textIntroductionDeliveryKey{connection: binding.facts.ConnectionNonce, generation: request.Generation}
	return owner.receiveTextIntroductionWith(ctx, job, want, binding, func(ctx context.Context, job *textJobIdentity, operation []byte) (*textIntroductionAttempt, error) {
		return owner.acceptDispatchedTextRecovery(ctx, job, operation, binding, request)
	}, nil)
}

type textIntroductionAcceptor func(context.Context, *textJobIdentity, []byte) (*textIntroductionAttempt, error)

func (owner *textContext) receiveTextIntroductionWith(ctx context.Context, job *textJobIdentity,
	want textIntroductionDeliveryKey, binding *textServiceBinding,
	accept textIntroductionAcceptor, deliveryReceived func()) (prepared *textIntroductionAttempt, outcome error) {
	if owner == nil || ctx == nil {
		return nil, errors.New("text Introduction receiver unavailable")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if accept == nil {
		return nil, errors.New("text Introduction acceptance owner unavailable")
	}
	owner.mu.Lock()
	if owner.textPublicationPairLifecycle.drainingLocked() {
		owner.mu.Unlock()
		return nil, errTextPublicationDraining
	}
	live := owner.liveTextServiceJobLocked(job, broker.Administration) && owner.textPublicationPairLifecycle.currentLocked() != nil
	owner.mu.Unlock()
	if !live {
		return nil, errors.New("text Introduction Publisher job unavailable")
	}
	lifetime, finish, err := owner.beginTextIntroductionExchange(ctx, job, broker.Administration)
	if err != nil {
		return nil, err
	}
	var retainedBinding *textServiceBinding
	defer func() {
		outcome = finish(outcome)
		if outcome != nil && retainedBinding != nil {
			outcome = errors.Join(outcome, retainedBinding.releaseTextIntroductionRecovery())
		}
		if outcome != nil {
			prepared = nil
		}
	}()
	delivery, err := owner.dispatchTextIntroductionDelivery(lifetime, job, want, binding)
	if err != nil {
		return nil, err
	}
	if deliveryReceived != nil {
		deliveryReceived()
	}
	operation := delivery.delivery.Operation()
	defer clear(operation)
	prepared, outcome = accept(lifetime, job, operation)
	if outcome == nil {
		outcome = owner.prepareTextResponder(lifetime, job, prepared)
	}
	if outcome == nil && want.generation == 1 {
		outcome = owner.retainTextIntroductionRecovery(prepared.binding)
		if outcome == nil {
			retainedBinding = prepared.binding
		}
	}
	status := uint8(0)
	if outcome != nil {
		status = 1
	}
	// The registration owns the terminal RESULT after routing. A recovery
	// attempt may be canceled after local refusal without stranding its lane.
	if err := owner.completeTextIntroductionDelivery(delivery.delivery, delivery.expires, status); err != nil {
		return nil, errors.Join(outcome, err)
	}
	if prepared != nil && !owner.endpoint.clock().Before(prepared.plaintext.Deadline) {
		return nil, errors.New("text Introduction delivery expired before handover")
	}
	return prepared, outcome
}

func (owner *textContext) prepareTextSubmissionStock(ctx context.Context, prefix *textSourceHandle) ([32]byte, state.ClosedProfileView, error) {
	return owner.prepareTextSubmissionStockWithCancellation(ctx, prefix, false)
}

func (owner *textContext) prepareTextRecoverySubmissionStock(ctx context.Context,
	prefix *textSourceHandle) ([32]byte, state.ClosedProfileView, error) {
	return owner.prepareTextSubmissionStockWithCancellation(ctx, prefix, true)
}

func (owner *textContext) prepareTextSubmissionStockWithCancellation(ctx context.Context, prefix *textSourceHandle,
	discardCanceled bool) ([32]byte, state.ClosedProfileView, error) {
	receiver, err := prefix.submissionRecipient()
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
		issue := owner.issueTextTokens
		if discardCanceled {
			issue = owner.issueTextRecoveryTokens
		}
		if err := issue(ctx, [][32]byte{receiver}, 1); err != nil {
			return [32]byte{}, state.ClosedProfileView{}, err
		}
	}
	return receiver, profile, nil
}
