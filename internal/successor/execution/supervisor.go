package execution

import (
	"context"
	"errors"
	"sync"
)

// Supervisor keeps cleanup capacity independently of pending/active Grant
// admission. A revoked lease cannot return a reservation whose Job is unjoined.
type Supervisor struct {
	mu        sync.Mutex
	authority *Authority
	sessions  map[*Session]struct{}
	closed    bool
	first     error
}

func NewSupervisor(authority *Authority) (*Supervisor, error) {
	if authority == nil {
		return nil, errors.New("local authority is absent")
	}
	return &Supervisor{authority: authority, sessions: make(map[*Session]struct{})}, nil
}

// Activate consumes exact local authority before retaining the independent
// cleanup slot. Neither receipt nor session attests a worker invocation.
func (supervisor *Supervisor) Activate(ctx context.Context, capability, principal [32]byte, surface Surface) (*Session, error) {
	if supervisor == nil {
		return nil, errors.New("execution supervisor is absent")
	}
	supervisor.mu.Lock()
	defer supervisor.mu.Unlock()
	if supervisor.closed {
		return nil, errors.New("execution generation is closed")
	}
	lease, _, err := supervisor.authority.Activate(ctx, capability, principal, surface)
	if err != nil {
		return nil, err
	}
	count := 0
	for session := range supervisor.sessions {
		if session.surface == surface {
			count++
		}
	}
	if count >= admissionCapacityFor(surface) {
		lease.Release()
		return nil, errors.New("execution cleanup capacity is exhausted")
	}
	session := &Session{supervisor: supervisor, lease: lease, surface: surface, done: make(chan struct{})}
	supervisor.sessions[session] = struct{}{}
	go func() { <-lease.Context().Done(); session.closeJoined() }()
	return session, nil
}

// fail denies effects in every retained session before any join or capacity
// return. The first physical cleanup failure belongs to this generation.
func (supervisor *Supervisor) fail(err error) {
	if err == nil {
		return
	}
	supervisor.mu.Lock()
	defer supervisor.mu.Unlock()
	if supervisor.first == nil {
		supervisor.first = err
	}
	supervisor.closed = true
	supervisor.authority.Close()
}

func (supervisor *Supervisor) release(session *Session, err error) {
	supervisor.mu.Lock()
	defer supervisor.mu.Unlock()
	if err == nil {
		delete(supervisor.sessions, session)
	}
}

// Close cancels every dependent lease before joining any session. Failed
// physical cleanup retains its slot permanently in this closed generation.
func (supervisor *Supervisor) Close() error {
	if supervisor == nil {
		return nil
	}
	supervisor.mu.Lock()
	supervisor.closed = true
	supervisor.authority.Close()
	pending := make([]*Session, 0, len(supervisor.sessions))
	for session := range supervisor.sessions {
		pending = append(pending, session)
	}
	supervisor.mu.Unlock()
	for _, session := range pending {
		_ = session.Close()
	}
	supervisor.mu.Lock()
	defer supervisor.mu.Unlock()
	return supervisor.first
}
