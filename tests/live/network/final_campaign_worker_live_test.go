//go:build live

package network_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var finalWorkerProcessStart = time.Now()
var finalWorkerTerminalArmed atomic.Bool
var finalWorkerTerminalClass atomic.Value
var finalWorkerTimelineOnce sync.Once
var finalWorkerTimeline time.Time
var finalWorkerTimelineErr error

func finalWorkerTimelineOrigin() (time.Time, error) {
	finalWorkerTimelineOnce.Do(func() {
		finalWorkerTimeline = finalWorkerProcessStart
		name := "ARDENTS_FINAL_CAMPAIGN_MONOTONIC_ANCHOR_MILLIS"
		if os.Getenv(name) == "" {
			return
		}
		anchor, parseErr := strconv.ParseInt(os.Getenv(name), 10, 64)
		hostMillis, clockErr := linuxMonotonicMillis()
		offset := hostMillis + anchor
		if parseErr != nil || clockErr != nil || offset < 0 {
			finalWorkerTimelineErr = errors.Join(parseErr, clockErr,
				errors.New("final worker campaign monotonic anchor is invalid"))
			return
		}
		finalWorkerTimeline = time.Now().Add(-time.Duration(offset) * time.Millisecond)
	})
	return finalWorkerTimeline, finalWorkerTimelineErr
}

func selectedFinalCell(identity string) bool {
	selected := os.Getenv("ARDENTS_FINAL_CELL")
	return selected == "" || selected == identity
}

func armFinalWorkerTerminal(terminal string) {
	if os.Getenv("ARDENTS_BLOCKED_CELL_WORKER") == "1" {
		finalWorkerTerminalClass.Store(terminal)
		finalWorkerTerminalArmed.Store(true)
	}
}

func publishFinalWorkerTerminal() {
	if !finalWorkerTerminalArmed.Swap(false) {
		return
	}
	value := struct {
		Schema   string `json:"schema"`
		CellID   string `json:"cell_id"`
		Terminal string `json:"terminal"`
	}{Schema: "ardents-h3-final-worker-terminal-v1", CellID: os.Getenv("ARDENTS_FINAL_CELL"),
		Terminal: finalWorkerTerminalClass.Load().(string)}
	raw, err := json.Marshal(value)
	if err == nil {
		fmt.Println(string(raw))
	}
}

func emitFinalWorkerCell(t *testing.T, identity, terminal string, started time.Time, ownedRoots ...string) {
	emitFinalWorkerResult(t, identity, terminal, started, nil, nil, ownedRoots...)
}

func emitFinalWorkerSustained(t *testing.T, identity, terminal string, started time.Time,
	measurement finalWorkerSustained, ownedRoots ...string,
) {
	emitFinalWorkerResult(t, identity, terminal, started, &measurement, nil, ownedRoots...)
}

func emitFinalWorkerPressure(t *testing.T, identity, terminal string, started time.Time,
	measurement finalPressureEvidence, ownedRoots ...string,
) {
	emitFinalWorkerResult(t, identity, terminal, started, nil, &measurement, ownedRoots...)
}

func emitFinalWorkerResult(t *testing.T, identity, terminal string, started time.Time,
	sustained *finalWorkerSustained, pressure *finalPressureEvidence, ownedRoots ...string,
) {
	t.Helper()
	if os.Getenv("ARDENTS_BLOCKED_CELL_WORKER") != "1" {
		return
	}
	terminalAt := time.Since(finalWorkerProcessStart)
	observers, rawObservers := collectFinalWorkerObservers(identity, ownedRoots)
	rawTelemetry, telemetryComplete := collectFinalWorkerTelemetry(identity, ownedRoots)
	if !telemetryComplete {
		t.Fatal("final worker telemetry is unstable or malformed")
	}
	if err := writeFinalWorkerHandoff(os.Getenv("ARDENTS_FINAL_WORKER_ROOT"), identity,
		rawObservers, rawTelemetry); err != nil {
		t.Fatal(err)
	}
	value := finalWorkerResult{Schema: "ardents-h3-final-worker-cell-v1", CellID: identity, Terminal: terminal,
		StartedOffsetMillis:  uint64(started.Sub(finalWorkerProcessStart).Milliseconds()),
		TerminalOffsetMillis: uint64(terminalAt.Milliseconds()), CleanupOffsetMillis: uint64(terminalAt.Milliseconds()),
		Observers: observers, Sustained: sustained, Pressure: pressure,
		ObserverSets: uint16(len(ownedRoots))}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(raw))
}
