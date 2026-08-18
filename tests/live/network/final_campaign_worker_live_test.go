//go:build live

package network_test

import (
	"encoding/json"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

var finalWorkerProcessStart = time.Now()
var finalWorkerTerminalArmed atomic.Bool
var finalWorkerTerminalClass atomic.Value

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
	t.Helper()
	if os.Getenv("ARDENTS_BLOCKED_CELL_WORKER") != "1" {
		return
	}
	terminalAt := time.Since(finalWorkerProcessStart)
	observers, rawObservers := collectFinalWorkerObservers(identity, ownedRoots)
	rawTelemetry, telemetryComplete := collectFinalWorkerTelemetry(ownedRoots)
	if !telemetryComplete {
		t.Fatal("final worker telemetry is unstable or malformed")
	}
	value := finalWorkerResult{Schema: "ardents-h3-final-worker-cell-v1", CellID: identity, Terminal: terminal,
		StartedOffsetMillis:  uint64(started.Sub(finalWorkerProcessStart).Milliseconds()),
		TerminalOffsetMillis: uint64(terminalAt.Milliseconds()), CleanupOffsetMillis: uint64(terminalAt.Milliseconds()),
		Observers: observers, RawObservers: rawObservers, RawTelemetry: rawTelemetry,
		ObserverSets: uint16(len(ownedRoots))}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(raw))
}
