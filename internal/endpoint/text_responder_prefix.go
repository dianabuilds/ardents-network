//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
)

// prepareTextResponder consumes the accepted capsule and live Publisher job.
// Issuance remains on Source; the data prefix uses separate Domain-3 members.
// This establishes only forwarding readiness, never a paired Attachment.
func (owner *textContext) prepareTextResponder(ctx context.Context, job *textJobIdentity, accepted *textIntroductionAttempt) error {
	if owner == nil || ctx == nil || ctx.Err() != nil || accepted == nil || accepted.binding == nil ||
		accepted.binding.owner != owner || accepted.binding.job != job {
		return errors.New("text Responder authority unavailable")
	}
	if err := accepted.binding.current(); err != nil {
		return err
	}
	bounded, cancel := context.WithDeadline(ctx, accepted.plaintext.Deadline)
	defer cancel()
	if err := owner.prepareTextResponderSource(bounded, job); err != nil {
		return err
	}
	owner.mu.Lock()
	_, _, err := owner.textPermissionProfileLocked()
	live := err == nil && owner.liveTextServiceJobLocked(job, broker.Administration)
	prefix := owner.responder.prefix
	owner.mu.Unlock()
	if !live {
		return errors.New("text Responder job unavailable")
	}
	if prefix == nil {
		prefix, err = owner.openTextPublisherPrefix(bounded, &owner.responder, 3)
		if err != nil {
			return err
		}
	}
	node, generation, until, err := prefix.DataJoinRecipient()
	if err != nil || node != accepted.plaintext.RendezvousNode || generation != accepted.plaintext.RendezvousDutyGeneration ||
		accepted.plaintext.Deadline.After(until) || bounded.Err() != nil {
		return errors.Join(err, bounded.Err(), errors.New("text Responder Rendezvous changed"))
	}
	if err := accepted.binding.current(); err != nil {
		return err
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.responder.prefix != prefix || !owner.liveTextServiceJobLocked(job, broker.Administration) ||
		bounded.Err() != nil || !owner.endpoint.clock().Before(accepted.plaintext.Deadline) {
		return errors.New("text Responder owner retired before handover")
	}
	select {
	case <-prefix.Done():
		return errors.New("text Responder prefix retired before handover")
	default:
		return nil
	}
}
