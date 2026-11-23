package funding

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/prefix"
	"github.com/dianabuilds/ardents-network/internal/route/role"
	"github.com/dianabuilds/ardents-network/internal/route/selection"
)

// Preparation retains a genuine independent role selection and finite shared
// Hosting reservation. It performs no dial and grants no ready physical prefix.
func PreparePrefix(ctx context.Context, plan Plan, selected *selection.Owner, budget *hosting.Budget, authority Authority, holder *stock.Owner) (prefix.Config, error) {
	if ctx == nil || selected == nil || budget == nil || authority.current == nil || authority.presentation == nil || holder == nil {
		return prefix.Config{}, errors.New("publisher prefix preparation unavailable")
	}
	if err := ctx.Err(); err != nil {
		return prefix.Config{}, err
	}
	leg, err := selected.Select()
	if err != nil {
		return prefix.Config{}, err
	}
	return PrepareSelectedPrefix(ctx, plan, leg, budget, authority, holder)
}

// A new physical reservation for an already retained leg grants no selection
// or accepting observation. Prefix reobserves the original genuine authority
// before every opening/effect and at its final handoff.
func PrepareSelectedPrefix(ctx context.Context, plan Plan, leg selection.Leg, budget *hosting.Budget, authority Authority, holder *stock.Owner) (prefix.Config, error) {
	if ctx == nil || budget == nil || authority.current == nil || authority.witness == nil || authority.presentation == nil || holder == nil {
		return prefix.Config{}, errors.New("retained Publisher leg preparation unavailable")
	}
	if err := ctx.Err(); err != nil {
		return prefix.Config{}, err
	}
	witness, err := authority.witness()
	if err != nil {
		return prefix.Config{}, err
	}
	if err := witness.CheckCommit(leg.Profile); err != nil {
		return prefix.Config{}, err
	}
	reservation, err := budget.Reserve(ctx, hosting.ReservationRequest{Work: plan.Work, Termination: plan.Termination, WorkUntil: plan.Deadline, HoldUntil: plan.Deadline.Add(5 * time.Second)})
	if err != nil {
		return prefix.Config{}, err
	}
	var once sync.Once
	var terminal error
	release := func() error { once.Do(func() { terminal = releaseRouteReservation(reservation) }); return terminal }
	if err := ctx.Err(); err != nil {
		return prefix.Config{}, errors.Join(err, release())
	}
	return prefix.Config{Leg: leg, Current: authority.current, NetworkWitness: witness, Deadline: plan.Deadline, Release: release,
		HoldRefill: func(caller context.Context, additional uint64) (func() error, error) {
			limit := admission.ForwardClass.ByteLimit()
			if err := budget.CoversJoint(caller, reservation, hosting.JointTraffic{Tx: limit, Rx: limit, Total: limit}); err != nil {
				return nil, err
			}
			held, err := budget.ReserveAdditionalJoint(caller, reservation, hosting.JointTraffic{Tx: additional, Rx: additional, Total: additional})
			if err != nil {
				return nil, err
			}
			return func() error { return releaseRouteReservation(held) }, nil
		}, Present: func(caller context.Context, hello ardp.Hello) ([]byte, error) {
			presentation := stock.Presentation{NetworkID: hello.NetworkID, StateGeneration: hello.StateGeneration, StateDigest: hello.StateDigest, ProfileDigest: hello.ProfileDigest, RecipientNodeID: hello.RecipientNodeID, RecipientDutyGeneration: hello.RecipientDutyGeneration, ChannelNonce: hello.ChannelNonce, Deadline: hello.Deadline}
			if err := authority.presentation(presentation); err != nil {
				return nil, err
			}
			class, err := role.AdmissionClass(hello.Purpose)
			if err != nil {
				return nil, err
			}
			return holder.Take(caller, presentation, uint8(class))
		}}, nil
}

func minRouteDeadline(end time.Time, bounds ...time.Time) time.Time {
	for _, bound := range bounds {
		if bound.Before(end) {
			end = bound
		}
	}
	return end
}
