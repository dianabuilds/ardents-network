//go:build linux

package main

import (
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
)

func failedNET14VFixture() (qualificationNetworkManifest, qualificationNetworkManifest, pairedWorkloadVerdict, []failedRelayResult, map[string]recoveryFaultRecord, map[string]recoveryFaultRecord) {
	origin := time.Unix(1_800_000_000, 0).UTC()
	failure := networkFailure{Episode: "drop-a", SegmentID: "segment-a", AtMillis: 20_000, DurationMillis: 10_000}
	relay := networkRelay{ID: "relay-a", Host: "reader", Container: "relay-a", UpstreamSegment: failure.SegmentID, ClientSegment: "segment-b"}
	baselineManifest := qualificationNetworkManifest{Cell: "net14ad", Carrier: "tcp-tls", Relays: []networkRelay{relay}}
	episodeManifest := baselineManifest
	episodeManifest.Cell, episodeManifest.Failures = "net14-recovery", []networkFailure{failure}
	binary := strings.Repeat("a1", 32)
	baseline := pairedWorkloadVerdict{Condition: streamqualification.NormalNetwork,
		ReaderNetwork: ownerNetworkVerdict{Started: origin},
		Relay: relayTrafficVerdict{BinarySHA256: binary, Segments: []relaySegmentTraffic{{ID: failure.SegmentID, Samples: []relaySegmentSample{
			{At: origin.Add(19 * time.Second), Bytes: 100}, {At: origin.Add(38 * time.Second), Bytes: (20 << 20) + 100},
		}}}}}
	samples := make([]relayCounterSample, 0, 21)
	for second := 19; second <= 39; second++ {
		progress := min(second-19, 19)
		samples = append(samples, relayCounterSample{At: origin.Add(time.Duration(second) * time.Second), UpstreamBytes: 100 + uint64(progress)*(28<<20)/19})
	}
	relays := []failedRelayResult{{ID: relay.ID, Host: relay.Host, Container: relay.Container, UpstreamSegment: relay.UpstreamSegment, ClientSegment: relay.ClientSegment, BinarySHA256: binary,
		TrafficControl: []string{`[{"kind":"htb","handle":"1:10","bytes":50331748},{"kind":"htb","handle":"1:20","bytes":0}]`}, Samples: samples}}
	starts := map[string]recoveryFaultRecord{failure.Episode: {Episode: failure.Episode, ActualMillis: origin.Add(20 * time.Second).UnixMilli()}}
	stops := map[string]recoveryFaultRecord{failure.Episode: {Episode: failure.Episode, ActualMillis: origin.Add(30 * time.Second).UnixMilli()}}
	return baselineManifest, episodeManifest, baseline, relays, starts, stops
}

func TestFailedNET14VRetainsAddedEpisodeBound(t *testing.T) {
	baselineManifest, episodeManifest, baseline, relays, starts, stops := failedNET14VFixture()
	criteria := evaluateFailedNET14V(baselineManifest, episodeManifest, baseline, relays, starts, stops)
	if len(criteria) != 2 || !criteria[0].Passed || !criteria[1].Passed {
		t.Fatalf("expected exact added-byte bound to pass: %+v", criteria)
	}
	for index := len(relays[0].Samples) - 3; index < len(relays[0].Samples); index++ {
		relays[0].Samples[index].UpstreamBytes += 4 << 20
	}
	criteria = evaluateFailedNET14V(baselineManifest, episodeManifest, baseline, relays, starts, stops)
	if criteria[1].Passed {
		t.Fatalf("failed attempt above eight MiB addition was accepted: %+v", criteria)
	}
}

func TestFailedNET14VRejectsUnboundOrIncompleteRelayEvidence(t *testing.T) {
	baselineManifest, episodeManifest, baseline, relays, starts, stops := failedNET14VFixture()
	relays[0].ID = "substituted"
	criteria := evaluateFailedNET14V(baselineManifest, episodeManifest, baseline, relays, starts, stops)
	if criteria[0].Passed {
		t.Fatalf("substituted relay was accepted: %+v", criteria)
	}
	baselineManifest, episodeManifest, baseline, relays, starts, stops = failedNET14VFixture()
	relays[0].Samples = relays[0].Samples[:1]
	criteria = evaluateFailedNET14V(baselineManifest, episodeManifest, baseline, relays, starts, stops)
	if criteria[0].Passed || criteria[1].Passed {
		t.Fatalf("incomplete samples were accepted: %+v", criteria)
	}
}
