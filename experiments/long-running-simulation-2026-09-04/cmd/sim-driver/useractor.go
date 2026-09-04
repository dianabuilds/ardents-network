//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

// persona is the common interface UserActor uses to invoke
// whichever persona was selected. All three persona types
// (honest, confused, impatient) satisfy it.
type persona interface {
	ID() string
	CooldownTicks() int
	NextAction(tick int, store *CredentialStore) UserAction
}

// UserActor is the S3.6 user simulation role. On each tick
// it picks one persona at random (equal weights by default)
// and invokes that persona's NextAction. The returned action
// is appended to the tick record (TickState.UserActions) and
// written to user_actions.jsonl under the evidence dir.
//
// The actor owns a credential store; the store is the
// authoritative state for "which persona owns which SI" and
// is shared across all three personas. The actor is the
// only caller of RecordAction on the store; the personas
// read and write the store directly during NextAction.
type UserActor struct {
	honest    *honestPersona
	confused  *confusedPersona
	impatient *impatientPersona
	rng       *rand.Rand
}

// NewUserActor constructs the three personas and a shared
// RNG. The RNG is seeded from the system clock so successive
// runs produce different confused-persona impossible
// sequences; reproducibility is not a S3.6 requirement.
func NewUserActor() *UserActor {
	seed := time.Now().UnixNano()
	rng := rand.New(rand.NewSource(seed))
	return &UserActor{
		honest:    newHonestPersona("persona-honest-1"),
		confused:  newConfusedPersona("persona-confused-1", rng),
		impatient: newImpatientPersona("persona-impatient-1"),
		rng:       rng,
	}
}

// pickPersona selects one of the three personas with equal
// probability. Exposed (lowercase) for the test cases that
// need to pin a specific persona; the S3.6 self-test calls
// the persona's NextAction directly, not pickPersona.
func (ua *UserActor) pickPersona() persona {
	r := ua.rng.Float64()
	switch {
	case r < 1.0/3.0:
		return ua.honest
	case r < 2.0/3.0:
		return ua.confused
	default:
		return ua.impatient
	}
}

// Tick runs one tick of the user simulation. It picks one
// persona, checks the persona's cooldown against the
// store's last_action_tick, and if the cooldown is
// satisfied invokes NextAction. If the persona returns a
// non-empty action, the action is recorded in the store
// (RecordAction) and returned to the caller. If the
// cooldown is not satisfied or the persona returns an
// empty action, the returned slice is nil.
func (ua *UserActor) Tick(tick int, store *CredentialStore) []UserAction {
	p := ua.pickPersona()
	cooldown := p.CooldownTicks()
	lastTick := store.LastActionTick(p.ID())
	if tick-lastTick < cooldown {
		return nil
	}
	action := p.NextAction(tick, store)
	if action.Verb == "" {
		return nil
	}
	store.RecordAction(p.ID(), tick)
	return []UserAction{action}
}

// WriteUserActions appends one tick's user actions to
// <evidence>/user_actions.jsonl. Each action is one JSON
// line. The function is a no-op if actions is empty.
//
// The function appends to the file (does not truncate) so
// the per-tick writes accumulate. The file is created on
// the first non-empty call. The started_at and finished_at
// fields are both set to the current UTC time because S3.6
// user actions are instantaneous (no real CLI call); S3.7
// will record a real interval when the actor shells out
// to the ardents CLI.
func WriteUserActions(evidenceDir string, tick int, actions []UserAction) error {
	if len(actions) == 0 {
		return nil
	}
	path := filepath.Join(evidenceDir, "user_actions.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("sim-driver: open user_actions.jsonl: %w", err)
	}
	defer f.Close()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, action := range actions {
		record := map[string]any{
			"tick":          tick,
			"persona_id":    action.PersonaID,
			"verb":          action.Verb,
			"args":          action.Args,
			"is_impossible": action.IsImpossible,
			"started_at":    now,
			"finished_at":   now,
		}
		line, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("sim-driver: marshal user action: %w", err)
		}
		if _, err := f.Write(append(line, '\n')); err != nil {
			return fmt.Errorf("sim-driver: write user action: %w", err)
		}
	}
	return nil
}
