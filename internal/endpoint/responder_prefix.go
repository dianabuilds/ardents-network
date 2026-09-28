//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
)

// prepareResponder consumes the accepted capsule and live Publisher job.
// Issuance remains on Source; the data prefix uses separate Domain-3 members.
// This establishes only forwarding readiness, never a paired Attachment.
func (owner *textContext) prepareResponder(ctx context.Context, job *jobIdentity, accepted *introductionAttempt) error {
	if owner == nil || ctx == nil || ctx.Err() != nil || accepted == nil ||
		!accepted.binding.servesJob(owner, job) {
		return errors.New("text Responder authority unavailable")
	}
	if err := accepted.binding.current(); err != nil {
		return err
	}
	bounded, cancel := context.WithDeadline(ctx, accepted.plaintext.Deadline)
	defer cancel()
	if err := owner.prepareResponderSource(bounded, job); err != nil {
		return err
	}
	owner.mu.Lock()
	_, _, err := owner.permissionProfileLocked()
	live := err == nil && owner.liveServiceJobLocked(job, broker.Administration)
	prefix := owner.responder.currentLocked()
	owner.mu.Unlock()
	if !live {
		return errors.New("text Responder job unavailable")
	}
	if prefix == nil {
		prefix, err = owner.openResponderPrefix(bounded)
		if err != nil {
			return err
		}
	}
	node, generation, until, err := prefix.dataJoinRecipient()
	if err != nil || node != accepted.plaintext.RendezvousNode || generation != accepted.plaintext.RendezvousDutyGeneration ||
		accepted.plaintext.Deadline.After(until) || bounded.Err() != nil {
		return errors.Join(err, bounded.Err(), errors.New("text Responder Rendezvous changed"))
	}
	if err := accepted.binding.current(); err != nil {
		return err
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if !prefix.currentLocked(&owner.responder) || !owner.liveServiceJobLocked(job, broker.Administration) ||
		bounded.Err() != nil || !owner.endpoint.clock().Before(accepted.plaintext.Deadline) {
		return errors.New("text Responder owner retired before handover")
	}
	if prefix.retired() {
		return errors.New("text Responder prefix retired before handover")
	}
	return nil
}
