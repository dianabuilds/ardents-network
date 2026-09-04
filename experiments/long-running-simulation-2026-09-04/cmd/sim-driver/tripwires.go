//go:build ignore

package main

import "strconv"

// TripWire is one named check the observer runs against the current
// tick and the accumulated state. Each wire returns (tripped, reason);
// a tripped wire aborts the run with the reason as the user-visible
// error, UNLESS Informational is true (added in S3.6): an
// informational wire is reported in the tick.json verdict block
// but does not set verdict.Accept to false and does not abort
// the run. S3.1's catalog has exactly four fatal wires (see
// DefaultCatalog); S3.6 adds two informational wires for the
// UserActor. Later slices (S3.4) add more without changing
// this signature.
type TripWire struct {
	Name          string
	Check         func(tick TickState, accumulated AccumulatedState) (bool, string)
	Informational bool
}

// DefaultCatalog returns the S3.1 + S3.6 trip-wires. The order is
// the order the observer reports them in the tick.json verdict,
// and is stable so a downstream reviewer can match the order to
// a known acceptance spec. The four S3.1 wires are fatal; the
// two S3.6 wires (user_impossible_action, user_retry_storm) are
// informational and do not abort the run.
func DefaultCatalog() []TripWire {
	return []TripWire{
		{Name: "generation_drift", Check: GenerationDriftWire},
		{Name: "source_exit", Check: SourceExitWire},
		{Name: "consumer_parse_error", Check: ConsumerParseErrorWire},
		{Name: "tick_budget", Check: TickBudgetWire},
		{Name: "user_impossible_action", Check: UserImpossibleActionWire, Informational: true},
		{Name: "user_retry_storm", Check: UserRetryStormWire, Informational: true},
	}
}

// GenerationDriftWire trips when the consumer-reported observed_digest
// for the current tick differs from the first non-empty observed
// digest seen in the run. For S3.1 (no adversary, no drift) the
// digest MUST be constant. The first tick of the run cannot trip the
// wire: the first observed digest is recorded as the reference, not
// compared to a prebake constant, so a benign prebake generation
// written in fixtures/current does not false-positive this check.
func GenerationDriftWire(tick TickState, accumulated AccumulatedState) (bool, string) {
	if tick.ParseError != "" {
		return false, ""
	}
	if accumulated.FirstObservedDigest == "" {
		return false, ""
	}
	if tick.Event.Generation != accumulated.FirstObservedDigest {
		return true, "tick " + strconv.Itoa(tick.Number) + " observed_digest=" + tick.Event.Generation +
			" differs from first observed_digest=" + accumulated.FirstObservedDigest
	}
	return false, ""
}

// SourceExitWire trips when ardents refresh-sources exits non-zero OR
// returns a Go error from exec. Either condition means the source
// container is no longer responsive on the network, and the run must
// abort rather than continue with stale state.
func SourceExitWire(tick TickState, accumulated AccumulatedState) (bool, string) {
	if tick.ConsumerError != nil {
		return true, "tick " + strconv.Itoa(tick.Number) + " exec error: " + tick.ConsumerError.Error()
	}
	if tick.ConsumerExitCode != 0 {
		return true, "tick " + strconv.Itoa(tick.Number) + " refresh-sources exit_code=" + strconv.Itoa(tick.ConsumerExitCode)
	}
	return false, ""
}

// ConsumerParseErrorWire trips when the consumer output cannot be
// parsed into exactly one source-wave-accepted event. A parse error
// means the consumer either failed to produce a valid event (the
// source returned garbage, the consumer crashed, the network
// dropped mid-flight) or produced zero events (the consumer is not
// reaching the source at all). Both are fatal for the run.
func ConsumerParseErrorWire(tick TickState, accumulated AccumulatedState) (bool, string) {
	if tick.ParseError != "" {
		return true, "tick " + strconv.Itoa(tick.Number) + " consumer parse error: " + tick.ParseError
	}
	return false, ""
}

// TickBudgetWire trips when the current tick's wall-clock duration
// exceeds the per-tick budget. The per-tick budget is 5 s by default
// (50x the 100 ms target) to absorb cold-start jitter on a Windows
// host; the run-level budget is enforced separately in main.go's
// tick loop and is not part of this wire. A trip here means a single
// refresh cycle took unreasonably long, almost certainly because the
// source or the consumer is wedged.
func TickBudgetWire(tick TickState, accumulated AccumulatedState) (bool, string) {
	budget := accumulated.PerTickBudget
	if budget <= 0 {
		budget = DefaultPerTickBudget
	}
	if tick.Duration > budget {
		return true, "tick " + strconv.Itoa(tick.Number) + " duration=" + tick.Duration.String() +
			" exceeds per-tick budget=" + budget.String()
	}
	return false, ""
}
