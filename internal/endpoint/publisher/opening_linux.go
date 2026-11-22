//go:build linux

package publisher

import (
	"context"
	"errors"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/endpoint/funding"
	executionruntime "github.com/dianabuilds/ardents-network/internal/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/publication/durable"
	"github.com/dianabuilds/ardents-network/internal/publication/instance"
	publicationruntime "github.com/dianabuilds/ardents-network/internal/publication/runtime"
	"github.com/dianabuilds/ardents-network/internal/route/prefix"
	"github.com/dianabuilds/ardents-network/internal/route/selection"
)

func Open(ctx context.Context, plan Plan, routePlan funding.Plan, authority funding.Authority, holder *stock.Owner, operation *executionruntime.Operation, shared ...*funding.Parents) (_ Publication, result error) {
	if ctx == nil || operation == nil || ctx != operation.Context() || operation.CheckPublisher() != nil || !authority.Available() || holder == nil || routePlan.Domain != 4 || !independentRoots(plan.InstanceRoot, plan.Root, routePlan.EntryRoot, routePlan.InteriorRoot, routePlan.SourceInteriorRoot, routePlan.ResponderInteriorRoot, routePlan.HostingRoot) {
		return Publication{}, errors.New("publisher composition unavailable")
	}
	if len(shared) > 1 || len(shared) == 1 && !shared[0].Matches(routePlan) {
		return Publication{}, errors.New("shared Publisher Route owners do not match")
	}
	ownedParents := len(shared) == 0
	var installation *selection.Installation
	var budget *hosting.Budget
	var err error
	if ownedParents {
		installation, err = selection.OpenInstallation(selection.InstallationConfig{EntryRoot: routePlan.EntryRoot, Current: authority.Current, Exclusions: routePlan.Exclusions})
		if err != nil {
			return Publication{}, err
		}
	} else {
		installation, budget = shared[0].Entry(), shared[0].Budget()
	}
	var sourceSelection, introSelection, responderSelection *selection.Owner
	var responderConfig prefix.Config
	var source, intro *prefix.Prefix
	var private *instance.Root
	var history *durable.Root
	var owner *publicationruntime.Publisher
	var once sync.Once
	var terminal error
	closeAll := func() error {
		once.Do(func() {
			var physical error
			if owner != nil {
				terminal = owner.Close()
				cleanup, complete := owner.CleanupResult()
				if !complete {
					cleanup = errors.Join(cleanup, errors.New("original composed Publisher cleanup did not join"))
				}
				physical = cleanup
			} else {
				terminal = errors.Join(intro.Close(), source.Close())
				for _, original := range []*prefix.Prefix{intro, source} {
					if original == nil {
						continue
					}
					cleanup, complete := original.CleanupResult()
					if !complete {
						cleanup = errors.Join(cleanup, errors.New("original partial Publisher Prefix cleanup did not join"))
					}
					physical = errors.Join(physical, cleanup)
				}
				if responderConfig.Release != nil {
					returned := responderConfig.Release()
					terminal = errors.Join(terminal, returned)
					physical = errors.Join(physical, returned)
				}
			}
			owned := errors.Join(private.Close(), history.Close(), responderSelection.Close(), introSelection.Close(), sourceSelection.Close())
			if ownedParents {
				if budget != nil {
					owned = errors.Join(owned, budget.Close())
				}
				owned = errors.Join(owned, installation.Close())
			}
			physical = errors.Join(physical, owned)
			terminal = errors.Join(terminal, physical)
			operation.RetainCleanup(physical)
		})
		return terminal
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, closeAll())
		}
	}()
	if ownedParents {
		budget, err = hosting.Open(routePlan.HostingRoot)
		if err != nil {
			return Publication{}, err
		}
	}
	sourceSelection, err = installation.Borrow(selection.RoleConfig{InteriorRoot: routePlan.SourceInteriorRoot, Domain: 1})
	if err != nil {
		return Publication{}, err
	}
	introSelection, err = installation.Borrow(selection.RoleConfig{InteriorRoot: routePlan.InteriorRoot, Domain: 4})
	if err != nil {
		return Publication{}, err
	}
	responderSelection, err = installation.Borrow(selection.RoleConfig{InteriorRoot: routePlan.ResponderInteriorRoot, Domain: 3})
	if err != nil {
		return Publication{}, err
	}
	var flow *funding.Flow
	var sourceLeg selection.Leg
	// Initial Publication funds Forward for Introduction and Responder, then
	// Registration. The existing second bootstrap batch supplies their issuer
	// Controls and the already selected Descriptor Store Control. Two issuer
	// Controls remain after preparation, so the first Rendezvous Forward batch
	// needs no separate Control refill on the incoming capsule path.
	// The same finite allocation is debited now; failed setup never refunds it.
	source, sourceLeg, flow, err = funding.OpenSource(ctx, routePlan, sourceSelection, budget, authority, holder, 4, true, operation.RetainCleanup)
	if err != nil {
		return Publication{}, err
	}
	var leg selection.Leg
	introConfig, err := funding.PreparePrefix(ctx, routePlan, introSelection, budget, authority, holder)
	if err != nil {
		return Publication{}, err
	}
	introConfig.Present = flow.OpeningPresentation(introConfig.Leg)
	leg = introConfig.Leg
	responderConfig, err = funding.PreparePrefix(ctx, routePlan, responderSelection, budget, authority, holder)
	if err != nil {
		return Publication{}, errors.Join(err, introConfig.Release())
	}
	responderConfig.Present = flow.OpeningPresentation(responderConfig.Leg)
	// Stock preparation grants no physical Responder: its first opening stays
	// after capsule validation and nonce commitment under the live Publisher.
	if err := flow.PrepareInitialOpening(ctx, leg, responderConfig.Leg); err != nil {
		return Publication{}, errors.Join(err, introConfig.Release())
	}
	intro, err = prefix.Open(ctx, introConfig)
	if err != nil {
		return Publication{}, errors.Join(err, introConfig.Release())
	}
	view, err := authority.Current()
	if err != nil {
		return Publication{}, err
	}
	duty, err := leg.IntroductionDuty(view, routePlan.Exclusions)
	if err != nil {
		return Publication{}, err
	}
	private, err = instance.Open(ctx, plan.InstanceRoot)
	if err != nil {
		return Publication{}, err
	}
	credential, err := private.Credential(ctx)
	if err != nil || credential.Delegation().Network != view.ProfileBinding().Network {
		return Publication{}, errors.Join(errors.New("publisher Credential Network mismatch"), err)
	}
	value := credential.Delegation()
	history, err = durable.Open(ctx, durable.Config{Root: plan.Root, Target: value.Target, Network: value.Network})
	if err != nil {
		return Publication{}, err
	}
	binding, err := private.Bind(ctx, history)
	if err != nil {
		return Publication{}, err
	}
	prepareResponder := func(caller context.Context) (prefix.Config, error) {
		// Retained leg is immutable. Selecting again would return fresh
		// observation timestamps and must not replace original role bindings.
		config, err := funding.PrepareSelectedPrefix(caller, routePlan, responderConfig.Leg, budget, authority, holder)
		if err == nil {
			config.Present = flow.OpeningPresentation(config.Leg)
		}
		return config, err
	}
	owner, err = publicationruntime.New(ctx, publicationruntime.Config{Operation: operation, Binding: binding, Source: source, Introduction: intro, Responder: responderConfig, PrepareResponder: prepareResponder,
		PrepareJoin: flow.PrepareJoin,
		PrepareSource: func(caller context.Context) (prefix.Config, error) {
			return funding.PrepareSelectedPrefix(caller, routePlan, sourceLeg, budget, authority, holder)
		},
		Duty: duty, Deadline: routePlan.Deadline, Exclusions: routePlan.Exclusions})
	if err != nil {
		return Publication{}, err
	}
	flow.FollowSource(owner.CurrentSource)
	return Publication{owner: owner, close: closeAll, serve: func(caller context.Context) error {
		contract, err := TextContract(operation)
		if err != nil {
			return err
		}
		for {
			stream, err := owner.NextStream(caller, contract)
			if err != nil {
				if owner.StreamAcquisitionStopped(err) {
					// This loop acquired no work. The owning Withdraw/Close
					// still returns all retained work and physical failures.
					return nil
				}
				return err
			}
			transferred, err := operation.DeliverPublisherStream(caller, contract, stream)
			if !transferred {
				_ = stream.Close()
				operation.RetainCleanup(stream.JoinedCleanup())
			}
			if err != nil {
				return err
			}
		}
	}}, nil
}
