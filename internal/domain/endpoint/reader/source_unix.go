//go:build linux || darwin

package reader

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/connection"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	executionruntime "github.com/dianabuilds/ardents-network/internal/domain/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/domain/reachability"
	"github.com/dianabuilds/ardents-network/internal/domain/route/prefix"
	"github.com/dianabuilds/ardents-network/internal/domain/route/selection"
)

// Histories, selection and shared Hosting remain Reader-owned through the
// original Connection's joined retirement. No failed opening redraws a set.
func openSource(operation *executionruntime.Operation, plan funding.Plan, authority funding.Authority, holder *stock.Owner, target string, shared *funding.Parents) (_ *connection.InitiatorSource, closeSource func() error, result error) {
	if operation == nil || operation.CheckReader() != nil || plan.Domain != 1 || !authority.Available() || holder == nil {
		return nil, nil, errors.New("original document Reader composition is unavailable")
	}
	if shared != nil && !shared.Matches(plan) {
		return nil, nil, errors.New("shared Reader Route owners do not match")
	}
	ownedParents := shared == nil
	var installation *selection.Installation
	var budget *hosting.Budget
	var err error
	if ownedParents {
		installation, err = selection.OpenInstallation(selection.InstallationConfig{EntryRoot: plan.EntryRoot, Current: authority.Current, Exclusions: plan.Exclusions})
		if err != nil {
			return nil, nil, err
		}
	} else {
		installation, budget = shared.Entry(), shared.Budget()
	}
	var selected *selection.Owner
	var source *connection.InitiatorSource
	var physical *prefix.Prefix
	history := &reachability.History{}
	var once sync.Once
	var terminal error
	closeAll := func() error {
		once.Do(func() {
			var cleanup error
			if source != nil {
				// Source retains its own exact physical result into this original
				// Operation; its full logical outcome remains with the caller.
				terminal = source.Close()
			} else {
				terminal = physical.Close()
				if physical != nil {
					var complete bool
					cleanup, complete = physical.CleanupResult()
					if !complete {
						cleanup = errors.Join(cleanup, errors.New("original partial Reader Prefix cleanup did not join"))
					}
				}
			}
			cleanup = errors.Join(cleanup, history.Close(), selected.Close())
			if ownedParents {
				if budget != nil {
					cleanup = errors.Join(cleanup, budget.Close())
				}
				cleanup = errors.Join(cleanup, installation.Close())
			}
			terminal = errors.Join(terminal, cleanup)
			operation.RetainCleanup(cleanup)
		})
		return terminal
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, closeAll())
		}
	}()
	selected, err = installation.Borrow(selection.RoleConfig{InteriorRoot: plan.InteriorRoot, Domain: 1})
	if err != nil {
		return nil, nil, err
	}
	if ownedParents {
		budget, err = hosting.Open(plan.HostingRoot)
		if err != nil {
			return nil, nil, err
		}
	}
	physical, leg, flow, err := funding.OpenSource(operation.Context(), plan, selected, budget, authority, holder, 1, true, operation.RetainCleanup)
	if err != nil {
		return nil, nil, err
	}
	choice, err := selection.NewRendezvous(leg, authority.Current, plan.Exclusions)
	if err != nil {
		return nil, nil, err
	}
	source, err = connection.NewInitiatorSource(connection.InitiatorSourceConfig{Operation: operation, Source: physical,
		PrepareRoute: flow.PrepareRoute,
		PrepareSource: func(caller context.Context) (prefix.Config, error) {
			return funding.PrepareSelectedPrefix(caller, plan, leg, budget, authority, holder)
		},
		History: history, Rendezvous: choice, TargetLink: target, Network: leg.Profile.Network, Profile: leg.Profile.Digest,
		WorkSafetyNotAfter: plan.Deadline, WorkSafetyMaximum: plan.Deadline, NoNewRecoveryAfter: plan.Deadline.Add(-5 * time.Second), Exclusions: plan.Exclusions})
	if err != nil {
		return nil, nil, err
	}
	return source, closeAll, nil
}
