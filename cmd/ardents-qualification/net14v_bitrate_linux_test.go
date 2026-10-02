//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
)

func qualificationNET14VBitratePair(t *testing.T, rates map[string]uint64, bursts int) (qualificationNetworkManifest, qualificationNetworkManifest, pairedWorkloadVerdict, pairedWorkloadVerdict) {
	t.Helper()
	manifest, _ := qualificationNodeManifest()
	manifest.Relays = qualificationRelays(manifest)
	recovery := manifest
	recovery.Cell = "net14-recovery"
	for index, at := range []uint64{20_000, 200_000, 400_000} {
		recovery.Failures = append(recovery.Failures, networkFailure{Episode: fmt.Sprintf("fault-%d", index), SegmentID: manifest.Paths[0].Segments[2].ID, AtMillis: at, DurationMillis: 10_000})
	}
	origin := time.Unix(10_000, 0).UTC()
	identity := strings.Repeat("ab", 32)
	pair := pairedWorkloadVerdict{Kind: "paired-workload", Seed: identity, CandidateSHA256: identity, EndpointUnitSHA256: identity, Profile: streamqualification.ClientToPublisher, Condition: streamqualification.NormalNetwork,
		ReaderNetwork: ownerNetworkVerdict{Started: origin, Stopped: origin.Add(600 * time.Second), DirectionalUseful: 1}, PublisherNetwork: ownerNetworkVerdict{Started: origin, Stopped: origin.Add(600 * time.Second), DirectionalUseful: 1},
		Relay: relayTrafficVerdict{BinarySHA256: identity}, NodeOwners: nodeOwnersVerdict{InventorySHA256: identity, BinarySHA256: identity}, Criteria: []streamqualification.Criterion{{Name: "fixture-prior-evidence", Passed: true}},
	}
	endpoints := map[string]*relayNodeTraffic{userEndpoint: {ID: userEndpoint}, publisherEndpoint: {ID: publisherEndpoint}}
	for _, path := range manifest.Paths {
		for _, segment := range path.Segments {
			observed := relaySegmentTraffic{ID: segment.ID, From: segment.From, To: segment.To}
			key := ""
			if segment.From == userEndpoint {
				key = "reader-tx"
			}
			if segment.To == userEndpoint {
				key = "reader-rx"
			}
			if segment.From == publisherEndpoint {
				key = "publisher-tx"
			}
			if segment.To == publisherEndpoint {
				key = "publisher-rx"
			}
			for second := 0; second <= 600; second++ {
				rate := uint64(8)
				if value, exists := rates[key]; exists && (bursts == 0 || second > 600-bursts) {
					rate = value
				}
				if second > 0 {
					observed.Bytes += rate / 8
				}
				observed.Samples = append(observed.Samples, relaySegmentSample{At: origin.Add(time.Duration(second) * time.Second), Elapsed: time.Duration(second) * time.Second, Bytes: observed.Bytes})
			}
			if owner := endpoints[segment.From]; owner != nil {
				owner.Tx += observed.Bytes
			}
			if owner := endpoints[segment.To]; owner != nil {
				owner.Rx += observed.Bytes
			}
			pair.Relay.Segments = append(pair.Relay.Segments, observed)
		}
	}
	pair.Relay.Endpoints = []relayNodeTraffic{*endpoints[userEndpoint], *endpoints[publisherEndpoint]}
	episode := pair
	episode.Condition = streamqualification.RecoveryNetwork
	return manifest, recovery, pair, episode
}

func qualificationNET14VCLI(t *testing.T, manifest, recovery qualificationNetworkManifest, baseline, episode pairedWorkloadVerdict) (net14vVerdict, error) {
	t.Helper()
	baselinePath, episodePath := writeQualificationJSON(t, manifest), writeQualificationJSON(t, recovery)
	_, baselineHash, err := readNetworkManifest(baselinePath)
	if err != nil {
		t.Fatal(err)
	}
	_, episodeHash, err := readNetworkManifest(episodePath)
	if err != nil {
		t.Fatal(err)
	}
	baseline.Relay.ManifestSHA256, episode.Relay.ManifestSHA256 = baselineHash, episodeHash
	var faults bytes.Buffer
	encoder := json.NewEncoder(&faults)
	for _, failure := range recovery.Failures {
		at := int64(10_000_000 + failure.AtMillis)
		for _, record := range []recoveryFaultRecord{{Host: "reader", ManifestSHA256: episodeHash, RunStartedMillis: episode.ReaderNetwork.Started.UnixMilli(), Kind: "recovery-fault-start", Episode: failure.Episode, Segment: failure.SegmentID, ScheduledMillis: at, ActualMillis: at}, {Host: "reader", ManifestSHA256: episodeHash, RunStartedMillis: episode.ReaderNetwork.Started.UnixMilli(), Kind: "recovery-fault-stop", Episode: failure.Episode, Segment: failure.SegmentID, ScheduledMillis: at + int64(failure.DurationMillis), ActualMillis: at + int64(failure.DurationMillis)}} {
			if err := encoder.Encode(record); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := encoder.Encode(recoveryFaultRecord{Kind: "recovery-faults-complete", Host: "reader", ManifestSHA256: episodeHash, RunStartedMillis: episode.ReaderNetwork.Started.UnixMilli(), ActualMillis: episode.ReaderNetwork.Started.Add(420 * time.Second).UnixMilli(), Episodes: len(recovery.Failures)}); err != nil {
		t.Fatal(err)
	}
	faultPath := t.TempDir() + "/faults.jsonl"
	if err := os.WriteFile(faultPath, faults.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err = run(context.Background(), []string{"verify-net14v", baselinePath, episodePath, writeQualificationJSON(t, baseline), writeQualificationJSON(t, episode), faultPath}, &output)
	var verdict net14vVerdict
	if decodeErr := json.Unmarshal(output.Bytes(), &verdict); decodeErr != nil {
		t.Fatalf("CLI did not emit verdict: %v / %v / %s", err, decodeErr, &output)
	}
	return verdict, err
}

func TestQualificationNET14VCLIDirectionalPercentiles(t *testing.T) {
	for _, test := range []struct {
		name, direction string
		rate            uint64
		bursts          int
		pass            bool
		bound           float64
	}{
		{"reader-18M", "reader-tx", 18_000_000, 0, false, 16_000_000},
		{"publisher-tx-30M", "publisher-tx", 30_000_000, 0, false, 25_000_000},
		{"publisher-rx-30M", "publisher-rx", 30_000_000, 0, false, 25_000_000},
		{"reader-rx-30M", "reader-rx", 30_000_000, 0, false, 25_000_000},
		{"reader-below", "reader-tx", 15_999_992, 0, true, 16_000_000},
		{"reader-exact", "reader-tx", 16_000_000, 0, true, 16_000_000},
		{"publisher-exact", "publisher-tx", 25_000_000, 0, true, 25_000_000},
		{"publisher-receive-access-budget", "publisher-rx", 18_000_000, 0, true, 25_000_000},
		{"reader-rx-exact", "reader-rx", 25_000_000, 0, true, 25_000_000},
		{"low-mean-bursts", "reader-tx", 18_000_000, 31, false, 16_000_000},
		{"nearest-rank-five-percent", "reader-tx", 18_000_000, 30, true, 16_000_000},
	} {
		t.Run(test.name, func(t *testing.T) {
			manifest, recovery, baseline, episode := qualificationNET14VBitratePair(t, map[string]uint64{test.direction: test.rate}, test.bursts)
			verdict, err := qualificationNET14VCLI(t, manifest, recovery, baseline, episode)
			if (err == nil) != test.pass {
				t.Fatalf("pass=%v err=%v criteria=%+v", test.pass, err, verdict.Criteria)
			}
			found := false
			for _, criterion := range verdict.Criteria {
				if criterion.Name == test.direction+"-p95-one-second-carrier-bitrate" {
					found = true
					if criterion.Passed != test.pass || criterion.Bound != test.bound {
						t.Fatalf("wrong directional verdict: %+v", criterion)
					}
				}
			}
			if !found {
				t.Fatal("missing percentile criterion")
			}
		})
	}
}

func TestQualificationNET14VCLIInvalidSeries(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*relaySegmentTraffic)
	}{
		{"missing", func(s *relaySegmentTraffic) { s.Samples = nil }},
		{"dropped", func(s *relaySegmentTraffic) { s.Samples = append(s.Samples[:20], s.Samples[21:]...) }},
		{"duplicate", func(s *relaySegmentTraffic) { s.Samples[20] = s.Samples[19] }},
		{"regression", func(s *relaySegmentTraffic) { s.Samples[20].Bytes = 0 }},
		{"uneven", func(s *relaySegmentTraffic) { s.Samples[20].Elapsed += 600 * time.Millisecond }},
		{"uneven-within-old-tolerance", func(s *relaySegmentTraffic) {
			elapsed := time.Duration(0)
			for i := range s.Samples {
				if i > 0 {
					interval := 900 * time.Millisecond
					if i%6 == 0 {
						interval = 1500 * time.Millisecond
					}
					elapsed += interval
				}
				s.Samples[i].Elapsed = elapsed
			}
		}},
		{"missing-monotonic", func(s *relaySegmentTraffic) {
			for i := range s.Samples {
				s.Samples[i].Elapsed = 0
			}
		}},
		{"shortened-monotonic", func(s *relaySegmentTraffic) {
			for i := range s.Samples {
				s.Samples[i].Elapsed = time.Duration(i) * 999 * time.Millisecond
			}
		}},
		{"late-start", func(s *relaySegmentTraffic) { s.Samples = s.Samples[1:] }},
		{"early-stop", func(s *relaySegmentTraffic) { s.Samples = s.Samples[:600] }},
		{"terminal-counter", func(s *relaySegmentTraffic) { s.Bytes-- }},
		{"wrong-direction", func(s *relaySegmentTraffic) { s.From = publisherEndpoint }},
	} {
		t.Run(test.name, func(t *testing.T) {
			manifest, recovery, baseline, episode := qualificationNET14VBitratePair(t, nil, 0)
			for i := range episode.Relay.Segments {
				if episode.Relay.Segments[i].From == userEndpoint {
					test.change(&episode.Relay.Segments[i])
					break
				}
			}
			verdict, err := qualificationNET14VCLI(t, manifest, recovery, baseline, episode)
			if err == nil {
				t.Fatal("invalid series passed CLI")
			}
			for _, c := range verdict.Criteria {
				if c.Name == "reader-tx-p95-one-second-carrier-bitrate" && c.Passed {
					t.Fatal("invalid series passed p95 criterion")
				}
			}
		})
	}
}

func TestQualificationNET14VCLICadenceSchedulingJitter(t *testing.T) {
	manifest, recovery, baseline, episode := qualificationNET14VBitratePair(t, nil, 0)
	for segment := range episode.Relay.Segments {
		elapsed := time.Duration(0)
		for index := range episode.Relay.Segments[segment].Samples {
			if index > 0 {
				interval := 950 * time.Millisecond
				if index%2 == 0 {
					interval = 1050 * time.Millisecond
				}
				elapsed += interval
			}
			episode.Relay.Segments[segment].Samples[index].Elapsed = elapsed
		}
	}
	if verdict, err := qualificationNET14VCLI(t, manifest, recovery, baseline, episode); err != nil {
		t.Fatalf("bounded scheduling jitter refused: %v / %+v", err, verdict.Criteria)
	}
}
