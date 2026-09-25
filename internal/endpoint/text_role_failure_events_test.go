//go:build linux

package endpoint

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node"
)

type textRoleFailureEvent struct {
	At     time.Time `json:"at"`
	Kind   string    `json:"kind"`
	State  string    `json:"state"`
	Reason string    `json:"reason"`
}

type textRoleFailureEvents struct {
	mu        sync.Mutex
	events    []textRoleFailureEvent
	truncated bool
}

func (history *textRoleFailureEvents) record(event node.Event) {
	if event.Kind != "route-diagnostic" && (event.Kind != "lifecycle" || event.State != "FAILED") {
		return
	}
	if event.Kind == "route-diagnostic" && event.Reason == "" {
		return
	}
	reason := ""
	if event.Kind == "route-diagnostic" {
		reason = event.Reason // Node emits only fixed local Route categories.
	}
	history.mu.Lock()
	defer history.mu.Unlock()
	const limit = 64
	if len(history.events) == limit {
		copy(history.events, history.events[1:])
		history.events = history.events[:limit-1]
		history.truncated = true
	}
	history.events = append(history.events, textRoleFailureEvent{At: event.At, Kind: event.Kind, State: event.State, Reason: reason})
}

func (history *textRoleFailureEvents) summary() string {
	history.mu.Lock()
	defer history.mu.Unlock()
	raw, err := json.Marshal(struct {
		Truncated bool                   `json:"truncated"`
		Events    []textRoleFailureEvent `json:"events"`
	}{Truncated: history.truncated, Events: history.events})
	if err != nil {
		return "unavailable"
	}
	return string(raw)
}

func TestTextRoleFailureEventsRetainsOnlyBoundedSafeFields(t *testing.T) {
	history := &textRoleFailureEvents{}
	for range 65 {
		history.record(node.Event{Kind: "route-diagnostic", State: "FAILED", At: time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC), Reason: "issuer-inner-tls-eof", Assignment: "private-assignment"})
	}
	history.record(node.Event{Kind: "lifecycle", State: "FAILED", Reason: "private-os-error"})
	summary := history.summary()
	if strings.Contains(summary, "private-") || !strings.Contains(summary, `"truncated":true`) {
		t.Fatalf("unsafe or unbounded failure summary: %s", summary)
	}
	var decoded struct {
		Events []textRoleFailureEvent `json:"events"`
	}
	if err := json.Unmarshal([]byte(summary), &decoded); err != nil || len(decoded.Events) != 64 || decoded.Events[63].Reason != "" {
		t.Fatalf("failure event summary did not retain bounded terminal order: %d events, %v", len(decoded.Events), err)
	}
}
