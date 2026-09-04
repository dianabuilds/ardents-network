//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"time"
)

// TickDocument is the on-disk shape of one ticks/tick-N.json. It is
// the per-tick artefact the observer's verdict is recorded into.
// state.observed_digest is the consumer-reported generation for this
// tick; for S3.1 (no adversary, no drift) the value MUST be constant
// across all 100 tick.json files. verdict.trip_wires is the per-wire
// result list in catalog order.
type TickDocument struct {
	Schema      string         `json:"schema"`
	TickNumber  int            `json:"tick_number"`
	StartedAt   time.Time      `json:"started_at"`
	CompletedAt time.Time      `json:"completed_at"`
	DurationMs  int64          `json:"duration_ms"`
	Consumer    ConsumerRecord `json:"consumer"`
	State       StateRecord    `json:"state"`
	Verdict     Verdict        `json:"verdict"`
}

// ConsumerRecord is the consumer-side slice of TickDocument. ExitCode
// is what ardents refresh-sources returned; RawOutputPath is the
// relative path (under evidence/) to the full stdout+stderr capture.
type ConsumerRecord struct {
	ExitCode      int    `json:"exit_code"`
	RawOutputPath string `json:"raw_output_path"`
}

// StateRecord is the observer's view of the consumer's reported
// state for this tick. ObservedDigest is the consumer-reported
// generation; SourceOutcomes is the four-slot production
// source_outcomes array (positions 2 and 3 are upstream fallbacks
// and stay "not-attempted" for S3.1's single-source configuration).
type StateRecord struct {
	ObservedDigest string    `json:"observed_digest"`
	SourceOutcomes [4]string `json:"source_outcomes"`
}

// TickSchema is the constant JSON schema tag TickDocument carries.
// Bumping it is an explicit contract change for downstream
// reviewers.
const TickSchema = "ardents-simulation-tick-v1"

// RunTimekeeper keeps the shared clock-observation file fresh at the
// configured interval until stop is closed. The file's mtime is
// what the production source client reads as its independent clock
// observation; the consumer rejects observations more than two
// seconds from its local clock, so the timekeeper must refresh the
// mtime well under that bound. The function returns when stop is
// closed; the goroutine that owns the tick loop closes stop after
// the last tick completes.
//
// The caller is responsible for creating the observation file
// before the timekeeper goroutine starts; this function only
// refreshes the mtime, it does not create the file. Chtimes errors
// on a missing file are silently ignored because the timekeeper
// cannot recover from a missing file (the production client would
// already have rejected the run on the first refresh-sources call).
func RunTimekeeper(observationPath string, interval time.Duration, stop <-chan struct{}) {
	if interval <= 0 {
		interval = 50 * time.Millisecond
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()
	now := time.Now()
	if err := os.Chtimes(observationPath, now, now); err != nil {
		_ = err
	}
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
			now := time.Now()
			if err := os.Chtimes(observationPath, now, now); err != nil {
				_ = err
			}
		}
	}
}

// WriteTickDocument marshals one tick's TickDocument to disk. The
// path is created if missing; the parent directory is the ticks/
// subdirectory of the evidence dir and is created by the tick
// loop in main.go.
func WriteTickDocument(path string, doc TickDocument) error {
	doc.Schema = TickSchema
	marshaled, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(marshaled, '\n'), 0o600)
}

// Pacer returns when the current tick has reached at least target
// elapsed time. The tick loop calls Pacer at the end of each tick
// to enforce the 100 ms target per-tick wall-clock budget; ticks
// that ran longer than the target are not slept again. Pacer is a
// no-op for negative or zero targets.
func Pacer(target time.Duration) {
	if target <= 0 {
		return
	}
	time.Sleep(target)
}
