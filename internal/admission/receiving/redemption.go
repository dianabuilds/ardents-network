package receiving

import (
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/spending"
)

// Approval is a verified token hour and any Hosting reservation acquired by
// the receiver's authorization adapter. Even a refused adapter result can own
// a reservation; Redeem releases it on every unsuccessful path.
type Approval struct {
	Window  time.Time
	Release func() error
}

// Redemption is one initial or replenishing use of a token. Deadline is the
// existing finite operation bound; replenishment cannot extend it.
type Redemption struct {
	Class    admission.Class
	Token    []byte
	Deadline time.Time
}

// Redeem authorizes, reserves capacity, durably spends, then checks the final
// deadline. authorize supplies current State verification and Hosting capacity.
// reserve supplies a new channel reservation, or nil for an existing channel's
// refill. Its rollback is called only on refusal: successful physical channel
// ownership remains with the caller. The returned Hosting release transfers to
// that same caller on success. A spent token is never refunded, even on expiry.
func Redeem(request Redemption, spends *spending.Ledger, clock func() time.Time, authorize func() (Approval, error), reserve func() (func(), error)) (Approval, error) {
	if request.Class.Lifetime() == 0 || request.Deadline.IsZero() || spends == nil || clock == nil || authorize == nil {
		return Approval{}, errors.New("closed admission token is unavailable")
	}
	approval, err := authorize()
	release := func() error {
		if approval.Release != nil {
			return approval.Release()
		}
		return nil
	}
	if err != nil || !spending.ValidWindow(approval.Window) {
		return Approval{}, errors.Join(errors.New("closed admission token is unavailable"), err, release())
	}
	var rollback func()
	if reserve != nil {
		rollback, err = reserve()
		if err != nil {
			if rollback != nil {
				rollback()
			}
			return Approval{}, errors.Join(errors.New("closed admission capacity is unavailable"), err, release())
		}
	}
	refuse := func(cause error) (Approval, error) {
		if rollback != nil {
			rollback()
		}
		return Approval{}, errors.Join(cause, release())
	}
	if err := spends.Spend(request.Token, approval.Window, clock().UTC()); err != nil {
		return refuse(errors.Join(errors.New("closed admission token is unavailable"), err))
	}
	// Read again after durable I/O: a pre-spend timestamp cannot authorize work
	// when persistence completed after the immutable deadline.
	if now := clock().UTC(); now.IsZero() || !now.Before(request.Deadline) {
		return refuse(errors.New("closed admission lease is unavailable"))
	}
	return approval, nil
}
