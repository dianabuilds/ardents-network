//go:build linux

package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/publication/durable"
	"github.com/dianabuilds/ardents-network/internal/successor/publication/instance"
	publicationruntime "github.com/dianabuilds/ardents-network/internal/successor/publication/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/prefix"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
)

func startPublicationHolder(ctx context.Context, plan publicationPlan, routePlan routePrefixPlan, authority admissionAuthority, holder *stock.Owner, operation *executionruntime.Operation, shared ...*installedRouteOwners) (_ publicationHandle, result error) {
	if ctx == nil || operation == nil || ctx != operation.Context() || operation.CheckPublisher() != nil || authority.current == nil || holder == nil || routePlan.Domain != 4 || !independentRouteRoots(plan.InstanceRoot, plan.Root, routePlan.EntryRoot, routePlan.InteriorRoot, routePlan.SourceInteriorRoot, routePlan.ResponderInteriorRoot, routePlan.HostingRoot) {
		return publicationHandle{}, errors.New("publisher composition unavailable")
	}
	if len(shared) > 1 || len(shared) == 1 && !shared[0].matches(routePlan) {
		return publicationHandle{}, errors.New("shared Publisher Route owners do not match")
	}
	ownedParents := len(shared) == 0
	var installation *selection.Installation
	var budget *hosting.Budget
	var err error
	if ownedParents {
		installation, err = selection.OpenInstallation(selection.InstallationConfig{EntryRoot: routePlan.EntryRoot, Current: authority.current, Exclusions: routePlan.Exclusions})
		if err != nil {
			return publicationHandle{}, err
		}
	} else {
		installation, budget = shared[0].entry, shared[0].budget
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
			return publicationHandle{}, err
		}
	}
	sourceSelection, err = installation.Borrow(selection.RoleConfig{InteriorRoot: routePlan.SourceInteriorRoot, Domain: 1})
	if err != nil {
		return publicationHandle{}, err
	}
	introSelection, err = installation.Borrow(selection.RoleConfig{InteriorRoot: routePlan.InteriorRoot, Domain: 4})
	if err != nil {
		return publicationHandle{}, err
	}
	responderSelection, err = installation.Borrow(selection.RoleConfig{InteriorRoot: routePlan.ResponderInteriorRoot, Domain: 3})
	if err != nil {
		return publicationHandle{}, err
	}
	var funding *routeTokenFlow
	var sourceLeg selection.Leg
	// Initial Publication funds Forward for Introduction and Responder, then
	// Registration. The existing second bootstrap batch supplies their issuer
	// Controls and the already selected Descriptor Store Control. Two issuer
	// Controls remain after preparation, so the first Rendezvous Forward batch
	// needs no separate Control refill on the incoming capsule path.
	// The same finite allocation is debited now; failed setup never refunds it.
	source, sourceLeg, funding, err = openFundedSource(ctx, routePlan, sourceSelection, budget, authority, holder, 4, true, operation.RetainCleanup)
	if err != nil {
		return publicationHandle{}, err
	}
	var leg selection.Leg
	introConfig, err := preparePublicationPrefix(ctx, routePlan, introSelection, budget, authority, holder)
	if err != nil {
		return publicationHandle{}, err
	}
	introConfig.Present = funding.openingPresentation(introConfig.Leg)
	leg = introConfig.Leg
	responderConfig, err = preparePublicationPrefix(ctx, routePlan, responderSelection, budget, authority, holder)
	if err != nil {
		return publicationHandle{}, errors.Join(err, introConfig.Release())
	}
	responderConfig.Present = funding.openingPresentation(responderConfig.Leg)
	// Stock preparation grants no physical Responder: its first opening stays
	// after capsule validation and nonce commitment under the live Publisher.
	if err := funding.prepareInitialOpening(ctx, leg, responderConfig.Leg); err != nil {
		return publicationHandle{}, errors.Join(err, introConfig.Release())
	}
	intro, err = prefix.Open(ctx, introConfig)
	if err != nil {
		return publicationHandle{}, errors.Join(err, introConfig.Release())
	}
	view, err := authority.current()
	if err != nil {
		return publicationHandle{}, err
	}
	duty, err := leg.IntroductionDuty(view, routePlan.Exclusions)
	if err != nil {
		return publicationHandle{}, err
	}
	private, err = instance.Open(ctx, plan.InstanceRoot)
	if err != nil {
		return publicationHandle{}, err
	}
	credential, err := private.Credential(ctx)
	if err != nil || credential.Delegation().Network != view.Profile().Network {
		return publicationHandle{}, errors.Join(errors.New("publisher Credential Network mismatch"), err)
	}
	value := credential.Delegation()
	history, err = durable.Open(ctx, durable.Config{Root: plan.Root, Target: value.Target, Network: value.Network})
	if err != nil {
		return publicationHandle{}, err
	}
	binding, err := private.Bind(ctx, history)
	if err != nil {
		return publicationHandle{}, err
	}
	prepareResponder := func(caller context.Context) (prefix.Config, error) {
		// Retained leg is immutable. Selecting again would return fresh
		// observation timestamps and must not replace original role bindings.
		config, err := prepareSelectedPublicationPrefix(caller, routePlan, responderConfig.Leg, budget, authority, holder)
		if err == nil {
			config.Present = funding.openingPresentation(config.Leg)
		}
		return config, err
	}
	owner, err = publicationruntime.New(ctx, publicationruntime.Config{Operation: operation, Binding: binding, Source: source, Introduction: intro, Responder: responderConfig, PrepareResponder: prepareResponder,
		PrepareJoin: funding.prepareJoin,
		PrepareSource: func(caller context.Context) (prefix.Config, error) {
			return prepareSelectedPublicationPrefix(caller, routePlan, sourceLeg, budget, authority, holder)
		},
		Duty: duty, Deadline: routePlan.Deadline, Exclusions: routePlan.Exclusions})
	if err != nil {
		return publicationHandle{}, err
	}
	funding.mu.Lock()
	funding.currentSource = owner.CurrentSource
	funding.mu.Unlock()
	return publicationHandle{owner: owner, close: closeAll, serve: func(caller context.Context) error {
		contract, err := publisherTextContract(operation)
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

// Preparation retains a genuine independent role selection and finite shared
// Hosting reservation. It performs no dial and grants no ready physical prefix.
func preparePublicationPrefix(ctx context.Context, plan routePrefixPlan, selected *selection.Owner, budget *hosting.Budget, authority admissionAuthority, holder *stock.Owner) (prefix.Config, error) {
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
	return prepareSelectedPublicationPrefix(ctx, plan, leg, budget, authority, holder)
}

// A new physical reservation for an already retained leg grants no selection
// or accepting observation. Prefix reobserves the original genuine authority
// before every opening/effect and at its final handoff.
func prepareSelectedPublicationPrefix(ctx context.Context, plan routePrefixPlan, leg selection.Leg, budget *hosting.Budget, authority admissionAuthority, holder *stock.Owner) (prefix.Config, error) {
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
