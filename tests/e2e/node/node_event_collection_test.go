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
