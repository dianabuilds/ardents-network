package execution

import (
	"context"
	"errors"
)

// Job is the one invocation lifetime in a Session, not a network context or a
// confinement receipt. Its retained identity never moves to a successor Job.
type Job struct {
	session                               *Session
	nonce                                 [32]byte
	ctx                                   context.Context
	cancel                                context.CancelFunc
	done                                  chan struct{}
	claimed, operation, retired, finished bool
	result                                error
}

func (job *Job) Context() context.Context { return job.ctx }
func (job *Job) Nonce() [32]byte          { return job.nonce }

func (job *Job) currentLocked() bool {
	return job.session.liveLocked() && job.session.job == job && !job.retired && !job.finished && job.ctx.Err() == nil
}

func (job *Job) Check() error {
	if job == nil || job.session == nil {
		return errors.New("execution Job is absent")
	}
	job.session.mu.Lock()
	defer job.session.mu.Unlock()
	if !job.currentLocked() {
		return errors.New("execution Job is retired")
	}
	return nil
}

// ClaimLifetime consumes exactly one cleanup handoff before INIT. The physical
// owner must independently pin its original scope; this claim grants no bytes.
func (job *Job) ClaimLifetime() error {
	if job == nil || job.session == nil {
		return errors.New("execution Job is absent")
	}
	job.session.mu.Lock()
	defer job.session.mu.Unlock()
	if !job.currentLocked() || job.claimed {
		return errors.New("execution lifetime is unavailable or claimed")
	}
	job.claimed = true
	return nil
}

// ClaimOperation reserves the sole complete use of the already claimed
// invocation. Finishing a use never makes a second use available.
func (job *Job) ClaimOperation() error {
	if job == nil || job.session == nil {
		return errors.New("execution Job is absent")
	}
	job.session.mu.Lock()
	defer job.session.mu.Unlock()
	if !job.currentLocked() || !job.claimed || job.operation {
		return errors.New("execution operation is unavailable or consumed")
	}
	job.operation = true
	return nil
}

// Retire atomically interrupts this original Job and reports whether it was
// still accepting work. The physical owner uses that ordering to distinguish
// an unexpected invocation failure from an already requested close.
func (job *Job) Retire() bool {
	if job == nil || job.session == nil {
		return false
	}
	job.session.mu.Lock()
	defer job.session.mu.Unlock()
	wasLive := job.currentLocked()
	job.retireLocked()
	return wasLive
}

// FailCleanup denies further effects in the entire generation when the exact
// invocation owner observes a borrowed resource’s physical close failure.
// It neither finishes the Job nor releases any unjoined cleanup reservation.
func (job *Job) FailCleanup(err error) {
	if job == nil || job.session == nil || err == nil {
		return
	}
	job.session.supervisor.fail(err)
}

func (job *Job) retireLocked() { job.retired = true; job.cancel() }

// Finish records the first physical cleanup result after the launch owner has
// joined its INIT, operation, callback and original worker descendants. Failure
// retains cleanup capacity and synchronously closes the generation before Done.
func (job *Job) Finish(err error) error {
	if job == nil || job.session == nil {
		return errors.New("execution Job is absent")
	}
	session := job.session
	session.mu.Lock()
	if job.finished {
		result := job.result
		session.mu.Unlock()
		return result
	}
	if session.job != job || !job.retired {
		session.mu.Unlock()
		return errors.New("execution cleanup does not match the original Job")
	}
	job.finished, job.result = true, err
	if err == nil {
		session.job = nil
	}
	session.mu.Unlock()
	if err != nil {
		session.supervisor.fail(err)
	}
	close(job.done)
	return err
}

// CompletedCurrent is a final result check, never live worker permission.
func (job *Job) CompletedCurrent() bool {
	if job == nil || job.session == nil {
		return false
	}
	job.session.mu.Lock()
	defer job.session.mu.Unlock()
	return job.session.liveLocked() && job.session.last == job && job.session.job == nil && job.finished && job.result == nil
}
