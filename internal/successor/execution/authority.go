package execution

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"sync"
	"time"
)

const (
	maximumConnectionAdmissionLoad    = 64
	maximumAdministrationCapabilities = 6
	capabilityLifetime                = 15 * time.Second
)

type pendingCapability struct {
	principal [32]byte
	surface   Surface
	issued    int64
	expires   int64
}

type activeSession struct {
	principal [32]byte
	surface   Surface
	deadline  int64
	cancel    context.CancelFunc
	timer     *time.Timer
}

// ActiveSession is an opaque lease over one consumed local Application
// session. It exposes only cancellation and idempotent terminal release.
type ActiveSession struct {
	authority *Authority
	id        [32]byte
	ctx       context.Context
	release   sync.Once
}

// Context is cancelled when the caller, exact Grant, drain deadline, or Authority
// ends the active session.
func (session *ActiveSession) Context() context.Context {
	if session == nil || session.ctx == nil {
		return context.Background()
	}
	return session.ctx
}

// Release returns the active session budget exactly once.
func (session *ActiveSession) Release() {
	if session == nil || session.authority == nil {
		return
	}
	session.release.Do(func() { session.authority.releaseActive(session.id) })
}

// Authority owns one process-local Local Grant capability/active-session tree.
// Restarting it drops both; it never authenticates a platform process or
// isolation claim.
type Authority struct {
	mu           sync.Mutex
	id           [32]byte
	grants       map[Surface]Grant
	capabilities map[[32]byte]pendingCapability
	active       map[[32]byte]*activeSession
	draining     map[Surface]bool
	clock        func() time.Time
	closed       bool
}

// New validates one finite local grant set.
func New(config Config) (*Authority, error) {
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
	capacity := maximumConnectionAdmissionLoad + maximumAdministrationCapabilities
	return &Authority{id: config.ID, grants: grants, capabilities: make(map[[32]byte]pendingCapability, capacity),
		active: make(map[[32]byte]*activeSession, maximumConnectionAdmissionLoad), draining: make(map[Surface]bool), clock: config.Clock}, nil
}

// Admit issues one short-lived, one-use capability for an exact Principal and
// granted surface.
func (authority *Authority) Admit(principal [32]byte, surface Surface) ([32]byte, error) {
	authority.mu.Lock()
	defer authority.mu.Unlock()
	issued := authority.clock()
	authority.pruneExpiredCapabilitiesLocked(issued)
	if authority.closed {
		return [32]byte{}, errors.New("broker is draining")
	}
	grant, ok := authority.grants[surface]
	if !ok || authority.draining[surface] || grant.Principal != principal {
		return [32]byte{}, errors.New("application Principal does not match Local Grant")
	}
	if authority.surfaceAdmissionLoadLocked(surface) >= admissionCapacityFor(surface) {
		return [32]byte{}, errors.New("capability budget exhausted")
	}
	var capability [32]byte
	if _, err := rand.Read(capability[:]); err != nil || capability == [32]byte{} {
		return [32]byte{}, errors.New("fresh local capability could not be created")
	}
	authority.capabilities[capability] = pendingCapability{principal: principal, surface: surface,
		issued: issued.UnixNano(), expires: issued.Add(capabilityLifetime).UnixNano()}
	return capability, nil
}

// Activate consumes one pending capability and returns an opaque active lease
// whose context owns all descendant Application work.
func (authority *Authority) Activate(parent context.Context, capability, principal [32]byte, surface Surface) (*ActiveSession, Receipt, error) {
	if parent == nil {
		return nil, Receipt{}, errors.New("active Application session has no parent context")
	}
	authority.mu.Lock()
	defer authority.mu.Unlock()
	pending, ok := authority.capabilities[capability]
	if ok {
		delete(authority.capabilities, capability)
	}
	grant, granted := authority.grants[surface]
	if !ok || capability == [32]byte{} || pending.principal != principal || pending.surface != surface ||
		authority.clock().UnixNano() > pending.expires || authority.closed || authority.draining[surface] || !granted || grant.Principal != principal {
		return nil, Receipt{}, errors.New("ephemeral capability is absent, replayed, or bound to another principal")
	}
	ctx, cancel := context.WithCancel(parent)
	if err := ctx.Err(); err != nil {
		cancel()
		return nil, Receipt{}, errors.New("active Application session parent is already cancelled")
	}
	state := &activeSession{principal: principal, surface: surface, cancel: cancel}
	authority.active[capability] = state
	receipt := Receipt{Session: commitment("session", capability), Principal: commitment("principal", principal),
		Authority: commitment("broker", authority.id), Grant: grantCommitment(authority.id, principal, surface),
		Surface: surface, IssuedAt: pending.issued, ExpiresAt: pending.expires}
	return &ActiveSession{authority: authority, id: capability, ctx: ctx}, receipt, nil
}

// Consume invalidates one capability before returning its opaque receipt.
func (authority *Authority) Consume(capability, principal [32]byte, surface Surface) (Receipt, error) {
	authority.mu.Lock()
	defer authority.mu.Unlock()
	pending, ok := authority.capabilities[capability]
	if ok {
		delete(authority.capabilities, capability)
	}
	if !ok || capability == [32]byte{} || surface == Connection || pending.principal != principal || pending.surface != surface ||
		authority.clock().UnixNano() > pending.expires {
		return Receipt{}, errors.New("ephemeral capability is absent, replayed, or bound to another principal")
	}
	return Receipt{Session: commitment("session", capability), Principal: commitment("principal", principal),
		Authority: commitment("broker", authority.id), Grant: grantCommitment(authority.id, principal, surface),
		Surface: surface, IssuedAt: pending.issued, ExpiresAt: pending.expires}, nil
}

// Revoke removes one exact grant, invalidates its outstanding capabilities,
// and cancels every matching active Connection session.
func (authority *Authority) Revoke(principal [32]byte, surface Surface) error {
	authority.mu.Lock()
	defer authority.mu.Unlock()
	grant, ok := authority.grants[surface]
	if !ok || grant.Principal != principal {
		return errors.New("application Principal does not match Local Grant")
	}
	delete(authority.grants, surface)
	for capability, pending := range authority.capabilities {
		if pending.surface == surface && pending.principal == principal {
			delete(authority.capabilities, capability)
		}
	}
	for id, session := range authority.active {
		if session.surface == surface && session.principal == principal {
			authority.cancelActiveLocked(id)
		}
	}
	return nil
}

// Active returns the current admission pressure without exposing capabilities.
func (authority *Authority) Active() uint32 {
	authority.mu.Lock()
	defer authority.mu.Unlock()
	authority.pruneExpiredCapabilitiesLocked(authority.clock())
	return uint32(len(authority.capabilities) + len(authority.active))
}

// Drain refuses new admission and invalidates outstanding capabilities for one
// grant only when that preselected grant permits finite drain.
func (authority *Authority) Drain(surface Surface) error {
	return errors.New("local Grant drain requires a finite terminal deadline")
}

// DrainUntil refuses new admission and lets already active work live only to
// the supplied finite boundary, and only for a pre-permitted Grant.
func (authority *Authority) DrainUntil(surface Surface, deadline time.Time) error {
	authority.mu.Lock()
	defer authority.mu.Unlock()
	grant, ok := authority.grants[surface]
	if !ok || !grant.PermitDrain || deadline.IsZero() {
		return errors.New("local Grant does not permit finite drain")
	}
	authority.draining[surface] = true
	for capability, pending := range authority.capabilities {
		if pending.surface == surface {
			delete(authority.capabilities, capability)
		}
	}
	now := authority.clock()
	for id, session := range authority.active {
		if session.surface != surface {
			continue
		}
		boundary := deadline.UTC()
		if session.deadline != 0 {
			current := time.Unix(0, session.deadline)
			if current.Before(boundary) {
				boundary = current
			}
		}
		if !now.Before(boundary) {
			authority.cancelActiveLocked(id)
			continue
		}
		if session.timer != nil {
			session.timer.Stop()
		}
		session.deadline = boundary.UnixNano()
		session.timer = time.AfterFunc(boundary.Sub(now), func() { authority.expireActive(id) })
	}
	return nil
}

// Close atomically refuses all new admission, invalidates every unconsumed
// capability, and cancels all active Connection sessions.
func (authority *Authority) Close() {
	authority.mu.Lock()
	defer authority.mu.Unlock()
	authority.closed = true
	clear(authority.capabilities)
	for id := range authority.active {
		authority.cancelActiveLocked(id)
	}
}

func (authority *Authority) expireActive(id [32]byte) {
	authority.mu.Lock()
	defer authority.mu.Unlock()
	authority.cancelActiveLocked(id)
}

func (authority *Authority) releaseActive(id [32]byte) {
	authority.mu.Lock()
	defer authority.mu.Unlock()
	authority.cancelActiveLocked(id)
}

func (authority *Authority) cancelActiveLocked(id [32]byte) {
	session, ok := authority.active[id]
	if !ok {
		return
	}
	delete(authority.active, id)
	if session.timer != nil {
		session.timer.Stop()
	}
	session.cancel()
}

func (authority *Authority) surfaceAdmissionLoadLocked(surface Surface) int {
	count := 0
	for _, pending := range authority.capabilities {
		if pending.surface == surface {
			count++
		}
	}
	for _, active := range authority.active {
		if active.surface == surface {
			count++
		}
	}
	return count
}

func (authority *Authority) pruneExpiredCapabilitiesLocked(now time.Time) {
	for capability, pending := range authority.capabilities {
		if now.UnixNano() > pending.expires {
			delete(authority.capabilities, capability)
		}
	}
}

func admissionCapacityFor(surface Surface) int {
	if surface == Connection {
		return maximumConnectionAdmissionLoad
	}
	return maximumAdministrationCapabilities
}

// Isolation reports the generic local isolation state.
func (authority *Authority) Isolation() IsolationObservation {
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
