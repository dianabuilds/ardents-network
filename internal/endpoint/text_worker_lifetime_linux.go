//go:build linux

package endpoint

import (
	"context"
	"errors"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
)

// textWorkerLifetime joins an observed worker to its existing local job.
// It is not a qualified-launch receipt: artifact/activation verification and
// installed stop authority remain separate prerequisites for Grant delivery.
// No Principal or Grant is created by initialization or possession of this owner.
type textWorkerLifetime struct {
	useMu         sync.Mutex
	closing       bool
	operationDone chan struct{}
	context       context.Context
	attachment    *textWorkerAttachment
	cancel        context.CancelFunc
	done          chan struct{}
	err           error
}

// initializeOwnedTextWorker consumes one previously unclaimed job reservation
// and its attachment. Repeated calls leave their input owned by the caller.
// The installed launch owner supplies its exact observed
// instance; this function pins cleanup before sending any INIT bytes.
func initializeOwnedTextWorker(ctx, startup context.Context, attachment *textWorkerAttachment, instance textWorkerInstance, job *textJobIdentity, snapshot []byte, artifact *textWorkerArtifact) (*textWorkerLifetime, error) {
	if job == nil || job.owner == nil {
		return nil, errors.Join(errors.New("text worker has no job owner"), attachment.Close())
	}
	surface := broker.Connection
	if instance.role == "publisher" {
		surface = broker.Administration
	}
	owner := job.owner
	owner.mu.Lock()
	if job.bound || job.finished {
		owner.mu.Unlock()
		return nil, errors.New("text worker job was already consumed")
	}
	job.bound = true
	current := ctx != nil && startup != nil && owner.liveLocked(owner.endpoint, surface) && owner.job == job && !job.retired &&
		attachment != nil && attachment.connection != nil && attachment.pid == instance.pid && attachment.uid == instance.uid
	owner.mu.Unlock()
	if !current {
		return nil, failTextWorkerInitialization(job, attachment, errors.New("text worker lifetime is unavailable"))
	}
	cleanup, err := newTextWorkerCleanup(instance)
	if err != nil {
		return nil, failTextWorkerInitialization(job, attachment, err)
	}
	bounded, cancel := context.WithCancel(job.context)
	lifetime := &textWorkerLifetime{attachment: attachment, cancel: cancel, done: make(chan struct{}), context: bounded}
	// Join the parent's cancellation callback as well as worker cleanup. The
	// callback only interrupts; it never waits for the lifetime it interrupted.
	callbackDone := make(chan struct{})
	stopParent := context.AfterFunc(ctx, func() { cancel(); close(callbackDone) })
	initializationDone := make(chan struct{})
	go func() {
		<-bounded.Done()
		if !stopParent() {
			<-callbackDone
		}
		owner.retireJob(job)
		attachmentErr := attachment.Close()
		lifetime.useMu.Lock()
		lifetime.closing = true
		operationDone := lifetime.operationDone
		lifetime.useMu.Unlock()
		if operationDone != nil {
			<-operationDone
		}
		<-initializationDone
		lifetime.err = errors.Join(attachmentErr, cleanup.Close())
		lifetime.err = owner.finishJobCleanup(job, lifetime.err)
		close(lifetime.done)
	}()
	initializationErr := artifact.verify()
	if initializationErr == nil {
		initializationErr = initializeTextWorker(startup, attachment, instance, job, snapshot)
	}
	close(initializationDone)
	if initializationErr != nil {
		return nil, errors.Join(initializationErr, lifetime.Close())
	}
	if bounded.Err() != nil {
		return nil, errors.Join(errors.New("text worker readiness was cancelled"), lifetime.Close())
	}
	return lifetime, nil
}

func failTextWorkerInitialization(job *textJobIdentity, attachment *textWorkerAttachment, cause error) error {
	// Without the pinned original cgroup, closing a socket cannot prove cleanup.
	// Keep the failure and terminalize the context, even if the peer later exits.
	job.owner.retireJob(job)
	return job.owner.finishJobCleanup(job, errors.Join(cause, attachment.Close()))
}

func (lifetime *textWorkerLifetime) Close() error {
	if lifetime == nil {
		return nil
	}
	lifetime.cancel()
	<-lifetime.done
	return lifetime.err
}

// beginUse reserves the one complete reader operation or publication lifetime.
// Cleanup interrupts the attachment first, then joins this application's I/O
// before releasing the context's job reservation or reporting Close complete.
func (lifetime *textWorkerLifetime) beginUse() (func(), error) {
	if lifetime == nil {
		return nil, errors.New("text worker lifetime is absent")
	}
	lifetime.useMu.Lock()
	defer lifetime.useMu.Unlock()
	if lifetime.closing || lifetime.context == nil || lifetime.context.Err() != nil || lifetime.operationDone != nil {
		return nil, errors.New("text worker operation is unavailable or already consumed")
	}
	lifetime.operationDone = make(chan struct{})
	return func() { close(lifetime.operationDone) }, nil
}
