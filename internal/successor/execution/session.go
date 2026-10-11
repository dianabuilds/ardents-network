package execution

import (
	"context"
	"crypto/rand"
	"errors"
	"sync"
)

// Session owns one local admission and its exact current/last Job. Closing
// local permission cannot release physical cleanup capacity prematurely.
type Session struct {
	mu         sync.Mutex
	supervisor *Supervisor
	lease      *ActiveSession
	surface    Surface
	job, last  *Job
	closed     bool
	done       chan struct{}
	closeOnce  sync.Once
	result     error
}

func (session *Session) Context() context.Context { return session.lease.Context() }

// Done closes after this original session reports its Job cleanup result to the
// supervisor. Failure retains cleanup capacity; consult Completion for that
// result. Job retirement alone does not close this signal.
func (session *Session) Done() <-chan struct{} { return session.done }

// Completion observes the original retained cleanup result without revoking or
// joining this session. A closed Done signal can carry failed cleanup; only a
// completed nil result establishes successful retirement.
func (session *Session) Completion() (error, bool) {
	if session == nil {
		return errors.New("execution session is absent"), false
	}
	select {
	case <-session.done:
		return session.result, true
	default:
		return nil, false
	}
}

func (session *Session) liveLocked() bool {
	if session == nil || session.closed || session.lease == nil || session.lease.Context().Err() != nil {
		return false
	}
	// The generation latch is the effect-admission boundary. Cancellation of
	// individual contexts follows it and cannot leave a sibling accepting work
	// while the supervisor is revoking the remaining leases.
	session.supervisor.mu.Lock()
	defer session.supervisor.mu.Unlock()
	return !session.supervisor.closed
}

func (session *Session) Check() error {
	if session == nil {
		return errors.New("execution session is absent")
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if !session.liveLocked() {
		return errors.New("execution session is retired")
	}
	return nil
}

// BeginJob reserves one fresh identity under this still-live session. The
// launch owner must retire and finish it on every exit, including before dial.
func (session *Session) BeginJob() (*Job, error) {
	if session == nil {
		return nil, errors.New("execution session is absent")
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if !session.liveLocked() || session.job != nil {
		return nil, errors.New("execution Job is unavailable")
	}
	job := &Job{session: session, done: make(chan struct{})}
	if _, err := rand.Read(job.nonce[:]); err != nil || job.nonce == [32]byte{} {
		return nil, errors.New("execution Job identity is unavailable")
	}
	job.ctx, job.cancel = context.WithCancel(session.Context())
	session.job, session.last = job, job
	return job, nil
}

func (session *Session) Close() error {
	if session == nil {
		return nil
	}
	session.lease.Release()
	<-session.done
	return session.result
}

func (session *Session) closeJoined() {
	session.closeOnce.Do(func() {
		session.lease.Release()
		session.mu.Lock()
		session.closed = true
		job := session.job
		if job != nil {
			job.retireLocked()
		}
		session.mu.Unlock()
		if job != nil {
			<-job.done
			session.result = job.result
		}
		session.supervisor.release(session, session.result)
		close(session.done)
	})
}
