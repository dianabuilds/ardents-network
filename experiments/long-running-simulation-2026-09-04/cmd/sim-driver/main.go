//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

const (
	// defaultTickCount is the S3.1 smoke slice's tick count. The
	// contract (README.md, AC4) fixes it at 100; later slices
	// (S3.2+) raise it via the sim-driver's own configuration
	// without changing this default.
	defaultTickCount = 100

	// tickTarget is the S3.1 per-tick wall-clock target. It is a
	// pacing target, NOT a hard SLA: the per-tick trip-wire uses
	// DefaultPerTickBudget (5 s) as the abort threshold, and the
	// run-level abort threshold is runBudget (30 s). A tick that
	// runs longer than tickTarget but under the per-tick budget
	// is still accepted; the pacer just doesn't sleep.
	tickTarget = 100 * time.Millisecond

	// runBudget is the S3.1 run-level wall-clock budget. The
	// contract's AC4 fixes the upper bound at 30 s; the main
	// tick loop aborts the run if elapsed time exceeds this
	// value, which is the run-level analogue of the per-tick
	// tick_budget trip-wire.
	runBudget = 30 * time.Second

	// currentGenerationFile is the relative path under the
	// evidence dir the slice 2 prebake writes the expected
	// generation to. The S3.1 sim-driver reads it for log
	// correlation but does not fail the run if it is absent; the
	// generation_drift trip-wire records the first observed
	// digest as the reference, not the prebake value.
	currentGenerationFile = "fixtures/current"
)

// Summary is the run-level artefact the sim-driver writes after
// the 100-tick smoke completes. StartedAt and CompletedAt bracket
// the entire run; TickCount is always defaultTickCount for S3.1;
// AcceptedCount + RejectCount are the observer's tally; FirstObservedDigest
// is what every subsequent tick was compared against by
// generation_drift.
type Summary struct {
	Schema              string    `json:"schema"`
	StartedAt           time.Time `json:"started_at"`
	CompletedAt         time.Time `json:"completed_at"`
	DurationMs          int64     `json:"duration_ms"`
	TickCount           int       `json:"tick_count"`
	AcceptedCount       int       `json:"accepted_count"`
	RejectCount         int       `json:"reject_count"`
	FirstObservedDigest string    `json:"first_observed_digest"`
	PerTickBudgetMs     int64     `json:"per_tick_budget_ms"`
	RunBudgetMs         int64     `json:"run_budget_ms"`
	ObservedDigests     []string  `json:"observed_digests"`
	Verdict             string    `json:"verdict"`
}

// SummarySchema is the constant JSON schema tag Summary carries.
const SummarySchema = "ardents-simulation-summary-v1"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "sim-driver:", err)
		os.Exit(2)
	}
}

func run(ctx context.Context, arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("usage: sim-driver <self-test|tick-loop> [EVIDENCE_DIR]")
	}
	switch arguments[0] {
	case "self-test":
		return runSelfTest(ctx)
	case "tick-loop":
		return runTickLoop(ctx, arguments[1:])
	default:
		return fmt.Errorf("unknown subcommand %q", arguments[0])
	}
}

func runTickLoop(ctx context.Context, arguments []string) error {
	if len(arguments) != 1 {
		return errors.New("usage: sim-driver tick-loop EVIDENCE_DIR")
	}
	evidenceDir, err := filepath.Abs(arguments[0])
	if err != nil {
		return fmt.Errorf("sim-driver: abs evidence dir: %w", err)
	}
	if err := EnsureSourceState(evidenceDir); err != nil {
		return err
	}
	expectedGeneration := readExpectedGeneration(filepath.Join(evidenceDir, currentGenerationFile))

	ticksDir := filepath.Join(evidenceDir, "ticks")
	plansDir := filepath.Join(evidenceDir, "plans")
	stateDir := filepath.Join(evidenceDir, "state")
	for _, dir := range []string{ticksDir, plansDir, stateDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("sim-driver: mkdir %s: %w", dir, err)
		}
	}

	// S3.1 re-uses ONE shared consumer state root across all 100
	// ticks. The slice 2 pilot uses one state root per consumer
	// (6 separate roots for 6 consumers) because 6 consumers run
	// in parallel and contend on the local-roles lease; S3.1 has
	// one consumer running 100 sequential refreshes, so the
	// lease is released between ticks and a single state root is
	// safe. Re-using the same state root skips the production
	// accept-offline step on every tick after the first, which
	// is the dominant per-tick cost (~500 ms on Windows Docker).
	// For S3.1 (no drift, no adversary) the state is invariant
	// across refreshes, so carrying the state forward is
	// semantically equivalent to a fresh accept-offline.
	//
	// The shared consumer state root starts as a copy of the
	// slice 2 prebake's source-a-state contents (marker +
	// generations/), which lets the FIRST tick skip the
	// accept-offline too. We cannot use source-a-state directly
	// because the source-a container holds the lease on it; the
	// consumer's state root must be a separate directory that
	// carries the same marker and generation data.
	consumerStateRoot := filepath.Join(stateDir, "consumer")
	if err := os.MkdirAll(consumerStateRoot, 0o700); err != nil {
		return fmt.Errorf("sim-driver: mkdir consumer state root: %w", err)
	}
	if err := copyPrebakedState(consumerStateRoot, filepath.Join(evidenceDir, "source-a-state")); err != nil {
		return fmt.Errorf("sim-driver: seed consumer state root from source-a-state: %w", err)
	}

	clockPath := filepath.Join(evidenceDir, "fixtures", "clock.observation")
	// The slice 2 prebake does not create clock.observation; the
	// clock-tick.sh container creates and refreshes it in slice 2.
	// S3.1 folds the Timekeeper role into the sim-driver itself,
	// so the sim-driver creates the file on first run. The file
	// must exist BEFORE the first refresh-sources call, otherwise
	// the production source client rejects the run with a missing-
	// observation error.
	freshClock, err := os.Create(clockPath)
	if err != nil {
		return fmt.Errorf("sim-driver: create clock observation file: %w", err)
	}
	_ = freshClock.Close()
	tkStop := make(chan struct{})
	tkDone := make(chan struct{})
	go func() {
		defer close(tkDone)
		RunTimekeeper(clockPath, 50*time.Millisecond, tkStop)
	}()
	defer func() { close(tkStop); <-tkDone }()

	runStart := time.Now().UTC()
	accumulated := AccumulatedState{
		PerTickBudget: DefaultPerTickBudget,
		RunStart:      runStart,
	}
	sharedPlan := filepath.Join(evidenceDir, "fixtures", "client.json")
	observedDigests := make([]string, 0, defaultTickCount)

	// S3.6: the user simulation runs in its own actor alongside
	// the network consumer. The actor owns the credential store
	// and the three personas; the tick loop invokes Tick on
	// every tick (after Timekeeper, before the consumer step)
	// and records the returned actions into the accumulated
	// state for the user_retry_storm wire.
	userActor := NewUserActor()
	credentialStore := NewCredentialStore()

	for tickNumber := 1; tickNumber <= defaultTickCount; tickNumber++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("sim-driver: context cancelled at tick %d: %w", tickNumber, ctx.Err())
		default:
		}
		if time.Since(runStart) > runBudget {
			return fmt.Errorf("sim-driver: run budget exceeded before tick %d (elapsed=%s, budget=%s)",
				tickNumber, time.Since(runStart), runBudget)
		}

		tickStateRoot := consumerStateRoot
		rawPath := filepath.Join(ticksDir, fmt.Sprintf("tick-%d-raw.log", tickNumber))
		tickPath := filepath.Join(ticksDir, fmt.Sprintf("tick-%d.json", tickNumber))
		privatePlan := filepath.Join(plansDir, fmt.Sprintf("tick-%d.json", tickNumber))

		// S3.6 user simulation step. Runs after the Timekeeper
		// mtime refresh (which is the goroutine, not a per-tick
		// call) and before the consumer refresh-sources call.
		// The user actions are mock and do not affect the
		// network layer's observed_digest.
		userActions := userActor.Tick(tickNumber, credentialStore)
		if writeErr := WriteUserActions(evidenceDir, tickNumber, userActions); writeErr != nil {
			return fmt.Errorf("sim-driver: tick %d write user actions: %w", tickNumber, writeErr)
		}
		accumulated.RecordUserActions(userActions, tickNumber)

		if err := WritePerTickPlan(sharedPlan, privatePlan, tickStateRoot); err != nil {
			return fmt.Errorf("sim-driver: tick %d write per-tick plan: %w", tickNumber, err)
		}

		tickStart := time.Now().UTC()
		output, exitCode, execErr := RunRefreshOnce(tickStateRoot, privatePlan)
		tickEnd := time.Now().UTC()

		if writeErr := os.WriteFile(rawPath, output, 0o600); writeErr != nil {
			return fmt.Errorf("sim-driver: tick %d write raw output: %w", tickNumber, writeErr)
		}

		event, parseErr := ReadSourceWaveEventFromBytes(output)
		parseErrorString := ""
		if parseErr != nil {
			parseErrorString = parseErr.Error()
		}

		tick := TickState{
			Number:           tickNumber,
			StartedAt:        tickStart,
			CompletedAt:      tickEnd,
			Duration:         tickEnd.Sub(tickStart),
			ConsumerExitCode: exitCode,
			ConsumerError:    execErr,
			Event:            event,
			ParseError:       parseErrorString,
			UserActions:      userActions,
		}
		verdict, _ := Observe(tick, &accumulated)

		observedDigest := event.Generation
		if parseErrorString != "" {
			observedDigest = ""
		}
		observedDigests = append(observedDigests, observedDigest)

		state := StateRecord{
			ObservedDigest: observedDigest,
			SourceOutcomes: event.SourceOutcomes,
		}
		doc := TickDocument{
			Schema:      TickSchema,
			TickNumber:  tickNumber,
			StartedAt:   tickStart,
			CompletedAt: tickEnd,
			DurationMs:  tickEnd.Sub(tickStart).Milliseconds(),
			Consumer: ConsumerRecord{
				ExitCode:      exitCode,
				RawOutputPath: filepath.ToSlash(filepath.Join("ticks", fmt.Sprintf("tick-%d-raw.log", tickNumber))),
			},
			State:   state,
			Verdict: verdict,
		}
		if writeErr := WriteTickDocument(tickPath, doc); writeErr != nil {
			return fmt.Errorf("sim-driver: tick %d write tick.json: %w", tickNumber, writeErr)
		}

		if !verdict.Accept {
			return fmt.Errorf("sim-driver: tick %d rejected: %s", tickNumber, verdict.Reason)
		}

		elapsed := time.Since(tickStart)
		if elapsed < tickTarget {
			Pacer(tickTarget - elapsed)
		}
	}

	runEnd := time.Now().UTC()
	summary := Summary{
		Schema:              SummarySchema,
		StartedAt:           runStart,
		CompletedAt:         runEnd,
		DurationMs:          runEnd.Sub(runStart).Milliseconds(),
		TickCount:           accumulated.TickCount,
		AcceptedCount:       accumulated.AcceptedCount,
		RejectCount:         accumulated.RejectCount,
		FirstObservedDigest: accumulated.FirstObservedDigest,
		PerTickBudgetMs:     DefaultPerTickBudget.Milliseconds(),
		RunBudgetMs:         runBudget.Milliseconds(),
		ObservedDigests:     observedDigests,
		Verdict:             "all " + itoaInt(defaultTickCount) + " ticks accepted; expected_generation=" + expectedGeneration,
	}
	summaryPath := filepath.Join(evidenceDir, "simulation-summary.json")
	if err := writeSummary(summaryPath, summary); err != nil {
		return fmt.Errorf("sim-driver: write summary: %w", err)
	}
	fmt.Printf("sim-driver: tick-loop complete ticks=%d accepted=%d first_observed_digest=%s elapsed=%s\n",
		accumulated.TickCount, accumulated.AcceptedCount, accumulated.FirstObservedDigest, runEnd.Sub(runStart))
	return nil
}

func writeSummary(path string, summary Summary) error {
	marshaled, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(marshaled, '\n'), 0o600)
}

func readExpectedGeneration(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	value := string(raw)
	if len(value) > 0 && value[len(value)-1] == '\n' {
		value = value[:len(value)-1]
	}
	return value
}
