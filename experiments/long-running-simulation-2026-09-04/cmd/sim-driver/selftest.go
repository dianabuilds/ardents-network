//go:build ignore

package main

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// runSelfTest exercises the four S3.1 trip-wires against synthetic
// inputs in-process. The self-test does NOT spin up Docker or
// invoke ardents refresh-sources; it only validates the trip-wire
// catalog and the observer verdict structure. The Docker-backed
// 100-tick smoke is run via `docker compose --profile build up` and
// is a separate acceptance gate (AC4).
//
// Each trip-wire is tested with TWO inputs: one that should NOT
// trip (the "ok" case) and one that SHOULD trip (the "trip" case).
// Both must produce the expected outcome; a flip on either side
// fails the test. The self-test prints "PASS" per wire and exits
// non-zero on the first failure.
func runSelfTest(ctx context.Context) error {
	_ = ctx
	okGeneration := strings.Repeat("aa", 32)
	driftedGeneration := strings.Repeat("bb", 32)
	parseErrorString := "sim-driver: no source-wave-accepted event in consumer output"
	zeroDuration := time.Duration(0)
	shortDuration := 10 * time.Millisecond
	longDuration := 10 * time.Second

	okAccumulated := AccumulatedState{
		FirstObservedDigest: okGeneration,
		PerTickBudget:       DefaultPerTickBudget,
	}
	emptyAccumulated := AccumulatedState{
		PerTickBudget: DefaultPerTickBudget,
	}

	cases := []struct {
		wire       TripWire
		name       string
		okTick     TickState
		tripTick   TickState
		okAccum    AccumulatedState
		tripAccum  AccumulatedState
		tripReason string
	}{
		{
			wire: TripWire{Name: "generation_drift", Check: GenerationDriftWire},
			name: "generation_drift",
			okTick: TickState{
				Number: 1, Event: SourceWaveEvent{Generation: okGeneration, SourceOutcomes: validOutcomes()},
			},
			okAccum: okAccumulated,
			tripTick: TickState{
				Number: 2, Event: SourceWaveEvent{Generation: driftedGeneration, SourceOutcomes: validOutcomes()},
			},
			tripAccum:  okAccumulated,
			tripReason: "differs from first observed_digest",
		},
		{
			wire: TripWire{Name: "generation_drift", Check: GenerationDriftWire},
			name: "generation_drift_first_tick_records_reference",
			okTick: TickState{
				Number: 1, Event: SourceWaveEvent{Generation: okGeneration, SourceOutcomes: validOutcomes()},
			},
			okAccum:   emptyAccumulated,
			tripTick:  TickState{},
			tripAccum: emptyAccumulated,
		},
		{
			wire: TripWire{Name: "source_exit", Check: SourceExitWire},
			name: "source_exit",
			okTick: TickState{
				Number: 1, ConsumerExitCode: 0, ConsumerError: nil,
				Event: SourceWaveEvent{Generation: okGeneration, SourceOutcomes: validOutcomes()},
			},
			okAccum: emptyAccumulated,
			tripTick: TickState{
				Number: 2, ConsumerExitCode: 2, ConsumerError: nil,
			},
			tripAccum:  emptyAccumulated,
			tripReason: "exit_code=2",
		},
		{
			wire: TripWire{Name: "source_exit", Check: SourceExitWire},
			name: "source_exit_exec_error",
			okTick: TickState{
				Number: 1, ConsumerExitCode: 0,
				Event: SourceWaveEvent{Generation: okGeneration, SourceOutcomes: validOutcomes()},
			},
			okAccum: emptyAccumulated,
			tripTick: TickState{
				Number: 2, ConsumerExitCode: -1,
				ConsumerError: fmt.Errorf("sim-driver: exec failed"),
			},
			tripAccum:  emptyAccumulated,
			tripReason: "exec error",
		},
		{
			wire: TripWire{Name: "consumer_parse_error", Check: ConsumerParseErrorWire},
			name: "consumer_parse_error",
			okTick: TickState{
				Number: 1, Event: SourceWaveEvent{Generation: okGeneration, SourceOutcomes: validOutcomes()},
			},
			okAccum: emptyAccumulated,
			tripTick: TickState{
				Number: 2, ParseError: parseErrorString,
			},
			tripAccum:  emptyAccumulated,
			tripReason: "consumer parse error",
		},
		{
			wire: TripWire{Name: "tick_budget", Check: TickBudgetWire},
			name: "tick_budget",
			okTick: TickState{
				Number: 1, Duration: shortDuration,
				Event: SourceWaveEvent{Generation: okGeneration, SourceOutcomes: validOutcomes()},
			},
			okAccum: AccumulatedState{PerTickBudget: DefaultPerTickBudget},
			tripTick: TickState{
				Number: 2, Duration: longDuration,
			},
			tripAccum:  AccumulatedState{PerTickBudget: DefaultPerTickBudget},
			tripReason: "exceeds per-tick budget",
		},
		{
			wire: TripWire{Name: "tick_budget", Check: TickBudgetWire},
			name: "tick_budget_zero_duration",
			okTick: TickState{
				Number: 3, Duration: zeroDuration,
				Event: SourceWaveEvent{Generation: okGeneration, SourceOutcomes: validOutcomes()},
			},
			okAccum:   AccumulatedState{PerTickBudget: DefaultPerTickBudget},
			tripTick:  TickState{},
			tripAccum: AccumulatedState{PerTickBudget: DefaultPerTickBudget},
		},
	}

	for _, c := range cases {
		okTripped, okReason := c.wire.Check(c.okTick, c.okAccum)
		if okTripped {
			return fmt.Errorf("self-test: %s ok-case tripped unexpectedly: %q", c.name, okReason)
		}
		fmt.Printf("sim-driver: self-test %s ok PASS\n", c.name)
		if c.tripTick.Number == 0 && c.tripReason == "" {
			continue
		}
		tripTripped, tripReason := c.wire.Check(c.tripTick, c.tripAccum)
		if !tripTripped {
			return fmt.Errorf("self-test: %s trip-case did not trip", c.name)
		}
		if c.tripReason != "" && !contains(tripReason, c.tripReason) {
			return fmt.Errorf("self-test: %s trip-case reason=%q, want substring %q",
				c.name, tripReason, c.tripReason)
		}
		fmt.Printf("sim-driver: self-test %s trip PASS\n", c.name)
	}

	if err := runSelfTestObserverRoundTrip(okGeneration); err != nil {
		return err
	}

	if err := runSelfTestUserSimulation(); err != nil {
		return err
	}

	fmt.Println("sim-driver: self-test passed")
	return nil
}

// runSelfTestObserverRoundTrip validates the observer's verdict
// aggregation: a tick with a tripped wire must produce
// accept=false and the tripped wire's reason must surface in
// verdict.reason. A clean tick must produce accept=true and
// record the first observed digest into accumulated.
func runSelfTestObserverRoundTrip(generation string) error {
	accumulated := AccumulatedState{PerTickBudget: DefaultPerTickBudget}
	cleanTick := TickState{
		Number: 1, Duration: 10 * time.Millisecond, ConsumerExitCode: 0,
		Event: SourceWaveEvent{Generation: generation, SourceOutcomes: validOutcomes()},
	}
	verdict, _ := Observe(cleanTick, &accumulated)
	if !verdict.Accept {
		return fmt.Errorf("self-test: observer clean tick should accept, got reason %q", verdict.Reason)
	}
	if accumulated.FirstObservedDigest != generation {
		return fmt.Errorf("self-test: observer clean tick should record first_observed_digest=%q, got %q",
			generation, accumulated.FirstObservedDigest)
	}
	if len(verdict.TripWires) != len(DefaultCatalog()) {
		return fmt.Errorf("self-test: observer verdict has %d wires, want %d", len(verdict.TripWires), len(DefaultCatalog()))
	}

	badTick := TickState{
		Number: 2, Duration: 10 * time.Millisecond, ConsumerExitCode: 3,
	}
	verdict, _ = Observe(badTick, &accumulated)
	if verdict.Accept {
		return fmt.Errorf("self-test: observer bad tick should reject, got accept=true")
	}
	if !contains(verdict.Reason, "source_exit") {
		return fmt.Errorf("self-test: observer bad tick reason=%q, want substring source_exit", verdict.Reason)
	}
	fmt.Println("sim-driver: self-test observer_round_trip PASS")
	return nil
}

// validOutcomes returns the S3.1 single-source outcomes array:
// the first slot is "valid" (the configured source-a) and the
// remaining three are "not-attempted" (no other source is
// configured for the smoke slice).
func validOutcomes() [4]string {
	return [4]string{"valid", "not-attempted", "not-attempted", "not-attempted"}
}

func contains(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	if len(needle) > len(haystack) {
		return false
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// runSelfTestUserSimulation exercises the five S3.6
// user-simulation self-test cases. The test cases are
// independent of the network layer: they use the personas
// and the credential store directly, not the UserActor. The
// observer's user_impossible_action wire is exercised in
// case 4 against a synthetic tick.
//
// Each case prints one PASS line. A failure returns an error
// and the self-test exits non-zero. The cases are:
//
//  1. honest_lifecycle_4_steps    honest persona emits the
//     4-step lifecycle in order
//  2. confused_impossible_rate    confused persona's
//     impossible-action rate is 4-6% over 1000 ticks
//  3. impatient_no_cooldown       impatient persona emits
//     one action per tick for 10 ticks
//  4. user_publish_without_open   user_impossible_action
//     wire fires on a publish-before-open action
//  5. credential_store_isolation  the store rejects two
//     personas owning the same SI id
func runSelfTestUserSimulation() error {
	// Case 1: honest_lifecycle_4_steps. The test sets the
	// store's last_action_tick to -100 so the cooldown is
	// "satisfied" (the persona does not actually check
	// cooldown internally; the UserActor does), then calls
	// NextAction 4 times. Each call advances the step
	// counter and emits the next verb in the lifecycle.
	if err := testHonestLifecycle4Steps(); err != nil {
		return err
	}
	fmt.Println("sim-driver: self-test honest_lifecycle_4_steps PASS")

	// Case 2: confused_impossible_rate. Run 1000 ticks
	// against a fresh confused persona. Count the actions
	// with IsImpossible=true and assert the rate is
	// between 4% and 6%. The test uses a fixed RNG seed
	// for reproducibility.
	if err := testConfusedImpossibleRate(); err != nil {
		return err
	}
	fmt.Println("sim-driver: self-test confused_impossible_rate PASS")

	// Case 3: impatient_no_cooldown. Set the store's
	// last_action_tick to 0 and call NextAction at ticks
	// 1..10. All 10 calls must return a non-empty action
	// (the impatient persona has no cooldown).
	if err := testImpatientNoCooldown(); err != nil {
		return err
	}
	fmt.Println("sim-driver: self-test impatient_no_cooldown PASS")

	// Case 4: user_publish_without_open. Manually
	// construct a UserAction with verb=endpoint_publish
	// and IsImpossible=true, attach it to a tick, and
	// verify the user_impossible_action wire trips.
	if err := testUserPublishWithoutOpen(); err != nil {
		return err
	}
	fmt.Println("sim-driver: self-test user_publish_without_open PASS")

	// Case 5: credential_store_isolation. Allocate the
	// same SI id to two different personas and verify the
	// second call returns an error.
	if err := testCredentialStoreIsolation(); err != nil {
		return err
	}
	fmt.Println("sim-driver: self-test credential_store_isolation PASS")

	return nil
}

// testHonestLifecycle4Steps verifies the honest persona emits
// the 4-step lifecycle in order. The test seeds the store
// with last_action_tick=-100 (cooldown sentinel) and calls
// NextAction 4 times at tick 0. The persona does not check
// cooldown internally; the 4 calls each advance the step
// counter and emit the next verb.
func testHonestLifecycle4Steps() error {
	store := NewCredentialStore()
	store.RecordAction("persona-honest-1", -100)
	persona := newHonestPersona("persona-honest-1")
	expected := []string{
		VerbServiceInstanceInitialize,
		VerbServiceInstanceAccept,
		VerbEndpointHeadlessStart,
		VerbEndpointOpen,
	}
	for i, want := range expected {
		action := persona.NextAction(0, store)
		if action.Verb != want {
			return fmt.Errorf("honest_lifecycle_4_steps: call %d got verb=%q, want %q",
				i+1, action.Verb, want)
		}
		if action.IsImpossible {
			return fmt.Errorf("honest_lifecycle_4_steps: call %d action is impossible, want possible", i+1)
		}
	}
	return nil
}

// testConfusedImpossibleRate runs 1000 ticks against a fresh
// confused persona with a fixed RNG seed and asserts the
// impossible-action rate is between 4% and 6%. The test
// counts actions where IsImpossible=true; non-impossible
// actions are the rest. The rate is the count of impossible
// actions divided by the total number of emitted actions
// (the confused persona emits on every call; there is no
// cooldown in the self-test path).
func testConfusedImpossibleRate() error {
	store := NewCredentialStore()
	rng := rand.New(rand.NewSource(42))
	persona := newConfusedPersona("persona-confused-1", rng)
	var impossible int
	const total = 1000
	for tick := 0; tick < total; tick++ {
		action := persona.NextAction(tick, store)
		if action.IsImpossible {
			impossible++
		}
	}
	rate := float64(impossible) / float64(total)
	if rate < 0.04 || rate > 0.06 {
		return fmt.Errorf("confused_impossible_rate: rate=%.4f (%d/%d), want 0.04-0.06",
			rate, impossible, total)
	}
	return nil
}

// testImpatientNoCooldown calls the impatient persona at
// ticks 1..10 with last_action_tick=0 and asserts every
// call returns a non-empty action. The impatient persona
// has CooldownTicks()=0, so it always emits.
func testImpatientNoCooldown() error {
	store := NewCredentialStore()
	store.RecordAction("persona-impatient-1", 0)
	persona := newImpatientPersona("persona-impatient-1")
	for tick := 1; tick <= 10; tick++ {
		action := persona.NextAction(tick, store)
		if action.Verb == "" {
			return fmt.Errorf("impatient_no_cooldown: tick %d returned empty action", tick)
		}
		if action.IsImpossible {
			return fmt.Errorf("impatient_no_cooldown: tick %d action is impossible, want possible", tick)
		}
	}
	return nil
}

// testUserPublishWithoutOpen constructs a confused action
// where the verb is endpoint_publish but the SI is in the
// Initialized state, and verifies the user_impossible_action
// wire trips. The test does not go through the persona; it
// manually builds the UserAction and the TickState, then
// calls Observe.
func testUserPublishWithoutOpen() error {
	store := NewCredentialStore()
	if err := store.AllocateSI("persona-confused-1", "si-test-001"); err != nil {
		return fmt.Errorf("user_publish_without_open: allocate: %w", err)
	}
	if err := store.TransitionSI("persona-confused-1", "si-test-001", VerbServiceInstanceInitialize); err != nil {
		return fmt.Errorf("user_publish_without_open: transition to initialized: %w", err)
	}
	action := UserAction{
		PersonaID:    "persona-confused-1",
		Verb:         VerbEndpointPublish,
		Args:         map[string]string{"si_id": "si-test-001"},
		IsImpossible: true,
	}
	tick := TickState{
		Number:      1,
		Event:       SourceWaveEvent{Generation: strings.Repeat("aa", 32), SourceOutcomes: validOutcomes()},
		UserActions: []UserAction{action},
	}
	accumulated := AccumulatedState{PerTickBudget: DefaultPerTickBudget}
	tripped, reason := UserImpossibleActionWire(tick, accumulated)
	if !tripped {
		return fmt.Errorf("user_publish_without_open: user_impossible_action wire did not trip on impossible publish action")
	}
	if !contains(reason, "is_impossible=true") {
		return fmt.Errorf("user_publish_without_open: wire reason=%q, want substring is_impossible=true", reason)
	}
	return nil
}

// testCredentialStoreIsolation verifies the store rejects
// two personas owning the same SI id. The first allocation
// succeeds; the second (by a different persona, same SI id)
// must return an error.
func testCredentialStoreIsolation() error {
	store := NewCredentialStore()
	if err := store.AllocateSI("persona-honest-1", "si-001"); err != nil {
		return fmt.Errorf("credential_store_isolation: first allocate: %w", err)
	}
	err := store.AllocateSI("persona-confused-1", "si-001")
	if err == nil {
		return fmt.Errorf("credential_store_isolation: second allocate succeeded, want error")
	}
	if !contains(err.Error(), "already allocated to another persona") {
		return fmt.Errorf("credential_store_isolation: error=%q, want substring 'already allocated to another persona'", err.Error())
	}
	return nil
}
