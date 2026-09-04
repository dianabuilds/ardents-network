//go:build ignore

package main

import (
	"fmt"
	"math/rand"
)

// Verb constants for the S3.6 user simulation. The verbs are
// the mock-state transition verbs; the real `ardents` CLI
// verbs come in S3.7.
const (
	VerbServiceInstanceInitialize = "service_instance_initialize"
	VerbServiceInstanceAccept     = "service_instance_accept"
	VerbEndpointHeadlessStart     = "endpoint_headless_start"
	VerbEndpointOpen              = "endpoint_open"
	VerbEndpointPublish           = "endpoint_publish"
	VerbEndpointWithdraw          = "endpoint_withdraw"
)

// lifecycleVerbs is the canonical 4-step honest lifecycle. The
// sequence is init -> accept -> headless -> open. The full
// 6-verb lifecycle (init -> accept -> headless -> open ->
// publish -> withdraw) is what the production CLI will drive
// in S3.7; S3.6 stops at open so the 4-step self-test has a
// clean assertion target and the credential store's state
// machine has a small, auditable surface.
var lifecycleVerbs = []string{
	VerbServiceInstanceInitialize,
	VerbServiceInstanceAccept,
	VerbEndpointHeadlessStart,
	VerbEndpointOpen,
}

// honestPersona emits the canonical 4-step lifecycle. The
// 10-tick cooldown is enforced by the UserActor (not the
// persona itself): the persona always emits when called, and
// the UserActor skips the call when the cooldown is not
// satisfied. The self-test calls NextAction directly and so
// bypasses the cooldown; this matches the AC3
// honest_lifecycle_4_steps test which calls NextAction four
// times at the same tick.
type honestPersona struct {
	id              string
	currentSINumber int
	currentStep     int
}

func newHonestPersona(id string) *honestPersona {
	return &honestPersona{id: id}
}

func (h *honestPersona) ID() string { return h.id }

// CooldownTicks returns the honest persona's cooldown in
// ticks. The UserActor reads this to decide whether to call
// NextAction; the self-test does not consult it.
func (h *honestPersona) CooldownTicks() int { return 10 }

// NextAction advances the persona's step counter and returns
// the verb for the current step. When the step counter wraps
// from 3 back to 0, a new SI is allocated for the persona.
// The store is updated synchronously: AllocateSI on step 0,
// TransitionSI on every step. Errors from the store are
// ignored because the only way to hit them in S3.6 is a
// double-allocate, which the store's own isolation check
// prevents.
func (h *honestPersona) NextAction(tick int, store *CredentialStore) UserAction {
	verb := lifecycleVerbs[h.currentStep]
	siID := fmt.Sprintf("si-%s-%03d", h.id, h.currentSINumber)
	if h.currentStep == 0 {
		_ = store.AllocateSI(h.id, siID)
	}
	_ = store.TransitionSI(h.id, siID, verb)
	h.currentStep++
	if h.currentStep >= len(lifecycleVerbs) {
		h.currentStep = 0
		h.currentSINumber++
	}
	return UserAction{
		PersonaID: h.id,
		Verb:      verb,
		Args:      map[string]string{"si_id": siID},
	}
}

// confusedPersona is like honest but with a configurable
// probability of returning an "impossible" action. The
// impossible action is one of: publish before open, withdraw
// without publish, open without headless. The impossible
// action's IsImpossible flag is set so the Observer's
// user_impossible_action wire fires. The impossible action
// does NOT advance the step counter and does NOT transition
// the SI in the store, so the next honest call from this
// persona sees the same step.
type confusedPersona struct {
	id              string
	rng             *rand.Rand
	impossibleRate  float64
	currentSINumber int
	currentStep     int
}

func newConfusedPersona(id string, rng *rand.Rand) *confusedPersona {
	return &confusedPersona{id: id, rng: rng, impossibleRate: 0.05}
}

func (c *confusedPersona) ID() string { return c.id }

// CooldownTicks returns the confused persona's cooldown in
// ticks. Same as honest: 10 ticks, enforced by the UserActor.
func (c *confusedPersona) CooldownTicks() int { return 10 }

// NextAction returns either the next honest verb or, with
// probability impossibleRate, an impossible verb. The
// impossible verb does NOT advance the step counter; the
// next honest call sees the same step.
func (c *confusedPersona) NextAction(tick int, store *CredentialStore) UserAction {
	if c.rng.Float64() < c.impossibleRate {
		impossible := []string{VerbEndpointPublish, VerbEndpointWithdraw, VerbEndpointOpen}
		verb := impossible[c.rng.Intn(len(impossible))]
		siID := fmt.Sprintf("si-%s-%03d", c.id, c.currentSINumber)
		return UserAction{
			PersonaID:    c.id,
			Verb:         verb,
			Args:         map[string]string{"si_id": siID},
			IsImpossible: true,
		}
	}
	verb := lifecycleVerbs[c.currentStep]
	siID := fmt.Sprintf("si-%s-%03d", c.id, c.currentSINumber)
	if c.currentStep == 0 {
		_ = store.AllocateSI(c.id, siID)
	}
	_ = store.TransitionSI(c.id, siID, verb)
	c.currentStep++
	if c.currentStep >= len(lifecycleVerbs) {
		c.currentStep = 0
		c.currentSINumber++
	}
	return UserAction{
		PersonaID: c.id,
		Verb:      verb,
		Args:      map[string]string{"si_id": siID},
	}
}

// impatientPersona is like honest but with no cooldown. The
// UserActor always calls NextAction; the persona always
// emits. Used to exercise the 1-action-per-tick rate under
// the S3.6 falsification criterion and the impatient AC3
// self-test case.
type impatientPersona struct {
	id              string
	currentSINumber int
	currentStep     int
}

func newImpatientPersona(id string) *impatientPersona {
	return &impatientPersona{id: id}
}

func (i *impatientPersona) ID() string { return i.id }

// CooldownTicks returns 0: the impatient persona has no
// cooldown and the UserActor always calls NextAction.
func (i *impatientPersona) CooldownTicks() int { return 0 }

func (i *impatientPersona) NextAction(tick int, store *CredentialStore) UserAction {
	verb := lifecycleVerbs[i.currentStep]
	siID := fmt.Sprintf("si-%s-%03d", i.id, i.currentSINumber)
	if i.currentStep == 0 {
		_ = store.AllocateSI(i.id, siID)
	}
	_ = store.TransitionSI(i.id, siID, verb)
	i.currentStep++
	if i.currentStep >= len(lifecycleVerbs) {
		i.currentStep = 0
		i.currentSINumber++
	}
	return UserAction{
		PersonaID: i.id,
		Verb:      verb,
		Args:      map[string]string{"si_id": siID},
	}
}
