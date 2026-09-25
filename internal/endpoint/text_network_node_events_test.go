//go:build linux

package endpoint

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node"
)

// textNetworkNodeEvents retains only the bounded, public lifecycle categories
// needed to explain a failed multi-Node fixture. It never keeps resource
// samples, assignment digests, or raw network material.
type textNetworkNodeEvents struct {
	mu     sync.Mutex
	recent []textNetworkNodeEvent
}

type textNetworkNodeEvent struct {
	at      time.Time
	kind    string
	state   string
	carrier string
	reason  string
}

const textNetworkNodeEventLimit = 16

func (events *textNetworkNodeEvents) record(event node.Event) {
	// The once-per-second sample would otherwise displace the READY and
	// terminal transitions while the other fixture Nodes are starting.
	if event.Kind == "resource-sample" {
		return
	}
	events.mu.Lock()
	defer events.mu.Unlock()
	if len(events.recent) == textNetworkNodeEventLimit {
		copy(events.recent, events.recent[1:])
		events.recent = events.recent[:textNetworkNodeEventLimit-1]
	}
	events.recent = append(events.recent, textNetworkNodeEvent{
		at: event.At.UTC(), kind: event.Kind, state: event.State,
		carrier: event.CarrierProfile, reason: event.Reason,
	})
}

func (events *textNetworkNodeEvents) snapshot() []textNetworkNodeEvent {
	events.mu.Lock()
	defer events.mu.Unlock()
	return append([]textNetworkNodeEvent(nil), events.recent...)
}

func TestTextNetworkNodeEventsKeepBoundedTransitionsAcrossSampling(t *testing.T) {
	var events textNetworkNodeEvents
	for index := range 20 {
		events.record(node.Event{Kind: "lifecycle", State: strconv.Itoa(index)})
		for range 20 {
			events.record(node.Event{Kind: "resource-sample", State: "OBSERVED"})
		}
	}
	tail := events.snapshot()
	if len(tail) != textNetworkNodeEventLimit || tail[0].state != "4" || tail[15].state != "19" {
		t.Fatalf("bounded lifecycle tail = %+v", tail)
	}
	tail[0].state = "changed"
	if events.snapshot()[0].state != "4" {
		t.Fatal("snapshot aliases retained diagnostic history")
	}
}
