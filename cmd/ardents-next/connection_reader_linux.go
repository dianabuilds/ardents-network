//go:build linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/connection"
	"github.com/dianabuilds/ardents-network/internal/successor/execution"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
	"github.com/dianabuilds/ardents-network/internal/successor/route/prefix"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
)

// This ordinary Reader consumes already provisioned finite Stock and a verified
// Target Link. Instance/Authority material is not a Reader input. Qualified
// original workers and genuine Route owners precede every Application effect.
func runExecutionTextRead(ctx context.Context, plan holderPlan, generation, principal [32]byte, target string, out io.Writer) (code int) {
	if ctx == nil || plan.Role != admission.AllocationUser || plan.Network == nil || plan.Route == nil || plan.Route.Domain != 1 ||
		!independentRouteRoots(plan.Network.Root, plan.Root, plan.Route.EntryRoot, plan.Route.InteriorRoot, plan.Route.HostingRoot) {
		return 2
	}
	local, err := executionruntime.New(execution.Config{ID: generation, Grants: []execution.Grant{{Principal: principal, Surface: execution.Connection}}})
	if err != nil {
		return 2
	}
	defer func() {
		if local.Close() != nil {
			code = 1
		}
	}()
	worker, err := local.LaunchReader(ctx, principal)
	if err != nil {
		return 1
	}
	defer func() {
		if worker.Close() != nil {
			code = 1
		}
	}()
	operation, err := worker.BeginOperation(ctx)
	if err != nil {
		return 1
	}
	defer operation.Close()
	contract, err := operation.BindStreamContract(512, uint64(textdocument.MaximumBytes+13))
	if err != nil {
		return 1
	}
	authority, err := openAdmissionAuthority("", plan.Network)
	if err != nil {
		return 1
	}
	defer func() {
		if authority.close() != nil {
			code = 1
		}
	}()
	original := authority.current
	if original == nil {
		return 1
	}
	authority = networkAdmissionAuthority(func() (network.RuntimeView, error) {
		if err := operation.CheckReader(); err != nil {
			return network.RuntimeView{}, err
		}
		view, err := original()
		return view, errors.Join(err, operation.CheckReader())
	}, authority.close, authority.witness)
	holder, err := stock.Open(plan.Root, plan.Role, authority.observe)
	if err != nil {
		return 1
	}
	defer func() {
		if holder.Close() != nil {
			code = 1
		}
	}()
	source, closeSource, err := openTextReaderSource(operation, *plan.Route, authority, holder, target)
	if err != nil {
		return 1
	}
	defer func() {
		if closeSource() != nil {
			code = 1
		}
	}()
	stream, err := source.OpenStream(contract)
	if err != nil {
		return 1
	}
	transferred, result, err := operation.DeliverReaderStream(contract, stream)
	if !transferred {
		_ = stream.Close()
		operation.RetainCleanup(stream.JoinedCleanup())
	}
	if err != nil || result == nil {
		return 1
	}
	defer result.Close()
	if err := closeSource(); err != nil {
		return 1
	}
	operation.Close()
	if worker.Close() != nil || !worker.CompletedCurrent() || ctx.Err() != nil {
		return 1
	}
	body, err := result.TakeBytes()
	if err != nil {
		return 1
	}
	defer clear(body)
	if json.NewEncoder(out).Encode(struct {
		Outcome  string `json:"outcome"`
		Document string `json:"document"`
	}{"text-received", string(body)}) != nil {
		return 2
	}
	return 0
}

// Histories, selection and shared Hosting remain command-owned through the
// original Connection's joined retirement. No failed opening redraws a set.
func openTextReaderSource(operation *executionruntime.Operation, plan routePrefixPlan, authority admissionAuthority, holder *stock.Owner, target string, shared ...*installedRouteOwners) (_ *connection.InitiatorSource, closeSource func() error, result error) {
	if operation == nil || operation.CheckReader() != nil || plan.Domain != 1 || authority.current == nil || holder == nil {
		return nil, nil, errors.New("original Text Reader composition is unavailable")
	}
	if len(shared) > 1 || len(shared) == 1 && !shared[0].matches(plan) {
		return nil, nil, errors.New("Reader shared Route owners do not match")
	}
	ownedParents := len(shared) == 0
	var installation *selection.Installation
	var budget *hosting.Budget
	var err error
	if ownedParents {
		installation, err = selection.OpenInstallation(selection.InstallationConfig{EntryRoot: plan.EntryRoot, Current: authority.current, Exclusions: plan.Exclusions})
		if err != nil {
			return nil, nil, err
		}
	} else {
		installation, budget = shared[0].entry, shared[0].budget
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
	physical, leg, _, err := openFundedSource(operation.Context(), plan, selected, budget, authority, holder, 1, operation.RetainCleanup)
	if err != nil {
		return nil, nil, err
	}
	choice, err := selection.NewRendezvous(leg, authority.current, plan.Exclusions)
	if err != nil {
		return nil, nil, err
	}
	source, err = connection.NewInitiatorSource(connection.InitiatorSourceConfig{Operation: operation, Source: physical,
		PrepareSource: func(caller context.Context) (prefix.Config, error) {
			return prepareSelectedPublicationPrefix(caller, plan, leg, budget, authority, holder)
		},
		History: history, Rendezvous: choice, TargetLink: target, Network: leg.Profile.Network, Profile: leg.Profile.Digest,
		WorkSafetyNotAfter: plan.Deadline, WorkSafetyMaximum: plan.Deadline, NoNewRecoveryAfter: plan.Deadline.Add(-5 * time.Second), Exclusions: plan.Exclusions})
	if err != nil {
		return nil, nil, err
	}
	return source, closeAll, nil
}
