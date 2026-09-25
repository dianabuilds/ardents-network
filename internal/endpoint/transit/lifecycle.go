package transit

import (
	"context"
	"crypto/tls"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

// Lease retains the exact presenting Grant and certificate until Finish reports
// whether they were presented. Finish erases Endpoint-local TLS enrollment
// before the journal commits its terminal state.
type Lease struct {
	Request     credential.Request
	Certificate tls.Certificate
	Grant       []byte
	Finish      func(bool) error
}

// Acquire owns the complete at-most-once journal transaction. Endpoint supplies
// only its issuer exchange and local TLS enrollment; the journal decides all
// durable transitions, stale completions, and terminal outcomes.
func (owner *Acquisition) Acquire(ctx context.Context, scope Scope,
	issue func(context.Context, credential.Request) (credential.Result, error),
	enroll func([]byte, tls.Certificate) (func(), error),
) (Lease, error) {
	attempt, err := owner.begin(scope)
	if err != nil {
		return Lease{}, err
	}
	if attempt.Phase == transitPending {
		result, err := issue(ctx, attempt.Request)
		if err != nil {
			if errors.Is(err, credential.ErrExchangeUnavailable) {
				if staleErr := owner.currentAttempt(attempt.Request.RequestID); staleErr != nil {
					return Lease{}, staleErr
				}
			} else {
				if failErr := owner.fail(attempt.Request.RequestID); failErr != nil {
					return Lease{}, failErr
				}
			}
			return Lease{}, transitAcquisitionOutcomeError{outcome: credential.Unavailable}
		}
		if err := owner.commit(attempt.Request.RequestID, result); err != nil {
			return Lease{}, err
		}
		if result.Outcome != credential.Issued {
			return Lease{}, transitAcquisitionOutcomeError{outcome: result.Outcome}
		}
	}
	presenting, err := owner.present(attempt.Request.RequestID, scope)
	if err != nil {
		return Lease{}, err
	}
	erase, err := enroll(presenting.Grant, presenting.Certificate)
	if err != nil {
		_ = owner.finish(presenting.Request.RequestID, false)
		return Lease{}, err
	}
	return Lease{Request: presenting.Request, Certificate: presenting.Certificate, Grant: presenting.Grant, Finish: func(presented bool) error {
		erase()
		return owner.finish(presenting.Request.RequestID, presented)
	}}, nil
}
