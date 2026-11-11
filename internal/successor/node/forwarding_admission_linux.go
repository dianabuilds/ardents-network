//go:build linux

package node

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
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

func (f *forwarding) admit(ctx context.Context, c receiver.Channel, raw []byte) (_ receiving.Grant, returned error) {
	started, stage := time.Now(), "purpose"
	defer func() { f.retainWorkRefusal(ctx, "admission", stage, started, returned) }()
	class, err := role.AdmissionClass(c.Hello.Purpose)
	if err != nil || class != admission.ForwardClass {
		return receiving.Grant{}, errors.Join(errors.New("original Forwarding purpose differs"), err)
	}
	stage = "receiving"
	return f.receiving.Accept(ctx, class, raw, c.Hello.Deadline, func() (func() error, error) {
		stage = "reservation"
		held, err := f.budget.Reserve(ctx, hosting.ReservationRequest{Work: f.config.Work, Termination: f.config.Termination, WorkUntil: c.Hello.Deadline, HoldUntil: c.Hello.Deadline.Add(5 * time.Second)})
		if err != nil {
			return nil, err
		}
		limit := class.ByteLimit()
		stage = "coverage"
		if err := f.budget.CoversJoint(ctx, held, hosting.JointTraffic{Tx: limit, Rx: limit, Total: limit}); err != nil {
			// No physical borrower or receiving spend exists yet. An operator's
			// smaller envelope cannot back the token's complete allowance.
			return nil, errors.Join(err, releaseReservation(held))
		}
		f.reservationsMu.Lock()
		f.reservations[c] = held
		f.reservationsMu.Unlock()
		stage = "retention"
		release, err := c.HoldReservation(func() error {
			f.reservationsMu.Lock()
			delete(f.reservations, c)
			f.reservationsMu.Unlock()
			return releaseReservation(held)
		})
		if err == nil {
			stage = "receiving-commit"
		}
		return release, err
	})
}
func releaseReservation(held *hosting.Reservation) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return held.Release(ctx)
}
func (f *forwarding) refill(ctx context.Context, c receiver.Channel, original receiving.Grant, remaining uint64, raw []byte) (_ receiving.Grant, returned error) {
	started, stage := time.Now(), "purpose"
	defer func() { f.retainWorkRefusal(ctx, "refill", stage, started, returned) }()
	limit := admission.ForwardClass.ByteLimit()
	if c.Hello.Purpose != ardp.PurposeForwarding || remaining == 0 || remaining >= limit {
		return receiving.Grant{}, errors.New("original Forwarding refill unavailable")
	}
	f.reservationsMu.Lock()
	held := f.reservations[c]
	f.reservationsMu.Unlock()
	stage = "receiving"
	return f.receiving.Refill(ctx, original, remaining, raw, func() (func() error, error) {
		stage = "coverage"
		if err := f.budget.CoversJoint(ctx, held, hosting.JointTraffic{Tx: limit, Rx: limit, Total: limit}); err != nil {
			return nil, err
		}
		delta := limit - remaining
		stage = "reservation"
		additional, err := f.budget.ReserveAdditionalJoint(ctx, held, hosting.JointTraffic{Tx: delta, Rx: delta, Total: delta})
		if err != nil {
			return nil, err
		}
		stage = "retention"
		release, err := c.HoldReservation(func() error { return releaseReservation(additional) })
		if err == nil {
			stage = "receiving-commit"
		}
		return release, err
	})
}
