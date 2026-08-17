//go:build live

package network_test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

var finalWorkerProcessStart = time.Now()

func selectedFinalCell(identity string) bool {
	selected := os.Getenv("ARDENTS_FINAL_CELL")
	return selected == "" || selected == identity
}

func emitFinalWorkerCell(t *testing.T, identity, terminal string, started time.Time) {
	t.Helper()
	if os.Getenv("ARDENTS_BLOCKED_CELL_WORKER") != "1" {
		return
	}
	terminalAt := time.Since(finalWorkerProcessStart)
	value := finalWorkerResult{Schema: "ardents-h3-final-worker-cell-v1", CellID: identity, Terminal: terminal,
		StartedOffsetMillis:  uint64(started.Sub(finalWorkerProcessStart).Milliseconds()),
		TerminalOffsetMillis: uint64(terminalAt.Milliseconds()), CleanupOffsetMillis: uint64(terminalAt.Milliseconds())}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(raw))
}
