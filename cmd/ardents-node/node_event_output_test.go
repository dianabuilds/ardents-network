package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/resource"
)

func TestNodeEventEmitterPublishesLatestBoundedDiagnostics(t *testing.T) {
	output, err := os.Create(filepath.Join(t.TempDir(), "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	diagnostics := filepath.Join(t.TempDir(), "diagnostics")
	if err := os.Mkdir(diagnostics, 0o700); err != nil {
		t.Fatal(err)
	}
	emit := nodeEventEmitter(output, diagnostics)
	for _, event := range []node.Event{
		{Schema: "ardents-node-event-v1", Kind: "lifecycle", State: "READY", At: time.Now()},
		{Schema: "ardents-node-event-v1", Kind: "resource-sample", State: "OBSERVED", At: time.Now(), Resource: &resource.Sample{MemoryBytes: 17 << 20}},
		{Schema: "ardents-node-event-v1", Kind: "lifecycle", State: "WITHDRAWN", At: time.Now()},
	} {
		if err := emit(t.Context(), event); err != nil {
			t.Fatal(err)
		}
	}
	assertDiagnosticState(t, filepath.Join(diagnostics, "lifecycle.json"), "WITHDRAWN", 0)
	assertDiagnosticState(t, filepath.Join(diagnostics, "resource.json"), "OBSERVED", 17<<20)
}

func assertDiagnosticState(t *testing.T, path, state string, memory uint64) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var event node.Event
	if err := json.Unmarshal(raw, &event); err != nil {
		t.Fatal(err)
	}
	if event.State != state || memory > 0 && (event.Resource == nil || event.Resource.MemoryBytes != memory) {
		t.Fatalf("diagnostic %s = %+v", path, event)
	}
}
