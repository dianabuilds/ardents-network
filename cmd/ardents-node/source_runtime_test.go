package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

type sourceRuntimeFixture struct {
	waitErr, closeErr error
	closed            bool
}

func (fixture *sourceRuntimeFixture) Current() (state.Snapshot, error) {
	return state.Snapshot{Generation: "accepted-generation", Epoch: 1}, nil
}
func (fixture *sourceRuntimeFixture) Wait(context.Context) error { return fixture.waitErr }
func (fixture *sourceRuntimeFixture) Close() error {
	fixture.closed = true
	return fixture.closeErr
}

func TestSourceRuntimeEmitsSafeTerminalFailureAfterReady(t *testing.T) {
	for _, scenario := range []struct {
		name, reason string
		wait, close  bool
	}{
		{name: "background", reason: "background-work", wait: true},
		{name: "cleanup", reason: "cleanup", close: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			failure := errors.New("private source failure detail")
			fixture := &sourceRuntimeFixture{}
			if scenario.wait {
				fixture.waitErr = failure
			}
			if scenario.close {
				fixture.closeErr = failure
			}
			var output bytes.Buffer
			if err := runOpenedSource(t.Context(), fixture, newEventOutput(&output)); !errors.Is(err, failure) || !fixture.closed {
				t.Fatalf("source terminal result = %v, closed=%v", err, fixture.closed)
			}
			if strings.Contains(output.String(), failure.Error()) {
				t.Fatal("source event exposed raw terminal error")
			}
			lines := bytes.Split(bytes.TrimSpace(output.Bytes()), []byte{'\n'})
			if len(lines) != 2 {
				t.Fatalf("source event count = %d", len(lines))
			}
			for index, expected := range []string{"source-ready", "source-failed"} {
				var event struct {
					Schema, Kind, Reason string
				}
				if err := json.Unmarshal(lines[index], &event); err != nil || event.Schema != "ardents-source-event-v1" || event.Kind != expected {
					t.Fatalf("source event %d = %+v, err=%v", index, event, err)
				}
				if index == 1 && event.Reason != scenario.reason {
					t.Fatalf("source failure category = %q", event.Reason)
				}
			}
		})
	}
}

func TestSourceRuntimeNormalShutdownEmitsOnlyReady(t *testing.T) {
	fixture := &sourceRuntimeFixture{}
	var output bytes.Buffer
	if err := runOpenedSource(t.Context(), fixture, newEventOutput(&output)); err != nil || !fixture.closed {
		t.Fatalf("normal source shutdown = %v, closed=%v", err, fixture.closed)
	}
	if bytes.Count(output.Bytes(), []byte{'\n'}) != 1 || !strings.Contains(output.String(), `"kind":"source-ready"`) {
		t.Fatalf("normal source events = %q", output.String())
	}
}
