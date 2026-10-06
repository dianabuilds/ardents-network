//go:build linux

package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/token"
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
		return opened.Issue(ctx, end, routeIssuerPreparation(ctx, authority, holder, leg, class, receivers))
	}}, nil
}

// Stock owns permission, request/retry identity and verified token deposit.
// Route supplies the actual mode, exact opening binding and final local guard.
func routeIssuerPreparation(ctx context.Context, authority admissionAuthority, holder *stock.Owner, leg selection.Leg, class uint8, receivers [][32]byte) func(prefix.IssuerBinding) (prefix.IssuerBatch, error) {
	return func(binding prefix.IssuerBinding) (prefix.IssuerBatch, error) {
		if class < 1 || class > 3 || ctx.Err() != nil {
			return prefix.IssuerBatch{}, errors.New("route issuance intent invalid")
		}
		view, err := authority.current()
		if err != nil || view.Profile().Digest != binding.ProfileDigest {
			return prefix.IssuerBatch{}, errors.Join(errors.New("route issuance profile differs"), err)
		}
		if len(receivers) == 0 {
			switch class {
			case 1:
				receivers = [][32]byte{view.Profile().IssuerNodeID}
			case 2:
				receivers = [][32]byte{leg.Entry.NodeID, leg.Interior.NodeID}
			default:
				return prefix.IssuerBatch{}, errors.New("route issuance receivers absent")
			}
		}
		var challenges []token.ClosedTokenContext
		for _, id := range receivers {
			member, err := view.Member(id, view.ObservedAt())
			if err != nil {
				return prefix.IssuerBatch{}, err
			}
			challenges = append(challenges, token.ClosedTokenContext{NetworkID: view.Profile().Network, ProfileDigest: binding.ProfileDigest, IssuerNodeID: view.Profile().IssuerNodeID, ReceiverNodeID: id, ReceiverDutyGeneration: member.DutyGeneration, Class: class, WindowStart: view.ObservedAt().UTC().Truncate(time.Hour)})
		}
		intent := stock.IssuanceIntent{Challenges: challenges, Selection: stock.ExchangeBinding{ID: binding.ID, ProfileDigest: binding.ProfileDigest}, Bootstrap: binding.Bootstrap, Deadline: binding.Deadline}
		if err := authority.intent(intent); err != nil {
			return prefix.IssuerBatch{}, err
		}
		attempt, err := holder.Begin(intent)
		if err != nil {
			return prefix.IssuerBatch{}, err
		}
		raw, end, err := attempt.Request()
		if err != nil {
			return prefix.IssuerBatch{}, errors.Join(err, attempt.Complete(nil, err))
		}
		return prefix.IssuerBatch{Request: raw, Deadline: end, Complete: attempt.CompleteBound}, nil
	}
}
