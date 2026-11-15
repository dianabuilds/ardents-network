//go:build linux

package session

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/route/bootstrap"
	"github.com/dianabuilds/ardents-network/internal/successor/route/prefix"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
)

// This composition retains actual selection and Hosting roots until the finite
// bootstrap transport joins. The subsequent admitted Prefix reopens those same
// retained roots; issuance cannot relabel the bootstrap generation.
func OpenBootstrap(ctx context.Context, plan funding.Plan, authority funding.Authority, holder *stock.Owner) (_ Prefix, result error) {
	if ctx == nil || !authority.Available() || holder == nil || plan.Domain != 1 || !independentRouteRoots(plan.EntryRoot, plan.InteriorRoot, plan.HostingRoot) {
		return Prefix{}, errors.New("route bootstrap composition invalid")
	}
	selected, err := selection.Open(selection.Config{EntryRoot: plan.EntryRoot, InteriorRoot: plan.InteriorRoot, Domain: 1, Current: authority.Current, Exclusions: plan.Exclusions})
	if err != nil {
		return Prefix{}, err
	}
	budget, err := hosting.Open(plan.HostingRoot)
	if err != nil {
		return Prefix{}, errors.Join(err, selected.Close())
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, selected.Close(), budget.Close())
		}
	}()
	leg, err := selected.Select()
	if err != nil {
		return Prefix{}, err
	}
	end := minRouteDeadline(plan.Deadline, leg.NotAfter, time.Now().Add(bootstrap.Lifetime).UTC().Truncate(time.Second))
	if bound, ok := ctx.Deadline(); ok {
		end = minRouteDeadline(end, bound.UTC().Truncate(time.Second))
	}
	reservation, err := budget.Reserve(ctx, hosting.ReservationRequest{Work: plan.Work, Termination: plan.Termination, WorkUntil: end, HoldUntil: end.Add(5 * time.Second)})
	if err != nil {
		return Prefix{}, err
	}
	var once sync.Once
	var releaseErr error
	release := func() error {
		once.Do(func() {
			releaseErr = errors.Join(releaseRouteReservation(reservation), selected.Close(), budget.Close())
		})
		return releaseErr
	}
	opened, err := prefix.OpenBootstrap(ctx, prefix.BootstrapConfig{Leg: leg, Current: authority.Current, Deadline: end, Release: release})
	if err != nil {
		return Prefix{}, errors.Join(err, release())
	}
	return Prefix{bootstrap: true, close: opened.Close, done: opened.Done(), issue: func(ctx context.Context, class uint8, receivers [][32]byte) error {
		return opened.Issue(ctx, end, funding.IssuerPreparation(ctx, authority, holder, leg, class, receivers))
	}}, nil
}
