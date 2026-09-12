//go:build linux

package endpoint

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// textPublisherRun retains publication and worker ownership after the startup
// caller returns. Its done channel publishes the immutable joined outcome.
// Cancellation is an abort; orderly withdrawal has a separate drain contract.
type textPublisherRun struct {
	owner        *textContext
	mu           sync.Mutex
	ending       bool
	withdrawDone chan struct{}
	link         targetlink.Link
	cancel       context.CancelFunc
	done         chan struct{}
	err          error
}

// startTextPublisher performs installed qualification before any registration
// or Descriptor effect. The separately authorized context must already hold
// its genuine offline permission; a snapshot cannot supply that authority.
func (owner *textContext) startTextPublisher(ctx context.Context, snapshot []byte) (*textPublisherRun, error) {
	worker, err := owner.launchTextWorker(ctx, snapshot)
	if err != nil {
		return nil, err
	}
	run, err := worker.startPublication(ctx)
	if err != nil {
		return nil, errors.Join(err, worker.Close(), owner.Close())
	}
	return run, nil
}

func (worker *qualifiedTextWorker) startPublication(ctx context.Context) (_ *textPublisherRun, resultErr error) {
	if worker == nil || worker.job == nil || ctx == nil || ctx.Err() != nil {
		return nil, errors.New("text Publisher startup unavailable")
	}
	owner := worker.job.owner
	owner.mu.Lock()
	current := owner.liveTextServiceJobLocked(worker.job, broker.Administration) && !owner.publicationStarting && !owner.publicationDraining && owner.registration == nil
	if current {
		owner.publicationStarting = true
		owner.publicationDrain = make(chan struct{})
	}
	owner.mu.Unlock()
	if !current {
		return nil, errors.New("text Publisher startup already owned or unavailable")
	}
	lifetime, cancel := context.WithCancel(worker.job.context)
	bounded, finish, operationErr := worker.beginOperation(lifetime, broker.Administration)
	transferred := false
	defer func() {
		if !transferred {
			cancel()
			if finish != nil {
				finish()
			}
			resultErr = errors.Join(resultErr, worker.Close(), owner.Close())
		}
		owner.mu.Lock()
		owner.publicationStarting = false
		owner.mu.Unlock()
	}()
	if operationErr != nil {
		return nil, operationErr
	}
	if _, err := owner.openTextPrefix(ctx); err != nil {
		return nil, err
	}
	prefix, err := owner.openTextIntroductionPrefix(ctx)
	if err != nil {
		return nil, err
	}
	_, until, err := prefix.IntroductionRecipient()
	if err != nil {
		return nil, err
	}
	expiry := owner.endpoint.clock().UTC().Truncate(time.Second).Add(600 * time.Second)
	if until.Before(expiry) {
		expiry = until
	}
	if _, err := owner.registerTextIntroduction(ctx, 1, expiry); err != nil {
		return nil, err
	}
	descriptor, err := owner.publishTextDescriptor(ctx)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	run := &textPublisherRun{owner: owner, link: targetlink.Link{Network: owner.endpoint.network, Target: descriptor.Descriptor.Target}, cancel: cancel, done: make(chan struct{})}
	transferred = true
	go func() {
		defer close(run.done)
		serveErr := worker.serveOperation(lifetime, bounded, finish, worker.produceNetwork)
		run.mu.Lock()
		run.ending = true
		withdrawn := run.withdrawDone
		run.mu.Unlock()
		if withdrawn != nil {
			<-withdrawn
		}
		run.err = errors.Join(serveErr, owner.Close())
		cancel()
	}()
	return run, nil
}

// Close aborts new and retained work and joins publication/worker cleanup.
func (run *textPublisherRun) Close() error {
	if run == nil {
		return nil
	}
	run.cancel()
	<-run.done
	return run.err
}
