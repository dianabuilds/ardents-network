//go:build linux

package route

import (
	"context"
	"errors"
	"time"
)

func (prefix *ClosedSourcePrefix) submissionPeer() (closedBootstrapPeer, error) {
	if prefix == nil || prefix.plan.domain != closedRoleDomainInitiator {
		return closedBootstrapPeer{}, errors.New("capsule submission requires Source ownership")
	}
	return prefix.terminalPeer(ClosedPurposeSubmission)
}

// SubmissionRecipient supplies the current Introduction duty for receiver-local
// token issuance, without accepting a callback address or a worker-selected Node.
func (prefix *ClosedSourcePrefix) SubmissionRecipient() ([32]byte, error) {
	peer, err := prefix.submissionPeer()
	return peer.node, err
}

// SubmitIntroduction sends one exact recipient-confidential capsule through a
// fresh class-1 Control channel. Success means delivery acknowledgement, never
// a joined data leg or authenticated Service response.
func (prefix *ClosedSourcePrefix) SubmitIntroduction(ctx context.Context, present ClosedTokenPresenter, operation []byte) (uint8, error) {
	if ctx == nil || ctx.Err() != nil {
		return 0, errors.New("capsule submission caller unavailable")
	}
	nonce, capsule, err := DecodeClosedIntroductionSubmission(operation)
	if err != nil {
		return 0, err
	}
	defer clear(capsule.Ciphertext)
	if !time.Now().Before(capsule.Expiry) {
		return 0, errors.New("capsule submission expired")
	}
	peer, err := prefix.submissionPeer()
	if err != nil {
		return 0, err
	}
	bounded, cancel := context.WithDeadline(ctx, capsule.Expiry)
	defer cancel()
	body, err := prefix.exchangeControl(bounded, ClosedPurposeSubmission, peer, present, operation)
	if err != nil {
		return 0, err
	}
	defer clear(body)
	status, proof, err := DecodeClosedDescriptorResult(body, nonce)
	if err == nil && len(proof) != 0 {
		err = errors.New("capsule submission returned unexpected proof")
	}
	return status, err
}
