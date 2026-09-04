//go:build ignore

package main

import (
	"time"
)

// DefaultPerTickBudget is the S3.1 per-tick wall-clock budget used
// when the run is started without an explicit override. It is 50x the
// 100 ms target to absorb cold-start jitter on a Windows Docker host
// without false-positiving the tick_budget wire on a healthy run.
const DefaultPerTickBudget = 5 * time.Second

// TickState is the observer's per-tick input. It is built by
// timekeeper.go after the consumer subprocess completes and is
// passed to Observe unchanged; the observer must not mutate it.
// UserActions is the S3.6 extension: the per-tick list of
// user-simulated actions emitted by the UserActor. The list is
// empty for ticks where no persona acted (cooldown or no
// persona selected). The user_impossible_action wire reads
// this list.
type TickState struct {
	Number           int
	StartedAt        time.Time
	CompletedAt      time.Time
	Duration         time.Duration
	ConsumerExitCode int
	ConsumerError    error
	Event            SourceWaveEvent
	ParseError       string
	UserActions      []UserAction
}

// AccumulatedState is the observer's cross-tick input. The first
// non-empty observed_digest seen in the run is recorded here as
// FirstObservedDigest; the generation_drift wire compares every
// subsequent tick's digest to it. PerTickBudget overrides the
// tick_budget wire's per-tick limit when non-zero. RunStart is used
// by the run-level wall-clock budget check in main.go.
// UserActionTicks is the S3.6 extension: per persona, the list
// of tick numbers on which the persona emitted an action in
// the recent window. The user_retry_storm wire reads this map
// to detect a persona that bursts more than 5 actions in 10
// ticks. The map is updated by RecordUserActions (called from
// main.go) before each Observe call.
type AccumulatedState struct {
	FirstObservedDigest string
	PerTickBudget       time.Duration
	RunStart            time.Time
	TickCount           int
	AcceptedCount       int
	RejectCount         int
	UserActionTicks     map[string][]int
}

// RecordUserActions appends the current tick to every acting
// persona's tick list and prunes entries older than 10 ticks.
// The S3.6 user_retry_storm wire checks if any persona's
// remaining list has more than 5 entries.
func (a *AccumulatedState) RecordUserActions(actions []UserAction, tick int) {
	if a.UserActionTicks == nil {
		a.UserActionTicks = make(map[string][]int)
	}
	for _, action := range actions {
		a.UserActionTicks[action.PersonaID] = append(a.UserActionTicks[action.PersonaID], tick)
	}
	for personaID, ticks := range a.UserActionTicks {
		filtered := ticks[:0]
		for _, t := range ticks {
			if tick-t <= 10 {
				filtered = append(filtered, t)
			}
		}
		a.UserActionTicks[personaID] = filtered
	}
}

// TripWireResult is one row of the verdict block in tick.json. The
// wire's Name is the catalog name; Tripped is true iff the wire
// returned (true, _); Reason is the wire's reason string when
// tripped, otherwise empty.
type TripWireResult struct {
	Name    string `json:"name"`
	Tripped bool   `json:"tripped"`
	Reason  string `json:"reason"`
}

// Verdict is the per-tick observer output. Accept is true iff every
// wire returned (false, _); Reason is the rejection reason of the
// first tripped wire, or the human-readable summary of the
// acceptance when nothing tripped.
type Verdict struct {
	Accept    bool             `json:"accept"`
	Reason    string           `json:"reason"`
	TripWires []TripWireResult `json:"trip_wires"`
}

// Observe runs the full trip-wire catalog against the current tick
// and the accumulated state, records the per-wire results, and
// returns the aggregate verdict. The first non-empty observed
// digest seen in the run is recorded into accumulated on accept; on
// reject the accumulated state is left untouched so the next tick
// (if any) can still report a stable reference.
//
// Informational wires (S3.6 addition: user_impossible_action,
// user_retry_storm) are reported in the per-wire results but do
// NOT set verdict.Accept to false and do NOT abort the run. The
// verdict.Reason is set only by a fatal wire trip; informational
// trips are visible in verdict.TripWires for downstream reviewers.
func Observe(tick TickState, accumulated *AccumulatedState) (Verdict, []TripWireResult) {
	catalog := DefaultCatalog()
	results := make([]TripWireResult, 0, len(catalog))
	verdict := Verdict{Accept: true, Reason: "all " + itoaInt(len(catalog)) + " trip-wires passed"}
	for _, wire := range catalog {
		tripped, reason := wire.Check(tick, *accumulated)
		results = append(results, TripWireResult{Name: wire.Name, Tripped: tripped, Reason: reason})
		if tripped && !wire.Informational {
			verdict.Accept = false
			verdict.Reason = wire.Name + ": " + reason
		}
	}
	if verdict.Accept && tick.ParseError == "" && tick.Event.Generation != "" {
		if accumulated.FirstObservedDigest == "" {
			accumulated.FirstObservedDigest = tick.Event.Generation
		}
		accumulated.AcceptedCount++
	} else if !verdict.Accept {
		accumulated.RejectCount++
	}
	accumulated.TickCount++
	verdict.TripWires = results
	return verdict, results
}

// itoaInt is a local helper that avoids pulling strconv into the
// observer file (the file already imports time only). The tripwires
// file uses strconv.Itoa; this keeps the verdict reason formatting
// local to the observer's responsibility.
func itoaInt(value int) string {
	if value == 0 {
		return "0"
	}
	negative := false
	if value < 0 {
		negative = true
		value = -value
	}
	digits := []byte{}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	if negative {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

// UserImpossibleActionWire trips when any user action emitted
// on this tick has IsImpossible=true. The wire is INFORMATIONAL
// (does not abort the run) because the S3.6 contract requires
// the confused persona to emit 5% impossible actions as part
// of its design; a run that never trips this wire would mean
// the confused persona is not exercising its contract.
//
// The wire is registered in DefaultCatalog (see tripwires.go)
// so the per-tick verdict reports it, but the Observe function
// does NOT downgrade verdict.Accept to false on a trip here.
// The trip is recorded in the tick.json verdict block for
// downstream reviewers.
func UserImpossibleActionWire(tick TickState, accumulated AccumulatedState) (bool, string) {
	for _, action := range tick.UserActions {
		if action.IsImpossible {
			return true, "tick " + itoaInt(tick.Number) + " user action verb=" + action.Verb +
				" persona=" + action.PersonaID + " is_impossible=true (informational)"
		}
	}
	return false, ""
}

// UserRetryStormWire trips when any persona has emitted more
// than 5 actions in the last 10 ticks. The wire is
// INFORMATIONAL (does not abort the run) because the S3.6
// impatient persona is expected to emit 1 action per tick for
// the full 100-tick run, which is 100 actions in 100 ticks;
// the wire's window is 10 ticks, so a 6+ burst within 10
// ticks is the trigger. The run is not aborted on a trip
// here; the verdict block records the trip.
func UserRetryStormWire(tick TickState, accumulated AccumulatedState) (bool, string) {
	for personaID, ticks := range accumulated.UserActionTicks {
		if len(ticks) > 5 {
			return true, "tick " + itoaInt(tick.Number) + " persona=" + personaID +
				" emitted " + itoaInt(len(ticks)) + " actions in last 10 ticks (informational)"
		}
	}
	return false, ""
}
