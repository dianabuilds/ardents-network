package endpoint

import (
	"context"
	"crypto/tls"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

// acquiredTransitCredential retains one presenting attempt until the caller
// reports whether its exact Transit Grant was presented. The finish closure
// erases the Endpoint-local TLS enrollment before the journal transition.
type acquiredTransitCredential struct {
	attempt transitAcquisitionAttempt
	finish  func(bool) error
}

// acquire owns the complete at-most-once journal transaction. Endpoint supplies
// only its issuer exchange and local TLS enrollment; the journal decides all
// durable transitions, stale completions, and terminal outcomes.
func (owner *transitAcquisition) acquire(ctx context.Context, scope transitAcquisitionScope,
	issue func(context.Context, credential.Request) (credential.Result, error),
	enroll func([]byte, tls.Certificate) (func(), error),
) (acquiredTransitCredential, error) {
	attempt, err := owner.begin(scope)
	if err != nil {
		return acquiredTransitCredential{}, err
	}
	if attempt.Phase == transitPending {
		result, err := issue(ctx, attempt.Request)
		if err != nil {
			if errors.Is(err, credential.ErrExchangeUnavailable) {
				if staleErr := owner.currentAttempt(attempt.Request.RequestID); staleErr != nil {
					return acquiredTransitCredential{}, staleErr
				}
			} else {
				if failErr := owner.fail(attempt.Request.RequestID); failErr != nil {
					return acquiredTransitCredential{}, failErr
				}
			}
			return acquiredTransitCredential{}, transitAcquisitionOutcomeError{outcome: credential.Unavailable}
		}
		if err := owner.commit(attempt.Request.RequestID, result); err != nil {
			return acquiredTransitCredential{}, err
		}
		if result.Outcome != credential.Issued {
			return acquiredTransitCredential{}, transitAcquisitionOutcomeError{outcome: result.Outcome}
		}
	}
	presenting, err := owner.present(attempt.Request.RequestID, scope)
	if err != nil {
		return acquiredTransitCredential{}, err
	}
	erase, err := enroll(presenting.Grant, presenting.Certificate)
	if err != nil {
		_ = owner.finish(presenting.Request.RequestID, false)
		return acquiredTransitCredential{}, err
	}
	return acquiredTransitCredential{attempt: presenting, finish: func(presented bool) error {
		erase()
		return owner.finish(presenting.Request.RequestID, presented)
	}}, nil
}
