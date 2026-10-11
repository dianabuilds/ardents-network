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

func startPublicationHolder(ctx context.Context, plan publicationPlan, routePlan routePrefixPlan, authority admissionAuthority, holder *stock.Owner, operation *executionruntime.Operation) (_ publicationHandle, result error) {
	if ctx == nil || operation == nil || operation.CheckPublisher() != nil || authority.current == nil || holder == nil || routePlan.Domain != 4 || !independentRouteRoots(plan.InstanceRoot, plan.Root, routePlan.EntryRoot, routePlan.InteriorRoot, routePlan.SourceInteriorRoot, routePlan.ResponderInteriorRoot, routePlan.HostingRoot) {
		return publicationHandle{}, errors.New("publisher composition unavailable")
	}
	installation, err := selection.OpenInstallation(selection.InstallationConfig{EntryRoot: routePlan.EntryRoot, Current: authority.current, Exclusions: routePlan.Exclusions})
	if err != nil {
		return publicationHandle{}, err
	}
	var sourceSelection, introSelection, responderSelection *selection.Owner
	var responderConfig prefix.Config
	var budget *hosting.Budget
	var source, intro *prefix.Prefix
	var private *instance.Root
	var history *durable.Root
	var owner *publicationruntime.Publisher
	var once sync.Once
	var terminal error
	closeAll := func() error {
		once.Do(func() {
			if owner != nil {
				terminal = owner.Close()
			} else {
				terminal = errors.Join(intro.Close(), source.Close())
				if responderConfig.Release != nil {
					terminal = errors.Join(terminal, responderConfig.Release())
				}
			}
			terminal = errors.Join(terminal, private.Close(), history.Close(), responderSelection.Close(), introSelection.Close(), sourceSelection.Close())
			if budget != nil {
				terminal = errors.Join(terminal, budget.Close())
			}
			terminal = errors.Join(terminal, installation.Close())
			operation.RetainCleanup(terminal)
		})
		return terminal
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, closeAll())
		}
	}()
	budget, err = hosting.Open(routePlan.HostingRoot)
	if err != nil {
		return publicationHandle{}, err
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
	source, _, err = openPublicationPrefix(ctx, routePlan, sourceSelection, budget, authority, holder)
	if err != nil {
		return publicationHandle{}, err
	}
	var leg selection.Leg
	intro, leg, err = openPublicationPrefix(ctx, routePlan, introSelection, budget, authority, holder)
	if err != nil {
		return publicationHandle{}, err
	}
	responderConfig, err = preparePublicationPrefix(ctx, routePlan, responderSelection, budget, authority, holder)
	if err != nil {
		return publicationHandle{}, err
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
	owner, err = publicationruntime.New(ctx, publicationruntime.Config{Operation: operation, Binding: binding, Source: source, Introduction: intro, Responder: responderConfig, Duty: duty, Deadline: routePlan.Deadline, Exclusions: routePlan.Exclusions})
	if err != nil {
		return publicationHandle{}, err
	}
	return publicationHandle{owner: owner, close: closeAll}, nil
}

// Selection and shared Hosting histories remain composition-owned. Each real
// prefix retains only its own finite reservation until its physical join.
func openPublicationPrefix(ctx context.Context, plan routePrefixPlan, selected *selection.Owner, budget *hosting.Budget, authority admissionAuthority, holder *stock.Owner) (*prefix.Prefix, selection.Leg, error) {
	config, err := preparePublicationPrefix(ctx, plan, selected, budget, authority, holder)
	if err != nil {
		return nil, selection.Leg{}, err
	}
	physical, err := prefix.Open(ctx, config)
	if err != nil {
		return nil, selection.Leg{}, errors.Join(err, config.Release())
	}
	return physical, config.Leg, nil
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
	return prefix.Config{Leg: leg, Current: authority.current, Deadline: plan.Deadline, Release: release,
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
