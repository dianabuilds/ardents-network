//go:build live

package network_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type finalPressureStateObservation struct {
	Schema       string `json:"schema"`
	Ordinal      uint16 `json:"ordinal"`
	State        string `json:"state"`
	OffsetMillis uint64 `json:"offset_millis"`
}

func writeFinalPressureState(t *testing.T, root, state string, ordinal uint16) finalPressureStateObservation {
	t.Helper()
	origin, err := finalWorkerTimelineOrigin()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "sync", "bridge", "pressure-state.jsonl")
	output, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	value := finalPressureStateObservation{Schema: "ardents-h3-pressure-state-v1", Ordinal: ordinal,
		State: state, OffsetMillis: uint64(time.Since(origin).Milliseconds())}
	if err := json.NewEncoder(output).Encode(value); err != nil {
		_ = output.Close()
		t.Fatal(err)
	}
	if err := output.Sync(); err != nil {
		_ = output.Close()
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	return value
}
