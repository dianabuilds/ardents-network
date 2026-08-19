//go:build live

package network_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFinalSustainedTopologyProvidesEveryRoleTelemetryStream(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repositoryRoot(t), "tests", "live", "blocked-entry.compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, service := range []string{"endpoint-resource-collector", "endpoint-carrier-collector",
		"bridge-resource-collector", "bridge-carrier-collector", "publisher-resource-collector",
		"publisher-carrier-collector"} {
		if !bytes.Contains(raw, []byte("\n  "+service+":")) {
			t.Fatalf("sustained telemetry topology omits %s", service)
		}
	}
	if bytes.Count(raw, []byte("ARDENTS_BLOCKED_TIMELINE_MONOTONIC_ANCHOR_MILLIS")) < 6 {
		t.Fatal("sustained telemetry collectors do not share the manifest timeline")
	}
}

func TestFinalBridgeHelperMetricsExcludePublisherRole(t *testing.T) {
	bridge := writeFinalResourceStream(t, 1<<20, 3, 1, 2, 601)
	publisher := writeFinalResourceStream(t, 2<<20, 5, 2, 4, 601)
	value, err := mergeFinalBridgeHelperResources(bridge, finalResourceEvidence{}, 0, 600_000)
	if err != nil {
		t.Fatal(err)
	}
	if err := admitFinalPublisherResources(publisher, 0, 600_000); err != nil {
		t.Fatal(err)
	}
	if value.HelperRSSP95MiB != 1 || value.HelperFDPeak != 3 || value.HelperSocketPeak != 1 ||
		value.ThreadsPeak != 2 {
		t.Fatalf("Bridge helper resources include the Publisher role: %+v", value)
	}
}

func TestFinalSustainedResourcesRequireTenMinutePeriodicCoverage(t *testing.T) {
	short := writeFinalResourceStream(t, 1, 1, 1, 1, 600)
	if _, err := mergeFinalBridgeHelperResources(short, finalResourceEvidence{}, 0, 600_000); err == nil {
		t.Fatal("599 seconds plus a post-cleanup record satisfied the ten-minute resource window")
	}
}

func TestFinalBridgeHelperP95ExcludesSetupAndCleanupSamples(t *testing.T) {
	path := writeFinalWindowedResourceStream(t)
	value, err := mergeFinalBridgeHelperResources(path, finalResourceEvidence{}, 100_000, 700_000)
	if err != nil {
		t.Fatal(err)
	}
	if value.HelperRSSP95MiB != 10 {
		t.Fatalf("active-window helper p95=%v", value.HelperRSSP95MiB)
	}
}

func writeFinalResourceStream(t *testing.T, rss uint64, fds, sockets, threads uint16, periodic int) string {
	t.Helper()
	var raw strings.Builder
	encoder := json.NewEncoder(&raw)
	for ordinal := range periodic {
		value := blockedProcessSample{Schema: "ardents-h3-process-resource-v1", Ordinal: uint16(ordinal),
			OffsetMillis: uint64(ordinal * 1_000), RSSBytes: rss, FDs: fds, Sockets: sockets,
			Processes: 2, Threads: threads}
		if err := encoder.Encode(value); err != nil {
			t.Fatal(err)
		}
	}
	if err := encoder.Encode(blockedProcessSample{Schema: "ardents-h3-process-resource-v1", Ordinal: uint16(periodic),
		OffsetMillis: uint64(periodic * 1_000), RSSBytes: rss, FDs: fds, Sockets: sockets, Processes: 2,
		Threads: threads, Boundary: "post-cleanup"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "resource.jsonl")
	if err := os.WriteFile(path, []byte(raw.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeFinalWindowedResourceStream(t *testing.T) string {
	t.Helper()
	var raw strings.Builder
	encoder := json.NewEncoder(&raw)
	for ordinal := range 701 {
		rss := uint64(1 << 20)
		if ordinal >= 669 && ordinal < 700 {
			rss = 10 << 20
		}
		if err := encoder.Encode(blockedProcessSample{Schema: "ardents-h3-process-resource-v1",
			Ordinal: uint16(ordinal), OffsetMillis: uint64(ordinal * 1_000), RSSBytes: rss, Processes: 2}); err != nil {
			t.Fatal(err)
		}
	}
	if err := encoder.Encode(blockedProcessSample{Schema: "ardents-h3-process-resource-v1", Ordinal: 701,
		OffsetMillis: 701_000, Processes: 2, Boundary: "post-cleanup"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "resource.jsonl")
	if err := os.WriteFile(path, []byte(raw.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
