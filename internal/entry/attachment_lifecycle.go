package entry

import (
	"context"
	"errors"
	"sync"
)

// attachmentLease keeps one opened carrier under Entry ownership until its
// terminal cleanup outcome is durably recorded.
type attachmentLease struct {
	owner   *owner
	id      uint64
	ordinal byte
	cleanup func() error
	once    sync.Once
	err     error
}

func (owner *owner) startAcquisition(parent context.Context) (context.Context, func(), error) {
	owner.mu.Lock()
	if owner.closing || owner.closed || owner.failed != nil {
		owner.mu.Unlock()
		return nil, nil, errors.New("entry local denial")
	}
	owner.acquisitions.Add(1)
	lifecycle := owner.lifecycle
	owner.mu.Unlock()

	ctx, cancel := context.WithCancel(parent)
	stop := context.AfterFunc(lifecycle, cancel)
	return ctx, func() {
		stop()
		cancel()
		owner.acquisitions.Done()
	}, nil
}

func (owner *owner) trackAttachment(ordinal byte, cleanup func() error) (*attachmentLease, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.closing || owner.closed || owner.failed != nil {
		return nil, errors.New("entry local denial")
	}
	owner.nextAttachment++
	lease := &attachmentLease{owner: owner, id: owner.nextAttachment, ordinal: ordinal, cleanup: cleanup}
	owner.attachments[lease.id] = lease
	return lease, nil
}

func (lease *attachmentLease) Close() error {
	lease.once.Do(func() {
		cleanupErr := normalizeCleanup(lease.cleanup())
		finishErr := lease.owner.finishContact(lease.ordinal, true, cleanupErr == nil)
		if cleanupErr != nil {
			lease.err = errors.Join(errors.New("entry local denial"), cleanupErr, finishErr)
		} else {
			lease.err = finishErr
		}
		lease.owner.mu.Lock()
		delete(lease.owner.attachments, lease.id)
		lease.owner.mu.Unlock()
	})
	return lease.err
}

func (owner *owner) settleClosingAttempt() error {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.failed != nil {
		return owner.failed
	}
	if owner.state.Attempt == nil || owner.state.Attempt.Terminal != "" {
		return nil
	}
	now := owner.config.Clock().UTC()
	next := owner.state.clone()
	next.Attempt.Terminal, next.Attempt.Ended = "entry-local-denial", now.UnixNano()
	if err := owner.retireInvalidVerifiedLocked(&next); err != nil {
		return err
	}
	next.settleReplacements()
	if err := owner.commit(next, false); err != nil {
		owner.failed = err
		return err
	}
	return nil
}
