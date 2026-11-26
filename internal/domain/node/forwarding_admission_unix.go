//go:build linux || darwin

package node

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/route/role"
)

// Forwarding admission retains actual work and refill reservations until the
// Route borrower joins; local lifetime supervision remains with the invocation.
func (f *forwarding) observeAdmission(binding receiving.Receiver) (receiving.Observation, error) {
	view, err := f.current()
	if err != nil {
		return receiving.Observation{}, err
	}
	p := view.Profile()
	m, err := view.Member(binding.NodeID, view.ObservedAt())
	if err != nil || m.DutyGeneration != binding.DutyGeneration {
		return receiving.Observation{}, errors.Join(errors.New("original receiving duty changed"), err)
	}
	facts := admissionFacts(p)
	end := m.NotAfter()
	if p.NotAfter.Before(end) {
		end = p.NotAfter
	}
	return receiving.Observation{Profile: facts, Receiver: binding, Now: view.ObservedAt(), NotAfter: end}, nil
}

func (f *forwarding) admit(ctx context.Context, c receiver.Channel, raw []byte) (receiving.Grant, error) {
	class, err := role.AdmissionClass(c.Hello.Purpose)
	if err != nil || class != admission.ForwardClass {
		return receiving.Grant{}, errors.Join(errors.New("original Forwarding purpose differs"), err)
	}
	return f.receiving.Accept(ctx, class, raw, c.Hello.Deadline, func() (func() error, error) {
		held, err := f.budget.Reserve(ctx, hosting.ReservationRequest{Work: f.config.Work, Termination: f.config.Termination, WorkUntil: c.Hello.Deadline, HoldUntil: c.Hello.Deadline.Add(5 * time.Second)})
		if err != nil {
			return nil, err
		}
		limit := class.ByteLimit()
		if err := f.budget.CoversJoint(ctx, held, hosting.JointTraffic{Tx: limit, Rx: limit, Total: limit}); err != nil {
			// No physical borrower or receiving spend exists yet. An operator's
			// smaller envelope cannot back the token's complete allowance.
			return nil, errors.Join(err, held.ReleaseJoined())
		}
		f.reservationsMu.Lock()
		f.reservations[c] = held
		f.reservationsMu.Unlock()
		return c.HoldReservation(func() error {
			f.reservationsMu.Lock()
			delete(f.reservations, c)
			f.reservationsMu.Unlock()
			return held.ReleaseJoined()
		})
	})
}
func (f *forwarding) refill(ctx context.Context, c receiver.Channel, original receiving.Grant, remaining uint64, raw []byte) (receiving.Grant, error) {
	f.reservationsMu.Lock()
	held := f.reservations[c]
	f.reservationsMu.Unlock()
	return RefillForwarding(ctx, c, original, remaining, raw, f.receiving, f.budget, held)
}

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
