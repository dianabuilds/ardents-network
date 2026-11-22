//go:build linux

package node

import (
	"context"
	"errors"
	"testing"
)

// These oracles encode the genuine per-role divergence the shared nodeLifetime
// skeleton must preserve byte-for-byte. They exercise the denial latch, the
// failed-observation notice policy and the role-owner cleanup hook with nil role
// owners (each role owner is nil-safe), so they prove the parameterization did not
// homogenize the delicate cause-vs-retired, owner-attribution and release-filtering
// semantics. Positive admission, the causal refusal matrix, physical cleanup
// ordering, cancellation and late-result join are owned by the retained per-role
// startup tests and the native lifecycle gates.

type lifetimeEquivalenceRole struct {
	name      string
	bareCause bool
	build     func(caller, ctx context.Context, cancel context.CancelFunc, retired error) (nodeRole, *nodeLifetime)
}

func lifetimeEquivalenceRoles() []lifetimeEquivalenceRole {
	return []lifetimeEquivalenceRole{
		{"Forwarding", true, func(caller, ctx context.Context, cancel context.CancelFunc, retired error) (nodeRole, *nodeLifetime) {
			f := &forwarding{nodeLifetime: nodeLifetime{caller: caller, ctx: ctx, cancel: cancel, retired: retired}}
			f.hooks = f
			return f, &f.nodeLifetime
		}},
		{"Issuer", true, func(caller, ctx context.Context, cancel context.CancelFunc, retired error) (nodeRole, *nodeLifetime) {
			f := &issuerInvocation{nodeLifetime: nodeLifetime{caller: caller, ctx: ctx, cancel: cancel, retired: retired}}
			f.hooks = f
			return f, &f.nodeLifetime
		}},
		{"Introduction", false, func(caller, ctx context.Context, cancel context.CancelFunc, retired error) (nodeRole, *nodeLifetime) {
			f := &introductionInvocation{nodeLifetime: nodeLifetime{caller: caller, ctx: ctx, cancel: cancel, retired: retired}}
			f.hooks = f
			return f, &f.nodeLifetime
		}},
		{"Descriptor", false, func(caller, ctx context.Context, cancel context.CancelFunc, retired error) (nodeRole, *nodeLifetime) {
			f := &descriptorInvocation{nodeLifetime: nodeLifetime{caller: caller, ctx: ctx, cancel: cancel, retired: retired}}
			f.hooks = f
			return f, &f.nodeLifetime
		}},
		{"Join", false, func(caller, ctx context.Context, cancel context.CancelFunc, retired error) (nodeRole, *nodeLifetime) {
			f := &joinInvocation{nodeLifetime: nodeLifetime{caller: caller, ctx: ctx, cancel: cancel, retired: retired}}
			f.hooks = f
			return f, &f.nodeLifetime
		}},
	}
}

// A foreign operational cause is returned bare by Forwarding and Issuer but lent
// the retirement notice by Introduction, Descriptor and JOIN. Every role retains
// the original cause with its true owner; none replaces it with the notice.
func TestNodeLifetimeFailedObservationNoticePolicy(t *testing.T) {
	for _, role := range lifetimeEquivalenceRoles() {
		t.Run(role.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			retired := errors.New("original " + role.name + " admission retired")
			r, core := role.build(ctx, ctx, cancel, retired)
			foreign := errors.New("foreign operational cause")
			got := r.failedObservation(foreign, "process")
			if role.bareCause {
				if got != foreign {
					t.Fatal("bare-cause role replaced the original foreign cause", got)
				}
			} else if got != retired {
				t.Fatal("retired-notice role leaked the foreign operational cause", got)
			}
			if core.first != foreign || core.firstOwner != "process" {
				t.Fatal("original cause/owner not retained", core.first, core.firstOwner)
			}
			if !core.denied || ctx.Err() == nil {
				t.Fatal("failed observation did not deny and cancel")
			}
		})
	}
}

// A bare caller/cancellation error racing the final observation is lent the
// retirement notice by every role, and the original caller cause is retained.
func TestNodeLifetimeFailedObservationCancellationLendsNotice(t *testing.T) {
	for _, role := range lifetimeEquivalenceRoles() {
		t.Run(role.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			retired := errors.New("original " + role.name + " admission retired")
			r, core := role.build(ctx, ctx, cancel, retired)
			got := r.failedObservation(ctx.Err(), "process")
			if got != retired {
				t.Fatal("bare caller cancellation was not lent the retirement notice", got)
			}
			if core.first != context.Canceled || core.firstOwner != "caller" {
				t.Fatal("original caller cancellation not retained", core.first, core.firstOwner)
			}
		})
	}
}

// The shared retired-observation policy (Introduction/Descriptor/JOIN) never
// returns a foreign cause to dependents and never re-retains the notice itself.
func TestNodeLifetimeRetiredObservationPolicy(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	retired := errors.New("original retirement notice")
	f := &joinInvocation{nodeLifetime: nodeLifetime{caller: ctx, ctx: ctx, cancel: cancel, retired: retired}}
	f.hooks = f
	foreign := errors.New("foreign cause")
	if got := f.retiredObservation(foreign, "reachability"); got != retired || f.first != foreign || f.firstOwner != "reachability" {
		t.Fatal("retired observation leaked foreign cause or lost owner", got, f.first, f.firstOwner)
	}
	// The exact retirement notice is never retained as a first cause. A fresh
	// uncanceled caller is used so the prior denial's cancellation does not leak.
	fresh, freshCancel := context.WithCancel(context.Background())
	defer freshCancel()
	g := &joinInvocation{nodeLifetime: nodeLifetime{caller: fresh, ctx: fresh, cancel: freshCancel, retired: retired}}
	g.hooks = g
	if got := g.retiredObservation(retired, "reachability"); got != retired || g.first != nil {
		t.Fatal("retirement notice became a retained first cause", got, g.first)
	}
}

// Forwarding's denial latch keeps the runtime owner default, the cancellation
// owner override and the retired-notice substitution; the shared latch used by the
// other roles keeps the supplied owner verbatim and skips the retired notice.
func TestNodeLifetimeDenialOwnerAttributionDivergence(t *testing.T) {
	newCore := func() (*nodeLifetime, context.CancelFunc) {
		ctx, cancel := context.WithCancel(context.Background())
		f := &joinInvocation{nodeLifetime: nodeLifetime{caller: ctx, ctx: ctx, cancel: cancel, retired: errors.New("original retirement notice")}}
		f.hooks = f
		return &f.nodeLifetime, cancel
	}
	t.Run("shared latch keeps supplied owner", func(t *testing.T) {
		core, cancel := newCore()
		defer cancel()
		foreign := errors.New("foreign cause")
		core.deny(foreign, "startup")
		if core.first != foreign || core.firstOwner != "startup" {
			t.Fatal("shared latch did not keep the supplied owner", core.first, core.firstOwner)
		}
	})
	t.Run("shared latch keeps cancellation owner verbatim", func(t *testing.T) {
		core, cancel := newCore()
		defer cancel()
		core.deny(context.Canceled, "network")
		if core.first != context.Canceled || core.firstOwner != "network" {
			t.Fatal("shared latch overrode the supplied cancellation owner", core.firstOwner)
		}
	})
	t.Run("shared latch skips the retired notice", func(t *testing.T) {
		core, cancel := newCore()
		defer cancel()
		core.deny(core.retired, "startup")
		if core.first != nil || !core.denied {
			t.Fatal("shared latch retained the retired notice as a first cause", core.first)
		}
	})
	t.Run("forwarding latch defaults runtime and overrides cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		f := &forwarding{nodeLifetime: nodeLifetime{caller: ctx, ctx: ctx, cancel: cancel, retired: errors.New("original retirement notice")}}
		f.hooks = f
		foreign := errors.New("foreign cause")
		f.deny(foreign)
		if f.first != foreign || f.firstOwner != "runtime" {
			t.Fatal("forwarding latch lost the runtime owner default", f.first, f.firstOwner)
		}
		g := &forwarding{nodeLifetime: nodeLifetime{caller: ctx, ctx: ctx, cancel: cancel, retired: errors.New("original retirement notice")}}
		g.hooks = g
		g.deny(context.Canceled, "network")
		if g.first != context.Canceled || g.firstOwner != "caller" {
			t.Fatal("forwarding latch did not override the cancellation owner", g.first, g.firstOwner)
		}
	})
	t.Run("forwarding latch substitutes the retired notice with the caller error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		f := &forwarding{nodeLifetime: nodeLifetime{caller: ctx, ctx: ctx, cancel: cancel, retired: errors.New("original retirement notice")}}
		f.hooks = f
		f.deny(f.retired, "network")
		if f.first != context.Canceled || f.firstOwner != "caller" {
			t.Fatal("forwarding latch did not substitute the retired notice", f.first, f.firstOwner)
		}
	})
}

// With no acquired role owner every role's cleanup hook passes the Route result
// through unchanged and retains no role cause. Introduction filters the joined
// release by the Registry failure while Descriptor retains the raw Store release;
// both collapse to pass-through when the owner is absent. Issuing's zero owner is a
// safe no-op. This proves the shared cleanup ordering delegates role attribution
// without inventing a role failure.
func TestNodeLifetimeCleanupRolePassThroughWithoutOwner(t *testing.T) {
	routeResult := errors.New("independent physical route result")
	for _, role := range lifetimeEquivalenceRoles() {
		t.Run(role.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			r, _ := role.build(ctx, ctx, cancel, errors.New("original retirement notice"))
			got, afterRoute := r.cleanupRole(routeResult)
			if got != routeResult {
				t.Fatal("cleanup hook altered the Route result without a role failure", got)
			}
			var retained []string
			if afterRoute != nil {
				afterRoute(func(owner string, cause error) {
					if cause != nil {
						retained = append(retained, owner)
					}
				})
			}
			if len(retained) != 0 {
				t.Fatal("cleanup hook retained a role cause without an acquired owner", retained)
			}
		})
	}
}
