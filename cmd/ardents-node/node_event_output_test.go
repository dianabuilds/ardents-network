package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/resource"
)

func TestNodeEventEmitterPublishesLatestBoundedDiagnostics(t *testing.T) {
	output := nodeEventOutput(t)
	ctx := nodeEventContext(t)
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
		if err := emit(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	assertDiagnosticState(t, filepath.Join(diagnostics, "lifecycle.json"), "WITHDRAWN", 0)
	assertDiagnosticState(t, filepath.Join(diagnostics, "resource.json"), "OBSERVED", 17<<20)
}

func TestNodeEventEmitterSerializesConcurrentDiagnostics(t *testing.T) {
	output := nodeEventOutput(t)
	ctx := nodeEventContext(t)
	diagnostics := filepath.Join(t.TempDir(), "diagnostics")
	if err := os.Mkdir(diagnostics, 0o700); err != nil {
		t.Fatal(err)
	}
	emit := nodeEventEmitter(output, diagnostics)
	var workers sync.WaitGroup
	for index := range 32 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			event := node.Event{Schema: "ardents-node-event-v1", Kind: "resource-sample", State: "OBSERVED", At: time.Now(),
				Resource: &resource.Sample{MemoryBytes: uint64(index + 1)}}
			if err := emit(ctx, event); err != nil {
				t.Errorf("concurrent emit %d: %v", index, err)
			}
		}()
	}
	workers.Wait()
	final := node.Event{Schema: "ardents-node-event-v1", Kind: "lifecycle", State: "WITHDRAWN", At: time.Now()}
	if err := emit(ctx, final); err != nil {
		t.Fatal(err)
	}
	assertDiagnosticState(t, filepath.Join(diagnostics, "lifecycle.json"), "WITHDRAWN", 0)
	raw, err := os.ReadFile(filepath.Join(diagnostics, "resource.json"))
	if err != nil {
		t.Fatal(err)
	}
	var observed node.Event
	if err := json.Unmarshal(raw, &observed); err != nil || observed.Resource == nil || observed.Resource.MemoryBytes < 1 || observed.Resource.MemoryBytes > 32 {
		t.Fatalf("concurrent resource diagnostic = %+v, %v", observed, err)
	}
}

func nodeEventContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// nodeEventOutput supplies the bounded writer contract required on Linux and
// drains each emitted event so the concurrent fixture cannot fill its pipe.
func nodeEventOutput(t *testing.T) *os.File {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	drained := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, reader)
		drained <- err
	}()
	t.Cleanup(func() {
		if err := writer.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			t.Errorf("close Node event writer: %v", err)
		}
		if err := <-drained; err != nil {
			t.Errorf("drain Node event output: %v", err)
		}
		if err := reader.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			t.Errorf("close Node event reader: %v", err)
		}
	})
	return writer
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
