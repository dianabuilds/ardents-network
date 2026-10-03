//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/endpoint/source"
)

// prepareIssuerStock funds issuer admission only for current requested
// work. Before the first admitted prefix this uses the second bootstrap batch;
// thereafter the last Control token can replenish stock within its allocation.
func (owner *dutyContext) prepareIssuerStock(ctx context.Context, requested [][32]byte, class uint8,
	opening *operationFlight, acquisition joinAcquisition, expected *source.Handle) error {
	owner.mu.Lock()
	_, _, err := owner.permissionProfileLocked()
	if err != nil || ctx.Err() != nil || !opening.admittedLocked(owner) ||
		!joinIssuanceCurrentLocked(owner, acquisition, expected) {
		owner.mu.Unlock()
		return errors.New("text issuer stock owner unavailable")
	}
	receivers, err := owner.tokens.RefillPlanLocked(requested, class, owner.source.CurrentLocked() != nil)
	if err != nil || len(receivers) == 0 {
		owner.mu.Unlock()
		return err
	}
	owner.mu.Unlock()
	return owner.issueTokensForOpeningWithCancellation(ctx, receivers, 1, opening, true, false, acquisition, expected)
}
