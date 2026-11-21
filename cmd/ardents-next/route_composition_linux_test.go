//go:build linux

package main

import (
	"context"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/endpoint/session"
	"github.com/dianabuilds/ardents-network/internal/reachability"
	"github.com/dianabuilds/ardents-network/internal/route/introduction"
	"net"
	"time"
)

type routeHandle struct {
	bootstrap         bool
	issue             func(context.Context, uint8, [][32]byte) error
	close             func() error
	replenish         func(context.Context) error
	done              <-chan struct{}
	register          func(context.Context, uint64) (routeRegistration, error)
	recipient         func(uint8) (routeRecipient, error)
	join              func(context.Context, routeJoinIntent) (net.Conn, error)
	publishDescriptor func(context.Context, []byte) error
	lookupDescriptor  func(context.Context, [32]byte, *reachability.History) ([]byte, error)
	submitCapsule     func(context.Context, routeRecipient, []byte) error
	sealSubmitCapsule func(context.Context, routeRecipient, routeCapsuleIntent) error
}

type routeJoinContext struct {
	open  func(context.Context) (routeHandle, error)
	close func() error
}

type routeRegistration struct {
	original *introduction.HolderRegistration
	close    func() error
	withdraw func(context.Context) error
	done     <-chan struct{}
	slot     [32]byte
	facts    introduction.RegistrationFacts
}

func startRoutePrefix(ctx context.Context, p routePrefixPlan, a admissionAuthority, h *stock.Owner) (routeHandle, error) {
	x, err := session.OpenPrefix(ctx, fundingPlan(p), fundingAuthority(a), h)
	if err != nil {
		return routeHandle{}, err
	}
	return testRouteHandle(x), nil
}
func startRouteBootstrap(ctx context.Context, p routePrefixPlan, a admissionAuthority, h *stock.Owner) (routeHandle, error) {
	x, err := session.OpenBootstrap(ctx, fundingPlan(p), fundingAuthority(a), h)
	if err != nil {
		return routeHandle{}, err
	}
	return testRouteHandle(x), nil
}
func newRouteJoinContext(ctx context.Context, p routePrefixPlan, a admissionAuthority, h *stock.Owner) (routeJoinContext, error) {
	x, err := session.OpenJoinContext(ctx, fundingPlan(p), fundingAuthority(a), h)
	if err != nil {
		return routeJoinContext{}, err
	}
	return routeJoinContext{close: x.Close, open: func(ctx context.Context) (routeHandle, error) {
		p, err := x.Open(ctx)
		if err != nil {
			return routeHandle{}, err
		}
		return testRouteHandle(p), nil
	}}, nil
}
func testRouteHandle(p session.Prefix) routeHandle {
	return routeHandle{bootstrap: p.Bootstrap(), close: p.Close, done: p.Done(), issue: p.Issue, replenish: p.Replenish,
		publishDescriptor: p.PublishDescriptor, lookupDescriptor: p.LookupDescriptor,
		register: func(ctx context.Context, revision uint64) (routeRegistration, error) {
			r, err := p.Register(ctx, revision)
			if err != nil {
				return routeRegistration{}, err
			}
			original := r.Owner().(*introduction.HolderRegistration)
			return routeRegistration{original: original, close: original.Close, withdraw: original.Withdraw, done: original.Done(), slot: original.Slot(), facts: r.Facts()}, nil
		},
		recipient: func(c uint8) (routeRecipient, error) { r, err := p.Recipient(c); return routeRecipient(r), err },
		join: func(ctx context.Context, i routeJoinIntent) (net.Conn, error) {
			return p.Join(ctx, session.JoinIntent(i))
		},
		submitCapsule: func(ctx context.Context, r routeRecipient, raw []byte) error {
			return p.SubmitCapsule(ctx, session.Recipient(r), raw)
		},
		sealSubmitCapsule: func(ctx context.Context, r routeRecipient, i routeCapsuleIntent) error {
			return p.SealSubmitCapsule(ctx, session.Recipient(r), session.CapsuleIntent(i))
		},
	}
}

func minRouteDeadline(end time.Time, bounds ...time.Time) time.Time {
	for _, bound := range bounds {
		if bound.Before(end) {
			end = bound
		}
	}
	return end
}
