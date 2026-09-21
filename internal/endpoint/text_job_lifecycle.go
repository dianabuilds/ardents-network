//go:build linux

package endpoint

import (
	"context"
	"crypto/rand"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
)

// textJobIdentity owns one invocation's nonce, verified worker Grant handoff,
// retirement, and immutable joined cleanup result. The text context retains
// only the admission reservation that points at this exact job.
type textJobIdentity struct {
	qualification *textQualificationRun
	workload      textServiceWorkloadBounds
	owner         *textContext
	nonce         [32]byte
	context       context.Context
	cancel        context.CancelFunc
	done          chan struct{}
	retired       bool
	bound         bool
	finished      bool
	cleanupErr    error
	workerGrant   *broker.Broker
}

type textJobRetirement struct {
	job *textJobIdentity
}

func newTextJobIdentity(owner *textContext) (*textJobIdentity, error) {
	if owner == nil || owner.lease == nil {
		return nil, errors.New("text worker identity is unavailable")
	}
	job := &textJobIdentity{owner: owner, done: make(chan struct{})}
	if _, err := rand.Read(job.nonce[:]); err != nil || job.nonce == [32]byte{} {
		return nil, errors.New("text worker identity is unavailable")
	}
	job.context, job.cancel = context.WithCancel(owner.lease.Context())
	return job, nil
}

func (job *textJobIdentity) currentLocked(owner *textContext, endpoint *endpoint, surface broker.Surface, nonce [32]byte) bool {
	return job != nil && owner != nil && job.owner == owner && owner.job == job && !job.retired &&
		nonce != [32]byte{} && job.nonce == nonce && owner.liveLocked(endpoint, surface)
}

// claimWorkerLocked consumes the one worker-lifetime handoff before INIT. It
// does not assert verified readiness or create a Grant.
func (job *textJobIdentity) claimWorkerLocked(owner *textContext) bool {
	if job == nil || owner == nil || job.owner != owner || job.bound || job.finished {
		return false
	}
	job.bound = true
	return true
}

// handoffGrantLocked publishes a Grant only to this exact live, claimed job.
// Rejected late handoffs are closed here and cannot attach to a replacement.
func (job *textJobIdentity) handoffGrantLocked(owner *textContext, grant *broker.Broker, lease *broker.ActiveSession) bool {
	if grant == nil || lease == nil {
		if grant != nil {
			grant.Close()
		}
		return false
	}
	if job == nil || owner == nil || job.owner != owner || owner.job != job || job.retired || !job.bound ||
		job.finished || job.workerGrant != nil || job.context.Err() != nil {
		lease.Release()
		grant.Close()
		return false
	}
	job.workerGrant = grant
	owner.verifiedJob = job
	return true
}

func (job *textJobIdentity) retire() {
	if job == nil || job.owner == nil {
		return
	}
	job.owner.mu.Lock()
	defer job.owner.mu.Unlock()
	job.retireLocked(job.owner)
}

func (job *textJobIdentity) retireLocked(owner *textContext) bool {
	if job == nil || owner == nil || job.owner != owner || owner.job != job {
		return false
	}
	clear(job.nonce[:])
	job.retired = true
	job.cancel()
	if job.workerGrant != nil {
		job.workerGrant.Close()
	}
	return true
}

func (job *textJobIdentity) stopLocked(owner *textContext) *textJobRetirement {
	if !job.retireLocked(owner) {
		return nil
	}
	return &textJobRetirement{job: job}
}

func (retirement *textJobRetirement) join() error {
	if retirement == nil || retirement.job == nil {
		return nil
	}
	job := retirement.job
	<-job.done
	retirement.job = nil
	return job.cleanupErr
}

// finishCleanup publishes the first joined cleanup result and releases only
// this job's reservation. A late completion cannot release a replacement or
// erase a prior failure.
func (job *textJobIdentity) finishCleanup(cleanupErr error) error {
	if job == nil || job.owner == nil {
		return errors.New("text worker cleanup does not match the retired job")
	}
	owner := job.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if job.finished {
		return job.cleanupErr
	}
	if owner.job != job || !job.retired {
		return errors.New("text worker cleanup does not match the retired job")
	}
	job.finished, job.cleanupErr = true, cleanupErr
	if cleanupErr != nil {
		owner.closed = true
		owner.endpoint.failTextContexts(cleanupErr)
	} else {
		owner.job = nil
	}
	close(job.done)
	return cleanupErr
}
