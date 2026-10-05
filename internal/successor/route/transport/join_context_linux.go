//go:build linux

package transport

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
)

// JoinContextConfig transfers existing role selection borrowers to a context.
// Reserve obtains a distinct genuine Hosting reservation for each physical
// prefix. Its returned function returns only that reservation, after join;
// installation selection and the shared Hosting budget remain composition-owned.
type JoinContextConfig struct {
	Source, Responder *selection.Owner
	Current           func() (network.RuntimeView, error)
	Present           Present
	Deadline          time.Time
	Reserve           func(context.Context) (func() error, error)
	Exclusions        []route.Member
}

// JoinContext owns one fixed local Route role and the original physical
// generations opened through it. Prefix retirement preserves selection; context
// retirement joins physical users before closing its role selection borrowers.
// It grants no Execution, Publication or Service authority.
type JoinContext struct {
	mu          sync.Mutex
	config      JoinContextConfig
	caller      context.Context
	ctx         context.Context
	cancel      context.CancelFunc
	opening     *joinContextOpening
	generations map[*joinContextOpening]struct{}
	choice      *selection.Rendezvous
	closed      bool
	failure     error
	closeOnce   sync.Once
	stopWatch   chan struct{}
	watchDone   chan struct{}
}

// JoinOpening retains the exact published Source and optional Responder. Its
// operations cannot resolve or close a later opening of the same context.
type JoinOpening struct {
	owner      *JoinContext
	generation *joinContextOpening
}

func (o *JoinOpening) Done() <-chan struct{} { return o.generation.readiness }

func (o *JoinOpening) Close() error {
	if o == nil {
		return nil
	}
	o.owner.mu.Lock()
	o.owner.sealLocked(o.generation)
	o.owner.mu.Unlock()
	return o.owner.retire(o.generation)
}

func (o *JoinOpening) Recipient(slot uint8) (network.RetainedDuty, time.Time, error) {
	if o == nil {
		return network.RetainedDuty{}, time.Time{}, net.ErrClosed
	}
	return o.owner.recipient(o.generation, slot)
}

func (o *JoinOpening) Join(ctx context.Context, config JoinConfig, choice uint8) (*Joined, error) {
	if o == nil {
		return nil, net.ErrClosed
	}
	return o.owner.join(o.generation, ctx, config, choice)
}

// This reservation is the generation identity. It is never rebound to another
// caller or published prefix, and a replacement waits for its completed join.
type joinContextOpening struct {
	caller            context.Context
	ctx               context.Context
	cancel            context.CancelFunc
	stopContext       func() bool
	contextStopped    chan struct{}
	opened            chan struct{}
	ready, closing    bool // guarded by JoinContext.mu
	source, responder *Prefix
	joins             map[*JoinAcquisition]struct{}
	operations        sync.WaitGroup
	readiness         chan struct{}
	readinessOnce     sync.Once
	once              sync.Once
	stopWatch         chan struct{}
	watchDone         chan struct{}
	result            error
}

// NewJoinContext takes ownership of Source and optional Responder only on
// success. Construction is unready: only Open can publish genuine prefixes.
func NewJoinContext(ctx context.Context, config JoinContextConfig) (*JoinContext, error) {
	if ctx == nil || ctx.Err() != nil || config.Source == nil || config.Source == config.Responder || config.Current == nil || config.Present == nil || config.Reserve == nil ||
		config.Deadline != config.Deadline.UTC().Truncate(time.Second) || !time.Now().Before(config.Deadline) || config.Deadline.After(time.Now().Add(admission.ForwardClass.Lifetime())) {
		return nil, errors.New("route JOIN context composition unavailable")
	}
	if end, ok := ctx.Deadline(); ok && config.Deadline.After(end) {
		return nil, errors.New("route JOIN context exceeds original caller bound")
	}
	config.Exclusions = append([]route.Member(nil), config.Exclusions...)
	child, cancel := context.WithDeadline(ctx, config.Deadline)
	c := &JoinContext{config: config, caller: ctx, ctx: child, cancel: cancel, generations: make(map[*joinContextOpening]struct{}), stopWatch: make(chan struct{}), watchDone: make(chan struct{})}
	go func() {
		defer close(c.watchDone)
		select {
		case <-child.Done():
			c.closeOnce.Do(c.close)
		case <-c.stopWatch:
		}
	}()
	return c, nil
}

func (c *JoinContext) availableLocked() error {
	if c.closed || c.failure != nil {
		return errors.Join(net.ErrClosed, c.failure)
	}
	return errors.Join(c.caller.Err(), c.ctx.Err())
}

func (c *JoinContext) currentLocked(g *joinContextOpening) error {
	if err := c.availableLocked(); err != nil {
		return err
	}
	if c.opening != g || g.closing {
		return net.ErrClosed
	}
	return errors.Join(g.caller.Err(), g.ctx.Err())
}

// observe performs no Context locking. In particular, Prefix/Joined terminal
// checks may observe the original authority after local admission has sealed.
func (c *JoinContext) observe() (network.RuntimeView, error) {
	if err := errors.Join(c.caller.Err(), c.ctx.Err()); err != nil {
		return network.RuntimeView{}, err
	}
	view, err := c.config.Current()
	return view, errors.Join(err, c.caller.Err(), c.ctx.Err())
}

func (c *JoinContext) openingCurrent(g *joinContextOpening) (network.RuntimeView, error) {
	if err := errors.Join(g.caller.Err(), g.ctx.Err()); err != nil {
		return network.RuntimeView{}, err
	}
	view, err := c.observe()
	return view, errors.Join(err, g.caller.Err(), g.ctx.Err())
}

func (c *JoinContext) openingAllowed(g *joinContextOpening) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.currentLocked(g)
}

// Open reserves one exact physical generation. A second caller cannot borrow
// its pending result or cancel its work. ClosePrefix must join it before reopen.
// The returned handle names only this published opening. Its Done signals
// readiness loss; Close retrieves and joins its original physical work.
func (c *JoinContext) Open(ctx context.Context) (_ *JoinOpening, result error) {
	if c == nil || ctx == nil || ctx.Err() != nil {
		return nil, errors.New("route JOIN opening caller unavailable")
	}
	c.mu.Lock()
	if err := c.availableLocked(); err != nil {
		c.mu.Unlock()
		return nil, err
	}
	if c.opening != nil {
		c.mu.Unlock()
		return nil, errors.New("route JOIN physical generation already held")
	}
	child, cancel := context.WithDeadline(ctx, c.config.Deadline)
	g := &joinContextOpening{caller: ctx, ctx: child, cancel: cancel, contextStopped: make(chan struct{}), opened: make(chan struct{}), joins: make(map[*JoinAcquisition]struct{}), readiness: make(chan struct{}), stopWatch: make(chan struct{}), watchDone: make(chan struct{})}
	g.stopContext = context.AfterFunc(c.ctx, func() { defer close(g.contextStopped); cancel() })
	c.opening = g
	c.generations[g] = struct{}{}
	c.mu.Unlock()
	var source, responder *Prefix
	defer func() {
		if result == nil {
			return
		}
		// Keep late results with their original reservation. No future Open
		// is possible until retire has joined this exact failed opening.
		c.mu.Lock()
		g.source, g.responder = source, responder
		c.sealLocked(g)
		close(g.watchDone) // no readiness watcher was started
		close(g.opened)
		c.mu.Unlock()
		result = errors.Join(result, c.retire(g))
	}()
	source, result = c.openPrefix(g, c.config.Source, nil)
	if result != nil {
		return nil, result
	}
	if c.config.Responder != nil {
		responder, result = c.openPrefix(g, c.config.Responder, source)
		if result != nil {
			return nil, result
		}
	}
	// Reobserve each original prefix outside locks, then publish only if the
	// same reservation, caller and original physical identities remain live.
	for _, p := range []*Prefix{source, responder} {
		if p != nil {
			if err := p.current(); err != nil {
				return nil, err
			}
		}
	}
	c.mu.Lock()
	source.registrationMu.Lock()
	if responder != nil {
		responder.registrationMu.Lock()
	}
	result = c.currentLocked(g)
	for _, p := range []*Prefix{source, responder} {
		if p != nil {
			result = errors.Join(result, p.localCurrent())
		}
	}
	if result == nil {
		g.source, g.responder, g.ready = source, responder, true
		go c.watchPrefix(g)
		close(g.opened)
	}
	if responder != nil {
		responder.registrationMu.Unlock()
	}
	source.registrationMu.Unlock()
	c.mu.Unlock()
	if result != nil {
		return nil, result
	}
	return &JoinOpening{owner: c, generation: g}, nil
}

func (c *JoinContext) openPrefix(g *joinContextOpening, owner *selection.Owner, source *Prefix) (*Prefix, error) {
	if err := c.openingAllowed(g); err != nil {
		return nil, err
	}
	leg, err := owner.Select()
	if err != nil {
		return nil, err
	}
	domain := uint8(1)
	if source != nil {
		domain = 3
	}
	if leg.EntryMember.RoleDomain != domain || leg.InteriorMember.RoleDomain != domain {
		return nil, errors.New("route JOIN role selection differs")
	}
	if c.config.Responder == nil {
		c.mu.Lock()
		err = c.currentLocked(g)
		choice := c.choice
		c.mu.Unlock()
		if err != nil {
			return nil, err
		}
		if choice == nil {
			choice, err = selection.NewRendezvous(leg, c.observe, c.config.Exclusions)
			if err != nil {
				return nil, err
			}
			c.mu.Lock()
			err = c.currentLocked(g)
			if err == nil {
				c.choice = choice
			}
			c.mu.Unlock()
			if err != nil {
				return nil, err
			}
		}
		if !choice.NotAfter().IsZero() {
			// Opening grants no selected-slot retry: it only verifies that a
			// previously fixed slot remains usable with this physical leg.
			if _, first := choice.DutyForLeg(leg, 0, c.config.Exclusions); first != nil {
				if _, other := choice.DutyForLeg(leg, 1, c.config.Exclusions); other != nil {
					return nil, errors.Join(first, other)
				}
			}
		}
	}
	if err := c.openingAllowed(g); err != nil {
		return nil, err
	}
	returned, err := c.config.Reserve(g.ctx)
	if err != nil {
		if returned != nil {
			cleanup := returned()
			c.failed(cleanup)
			err = errors.Join(err, cleanup)
		}
		return nil, err
	}
	if returned == nil {
		return nil, errors.New("route JOIN reservation return absent")
	}
	var once sync.Once
	var cleanup error
	release := func() error {
		once.Do(func() { cleanup = returned(); c.failed(cleanup) })
		return cleanup
	}
	if err := c.openingAllowed(g); err != nil {
		return nil, errors.Join(err, release())
	}
	deadline := c.config.Deadline
	if end, ok := g.caller.Deadline(); ok && end.Before(deadline) {
		deadline = end.UTC().Truncate(time.Second)
	}
	config := PrefixConfig{Leg: leg, Current: func() (network.RuntimeView, error) { return c.openingCurrent(g) }, Present: c.config.Present, Deadline: deadline, Release: release}
	var p *Prefix
	if source == nil {
		p, err = OpenPrefix(g.ctx, config)
	} else {
		p, err = OpenResponderPrefix(g.ctx, source, config)
	}
	if err != nil {
		c.failedOpening(err)
		return nil, errors.Join(err, release())
	}
	return p, nil
}

// Recipient is initiating-only. The Publisher consumes an incoming exact
// recipient through Join and cannot use this method to choose a substitute.
func (c *JoinContext) recipient(g *joinContextOpening, slot uint8) (network.RetainedDuty, time.Time, error) {
	if c == nil {
		return network.RetainedDuty{}, time.Time{}, net.ErrClosed
	}
	c.mu.Lock()
	if g == nil || !g.ready || c.config.Responder != nil || c.choice == nil {
		c.mu.Unlock()
		return network.RetainedDuty{}, time.Time{}, errors.New("route JOIN initiating prefix unavailable")
	}
	if err := c.currentLocked(g); err != nil {
		c.mu.Unlock()
		return network.RetainedDuty{}, time.Time{}, err
	}
	g.operations.Add(1)
	choice := c.choice
	c.mu.Unlock()
	defer g.operations.Done()
	if err := g.source.current(); err != nil {
		return network.RetainedDuty{}, time.Time{}, err
	}
	duty, err := choice.DutyForLeg(g.source.config.Leg, slot, c.config.Exclusions)
	if err != nil {
		return network.RetainedDuty{}, time.Time{}, err
	}
	c.mu.Lock()
	err = c.handoffLocked(g, g.source, nil)
	c.mu.Unlock()
	if err != nil {
		return network.RetainedDuty{}, time.Time{}, err
	}
	return duty, minDeadline(choice.NotAfter(), g.source.config.Deadline), nil
}

// Join retains exact original prefix acquisitions before observing a recipient
// or presenting stock. Success transfers that acquisition with the Joined stream.
func (c *JoinContext) join(g *joinContextOpening, ctx context.Context, config JoinConfig, choice uint8) (*Joined, error) {
	if c == nil || ctx == nil || ctx.Err() != nil {
		return nil, errors.New("route JOIN caller unavailable")
	}
	c.mu.Lock()
	if g == nil || !g.ready {
		c.mu.Unlock()
		return nil, errors.New("route JOIN physical prefix unavailable")
	}
	if err := c.currentLocked(g); err != nil {
		c.mu.Unlock()
		return nil, err
	}
	var a *JoinAcquisition
	var err error
	if g.responder == nil {
		a, err = g.source.AcquireSourceJoin(ctx)
	} else {
		a, err = g.responder.AcquireResponderJoin(ctx)
	}
	if err != nil {
		c.mu.Unlock()
		return nil, err
	}
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		c.mu.Unlock()
		return nil, errors.Join(net.ErrClosed, a.Close())
	}
	a.context = c
	a.generation = g
	a.mu.Unlock()
	g.joins[a] = struct{}{}
	g.operations.Add(1)
	retained := c.choice
	c.mu.Unlock()
	defer g.operations.Done()
	if g.responder == nil {
		var duty network.RetainedDuty
		duty, err = retained.DutyForLeg(g.source.config.Leg, choice, c.config.Exclusions)
		if err == nil && (duty != config.Duty || config.Deadline.After(retained.NotAfter())) {
			err = errors.New("route JOIN differs from retained context choice")
		}
	}
	if err == nil {
		for _, p := range []*Prefix{g.source, g.responder} {
			if p == nil {
				continue
			}
			view, currentErr := p.config.Current()
			if currentErr != nil {
				err = currentErr
				break
			}
			duty, dutyErr := p.config.Leg.RendezvousDuty(view, config.Duty.NodeID, config.Duty.RecordGeneration, c.config.Exclusions)
			if dutyErr != nil || duty != config.Duty {
				err = errors.Join(errors.New("route JOIN exact incoming recipient unavailable"), dutyErr)
				break
			}
		}
	}
	if err != nil {
		return nil, errors.Join(err, a.Close())
	}
	joined, err := a.Join(ctx, config)
	if err != nil {
		return nil, errors.Join(err, a.Close())
	}
	c.mu.Lock()
	err = errors.Join(c.handoffLocked(g, g.source, g.responder), ctx.Err())
	c.mu.Unlock()
	if err != nil {
		return nil, errors.Join(err, joined.Close())
	}
	return joined, nil
}

// Local publication and each original Prefix.Seal use these same role locks.
// The Context lock is already held; no Current callback or I/O is allowed here.
func (c *JoinContext) handoffLocked(g *joinContextOpening, source, responder *Prefix) error {
	source.registrationMu.Lock()
	defer source.registrationMu.Unlock()
	if responder != nil {
		responder.registrationMu.Lock()
		defer responder.registrationMu.Unlock()
	}
	result := c.currentLocked(g)
	for _, p := range []*Prefix{source, responder} {
		if p != nil {
			result = errors.Join(result, p.localCurrent())
		}
	}
	return result
}

func (c *JoinContext) watchPrefix(g *joinContextOpening) {
	retired := false
	defer func() {
		close(g.watchDone)
		if retired {
			c.mu.Lock()
			delete(c.generations, g)
			c.mu.Unlock()
		}
	}()
	var responderDone <-chan struct{}
	if g.responder != nil {
		responderDone = g.responder.Done()
	}
	select {
	case <-g.source.Done():
	case <-responderDone:
	case <-g.stopWatch:
		return
	}
	g.once.Do(func() { c.retireOpening(g) })
	retired = true
}

// sealLocked performs only synchronous local revocation. Ready physical
// contexts stay live until bounded terminal output and all borrowers join.
func (c *JoinContext) sealLocked(g *joinContextOpening) {
	if g == nil || g.closing {
		return
	}
	g.closing = true
	g.readinessOnce.Do(func() { close(g.readiness) })
	g.source.Seal()
	g.responder.Seal()
	if !g.ready {
		g.cancel()
	}
}

func (c *JoinContext) failed(err error) {
	if err == nil {
		return
	}
	c.mu.Lock()
	c.failure = errors.Join(c.failure, err)
	c.sealLocked(c.opening)
	c.mu.Unlock()
}

func (c *JoinContext) failedOpening(err error) {
	var failed *prefixOpeningFailure
	if errors.As(err, &failed) {
		c.failed(failed.retirement)
	}
}

// completed is called after the acquisition has released every original role
// lock. An explicit stream Close failure immediately denies subsequent work.
func (c *JoinContext) completed(a *JoinAcquisition, result error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	g := a.generation
	if g == nil {
		return
	}
	if _, active := g.joins[a]; !active {
		return
	}
	delete(g.joins, a)
	if result != nil {
		c.failure = errors.Join(c.failure, result)
		c.sealLocked(c.opening)
	}
}

// ClosePrefix joins this physical generation, preserving context selection.
// A failed retirement stays terminal and cannot be erased by another Open.
func (c *JoinContext) ClosePrefix() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	g := c.opening
	c.sealLocked(g)
	if g == nil {
		result := c.failure
		c.mu.Unlock()
		return result
	}
	c.mu.Unlock()
	result := c.retire(g)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failure != nil {
		return c.failure
	}
	return result
}

func (c *JoinContext) retire(g *joinContextOpening) error {
	g.once.Do(func() { c.retireOpening(g) })
	<-g.watchDone
	c.mu.Lock()
	delete(c.generations, g)
	c.mu.Unlock()
	return g.result
}

func (c *JoinContext) retireOpening(g *joinContextOpening) {
	c.mu.Lock()
	c.sealLocked(g)
	c.mu.Unlock()
	close(g.stopWatch)
	<-g.opened
	// A canceled opening may have produced a late original handle.
	g.source.Seal()
	g.responder.Seal()
	g.operations.Wait()
	c.mu.Lock()
	acquisitions := make([]*JoinAcquisition, 0, len(g.joins))
	for acquisition := range g.joins {
		acquisitions = append(acquisitions, acquisition)
	}
	c.mu.Unlock()
	var result error
	for _, acquisition := range acquisitions {
		result = errors.Join(result, acquisition.Close())
	}
	result = errors.Join(result, g.responder.Close(), g.source.Close())
	g.cancel()
	if !g.stopContext() {
		<-g.contextStopped
	}
	c.mu.Lock()
	g.result = result
	if result != nil {
		c.failure = errors.Join(c.failure, result)
	}
	if c.opening == g {
		c.opening = nil
	}
	c.mu.Unlock()
}

func (c *JoinContext) close() {
	close(c.stopWatch)
	c.mu.Lock()
	c.closed = true
	all := make([]*joinContextOpening, 0, len(c.generations))
	for g := range c.generations {
		c.sealLocked(g)
		all = append(all, g)
	}
	c.mu.Unlock()
	for _, g := range all {
		_ = c.retire(g)
	}
	// Context selection outlives every physical generation and acquisition.
	result := errors.Join(c.config.Responder.Close(), c.config.Source.Close())
	c.cancel()
	c.mu.Lock()
	if result != nil {
		c.failure = errors.Join(c.failure, result)
	}
	c.mu.Unlock()
}

// Close retires the context permanently. Repeated calls retain its completed
// physical and selection cleanup result; no timeout substitutes for the joins.
func (c *JoinContext) Close() error {
	if c == nil {
		return nil
	}
	c.closeOnce.Do(c.close)
	<-c.watchDone
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.failure
}
