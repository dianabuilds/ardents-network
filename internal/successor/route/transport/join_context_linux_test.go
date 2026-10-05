//go:build linux

package transport

import (
	"context"
	"errors"
	"io"
	"reflect"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
)

func TestJoinContextMissingOwnersCannotAcquireReadiness(t *testing.T) {
	caller, cancel := context.WithCancel(t.Context())
	cancel()
	for _, ctx := range []context.Context{nil, caller, t.Context()} {
		owner, err := NewJoinContext(ctx, JoinContextConfig{Deadline: time.Now().UTC().Truncate(time.Second).Add(time.Minute)})
		if err == nil || owner != nil {
			t.Fatal("missing genuine role owners created a JOIN context")
		}
	}
}

func TestJoinContextPreWireRefusalDoesNotRetainCompletedBorrowers(t *testing.T) {
	c, g := joinContextLifetime(t)
	defer c.Close()
	g.source = joinContextRetiringPrefix(t, g.ctx, nil)
	g.source.config.Leg.EntryMember.RoleDomain = 1
	g.ready = true
	close(g.opened)
	// A zero selection has no authenticated authority. Choice 2 must refuse
	// before observing it, presenting a token or opening any physical channel.
	c.choice = new(selection.Rendezvous)
	retained := c.choice
	presentations := 0
	g.source.config.Present = func(context.Context, ardp.Hello) ([]byte, error) {
		presentations++
		return nil, errors.New("no successful Admission fixture")
	}
	for range 64 {
		if joined, err := c.join(g, t.Context(), JoinConfig{}, 2); joined != nil || err == nil {
			t.Fatal("invalid pre-wire attempt succeeded", err)
		}
		if len(g.joins) != 0 || len(g.source.joins) != 0 {
			t.Fatal("joined refusal retained completed borrowers", len(g.joins), len(g.source.joins))
		}
	}
	if presentations != 0 || c.choice != retained || c.failure != nil {
		t.Fatal("pre-wire refusal changed presentation, selection or terminal authority")
	}
	opening := &JoinOpening{owner: c, generation: g}
	if err := opening.Close(); err != nil {
		t.Fatal(err)
	}
	if len(c.generations) != 0 || len(g.joins) != 0 {
		t.Fatal("fully joined generation remained in owner index")
	}
	if err := opening.Close(); err != nil || c.choice != retained {
		t.Fatal("old handle lost its exact completed result or changed selection", err)
	}
}

// These fixtures model local lifetime and failed cleanup only. They have no
// selection authority, signed Network, admission or successful physical opening.
// Genuine Context Open/JOIN tests belong to command composition with real owners.
func joinContextLifetime(t *testing.T) (*JoinContext, *joinContextOpening) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	c := &JoinContext{caller: ctx, ctx: ctx, cancel: cancel, generations: make(map[*joinContextOpening]struct{}), stopWatch: make(chan struct{}), watchDone: make(chan struct{})}
	close(c.watchDone) // fixture creates no context watcher
	child, stop := context.WithCancel(ctx)
	g := &joinContextOpening{caller: ctx, ctx: child, cancel: stop, stopContext: func() bool { return true }, opened: make(chan struct{}), joins: make(map[*JoinAcquisition]struct{}), readiness: make(chan struct{}), stopWatch: make(chan struct{}), watchDone: make(chan struct{})}
	close(g.watchDone) // fixture creates no physical readiness watcher
	c.opening = g
	c.generations[g] = struct{}{}
	return c, g
}

func joinContextRetiringPrefix(t *testing.T, ctx context.Context, release func() error) *Prefix {
	t.Helper()
	child, cancel := context.WithCancel(ctx)
	p := &Prefix{ctx: child, caller: ctx, cancel: cancel, done: make(chan struct{}), closing: make(chan struct{}), release: release}
	close(p.done) // no reader, admission, or readiness watcher was created
	return p
}

func TestJoinContextCloseWaitsForItsPendingOpening(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c, g := joinContextLifetime(t)
		unrelated, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := c.Open(unrelated); err == nil || g.ctx.Err() != nil {
			t.Fatal("unrelated canceled caller affected the original opening")
		}
		finished := make(chan error, 1)
		go func() { finished <- c.ClosePrefix() }()
		synctest.Wait()
		if g.ctx.Err() == nil {
			t.Fatal("pending physical opening was not interrupted")
		}
		select {
		case <-g.readiness:
		default:
			t.Fatal("pending opening was not synchronously sealed")
		}
		select {
		case <-finished:
			t.Fatal("ClosePrefix reported completion before original opening joined")
		default:
		}
		if _, err := c.Open(t.Context()); err == nil {
			t.Fatal("replacement opened while original physical result was pending")
		}
		cause := errors.New("late original reservation return failed")
		returns := 0
		late := joinContextRetiringPrefix(t, g.ctx, func() error { returns++; return cause })
		c.mu.Lock()
		g.source = late
		close(g.opened)
		c.mu.Unlock()
		first := <-finished
		if !errors.Is(first, cause) || returns != 1 {
			t.Fatal("late original result did not join and preserve return failure", first, returns)
		}
		if again := c.ClosePrefix(); again != first || returns != 1 {
			t.Fatal("repeated prefix close replaced the retained terminal result")
		}
		if _, err := c.Open(t.Context()); !errors.Is(err, cause) {
			t.Fatal("failed cleanup permitted replacement", err)
		}
		if err := c.Close(); !errors.Is(err, cause) {
			t.Fatal("context close lost original cleanup failure", err)
		}
	})
}

func TestJoinContextManualClosePreservesParentsUntilBorrowersJoin(t *testing.T) {
	c, g := joinContextLifetime(t)
	var order []string
	release := func(role string) func() error {
		return func() error {
			if err := g.ctx.Err(); err != nil {
				t.Error("manual retirement canceled the shared physical parent before terminal join", err)
			}
			if !c.mu.TryLock() {
				t.Error("reservation return ran under Context lock")
			} else {
				c.mu.Unlock()
			}
			order = append(order, role)
			return nil
		}
	}
	g.source = joinContextRetiringPrefix(t, g.ctx, release("Source"))
	g.responder = joinContextRetiringPrefix(t, g.ctx, release("Responder"))
	g.responder.source = g.source
	g.ready = true // local retirement state only; no authority is fabricated
	close(g.opened)
	if err := c.ClosePrefix(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(order, []string{"Responder", "Source"}) || g.ctx.Err() == nil {
		t.Fatal("original dependencies were not joined before parent cancellation", order)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestJoinContextCompletedFailureSealsAndCannotBeErased(t *testing.T) {
	c, g := joinContextLifetime(t)
	a := &JoinAcquisition{generation: g}
	g.joins[a] = struct{}{}
	cause := errors.New("joined physical return failed")
	c.completed(a, cause)
	if g.ctx.Err() == nil {
		t.Fatal("physical cleanup failure did not seal pending generation")
	}
	c.mu.Lock()
	retained := c.failure
	c.mu.Unlock()
	c.completed(a, nil)
	c.completed(a, errors.New("replacement result"))
	c.mu.Lock()
	unchanged := c.failure == retained
	c.mu.Unlock()
	if !unchanged {
		t.Fatal("repeated acquisition completion replaced original terminal result")
	}
	if _, err := c.Open(t.Context()); !errors.Is(err, cause) {
		t.Fatal("joined cleanup failure permitted a new opening", err)
	}
	close(g.opened)
	if err := c.Close(); !errors.Is(err, cause) {
		t.Fatal("context close erased a completed acquisition failure", err)
	}
	if err := c.Close(); err != retained {
		t.Fatal("repeated context close replaced retained failure", err)
	}
}

func TestJoinContextOpeningFailureUsesRetirementProvenance(t *testing.T) {
	c, g := joinContextLifetime(t)
	operation := errors.New("opening refused before physical transfer")
	c.failedOpening(errors.Join(operation, context.Canceled, io.EOF))
	c.mu.Lock()
	untouched := c.failure == nil && !g.closing
	c.mu.Unlock()
	if !untouched || g.ctx.Err() != nil {
		t.Fatal("operation error class fabricated a physical cleanup failure")
	}
	cause := errors.New("original physical return failed")
	c.failedOpening(errors.Join(context.Canceled, &prefixOpeningFailure{operation: operation, retirement: errors.Join(io.ErrUnexpectedEOF, cause)}))
	c.mu.Lock()
	retained := c.failure
	c.mu.Unlock()
	if !errors.Is(retained, cause) || !errors.Is(retained, io.ErrUnexpectedEOF) || errors.Is(retained, operation) || g.ctx.Err() == nil {
		t.Fatal("explicit retirement provenance was discarded or replaced", retained)
	}
	close(g.opened)
	if err := c.Close(); !errors.Is(err, cause) || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatal("final context close lost nested physical failure", err)
	}
}

type joinContextDeferredCaller struct {
	context.Context
	canceled atomic.Bool
}

func (c *joinContextDeferredCaller) Err() error {
	if c.canceled.Load() {
		return context.Canceled
	}
	return nil
}

func TestJoinContextObservesOriginalCallersWithoutPropagation(t *testing.T) {
	for _, original := range []string{"context", "opening"} {
		t.Run(original, func(t *testing.T) {
			c, g := joinContextLifetime(t)
			caller := &joinContextDeferredCaller{Context: context.Background()}
			if original == "context" {
				c.caller = caller
			} else {
				g.caller = caller
			}
			observed := 0
			c.config.Current = func() (network.RuntimeView, error) {
				observed++
				return network.RuntimeView{}, errors.New("no authenticated Network fixture")
			}
			caller.canceled.Store(true)
			if err := c.openingAllowed(g); !errors.Is(err, context.Canceled) {
				t.Fatal("original caller was hidden by delayed derived cancellation", err)
			}
			if _, err := c.openingCurrent(g); !errors.Is(err, context.Canceled) || observed != 0 {
				t.Fatal("canceled original caller reached Network observation", err, observed)
			}
			close(g.opened)
			if err := c.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
