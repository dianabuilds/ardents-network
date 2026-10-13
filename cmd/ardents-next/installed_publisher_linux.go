//go:build linux

package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
)

// installedPublisher serializes local Administration handoffs. Publication
// still owns readiness/refresh/withdrawal and Instance authority; Execution
// owns the original snapshot worker, and Route owns its shared resources.
type installedPublisher struct {
	ctx           context.Context
	cancel        context.CancelFunc
	local         *executionruntime.Owner
	principal     [32]byte
	plan          publicationPlan
	routePlan     routePrefixPlan
	routes        *installedRouteOwners
	authority     admissionAuthority
	holder        *stock.Owner
	check         func() error
	failed        context.CancelFunc
	mu            sync.Mutex
	closed        bool
	busy          bool
	withdrawing   bool
	pendingCancel context.CancelFunc
	pendingDone   chan struct{}
	active        *installedPublished
	flights       sync.WaitGroup
	once          sync.Once
	result        error
}

type installedPublished struct {
	handle    publicationHandle
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

func (owner *installedPublisher) Publish(context.Context) error {
	return errors.New("text publication requires an authorized snapshot")
}

func (owner *installedPublisher) PublishSnapshot(caller context.Context, snapshot []byte) (result error) {
	if owner == nil || caller == nil || owner.ctx == nil || owner.local == nil || owner.holder == nil || owner.check == nil || owner.authority.current == nil || !owner.routes.matches(owner.routePlan) {
		return errors.New("installed Publisher is unavailable")
	}
	if err := errors.Join(caller.Err(), owner.ctx.Err(), owner.check()); err != nil {
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
	candidate := &installedPublished{cancel: setupCancel, served: make(chan struct{})}
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
	view, err := owner.authority.current()
	if err := errors.Join(err, ctx.Err(), owner.check()); err != nil {
		return err
	}
	plan := owner.routePlan
	plan.Deadline = time.Now().Add(admission.RegistrationClass.Lifetime()).UTC().Truncate(time.Second)
	if end := view.Profile().NotAfter; end.Before(plan.Deadline) {
		plan.Deadline = end
	}
	if end, bounded := ctx.Deadline(); bounded && end.Before(plan.Deadline) {
		plan.Deadline = end.UTC().Truncate(time.Second)
	}
	ctx, deadlineCancel := context.WithDeadline(ctx, plan.Deadline)
	candidate.cancel = func() { setupCancel(); deadlineCancel() }
	candidate.worker, err = owner.local.LaunchPublisher(ctx, owner.principal, snapshot)
	if err != nil {
		return err
	}
	candidate.operation, err = candidate.worker.BeginOperation(ctx)
	if err != nil {
		return err
	}
	authority := owner.authority
	originalCurrent := authority.current
	authority.current = func() (network.RuntimeView, error) {
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
	candidate.handle, err = startPublicationHolder(candidate.operation.Context(), owner.plan, plan, authority, owner.holder, candidate.operation, owner.routes)
	if err != nil {
		return err
	}
	if _, err := candidate.handle.publish(caller); err != nil {
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
		candidate.serveErr = candidate.handle.serve(ctx)
		unexpected := ctx.Err() == nil && !candidate.retiring.Load()
		close(candidate.served)
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

func (owner *installedPublisher) PublishedLink(caller context.Context) (_ string, result error) {
	if caller == nil || owner == nil || owner.ctx == nil || owner.local == nil || owner.check == nil {
		return "", errors.New("installed Publication is unavailable")
	}
	if err := errors.Join(caller.Err(), owner.ctx.Err(), owner.check()); err != nil {
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
	link, err := active.handle.link(request.Context())
	if checked := errors.Join(err, request.Check(), caller.Err(), owner.ctx.Err(), owner.check()); checked != nil {
		return "", checked
	}
	return link, nil
}

func (owner *installedPublisher) Withdraw(caller context.Context) (result error) {
	if owner == nil || caller == nil || owner.ctx == nil || owner.local == nil || owner.check == nil {
		return errors.New("installed Publication is unavailable")
	}
	if err := errors.Join(caller.Err(), owner.ctx.Err(), owner.check()); err != nil {
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
		return errors.Join(request.Check(), caller.Err(), owner.ctx.Err(), owner.check())
	}
	active.retiring.Store(true)
	return errors.Join(active.handle.withdraw(request.Context()), active.Close(), request.Check(), caller.Err(), owner.ctx.Err(), owner.check())
}

func (publication *installedPublished) Close() error {
	publication.retiring.Store(true)
	publication.once.Do(func() {
		publication.cancel()
		if publication.handle.close != nil {
			publication.result = publication.handle.close()
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

func (owner *installedPublisher) Close() error {
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
