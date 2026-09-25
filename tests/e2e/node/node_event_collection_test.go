package state_test

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

func collectNodeEvents(input io.Reader, process *nodeProcess) {
	defer close(process.events)
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		var event nodeEvent
		if json.Unmarshal(scanner.Bytes(), &event) != nil {
			continue
		}
		if event.Schema == "ardents-node-event-v1" && event.Kind == "route-diagnostic" && event.Reason != "" {
			process.waitMu.Lock()
			if process.firstRouteDiagnostic == "" {
				process.firstRouteDiagnostic = event.Reason
			}
			if event.At.IsZero() {
				event.At = time.Now().UTC()
			}
			const retainedRouteDiagnostics = 64
			if len(process.routeDiagnostics) == retainedRouteDiagnostics {
				dropped := process.routeDiagnostics[0].at
				if dropped.After(process.latestDroppedRouteDiagnostic) {
					process.latestDroppedRouteDiagnostic = dropped
				}
				copy(process.routeDiagnostics, process.routeDiagnostics[1:])
				process.routeDiagnostics = process.routeDiagnostics[:retainedRouteDiagnostics-1]
			}
			process.routeDiagnostics = append(process.routeDiagnostics, routeDiagnosticObservation{at: event.At, reason: event.Reason})
			process.waitMu.Unlock()
		}
		// Lifecycle consumers wait between operations. Periodic resource samples
		// must still be drained so their output cannot stall the real Node.
		// Malformed schemas or non-periodic states remain observable events.
		if event.Schema == "ardents-node-event-v1" && event.Kind == "resource-sample" && event.State == "OBSERVED" {
			continue
		}
		// Preserve already emitted lifecycle evidence even after process exit.
		select {
		case process.events <- event:
			continue
		default:
		}
		select {
		case process.events <- event:
		case <-process.done:
			return
		}
	}
}

func TestNodeLifecycleCollectionDoesNotBackpressureOnPeriodicSamples(t *testing.T) {
	sample := "{\"schema\":\"ardents-node-event-v1\",\"kind\":\"resource-sample\",\"state\":\"OBSERVED\"}\n"
	terminal := "{\"schema\":\"ardents-node-event-v1\",\"kind\":\"lifecycle\",\"state\":\"WITHDRAWN\"}\n"
	process := &nodeProcess{events: make(chan nodeEvent, 32), done: make(chan struct{})}
	collected := make(chan struct{})
	go func() {
		defer close(collected)
		collectNodeEvents(strings.NewReader(strings.Repeat(sample, 4096)+terminal), process)
	}()
	t.Cleanup(func() { close(process.done); <-collected })
	select {
	case <-collected:
	case <-time.After(time.Second):
		t.Fatal("periodic samples blocked the lifecycle event collector while the scenario was busy")
	}
	event, open := <-process.events
	if !open || event.Kind != "lifecycle" || event.State != "WITHDRAWN" {
		t.Fatalf("terminal lifecycle event lost: %+v", event)
	}
	if _, open := <-process.events; open {
		t.Fatal("periodic samples escaped into the lifecycle queue")
	}
}

func TestNodeLifecycleCollectionRetainsBufferedTerminalAfterExit(t *testing.T) {
	for range 100 {
		process := &nodeProcess{events: make(chan nodeEvent, 1), done: make(chan struct{})}
		close(process.done)
		collectNodeEvents(strings.NewReader("{\"schema\":\"ardents-node-event-v1\",\"kind\":\"lifecycle\",\"state\":\"WITHDRAWN\"}\n"), process)
		event, open := <-process.events
		if !open || event.State != "WITHDRAWN" {
			t.Fatal("process exit discarded an already emitted terminal event")
		}
	}
}

func TestNodeLifecycleCollectionRetainsFirstRouteDiagnostic(t *testing.T) {
	process := &nodeProcess{events: make(chan nodeEvent, 4), done: make(chan struct{})}
	input := strings.Join([]string{
		`{"schema":"ardents-node-event-v1","kind":"route-diagnostic","state":"FAILED","reason":"issuer-inner-tls-eof"}`,
		`{"schema":"ardents-node-event-v1","kind":"route-diagnostic","state":"FAILED","reason":"later-cleanup"}`,
	}, "\n") + "\n"
	collectNodeEvents(strings.NewReader(input), process)
	if got := process.firstRouteDiagnosticReason(); got != "issuer-inner-tls-eof" {
		t.Fatalf("first route diagnostic = %q", got)
	}
}

func TestNodeLifecycleCollectionRetainsBoundedDatedRouteDiagnostics(t *testing.T) {
	process := &nodeProcess{events: make(chan nodeEvent, 80), done: make(chan struct{})}
	var input strings.Builder
	for range 65 {
		input.WriteString(`{"schema":"ardents-node-event-v1","kind":"route-diagnostic","state":"FAILED","at":"2026-01-01T12:00:00Z","reason":"entry-carrier-tcp-dial-refused"}` + "\n")
	}
	collectNodeEvents(strings.NewReader(input.String()), process)
	observed, truncated := process.routeDiagnosticsAfter(time.Date(2026, time.January, 1, 11, 59, 59, 0, time.UTC))
	if !truncated || len(observed) != 64 || !observed[0].at.Equal(time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("bounded Route observations = %d, truncated %t", len(observed), truncated)
	}
	if _, lost := process.routeDiagnosticsAfter(time.Date(2026, time.January, 1, 12, 0, 1, 0, time.UTC)); lost {
		t.Fatal("older discarded diagnostics contaminated a later failure window")
	}
}
