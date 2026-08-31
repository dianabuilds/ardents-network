package broker

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"sync"
	"time"
)

const (
	maximumSessions = 6
	sessionLifetime = 15 * time.Second
)

type session struct {
	principal [32]byte
	surface   Surface
	issued    int64
	expires   int64
}

type activeSession struct {
	principal [32]byte
	surface   Surface
	expires   int64
	cancel    context.CancelFunc
	timer     *time.Timer
}

// ActiveSession is an opaque lease over one consumed local Application
// session. It exposes only cancellation and idempotent terminal release.
type ActiveSession struct {
	broker  *Broker
	id      [32]byte
	ctx     context.Context
	release sync.Once
}

// Context is cancelled when the caller, exact Grant, drain deadline, or Broker
// ends the active session.
func (session *ActiveSession) Context() context.Context {
	if session == nil || session.ctx == nil {
		return context.Background()
	}
	return session.ctx
}

// Release returns the active session budget exactly once.
func (session *ActiveSession) Release() {
	if session == nil || session.broker == nil {
		return
	}
	session.release.Do(func() { session.broker.releaseActive(session.id) })
}

// Broker owns one process-local Local Grant/session tree. Restarting it drops
// every session; it never authenticates a platform process or isolation claim.
type Broker struct {
	mu       sync.Mutex
	id       [32]byte
	grants   map[Surface]Grant
	sessions map[[32]byte]session
	active   map[[32]byte]*activeSession
	draining map[Surface]bool
	clock    func() time.Time
	closed   bool
}

// New validates one finite local grant set.
func New(config Config) (*Broker, error) {
	if config.ID == [32]byte{} || len(config.Grants) == 0 || len(config.Grants) > 2 {
		return nil, errors.New("broker configuration is incomplete")
	}
	grants := make(map[Surface]Grant, len(config.Grants))
	for _, grant := range config.Grants {
		if grant.Principal == [32]byte{} || (grant.Surface != Connection && grant.Surface != Administration) {
			return nil, errors.New("broker grant is invalid")
		}
		if _, exists := grants[grant.Surface]; exists {
			return nil, errors.New("broker grant surface is duplicated")
		}
		grants[grant.Surface] = grant
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	return &Broker{id: config.ID, grants: grants, sessions: make(map[[32]byte]session, maximumSessions),
		active: make(map[[32]byte]*activeSession, maximumSessions), draining: make(map[Surface]bool), clock: config.Clock}, nil
}

// Admit issues one short-lived, one-use capability for an exact Principal and
// granted surface.
func (broker *Broker) Admit(principal [32]byte, surface Surface) ([32]byte, error) {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	if broker.closed {
		return [32]byte{}, errors.New("broker is draining")
	}
	grant, ok := broker.grants[surface]
	if !ok || broker.draining[surface] || grant.Principal != principal {
		return [32]byte{}, errors.New("application Principal does not match Local Grant")
	}
	if len(broker.sessions)+len(broker.active) >= maximumSessions {
		return [32]byte{}, errors.New("session budget exhausted")
	}
	var capability [32]byte
	if _, err := rand.Read(capability[:]); err != nil || capability == [32]byte{} {
		return [32]byte{}, errors.New("fresh local session could not be created")
	}
	issued := broker.clock()
	broker.sessions[capability] = session{principal: principal, surface: surface,
		issued: issued.UnixNano(), expires: issued.Add(sessionLifetime).UnixNano()}
	return capability, nil
}

// Activate consumes one pending capability and returns an opaque active lease
// whose context owns all descendant Application work.
func (broker *Broker) Activate(parent context.Context, capability, principal [32]byte, surface Surface) (*ActiveSession, Receipt, error) {
	if parent == nil {
		return nil, Receipt{}, errors.New("active Application session has no parent context")
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	pending, ok := broker.sessions[capability]
	if ok {
		delete(broker.sessions, capability)
	}
	grant, granted := broker.grants[surface]
	if !ok || capability == [32]byte{} || pending.principal != principal || pending.surface != surface ||
		broker.clock().UnixNano() > pending.expires || broker.closed || broker.draining[surface] || !granted || grant.Principal != principal {
		return nil, Receipt{}, errors.New("ephemeral session is absent, replayed, or bound to another principal")
	}
	ctx, cancel := context.WithCancel(parent)
	if err := ctx.Err(); err != nil {
		cancel()
		return nil, Receipt{}, errors.New("active Application session parent is already cancelled")
	}
	state := &activeSession{principal: principal, surface: surface, expires: pending.expires, cancel: cancel}
	broker.active[capability] = state
	remaining := time.Duration(pending.expires - broker.clock().UnixNano())
	if remaining <= 0 {
		broker.cancelActiveLocked(capability)
		return nil, Receipt{}, errors.New("ephemeral session expired before activation")
	}
	state.timer = time.AfterFunc(remaining, func() { broker.expireActive(capability) })
	receipt := Receipt{Session: commitment("session", capability), Principal: commitment("principal", principal),
		Broker: commitment("broker", broker.id), Grant: grantCommitment(broker.id, principal, surface),
		Surface: surface, IssuedAt: pending.issued, ExpiresAt: pending.expires}
	return &ActiveSession{broker: broker, id: capability, ctx: ctx}, receipt, nil
}

// Consume invalidates one capability before returning its opaque receipt.
func (broker *Broker) Consume(capability, principal [32]byte, surface Surface) (Receipt, error) {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	session, ok := broker.sessions[capability]
	if ok {
		delete(broker.sessions, capability)
	}
	if !ok || capability == [32]byte{} || surface == Connection || session.principal != principal || session.surface != surface ||
		broker.clock().UnixNano() > session.expires {
		return Receipt{}, errors.New("ephemeral session is absent, replayed, or bound to another principal")
	}
	return Receipt{Session: commitment("session", capability), Principal: commitment("principal", principal),
		Broker: commitment("broker", broker.id), Grant: grantCommitment(broker.id, principal, surface),
		Surface: surface, IssuedAt: session.issued, ExpiresAt: session.expires}, nil
}

// Revoke removes one exact grant, invalidates its outstanding capabilities,
// and cancels every matching active Connection session.
func (broker *Broker) Revoke(principal [32]byte, surface Surface) error {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	grant, ok := broker.grants[surface]
	if !ok || grant.Principal != principal {
		return errors.New("application Principal does not match Local Grant")
	}
	delete(broker.grants, surface)
	for capability, session := range broker.sessions {
		if session.surface == surface && session.principal == principal {
			delete(broker.sessions, capability)
		}
	}
	for id, session := range broker.active {
		if session.surface == surface && session.principal == principal {
			broker.cancelActiveLocked(id)
		}
	}
	return nil
}

// Active returns the current admission pressure without exposing capabilities.
func (broker *Broker) Active() uint32 {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	return uint32(len(broker.sessions) + len(broker.active))
}

// Drain refuses new admission and invalidates outstanding capabilities for one
// grant only when that preselected grant permits finite drain.
func (broker *Broker) Drain(surface Surface) error {
	return errors.New("local Grant drain requires a finite terminal deadline")
}

// DrainUntil refuses new admission and lets already active work live only to
// the supplied finite boundary, and only for a pre-permitted Grant.
func (broker *Broker) DrainUntil(surface Surface, deadline time.Time) error {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	grant, ok := broker.grants[surface]
	if !ok || !grant.PermitDrain || deadline.IsZero() {
		return errors.New("local Grant does not permit finite drain")
	}
	broker.draining[surface] = true
	for capability, session := range broker.sessions {
		if session.surface == surface {
			delete(broker.sessions, capability)
		}
	}
	now := broker.clock()
	for id, session := range broker.active {
		if session.surface != surface {
			continue
		}
		boundary := deadline
		expires := time.Unix(0, session.expires)
		if expires.Before(boundary) {
			boundary = expires
		}
		if !now.Before(boundary) {
			broker.cancelActiveLocked(id)
			continue
		}
		if session.timer != nil {
			session.timer.Stop()
		}
		session.timer = time.AfterFunc(boundary.Sub(now), func() { broker.expireActive(id) })
	}
	return nil
}

// Close atomically refuses all new admission, invalidates every unconsumed
// capability, and cancels all active Connection sessions.
func (broker *Broker) Close() {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	broker.closed = true
	clear(broker.sessions)
	for id := range broker.active {
		broker.cancelActiveLocked(id)
	}
}

func (broker *Broker) expireActive(id [32]byte) {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	broker.cancelActiveLocked(id)
}

func (broker *Broker) releaseActive(id [32]byte) {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	broker.cancelActiveLocked(id)
}

func (broker *Broker) cancelActiveLocked(id [32]byte) {
	session, ok := broker.active[id]
	if !ok {
		return
	}
	delete(broker.active, id)
	if session.timer != nil {
		session.timer.Stop()
	}
	session.cancel()
}

// Isolation reports the only M10 isolation state selected by R-085.
func (broker *Broker) Isolation() IsolationObservation {
	return IsolationObservation{state: GenericUnqualified}
}

func commitment(kind string, value [32]byte) [32]byte {
	return sha256.Sum256(append([]byte("ardents-application-broker-"+kind+"-v1\x00"), value[:]...))
}

func grantCommitment(id, principal [32]byte, surface Surface) [32]byte {
	value := append([]byte("ardents-application-broker-grant-v1\x00"), id[:]...)
	value = append(value, principal[:]...)
	value = append(value, surface...)
	return sha256.Sum256(value)
}
