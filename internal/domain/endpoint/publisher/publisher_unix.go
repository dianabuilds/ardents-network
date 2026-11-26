//go:build linux || darwin

package publisher

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/application/document"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	executionruntime "github.com/dianabuilds/ardents-network/internal/domain/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/network"
)

// Publisher serializes local Administration handoffs. Publication
// still owns readiness/refresh/withdrawal and Instance authority; Execution
// owns the original snapshot worker, and Route owns its shared resources.
type Publisher struct {
	ctx           context.Context
	cancel        context.CancelFunc
	local         *executionruntime.Owner
	principal     [32]byte
	plan          Plan
	routePlan     funding.Plan
	routes        *funding.Parents
	authority     funding.Authority
	holder        *stock.Owner
	check         func() error
	boundary      func() error
	failed        context.CancelFunc
	mu            sync.Mutex
	closed        bool
	busy          bool
	withdrawing   bool
	pendingCancel context.CancelFunc
	pendingDone   chan struct{}
	active        *published
	flights       sync.WaitGroup
	once          sync.Once
	result        error
}

type published struct {
	handle    Publication
	operation *executionruntime.Operation
	worker    *executionruntime.Invocation
	cancel    context.CancelFunc
	served    chan struct{}
	serveErr  error
	started   bool
	retiring  atomic.Bool
	once      sync.Once
	result    error
}

func (owner *Publisher) Publish(context.Context) error {
	return errors.New("document publication requires an authorized snapshot")
}

func (owner *Publisher) PublishSnapshot(caller context.Context, snapshot []byte) error {
	return owner.publish(caller, snapshot, nil)
}

// PublishByteStream exposes one authenticated exchange with a locally supplied
// Application port. The retained Publication still owns readiness and withdrawal.
func (owner *Publisher) PublishByteStream(caller context.Context, port document.ByteStreamPort) error {
	if port == nil {
		return errors.New("local byte-stream Application is unavailable")
	}
	return owner.publish(caller, nil, port)
}

func (owner *Publisher) publish(caller context.Context, snapshot []byte, port document.ByteStreamPort) (result error) {
	if owner == nil || caller == nil || owner.ctx == nil || owner.local == nil || owner.holder == nil || owner.check == nil || !owner.authority.Available() || !owner.routes.Matches(owner.routePlan) {
		return errors.New("installed Publisher is unavailable")
	}
	if err := errors.Join(caller.Err(), owner.ctx.Err(), owner.boundary()); err != nil {
		return err
	}
	ctx, setupCancel := context.WithCancel(owner.ctx)
	owner.mu.Lock()
	if owner.closed || owner.busy || owner.withdrawing || owner.active != nil {
		owner.mu.Unlock()
		setupCancel()
		return errors.New("installed Publication transition is unavailable")
	}
	owner.busy = true
	owner.pendingCancel = setupCancel
	pendingDone := make(chan struct{})
	owner.pendingDone = pendingDone
	owner.flights.Add(1)
	owner.mu.Unlock()
	defer func() {
		owner.mu.Lock()
		owner.busy = false
		owner.pendingCancel, owner.pendingDone = nil, nil
		close(pendingDone)
		owner.mu.Unlock()
		owner.flights.Done()
	}()
	candidate := &published{cancel: setupCancel, served: make(chan struct{})}
	callbackDone := make(chan struct{})
	stopCaller := context.AfterFunc(caller, func() { defer close(callbackDone); setupCancel() })
	var callerOnce sync.Once
	joinCaller := func() {
		callerOnce.Do(func() {
			if !stopCaller() {
				<-callbackDone
			}
		})
	}
	defer func() {
		joinCaller()
		if result != nil {
			result = errors.Join(result, candidate.Close())
		}
	}()
	view, err := owner.authority.Current()
	if err := errors.Join(err, ctx.Err(), owner.check()); err != nil {
		return err
	}
	plan := owner.routePlan
	plan.Deadline = time.Now().Add(admission.RegistrationClass.Lifetime()).UTC().Truncate(time.Second)
	if end := view.ProfileBinding().NotAfter; end.Before(plan.Deadline) {
		plan.Deadline = end
	}
	if end, bounded := ctx.Deadline(); bounded && end.Before(plan.Deadline) {
		plan.Deadline = end.UTC().Truncate(time.Second)
	}
	ctx, deadlineCancel := context.WithDeadline(ctx, plan.Deadline)
	candidate.cancel = func() { setupCancel(); deadlineCancel() }
	if port == nil {
		candidate.worker, err = owner.local.LaunchPublisher(ctx, owner.principal, snapshot)
	} else {
		candidate.worker, err = owner.local.LaunchByteStreamPublisher(ctx, owner.principal)
	}
	if err != nil {
		return err
	}
	candidate.operation, err = candidate.worker.BeginOperation(ctx)
	if err != nil {
		return err
	}
	authority := owner.authority
	originalCurrent := authority.Current
	current := func() (network.RuntimeView, error) {
		// The installed authority already reobserves the original Startup on
		// both sides of CurrentRuntime. Add only this operation's live rights;
		// repeating the same full installed observation here doubles native I/O
		// without adding another effect or authority boundary.
		if err := errors.Join(ctx.Err(), candidate.operation.CheckPublisher()); err != nil {
			return network.RuntimeView{}, err
		}
		view, err := originalCurrent()
		return view, errors.Join(err, ctx.Err(), candidate.operation.CheckPublisher())
	}
	authority = funding.Observe(current, authority.Witness, authority.Presentation)
	candidate.handle, err = openPublication(candidate.operation.Context(), owner.plan, plan, authority, owner.holder, candidate.operation, port, owner.routes)
	if err != nil {
		return err
	}
	if _, err := candidate.handle.owner.Publish(caller); err != nil {
		return err
	}
	if err := errors.Join(caller.Err(), ctx.Err(), owner.check(), candidate.operation.CheckPublisher()); err != nil {
		return err
	}
	// Relinquish setup-only interruption before the successful local handoff;
	// the original Endpoint/worker lifetime remains independently checked.
	joinCaller()
	if err := errors.Join(caller.Err(), ctx.Err()); err != nil {
		return err
	}
	owner.mu.Lock()
	if owner.closed || owner.withdrawing || owner.active != nil || caller.Err() != nil || ctx.Err() != nil {
		owner.mu.Unlock()
		return errors.New("installed Publication retired before handoff")
	}
	owner.active = candidate
	owner.pendingCancel, owner.pendingDone = nil, nil
	candidate.started = true
	go func() {
		candidate.serveErr = candidate.handle.Serve(ctx)
		unexpected := ctx.Err() == nil && !candidate.retiring.Load()
		close(candidate.served)
		if stream, ok := port.(*publishedBytePort); ok {
			result := candidate.Close()
			cleanup, complete := candidate.worker.CleanupResult()
			owner.mu.Lock()
			if owner.active == candidate {
				owner.active = nil
			}
			owner.mu.Unlock()
			stream.finish(result)
			if (!complete || cleanup != nil) && owner.failed != nil {
				owner.failed()
			}
			return
		}
		// Join a failed/expired consumer without requiring another local IPC
		// request. Close waits only for the already completed serve loop.
		if !candidate.retiring.Load() {
			_ = candidate.Close()
		}
		if unexpected && owner.failed != nil {
			owner.failed()
		}
	}()
	owner.mu.Unlock()
	return nil
}

func (owner *Publisher) PublishedLink(caller context.Context) (_ string, result error) {
	if caller == nil || owner == nil || owner.ctx == nil || owner.local == nil || owner.check == nil {
		return "", errors.New("installed Publication is unavailable")
	}
	if err := errors.Join(caller.Err(), owner.ctx.Err(), owner.boundary()); err != nil {
		return "", err
	}
	request, err := owner.local.AdmitAdministration(caller, owner.principal)
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, request.Close()) }()
	owner.mu.Lock()
	active := owner.active
	closed := owner.closed
	owner.mu.Unlock()
	if closed || active == nil {
		return "", errors.New("installed Publication is unavailable")
	}
	link, err := active.handle.owner.Link(request.Context())
	if checked := errors.Join(err, request.Check(), caller.Err(), owner.ctx.Err(), owner.check()); checked != nil {
		return "", checked
	}
	return link, nil
}

func (owner *Publisher) Withdraw(caller context.Context) (result error) {
	if owner == nil || caller == nil || owner.ctx == nil || owner.local == nil || owner.check == nil {
		return errors.New("installed Publication is unavailable")
	}
	if err := errors.Join(caller.Err(), owner.ctx.Err(), owner.boundary()); err != nil {
		return err
	}
	request, err := owner.local.AdmitAdministration(caller, owner.principal)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, request.Close()) }()
	owner.mu.Lock()
	if owner.closed || owner.withdrawing || owner.busy && owner.pendingCancel == nil {
		owner.mu.Unlock()
		return errors.New("installed Publication transition is unavailable")
	}
	owner.withdrawing = true
	pendingCancel, pendingDone := owner.pendingCancel, owner.pendingDone
	owner.flights.Add(1)
	owner.mu.Unlock()
	defer func() {
		owner.mu.Lock()
		owner.busy = false
		owner.withdrawing = false
		owner.mu.Unlock()
		owner.flights.Done()
	}()
	if pendingCancel != nil {
		pendingCancel()
		<-pendingDone
	}
	owner.mu.Lock()
	active := owner.active
	owner.active = nil
	owner.busy = true
	owner.mu.Unlock()
	if active == nil {
		return errors.Join(errors.New("installed Publication is unavailable"), request.Check(), caller.Err(), owner.ctx.Err(), owner.check())
	}
	active.retiring.Store(true)
	return errors.Join(active.handle.owner.Withdraw(request.Context()), active.Close(), request.Check(), caller.Err(), owner.ctx.Err(), owner.check())
}

func (publication *published) Close() error {
	publication.retiring.Store(true)
	publication.once.Do(func() {
		publication.cancel()
		if publication.handle.owner != nil {
			publication.result = publication.handle.Close()
		}
		if publication.operation != nil {
			publication.operation.Close()
		}
		if publication.worker != nil {
			publication.result = errors.Join(publication.result, publication.worker.Close())
		}
		// Only a committed Publication starts this consumer; failed setup
		// retains its partial owners without waiting for an absent serve loop.
		if publication.started {
			<-publication.served
			publication.result = errors.Join(publication.result, publication.serveErr)
		}
	})
	return publication.result
}

func (owner *Publisher) Close() error {
	if owner == nil {
		return nil
	}
	if owner.cancel != nil {
		owner.cancel()
	}
	owner.once.Do(func() {
		owner.mu.Lock()
		owner.closed = true
		owner.mu.Unlock()
		owner.flights.Wait()
		owner.mu.Lock()
		active := owner.active
		owner.active = nil
		owner.mu.Unlock()
		if active != nil {
			owner.result = active.Close()
		}
	})
	return owner.result
}

// Config binds the original installed participant. This coordinator borrows
// its Execution, Network, Stock and Parents; Close joins only its Publications.
type Config struct {
	Context   context.Context
	Local     *executionruntime.Owner
	Principal [32]byte
	Plan      Plan
	Route     funding.Plan
	Parents   *funding.Parents
	Authority funding.Authority
	Holder    *stock.Owner
	Check     func() error
	// Boundary is the full-content installed observation at each publication
	// operation's admission entry (PublishSnapshot/PublishedLink/Withdraw); Check
	// is the identity-only inner bracket. Nil Boundary falls back to Check.
	Boundary func() error
	Failed   context.CancelFunc
}

// New retains one participant lifetime. Snapshot authority is checked afresh
// through Execution and Publication for every actual PublishSnapshot.
func New(c Config) *Publisher {
	ctx, cancel := context.WithCancel(c.Context)
	boundary := c.Boundary
	if boundary == nil {
		boundary = c.Check
	}
	return &Publisher{ctx: ctx, cancel: cancel, local: c.Local, principal: c.Principal, plan: c.Plan,
		routePlan: c.Route, routes: c.Parents, authority: c.Authority, holder: c.Holder, check: c.Check, boundary: boundary, failed: c.Failed}
}
