//go:build linux && text_worker_installed

package state_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// The event timestamp is set by the Node before the collector reads stdout.
// Sorting observations from all Nodes avoids treating process-list order as
// causal order. These remain observations, not proof of the operation's cause.
func installedCommandRouteDiagnosticsSince(t *testing.T, sourcePlan map[string]any, since time.Time) string {
	t.Helper()
	processes, present := sourcePlan["diagnostic_node_processes"].([]*nodeProcess)
	if !present {
		return installedCommandRouteDiagnostics(t, sourcePlan)
	}
	type observed struct {
		at     time.Time
		node   int
		reason string
	}
	var events []observed
	truncated := false
	for index, process := range processes {
		collected, lost := process.routeDiagnosticsAfter(since)
		truncated = truncated || lost
		for _, event := range collected {
			events = append(events, observed{at: event.at, node: index + 1, reason: event.reason})
		}
	}
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].at.Equal(events[j].at) {
			return events[i].node < events[j].node
		}
		return events[i].at.Before(events[j].at)
	})
	var lines []string
	if truncated {
		lines = append(lines, "earlier diagnostics truncated")
	}
	for _, event := range events {
		lines = append(lines, fmt.Sprintf("%s node-%d %s", event.at.Format(time.RFC3339Nano), event.node, event.reason))
	}
	if len(lines) == 0 {
		return "none observed since prior successful Route command began"
	}
	return strings.Join(lines, "\n")
}

func TestInstalledCommandRouteDiagnosticsReportsOnlyFixedReason(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "route-diagnostic.json"), []byte(`{"kind":"route-diagnostic","reason":"issuer-outer-read-eof"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := installedCommandRouteDiagnostics(t, map[string]any{"route_diagnostic_paths": []string{directory}}); got != "issuer-outer-read-eof" {
		t.Fatalf("diagnostics = %q", got)
	}
}

func TestInstalledCommandRouteDiagnosticsPrefersFirstCollectedReason(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "route-diagnostic.json"), []byte(`{"kind":"route-diagnostic","reason":"later-cleanup"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	process := &nodeProcess{firstRouteDiagnostic: "issuer-inner-tls-eof"}
	if got := installedCommandRouteDiagnostics(t, map[string]any{
		"diagnostic_node_processes": []*nodeProcess{process},
		"route_diagnostic_paths":    []string{directory},
	}); got != "issuer-inner-tls-eof" {
		t.Fatalf("diagnostics = %q", got)
	}
}

func TestInstalledCommandRouteDiagnosticsOrderOnlyCurrentFailureWindow(t *testing.T) {
	cutoff := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)
	first := &nodeProcess{routeDiagnostics: []routeDiagnosticObservation{
		{at: cutoff.Add(-time.Second), reason: "unrelated-old-failure"},
		{at: cutoff.Add(3 * time.Second), reason: "entry-carrier-tcp-dial-refused"},
	}}
	second := &nodeProcess{routeDiagnostics: []routeDiagnosticObservation{
		{at: cutoff.Add(2 * time.Second), reason: "issuer-inner-tls-eof"},
	}}
	got := installedCommandRouteDiagnosticsSince(t, map[string]any{"diagnostic_node_processes": []*nodeProcess{first, second}}, cutoff)
	want := "2026-01-01T12:00:02Z node-2 issuer-inner-tls-eof\n2026-01-01T12:00:03Z node-1 entry-carrier-tcp-dial-refused"
	if got != want {
		t.Fatalf("diagnostics = %q, want %q", got, want)
	}
}
