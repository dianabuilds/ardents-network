//go:build ignore

package main

import "fmt"

// SIState is the lifecycle state of a Service Instance tracked by
// the S3.6 in-memory credential store. The numeric values are
// stable across runs (the observer and the test cases compare
// them by value) and follow the production lifecycle order:
//
//	0 Uninitialized -> 1 Initialized -> 2 Accepted -> 3 Headless ->
//	4 Open -> 5 Published -> 6 Withdrawn
//
// The numeric gaps are intentional: future states can be inserted
// without renumbering existing values.
type SIState int

const (
	SIUninitialized SIState = 0
	SIInitialized   SIState = 1
	SIAccepted      SIState = 2
	SIHeadless      SIState = 3
	SIOpen          SIState = 4
	SIPublished     SIState = 5
	SIWithdrawn     SIState = 6
)

// String returns the lowercase wire name for the state.
func (s SIState) String() string {
	switch s {
	case SIUninitialized:
		return "uninitialized"
	case SIInitialized:
		return "initialized"
	case SIAccepted:
		return "accepted"
	case SIHeadless:
		return "headless"
	case SIOpen:
		return "open"
	case SIPublished:
		return "published"
	case SIWithdrawn:
		return "withdrawn"
	}
	return fmt.Sprintf("unknown(%d)", int(s))
}

// UserAction is one user-emitted action. It is the wire shape
// the UserActor writes to user_actions.jsonl and the shape the
// Observer's user_impossible_action wire checks. Args is a
// free-form key/value map (si_id, etc.) opaque to the observer.
type UserAction struct {
	PersonaID    string            `json:"persona_id"`
	Verb         string            `json:"verb"`
	Args         map[string]string `json:"args,omitempty"`
	IsImpossible bool              `json:"is_impossible"`
}

// CredentialSnapshot is the read-only view of the store returned
// by Snapshot. The maps are deep copies, safe to retain.
type CredentialSnapshot struct {
	Personas map[string]PersonaSnapshot `json:"personas"`
}

// PersonaSnapshot is one persona's state at snapshot time.
type PersonaSnapshot struct {
	OwnedServiceInstances map[string]SIState `json:"owned_service_instances"`
	RetryCount            int                `json:"retry_count"`
	LastActionTick        int                `json:"last_action_tick"`
}

// CredentialStore is the in-memory store the UserActor reads and
// writes. It is single-threaded: the tick loop calls
// UserActor.Tick -> persona.NextAction -> store on a single
// goroutine. The S3.6 contract is explicit that thread-safety
// is not required.
type CredentialStore struct {
	personas map[string]*personaState
}
type personaState struct {
	ownedServiceInstances map[string]SIState
	retryCount            int
	lastActionTick        int
}

// NewCredentialStore returns an empty store. The three personas
// are NOT pre-registered; the UserActor registers them on first
// use via RecordAction.
func NewCredentialStore() *CredentialStore {
	return &CredentialStore{personas: make(map[string]*personaState)}
}

func (c *CredentialStore) getOrCreate(personaID string) *personaState {
	p, ok := c.personas[personaID]
	if !ok {
		p = &personaState{
			ownedServiceInstances: make(map[string]SIState),
			lastActionTick:        -1000,
		}
		c.personas[personaID] = p
	}
	return p
}

// AllocateSI registers a new SI for the persona, in the
// Uninitialized state. The SI id is globally unique across all
// personas; the second call with the same siID (by any persona)
// returns an error so the credential_store_isolation test case
// can prove isolation.
func (c *CredentialStore) AllocateSI(personaID, siID string) error {
	if personaID == "" || siID == "" {
		return fmt.Errorf("sim-driver: credential-store allocate-si empty persona or si id")
	}
	for otherID, other := range c.personas {
		if _, exists := other.ownedServiceInstances[siID]; exists {
			return fmt.Errorf("SI id %q already allocated to another persona (owner=%q)", siID, otherID)
		}
	}
	p := c.getOrCreate(personaID)
	p.ownedServiceInstances[siID] = SIUninitialized
	return nil
}

// TransitionSI moves the persona's SI from its current state to
// the next state implied by verb. The transition is rejected if
// the verb is not legal from the current state; the call returns
// an error and the state is unchanged. Legal transitions:
// Uninitialized->Initialized->Accepted->Headless->Open->Published->Withdrawn.
func (c *CredentialStore) TransitionSI(personaID, siID, verb string) error {
	p, ok := c.personas[personaID]
	if !ok {
		return fmt.Errorf("sim-driver: credential-store transition-si unknown persona %q", personaID)
	}
	state, exists := p.ownedServiceInstances[siID]
	if !exists {
		return fmt.Errorf("sim-driver: credential-store transition-si persona %q does not own SI %q", personaID, siID)
	}
	next, ok := nextStateForVerb(verb, state)
	if !ok {
		return fmt.Errorf("sim-driver: credential-store transition-si verb %q is not legal from state %s for SI %q", verb, state, siID)
	}
	p.ownedServiceInstances[siID] = next
	return nil
}

// RecordAction updates the persona's last_action_tick and
// increments retry_count. Called by the UserActor after a
// successful (non-empty) NextAction.
func (c *CredentialStore) RecordAction(personaID string, tick int) {
	p := c.getOrCreate(personaID)
	p.lastActionTick = tick
	p.retryCount++
}

// LastActionTick returns the persona's last action tick, or the
// initial sentinel -1000 if the persona has never acted.
func (c *CredentialStore) LastActionTick(personaID string) int {
	p, ok := c.personas[personaID]
	if !ok {
		return -1000
	}
	return p.lastActionTick
}

// Snapshot returns a deep copy of the store's state.
func (c *CredentialStore) Snapshot() CredentialSnapshot {
	snap := CredentialSnapshot{Personas: make(map[string]PersonaSnapshot, len(c.personas))}
	for id, p := range c.personas {
		si := make(map[string]SIState, len(p.ownedServiceInstances))
		for k, v := range p.ownedServiceInstances {
			si[k] = v
		}
		snap.Personas[id] = PersonaSnapshot{
			OwnedServiceInstances: si,
			RetryCount:            p.retryCount,
			LastActionTick:        p.lastActionTick,
		}
	}
	return snap
}

func nextStateForVerb(verb string, from SIState) (SIState, bool) {
	switch verb {
	case "service_instance_initialize":
		if from == SIUninitialized {
			return SIInitialized, true
		}
	case "service_instance_accept":
		if from == SIInitialized {
			return SIAccepted, true
		}
	case "endpoint_headless_start":
		if from == SIAccepted {
			return SIHeadless, true
		}
	case "endpoint_open":
		if from == SIHeadless {
			return SIOpen, true
		}
	case "endpoint_publish":
		if from == SIOpen {
			return SIPublished, true
		}
	case "endpoint_withdraw":
		if from == SIPublished {
			return SIWithdrawn, true
		}
	}
	return from, false
}
