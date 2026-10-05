//go:build linux

package join

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
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
	source := joinContextRetiringPrefix(t, g.ctx, nil)
	g.source = source
	g.acquire = func(ctx context.Context) (*JoinAcquisition, error) {
		return startAcquisition(ctx, func(func(), func() error) (joinClaim, error) {
			claim := &acquisitionLifetimeClaim{}
			source.claims = append(source.claims, claim)
			return claim, nil
		})
	}
	g.ready = true
	close(g.opened)
	// This selection has no authenticated authority. Refusal must happen before
	// any presentation/physical opening; the local seam only lends lifetime.
	c.choice = new(selection.Rendezvous)
	retained := c.choice
	for range 64 {
		if joined, err := c.join(g, t.Context(), JoinConfig{}, 2); joined != nil || err == nil {
			t.Fatal("invalid pre-wire attempt succeeded", err)
		}
		if len(g.joins) != 0 {
			t.Fatal("joined refusal retained completed borrowers", len(g.joins))
		}
		for _, claim := range source.claims {
			if claim.returns.Load() != 1 {
				t.Fatal("completed refusal retained or returned original claim twice")
			}
		}
	}
	if source.observations != 0 || c.choice != retained || c.failure != nil {
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

// Only local retirement is represented here. All authority/recipient operations
// refuse and no physical opening or successful presentation is provided.
type contextRetirement struct {
	ctx           context.Context
	cancel        context.CancelFunc
	done, closing chan struct{}
	release       func() error
	once, seal    sync.Once
	result        error
	claims        []*acquisitionLifetimeClaim
	observations  int
}

func joinContextRetiringPrefix(t *testing.T, ctx context.Context, release func() error) *contextRetirement {
	t.Helper()
	child, cancel := context.WithCancel(ctx)
	p := &contextRetirement{ctx: child, cancel: cancel, done: make(chan struct{}), closing: make(chan struct{}), release: release}
	close(p.done)
	return p
}
func (p *contextRetirement) Current() error {
	p.observations++
	return errors.New("no authenticated Prefix fixture")
}
func (p *contextRetirement) Rendezvous(_ *selection.Rendezvous, choice uint8, _ []route.Member) (network.RetainedDuty, time.Time, error) {
	if choice > 1 {
		return network.RetainedDuty{}, time.Time{}, errors.New("invalid choice before authority observation")
	}
	return network.RetainedDuty{}, time.Time{}, p.Current()
}
func (p *contextRetirement) CheckRendezvous(network.RetainedDuty, []route.Member) error {
	return p.Current()
}
func (p *contextRetirement) Done() <-chan struct{} { return p.done }
func (p *contextRetirement) Seal()                 { p.seal.Do(func() { close(p.closing) }) }
func (p *contextRetirement) Close() error {
	p.once.Do(func() {
		p.Seal()
		p.cancel()
		if p.release != nil {
			p.result = p.release()
		}
	})
	return p.result
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
		go func() { finished <- (&JoinOpening{owner: c, generation: g}).Close() }()
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
			t.Fatal("exact opening Close reported completion before original opening joined")
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
		if again := (&JoinOpening{owner: c, generation: g}).Close(); again != first || returns != 1 {
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
	g.ready = true // local retirement state only; no authority is fabricated
	close(g.opened)
	if err := (&JoinOpening{owner: c, generation: g}).Close(); err != nil {
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
	a := &JoinAcquisition{}
	g.joins[a] = struct{}{}
	cause := errors.New("joined physical return failed")
	c.completed(g, a, cause)
	if g.ctx.Err() == nil {
		t.Fatal("physical cleanup failure did not seal pending generation")
	}
	c.mu.Lock()
	retained := c.failure
	c.mu.Unlock()
	c.completed(g, a, nil)
	c.completed(g, a, errors.New("replacement result"))
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
	c.failed(errors.Join(io.ErrUnexpectedEOF, cause))
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

func TestJoinAcquisitionUnpublishedSetupCleanupRetainsFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		owner, generation := joinContextLifetime(t)
		generation.source = joinContextRetiringPrefix(t, generation.ctx, nil)
		generation.ready = true
		close(generation.opened)
		acquisition, claim := lifetimeAcquisition(t, t.Context())
		acquisition.completion = func(result error) { owner.completed(generation, acquisition, result) }
		generation.joins[acquisition] = struct{}{}
		acquisition.opening = make(chan struct{})
		closed := make(chan error, 1)
		go func() { closed <- acquisition.Close() }()
		synctest.Wait()
		select {
		case <-closed:
			t.Fatal("Close did not join unpublished setup")
		default:
		}
		cause := errors.New("unpublished physical setup close failed")
		physical := newLifecycleConn(false)
		physical.closeFailure = cause
		returns := 0
		retirement := acquisition.cleanupJoin(nil, &unpublishedSetupRetirement{conn: transport.Retain(physical), release: func() { returns++ }})
		if !errors.Is(retirement, cause) {
			t.Fatal("immediate setup cleanup lost failure", retirement)
		}
		close(acquisition.opening)
		first := <-closed
		if !errors.Is(first, cause) || acquisition.Close() != first || returns != 1 || claim.returns.Load() != 1 {
			t.Fatal("joined acquisition discarded or replaced setup cleanup failure", first, returns)
		}
		if _, err := owner.Open(t.Context()); !errors.Is(err, cause) {
			t.Fatal("cleanup failure permitted context reopen", err)
		}
		firstContext := owner.Close()
		if !errors.Is(firstContext, cause) || owner.Close() != firstContext || returns != 1 {
			t.Fatal("context lost setup terminal failure or released twice", firstContext, returns)
		}
	})
}

func TestJoinAcquisitionCleanSetupRetirementDoesNotSealContext(t *testing.T) {
	owner, generation := joinContextLifetime(t)
	generation.source = joinContextRetiringPrefix(t, generation.ctx, nil)
	generation.ready = true
	close(generation.opened)
	acquisition, claim := lifetimeAcquisition(t, t.Context())
	acquisition.completion = func(result error) { owner.completed(generation, acquisition, result) }
	generation.joins[acquisition] = struct{}{}
	returns := 0
	if err := acquisition.cleanupJoin(nil, &unpublishedSetupRetirement{release: func() { returns++ }}); err != nil {
		t.Fatal(err)
	}
	if err := acquisition.Close(); err != nil || owner.failure != nil || generation.closing || returns != 1 || claim.returns.Load() != 1 {
		t.Fatal("clean setup retirement fabricated terminal context failure", err, owner.failure, returns)
	}
	if err := owner.currentLocked(generation); err != nil {
		t.Fatal("clean original retirement denied local context", err)
	}
	if err := owner.Close(); err != nil || returns != 1 {
		t.Fatal("clean context joined incorrectly", err, returns)
	}
}

// This failure control joins a real physical close; it cannot open a channel.
type unpublishedSetupRetirement struct {
	conn    net.Conn
	release func()
}

func (p *unpublishedSetupRetirement) CloseSetup() error {
	var err error
	if p.conn != nil {
		err = p.conn.Close()
	}
	if p.release != nil {
		p.release()
	}
	return err
}

// lifecycleConn gates actual physical output and completion independently.
// It supplies no successful authority, Admission, or resource-transfer result.
type lifecycleConn struct {
	mu                    sync.Mutex
	closed                chan struct{}
	closeOnce             sync.Once
	writeGate             chan struct{}
	writes                chan []byte
	output                []byte
	writeEnd              time.Time
	partial, closeFailure error
	input                 *bytes.Reader
	readGate              chan struct{}
	writeIgnoresClose     bool
}

func newLifecycleConn(gated bool) *lifecycleConn {
	c := &lifecycleConn{closed: make(chan struct{}), writeGate: make(chan struct{}), writes: make(chan []byte, 32)}
	if !gated {
		close(c.writeGate)
	}
	return c
}
func (c *lifecycleConn) Read(p []byte) (int, error) {
	if c.input != nil && c.input.Len() > 0 {
		return c.input.Read(p)
	}
	if c.readGate != nil {
		<-c.readGate
	}
	<-c.closed
	return 0, net.ErrClosed
}
func (c *lifecycleConn) Write(p []byte) (int, error) {
	c.writes <- append([]byte(nil), p...)
	if c.writeIgnoresClose {
		<-c.writeGate
	} else {
		select {
		case <-c.writeGate:
		case <-c.closed:
			return 0, net.ErrClosed
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.partial != nil {
		c.output = append(c.output, p[:3]...)
		return 3, c.partial
	}
	c.output = append(c.output, p...)
	return len(p), nil
}
func (c *lifecycleConn) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return c.closeFailure
}
func (c *lifecycleConn) LocalAddr() net.Addr             { return &net.TCPAddr{} }
func (c *lifecycleConn) RemoteAddr() net.Addr            { return &net.TCPAddr{} }
func (c *lifecycleConn) SetDeadline(t time.Time) error   { return c.SetWriteDeadline(t) }
func (c *lifecycleConn) SetReadDeadline(time.Time) error { return nil }
func (c *lifecycleConn) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	c.writeEnd = t
	c.mu.Unlock()
	return nil
}
