//go:build linux

package main

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

// One holder console retains one fixed local Route context independently of
// its physical generations. Route owns its choice, acquisitions and join;
// command composition owns installation selection and the shared Hosting root.
func newRouteJoinContext(ctx context.Context, plan routePrefixPlan, authority admissionAuthority, holder *stock.Owner) (_ routeJoinContext, result error) {
	roots := []string{plan.EntryRoot, plan.InteriorRoot, plan.HostingRoot}
	if plan.Domain == 3 {
		roots = append(roots, plan.SourceInteriorRoot)
	}
	if ctx == nil || authority.current == nil || authority.presentation == nil || holder == nil || plan.Domain != 1 && plan.Domain != 3 || !independentRouteRoots(roots...) {
		return routeJoinContext{}, errors.New("route JOIN context setup unavailable")
	}
	installation, err := selection.OpenInstallation(selection.InstallationConfig{EntryRoot: plan.EntryRoot, Current: authority.current, Exclusions: plan.Exclusions})
	if err != nil {
		return routeJoinContext{}, err
	}
	budget, err := hosting.Open(plan.HostingRoot)
	if err != nil {
		return routeJoinContext{}, errors.Join(err, installation.Close())
	}
	var source, responder *selection.Owner
	var owner *transport.JoinContext
	var once sync.Once
	var terminal error
	closeAll := func() error {
		once.Do(func() {
			if owner != nil {
				terminal = owner.Close()
			} else {
				terminal = errors.Join(responder.Close(), source.Close())
			}
			terminal = errors.Join(terminal, installation.Close(), budget.Close())
		})
		return terminal
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, closeAll())
		}
	}()
	interior := plan.InteriorRoot
	if plan.Domain == 3 {
		interior = plan.SourceInteriorRoot
	}
	source, err = installation.Borrow(selection.RoleConfig{InteriorRoot: interior, Domain: 1})
	if err != nil {
		return routeJoinContext{}, err
	}
	if plan.Domain == 3 {
		responder, err = installation.Borrow(selection.RoleConfig{InteriorRoot: plan.InteriorRoot, Domain: 3})
		if err != nil {
			return routeJoinContext{}, err
		}
	}
	deadline := plan.Deadline
	if end, ok := ctx.Deadline(); ok && end.Before(deadline) {
		deadline = end.UTC().Truncate(time.Second)
	}
	owner, err = transport.NewJoinContext(ctx, transport.JoinContextConfig{
		Source: source, Responder: responder, Current: authority.current, Deadline: deadline, Exclusions: plan.Exclusions,
		Reserve: func(caller context.Context) (func() error, error) {
			reservation, err := budget.Reserve(caller, hosting.ReservationRequest{Work: plan.Work, Termination: plan.Termination, WorkUntil: deadline, HoldUntil: deadline.Add(5 * time.Second)})
			if err != nil {
				return nil, err
			}
			return func() error { return releaseRouteReservation(reservation) }, nil
		},
		Present: func(caller context.Context, hello ardp.Hello) ([]byte, error) {
			presentation := stock.Presentation{NetworkID: hello.NetworkID, StateGeneration: hello.StateGeneration, StateDigest: hello.StateDigest, ProfileDigest: hello.ProfileDigest, RecipientNodeID: hello.RecipientNodeID, RecipientDutyGeneration: hello.RecipientDutyGeneration, ChannelNonce: hello.ChannelNonce, Deadline: hello.Deadline}
			if err := authority.presentation(presentation); err != nil {
				return nil, err
			}
			return holder.Take(caller, presentation, 2)
		},
	})
	if err != nil {
		return routeJoinContext{}, err
	}
	return routeJoinContext{close: closeAll, open: func(caller context.Context) (routeHandle, error) {
		opening, err := owner.Open(caller)
		if err != nil {
			return routeHandle{}, err
		}
		return routeHandle{close: opening.Close, done: opening.Done(), recipient: func(slot uint8) (routeRecipient, error) {
			duty, until, err := opening.Recipient(slot)
			if err != nil {
				return routeRecipient{}, err
			}
			return routeRecipient{Node: duty.NodeID, Generation: duty.RecordGeneration, NotAfter: until}, nil
		}, join: func(caller context.Context, intent routeJoinIntent) (net.Conn, error) {
			view, err := authority.current()
			if err != nil {
				return nil, err
			}
			duty, err := view.RetainDuty(intent.Node, view.ObservedAt())
			if err != nil || duty.RecordGeneration != intent.Generation {
				return nil, errors.Join(errors.New("JOIN recipient unavailable"), err)
			}
			// Route verifies the exact current duty against the original
			// roles and retained choice before stock presentation or I/O.
			stream, err := opening.Join(caller, transport.JoinConfig{Duty: duty, Secret: intent.Secret, Context: intent.Context, Deadline: intent.Deadline, SetupDeadline: intent.SetupDeadline}, intent.Choice)
			if err != nil {
				return nil, err
			}
			return stream, nil
		}}, nil
	}}, nil
}
