//go:build linux

package endpoint

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node"
)

// networkNodeEvents retains only the bounded, public lifecycle categories
// needed to explain a failed multi-Node fixture. It never keeps resource
// samples, assignment digests, or raw network material.
type networkNodeEvents struct {
	mu     sync.Mutex
	recent []networkNodeEvent
}

type networkNodeEvent struct {
	at      time.Time
	kind    string
	state   string
	carrier string
	reason  string
}

const networkNodeEventLimit = 16

func (events *networkNodeEvents) record(event node.Event) {
	// The once-per-second sample would otherwise displace the READY and
	// terminal transitions while the other fixture Nodes are starting.
	if event.Kind == "resource-sample" {
		return
	}
	events.mu.Lock()
	defer events.mu.Unlock()
	if len(events.recent) == networkNodeEventLimit {
		copy(events.recent, events.recent[1:])
		events.recent = events.recent[:networkNodeEventLimit-1]
	}
	events.recent = append(events.recent, networkNodeEvent{
		at: event.At.UTC(), kind: event.Kind, state: event.State,
		carrier: event.CarrierProfile, reason: event.Reason,
	})
}

func (events *networkNodeEvents) snapshot() []networkNodeEvent {
	events.mu.Lock()
	defer events.mu.Unlock()
	return append([]networkNodeEvent(nil), events.recent...)
}

func TestTextNetworkNodeEventsKeepBoundedTransitionsAcrossSampling(t *testing.T) {
	var events networkNodeEvents
	for index := range 20 {
		events.record(node.Event{Kind: "lifecycle", State: strconv.Itoa(index)})
		for range 20 {
			events.record(node.Event{Kind: "resource-sample", State: "OBSERVED"})
		}
	}
	tail := events.snapshot()
	if len(tail) != networkNodeEventLimit || tail[0].state != "4" || tail[15].state != "19" {
		t.Fatalf("bounded lifecycle tail = %+v", tail)
	}
	tail[0].state = "changed"
	if events.snapshot()[0].state != "4" {
		t.Fatal("snapshot aliases retained diagnostic history")
	}
}
