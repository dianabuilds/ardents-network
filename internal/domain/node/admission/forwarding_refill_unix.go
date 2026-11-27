//go:build linux || darwin

package admission

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/domain/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/domain/route/receiver"
)

// RefillForwarding retains the additional Hosting reservation on the original
// Route channel before Admission can spend the refill.
func RefillForwarding(ctx context.Context, c receiver.Channel, original receiving.Grant, remaining uint64, raw []byte, owner *receiving.Owner, budget *hosting.Budget, held *hosting.Reservation) (receiving.Grant, error) {
	limit := admission.ForwardClass.ByteLimit()
	if c.Hello.Purpose != ardp.PurposeForwarding || remaining == 0 || remaining >= limit {
		return receiving.Grant{}, errors.New("original Forwarding refill unavailable")
	}
	return owner.Refill(ctx, original, remaining, raw, func() (func() error, error) {
		if err := budget.CoversJoint(ctx, held, hosting.JointTraffic{Tx: limit, Rx: limit, Total: limit}); err != nil {
			return nil, err
		}
		delta := limit - remaining
		additional, err := budget.ReserveAdditionalJoint(ctx, held, hosting.JointTraffic{Tx: delta, Rx: delta, Total: delta})
		if err != nil {
			return nil, err
		}
		return c.HoldReservation(additional.ReleaseJoined)
	})
}
