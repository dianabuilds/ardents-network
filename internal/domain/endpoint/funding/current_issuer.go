package funding

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
)

var ErrInvalidCurrentIssuer = errors.New("invalid current Issuer configuration")

// IssueCurrent opens the selected original authority, issues once and joins
// its authority owner before making a result available.
func IssueCurrent(ctx context.Context, plan issuer.Plan, profile string, network *state.AuthorityDeclaration, batch []byte, kind quota.Kind) (issuer.Result, error) {
	if ctx == nil || !ValidAdmissionAuthority(profile, network, plan.KeyRoot, plan.AdmissionRoot, plan.ResultRoot) {
		return issuer.Result{}, ErrInvalidCurrentIssuer
	}
	authority, err := OpenAdmissionAuthority(profile, network)
	if err != nil {
		return issuer.Result{}, err
	}
	result := issuer.IssueCurrent(ctx, plan, batch, kind, func() (admission.AuthorityFacts, time.Time, error) {
		return authority.Issuer(plan.KeyBinding.Signer)
	})
	if authority.Close() != nil {
		result.Response = nil
		result.Phase, result.Outcome = "close-network", "storage-uncertain"
	}
	return result, nil
}
