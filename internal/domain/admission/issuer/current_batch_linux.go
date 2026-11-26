//go:build linux

package issuer

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/quota"
)

// IssueCurrentBatch returns the canonical response after current-authority
// issuance and retains cancellation and uncertain retirement as errors.
func IssueCurrentBatch(ctx context.Context, plan Plan, batch []byte, kind quota.Kind, observe func() (admission.AuthorityFacts, time.Time, error)) ([]byte, error) {
	if ctx == nil {
		return nil, errors.New("issuer context is unavailable")
	}
	result := IssueCurrent(ctx, plan, batch, kind, observe)
	if err := ctx.Err(); err != nil {
		clear(result.Response)
		return nil, err
	}
	_, outcome := result.Status()
	if outcome == "storage-uncertain" {
		clear(result.Response)
		return nil, errors.New("route issuer retirement uncertain")
	}
	if result.Response != nil {
		return result.Response, nil
	}
	status := admission.ClosedTokenUnavailable
	if outcome == string(quota.Exhausted) {
		status = admission.ClosedTokenExhausted
	}
	return admission.EncodeClosedTokenBatchResult(admission.ClosedTokenBatchResult{Status: status})
}
