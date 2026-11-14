//go:build linux

package main

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
func startRouteBootstrap(ctx context.Context, plan routePrefixPlan, authority admissionAuthority, holder *stock.Owner) (_ routeHandle, result error) {
	if ctx == nil || authority.current == nil || holder == nil || plan.Domain != 1 || !independentRouteRoots(plan.EntryRoot, plan.InteriorRoot, plan.HostingRoot) {
		return routeHandle{}, errors.New("route bootstrap composition invalid")
	}
	selected, err := selection.Open(selection.Config{EntryRoot: plan.EntryRoot, InteriorRoot: plan.InteriorRoot, Domain: 1, Current: authority.current, Exclusions: plan.Exclusions})
	if err != nil {
		return routeHandle{}, err
	}
	budget, err := hosting.Open(plan.HostingRoot)
	if err != nil {
		return routeHandle{}, errors.Join(err, selected.Close())
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, selected.Close(), budget.Close())
		}
	}()
	leg, err := selected.Select()
	if err != nil {
		return routeHandle{}, err
	}
	end := minRouteDeadline(plan.Deadline, leg.NotAfter, time.Now().Add(bootstrap.Lifetime).UTC().Truncate(time.Second))
	if bound, ok := ctx.Deadline(); ok {
		end = minRouteDeadline(end, bound.UTC().Truncate(time.Second))
	}
	reservation, err := budget.Reserve(ctx, hosting.ReservationRequest{Work: plan.Work, Termination: plan.Termination, WorkUntil: end, HoldUntil: end.Add(5 * time.Second)})
	if err != nil {
		return routeHandle{}, err
	}
	var once sync.Once
	var releaseErr error
	release := func() error {
		once.Do(func() {
			releaseErr = errors.Join(releaseRouteReservation(reservation), selected.Close(), budget.Close())
		})
		return releaseErr
	}
	opened, err := prefix.OpenBootstrap(ctx, prefix.BootstrapConfig{Leg: leg, Current: authority.current, Deadline: end, Release: release})
	if err != nil {
		return routeHandle{}, errors.Join(err, release())
	}
	return routeHandle{bootstrap: true, close: opened.Close, done: opened.Done(), issue: func(ctx context.Context, class uint8, receivers [][32]byte) error {
		return opened.Issue(ctx, end, funding.IssuerPreparation(ctx, fundingAuthority(authority), holder, leg, class, receivers))
	}}, nil
}
