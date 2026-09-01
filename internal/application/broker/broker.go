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

// Broker owns one process-local Local Grant capability/active-session tree.
// Restarting it drops both; it never authenticates a platform process or
// isolation claim.
type Broker struct {
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
	capacity := maximumConnectionAdmissionLoad + maximumAdministrationCapabilities
	return &Broker{id: config.ID, grants: grants, capabilities: make(map[[32]byte]pendingCapability, capacity),
		active: make(map[[32]byte]*activeSession, maximumConnectionAdmissionLoad), draining: make(map[Surface]bool), clock: config.Clock}, nil
}

// Admit issues one short-lived, one-use capability for an exact Principal and
// granted surface.
func (broker *Broker) Admit(principal [32]byte, surface Surface) ([32]byte, error) {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	issued := broker.clock()
	broker.pruneExpiredCapabilitiesLocked(issued)
	if broker.closed {
		return [32]byte{}, errors.New("broker is draining")
	}
	grant, ok := broker.grants[surface]
	if !ok || broker.draining[surface] || grant.Principal != principal {
		return [32]byte{}, errors.New("application Principal does not match Local Grant")
	}
	if broker.surfaceAdmissionLoadLocked(surface) >= admissionCapacityFor(surface) {
		return [32]byte{}, errors.New("capability budget exhausted")
	}
	var capability [32]byte
	if _, err := rand.Read(capability[:]); err != nil || capability == [32]byte{} {
		return [32]byte{}, errors.New("fresh local capability could not be created")
	}
	broker.capabilities[capability] = pendingCapability{principal: principal, surface: surface,
		issued: issued.UnixNano(), expires: issued.Add(capabilityLifetime).UnixNano()}
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
	pending, ok := broker.capabilities[capability]
	if ok {
		delete(broker.capabilities, capability)
	}
	grant, granted := broker.grants[surface]
	if !ok || capability == [32]byte{} || pending.principal != principal || pending.surface != surface ||
		broker.clock().UnixNano() > pending.expires || broker.closed || broker.draining[surface] || !granted || grant.Principal != principal {
		return nil, Receipt{}, errors.New("ephemeral capability is absent, replayed, or bound to another principal")
	}
	ctx, cancel := context.WithCancel(parent)
	if err := ctx.Err(); err != nil {
		cancel()
		return nil, Receipt{}, errors.New("active Application session parent is already cancelled")
	}
	state := &activeSession{principal: principal, surface: surface, cancel: cancel}
	broker.active[capability] = state
	receipt := Receipt{Session: commitment("session", capability), Principal: commitment("principal", principal),
		Broker: commitment("broker", broker.id), Grant: grantCommitment(broker.id, principal, surface),
		Surface: surface, IssuedAt: pending.issued, ExpiresAt: pending.expires}
	return &ActiveSession{broker: broker, id: capability, ctx: ctx}, receipt, nil
}

// Consume invalidates one capability before returning its opaque receipt.
func (broker *Broker) Consume(capability, principal [32]byte, surface Surface) (Receipt, error) {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	pending, ok := broker.capabilities[capability]
	if ok {
		delete(broker.capabilities, capability)
	}
	if !ok || capability == [32]byte{} || surface == Connection || pending.principal != principal || pending.surface != surface ||
		broker.clock().UnixNano() > pending.expires {
		return Receipt{}, errors.New("ephemeral capability is absent, replayed, or bound to another principal")
	}
	return Receipt{Session: commitment("session", capability), Principal: commitment("principal", principal),
		Broker: commitment("broker", broker.id), Grant: grantCommitment(broker.id, principal, surface),
		Surface: surface, IssuedAt: pending.issued, ExpiresAt: pending.expires}, nil
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
	for capability, pending := range broker.capabilities {
		if pending.surface == surface && pending.principal == principal {
			delete(broker.capabilities, capability)
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
	broker.pruneExpiredCapabilitiesLocked(broker.clock())
	return uint32(len(broker.capabilities) + len(broker.active))
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
	for capability, pending := range broker.capabilities {
		if pending.surface == surface {
			delete(broker.capabilities, capability)
		}
	}
	now := broker.clock()
	for id, session := range broker.active {
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
			broker.cancelActiveLocked(id)
			continue
		}
		if session.timer != nil {
			session.timer.Stop()
		}
		session.deadline = boundary.UnixNano()
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
	clear(broker.capabilities)
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

func (broker *Broker) surfaceAdmissionLoadLocked(surface Surface) int {
	count := 0
	for _, pending := range broker.capabilities {
		if pending.surface == surface {
			count++
		}
	}
	for _, active := range broker.active {
		if active.surface == surface {
			count++
		}
	}
	return count
}

func (broker *Broker) pruneExpiredCapabilitiesLocked(now time.Time) {
	for capability, pending := range broker.capabilities {
		if now.UnixNano() > pending.expires {
			delete(broker.capabilities, capability)
		}
	}
}

func admissionCapacityFor(surface Surface) int {
	if surface == Connection {
		return maximumConnectionAdmissionLoad
	}
	return maximumAdministrationCapabilities
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
