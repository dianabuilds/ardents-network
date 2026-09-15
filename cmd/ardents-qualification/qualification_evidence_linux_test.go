//go:build linux

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/resource"
)

func TestQualificationNodeResultsRequireEveryBoundedRouteOwner(t *testing.T) {
	manifest, ids := qualificationNodeManifest()
	for index := 6; index <= 16; index++ {
		ids = append(ids, fmt.Sprintf("%064x", index))
	}
	inputs := qualificationNodeInputs(t, ids)
	sources := qualificationSourceInputs(t)
	inventory := strings.Repeat("ef", 32)
	ownerSlices := qualificationOwnerSliceInputs()
	owners := map[streamqualification.Role]ownerNetworkVerdict{
		streamqualification.ReaderRole:    {Started: time.Unix(1000, 0).UTC(), Stopped: time.Unix(1597, 0).UTC(), P95RSSBytes: 64 << 20, MeanCPUPercent: 5},
		streamqualification.PublisherRole: {Started: time.Unix(1000, 0).UTC(), Stopped: time.Unix(1597, 0).UTC(), P95RSSBytes: 128 << 20, MeanCPUPercent: 10},
	}
	path := writeQualificationJSON(t, nodeResultsInput{InventorySHA256: inventory, OwnerSlices: ownerSlices, Nodes: inputs, Sources: sources})
	verdict, criteria, err := readNodeResults(path, manifest, inventory, owners)
	if err != nil || !streamqualification.CriteriaPassed(criteria) || len(verdict.Nodes) != 16 {
		t.Fatalf("complete Node owner evidence refused: verdict=%+v criteria=%+v err=%v", verdict, criteria, err)
	}
	if _, criteria, err := readNodeResults(path, manifest, strings.Repeat("de", 32), owners); err == nil || streamqualification.CriteriaPassed(criteria) {
		t.Fatal("Node owner evidence accepted under a different inventory digest")
	}
	missingPath := writeQualificationJSON(t, nodeResultsInput{InventorySHA256: inventory, OwnerSlices: ownerSlices, Nodes: inputs[:15], Sources: sources})
	if _, criteria, err := readNodeResults(missingPath, manifest, inventory, owners); err == nil || streamqualification.CriteriaPassed(criteria) {
		t.Fatal("Node owner evidence accepted without the sixteenth owner")
	}
	missingSourcePath := writeQualificationJSON(t, nodeResultsInput{InventorySHA256: inventory, OwnerSlices: ownerSlices, Nodes: inputs, Sources: sources[:1]})
	if _, criteria, err := readNodeResults(missingSourcePath, manifest, inventory, owners); err == nil || streamqualification.CriteriaPassed(criteria) {
		t.Fatal("Node owner evidence accepted without both live State Sources")
	}
	for owner := 0; owner < 3; owner++ {
		setQualificationNodeRSS(t, &inputs[owner], 160<<20)
	}
	path = writeQualificationJSON(t, nodeResultsInput{InventorySHA256: inventory, OwnerSlices: ownerSlices, Nodes: inputs, Sources: sources})
	if _, criteria, err := readNodeResults(path, manifest, inventory, owners); err == nil || streamqualification.CriteriaPassed(criteria) {
		t.Fatal("aggregate User owner RSS accepted above its whole-owner limit")
	}
	for owner := 0; owner < 3; owner++ {
		setQualificationNodeRSS(t, &inputs[owner], 8<<20)
	}
	for owner := 3; owner < len(inputs); owner++ {
		for index := range inputs[owner].Samples {
			inputs[owner].Samples[index].CPUUsageNSec = uint64(index) * 80_000_000
		}
	}
	path = writeQualificationJSON(t, nodeResultsInput{InventorySHA256: inventory, OwnerSlices: ownerSlices, Nodes: inputs, Sources: sources})
	if _, criteria, err := readNodeResults(path, manifest, inventory, owners); err == nil || streamqualification.CriteriaPassed(criteria) {
		t.Fatal("aggregate Publisher owner CPU accepted above its whole-owner limit")
	}
	for owner := 3; owner < len(inputs); owner++ {
		for index := range inputs[owner].Samples {
			inputs[owner].Samples[index].CPUUsageNSec = uint64(index) * 10_000_000
		}
	}
	for index := range inputs[0].Samples {
		inputs[0].Samples[index].MemoryCurrent = 513 << 20
	}
	path = writeQualificationJSON(t, nodeResultsInput{InventorySHA256: inventory, OwnerSlices: ownerSlices, Nodes: inputs, Sources: sources})
	if _, criteria, err := readNodeResults(path, manifest, inventory, owners); err == nil || streamqualification.CriteriaPassed(criteria) {
		t.Fatal("over-budget User-side Node owner accepted")
	}
}

func TestQualificationRelayResultsBindAllDirectionalSegments(t *testing.T) {
	manifest, _ := qualificationNodeManifest()
	manifest.Relays = qualificationRelays(manifest)
	binary := strings.Repeat("ab", 32)
	inputs := make([]relayResultInput, 0, 6)
	for _, relay := range manifest.Relays {
		samples := make([]relayCounterSample, 598)
		for index := range samples {
			samples[index] = relayCounterSample{At: time.Unix(int64(index), 0).UTC(), UpstreamBytes: 50, ClientBytes: 40}
		}
		inputs = append(inputs, relayResultInput{ID: relay.ID, Host: relay.Host, Container: relay.Container,
			UpstreamSegment: relay.UpstreamSegment, ClientSegment: relay.ClientSegment, BinarySHA256: binary,
			UpstreamBytes: 90, ClientBytes: 80, Samples: samples, TrafficControl: []string{`[{"kind":"htb","handle":"1:10","bytes":100},{"kind":"htb","handle":"1:20","stats":{"bytes":100}}]`}})
	}
	path := writeQualificationJSON(t, inputs)
	verdict, criteria, err := readRelayResults(path, manifest, strings.Repeat("cd", 32))
	if err != nil || !streamqualification.CriteriaPassed(criteria) || len(verdict.Segments) != 12 || len(verdict.Nodes) != 5 || len(verdict.Endpoints) != 2 {
		t.Fatalf("complete relay evidence refused: verdict=%+v criteria=%+v err=%v", verdict, criteria, err)
	}
	inputs[0].TrafficControl = nil
	path = writeQualificationJSON(t, inputs)
	if _, criteria, err := readRelayResults(path, manifest, strings.Repeat("cd", 32)); err != nil || streamqualification.CriteriaPassed(criteria) {
		t.Fatalf("incomplete relay counters were not represented as failed criteria: %v", err)
	}
}

func TestQualificationNET14VRejectsExpensiveIndividualEpisode(t *testing.T) {
	origin := time.Unix(10_000, 0).UTC()
	failure := networkFailure{Episode: "relay-loss", SegmentID: "segment-1", AtMillis: 20_000, DurationMillis: 10_000}
	baseline := pairedWorkloadVerdict{ReaderNetwork: ownerNetworkVerdict{Started: origin}, Relay: relayTrafficVerdict{Segments: []relaySegmentTraffic{{ID: failure.SegmentID, Samples: []relaySegmentSample{{At: origin.Add(20 * time.Second), Bytes: 100}, {At: origin.Add(38 * time.Second), Bytes: 1 << 20}}}}}}
	episode := pairedWorkloadVerdict{ReaderNetwork: ownerNetworkVerdict{Started: origin}, Relay: relayTrafficVerdict{Segments: []relaySegmentTraffic{{ID: failure.SegmentID, Samples: []relaySegmentSample{{At: origin.Add(20 * time.Second), Bytes: 100}, {At: origin.Add(38 * time.Second), Bytes: 10 << 20}}}}}}
	criteria := relayEpisodeCriteria(baseline, episode, []networkFailure{failure}, true)
	if len(criteria) != 1 || criteria[0].Passed || criteria[0].Observed <= 8<<20 {
		t.Fatalf("expensive recovery episode accepted: %+v", criteria)
	}
}
func TestQualificationNET14VReusesPassedNormalEvidence(t *testing.T) {
	failure := networkFailure{Episode: "relay-loss", SegmentID: "segment-1", AtMillis: 20_000, DurationMillis: 10_000}
	baselineManifest := qualificationNetworkManifest{Version: 1, Cell: "net14ad", Carrier: "tcp-tls"}
	episodeManifest := baselineManifest
	episodeManifest.Cell = "net14-recovery"
	episodeManifest.Failures = []networkFailure{failure}
	segments := make([]relaySegmentTraffic, 12)
	for index := range segments {
		segments[index].ID = fmt.Sprintf("segment-%d", index+1)
	}
	baseline := pairedWorkloadVerdict{
		Profile: streamqualification.ClientToPublisher, Condition: streamqualification.NormalNetwork,
		ReaderNetwork: ownerNetworkVerdict{DirectionalUseful: 1}, PublisherNetwork: ownerNetworkVerdict{DirectionalUseful: 1},
		Relay: relayTrafficVerdict{Segments: segments},
	}
	episode := baseline
	episode.Condition = streamqualification.RecoveryNetwork
	criteria := evaluateNET14V(baselineManifest, episodeManifest, baseline, episode)
	if len(criteria) == 0 || criteria[0].Name != "net14v-comparable-pair" || !criteria[0].Passed {
		t.Fatalf("passed NET-14AD evidence was not reused as the recovery baseline: %+v", criteria)
	}
	baseline.Condition = streamqualification.RecoveryNetwork
	criteria = evaluateNET14V(baselineManifest, episodeManifest, baseline, episode)
	if criteria[0].Passed {
		t.Fatal("NET-14V accepted a redundant non-normal baseline")
	}
}
func qualificationOwnerSliceInputs() []ownerSliceResultInput {
	origin := time.Unix(1000, 0).UTC()
	inputs := make([]ownerSliceResultInput, 0, 2)
	for index, host := range []string{"reader", "publisher"} {
		quota, cpuMax, memoryMax := "50%", "50000 100000", uint64(512<<20)
		cpuStep := uint64(2_000_000)
		if host == "publisher" {
			quota, cpuMax, memoryMax, cpuStep = "100%", "100000 100000", uint64(1<<30), uint64(4_000_000)
		}
		input := ownerSliceResultInput{Host: host, Unit: qualificationOwnerSlice, CPUQuota: quota, CPUMax: cpuMax, MemoryMax: memoryMax,
			Receipt: []string{"ActiveState=active", "ControlGroup=" + qualificationOwnerControlGroup, "CPU_MAX=" + cpuMax,
				"MEMORY_MAX=" + strconv.FormatUint(memoryMax, 10), "IPAccounting=yes", "DropInPaths=/run/systemd/system.control/ardents-qualification-owner.slice.d/50-CPUQuota.conf"}}
		for second := 0; second < 598; second++ {
			input.Samples = append(input.Samples, nodeOwnerSampleInput{At: origin.Add(time.Duration(second) * time.Second),
				MemoryCurrent: uint64(64+index*64) << 20, CPUUsageNSec: uint64(second) * cpuStep,
				IPIngressBytes: uint64(second) * 10_000, IPEgressBytes: uint64(second) * 20_000})
		}
		inputs = append(inputs, input)
	}
	return inputs
}

func qualificationNodeInputs(t *testing.T, ids []string) []nodeResultInput {
	t.Helper()
	origin := time.Unix(1000, 0).UTC()
	policy := resource.HostingPolicy{Provider: "provider", Start: origin.Add(-time.Hour), End: origin.Add(time.Hour), Unit: "byte", Quantity: 1 << 40, Directions: "tx+rx", Interfaces: []string{"eth0"}, LowWatermarkBytes: 1}
	inputs := make([]nodeResultInput, 0, len(ids))
	for index, id := range ids {
		host := "publisher"
		memory := uint64(8 << 20)
		if index < 3 {
			host = "reader"
		}
		input := nodeResultInput{ID: id, Host: host, PlanSHA256: fmt.Sprintf("%064x", index+1), InvocationID: fmt.Sprintf("%032x", index+1), BinarySHA256: strings.Repeat("ab", 32), ActiveState: "inactive", Result: "success", Slice: qualificationOwnerSlice}
		ready, _ := json.Marshal(node.Event{Schema: "ardents-node-event-v1", Kind: "lifecycle", State: "READY", At: origin})
		input.Journal = append(input.Journal, string(ready))
		for second := 0; second < 598; second++ {
			at := origin.Add(time.Duration(second) * time.Second)
			hosting := resource.HostingSample{At: at, Policy: policy, Observation: resource.HostingObservation{UsedBytes: uint64(second)}}
			event, _ := json.Marshal(node.Event{Schema: "ardents-node-event-v1", Kind: "resource-sample", State: "OBSERVED", At: at, Resource: &resource.Sample{MemoryBytes: memory, RSSBytes: memory}, Hosting: &hosting})
			input.Journal = append(input.Journal, string(event))
			input.Samples = append(input.Samples, nodeOwnerSampleInput{At: at, MemoryCurrent: memory, CPUUsageNSec: uint64(second) * 10_000_000, IPIngressBytes: uint64(second) * 1000, IPEgressBytes: uint64(second) * 2000})
		}
		withdrawn, _ := json.Marshal(node.Event{Schema: "ardents-node-event-v1", Kind: "lifecycle", State: "WITHDRAWN", At: origin.Add(598 * time.Second)})
		input.Journal = append(input.Journal, string(withdrawn))
		inputs = append(inputs, input)
	}
	return inputs
}

func qualificationSourceInputs(t *testing.T) []nodeResultInput {
	t.Helper()
	origin := time.Unix(1000, 0).UTC()
	inputs := make([]nodeResultInput, 0, 2)
	for index, host := range []string{"reader", "publisher"} {
		input := nodeResultInput{ID: fmt.Sprintf("%064x", 17+index), Host: host, PlanSHA256: fmt.Sprintf("%064x", 17+index),
			InvocationID: fmt.Sprintf("%032x", 17+index), BinarySHA256: strings.Repeat("ab", 32),
			ActiveState: "inactive", Result: "success", Slice: qualificationOwnerSlice}
		ready, _ := json.Marshal(map[string]any{"schema": "ardents-source-event-v1", "kind": "source-ready"})
		input.Journal = append(input.Journal, string(ready))
		for second := 0; second < 598; second++ {
			at := origin.Add(time.Duration(second) * time.Second)
			event, _ := json.Marshal(map[string]any{"schema": "ardents-h3-resource-sample-v1", "kind": "resource-sample",
				"at": at, "resource": resource.Sample{MemoryBytes: 4 << 20, RSSBytes: 4 << 20}})
			input.Journal = append(input.Journal, string(event))
			input.Samples = append(input.Samples, nodeOwnerSampleInput{At: at, MemoryCurrent: 4 << 20,
				CPUUsageNSec: uint64(second) * 1_000_000, IPIngressBytes: uint64(second) * 100, IPEgressBytes: uint64(second) * 200})
		}
		inputs = append(inputs, input)
	}
	return inputs
}
func qualificationNodeManifest() (qualificationNetworkManifest, []string) {
	ids := []string{strings.Repeat("1", 64), strings.Repeat("2", 64), strings.Repeat("3", 64), strings.Repeat("4", 64), strings.Repeat("5", 64)}
	forward := append(append([]string{userEndpoint}, ids...), publisherEndpoint)
	reverse := []string{publisherEndpoint, ids[4], ids[3], ids[2], ids[1], ids[0], userEndpoint}
	path := func(name string, points []string, reversePath bool) networkPath {
		segments := make([]networkSegment, 0, 6)
		for index := 0; index < 6; index++ {
			physical := index
			if reversePath {
				physical = 5 - index
			}
			delay := uint64(6666)
			if physical == 5 {
				delay = 6670
			}
			loss := uint64(0)
			if physical == 2 {
				loss = 1000
			}
			segments = append(segments, networkSegment{ID: fmt.Sprintf("%s-%d", name, index), From: points[index], To: points[index+1],
				DelayMicros: delay, LossPartsPerMillion: loss})
		}
		capBits := uint64(20_000_000)
		if reversePath {
			capBits = 100_000_000
		}
		return networkPath{Name: name, CapBitsPerS: capBits, Segments: segments}
	}
	manifest := qualificationNetworkManifest{Version: 1, Cell: "net14ad", Carrier: "tcp-tls",
		Generator: networkGenerator{Algorithm: "linux-netem", Version: "qualification-test-v1"}, SeedSchedule: []string{strings.Repeat("01", 32)},
		Paths: []networkPath{path("user-to-publisher", forward, false), path("publisher-to-user", reverse, true)}}
	return manifest, ids
}

func qualificationRelays(manifest qualificationNetworkManifest) []networkRelay {
	relays := make([]networkRelay, 0, 6)
	targetPorts := [...]int{49000, 49001, 49002, 49002, 49003, 49004}
	for index := 0; index < 6; index++ {
		host, publicHost := "publisher", "192.0.2.20"
		if index < 4 {
			host, publicHost = "reader", "192.0.2.10"
		}
		mode := "net14ad-delay"
		if manifest.Paths[0].Segments[index].LossPartsPerMillion != 0 {
			mode = "net14ad-loss"
		}
		upstreamRate := uint64(100_000_000)
		if index == 0 {
			upstreamRate = 20_000_000
		}
		relays = append(relays, networkRelay{ID: fmt.Sprintf("relay-%d", index), Host: host, Container: fmt.Sprintf("relay-container-%d", index),
			Network: "tcp", Listen: fmt.Sprintf(":%d", 48000+index), PublicEndpoint: fmt.Sprintf("%s:%d", publicHost, 48000+index),
			Target: fmt.Sprintf("172.17.0.1:%d", targetPorts[index]), Mode: mode, UpstreamRate: upstreamRate, ClientRate: 100_000_000,
			UpstreamSegment: manifest.Paths[0].Segments[index].ID, ClientSegment: manifest.Paths[1].Segments[5-index].ID})
	}
	return relays
}

func TestNetworkManifestRequiresExactPublicRelayEndpoint(t *testing.T) {
	manifest, _ := qualificationNodeManifest()
	manifest.Relays = qualificationRelays(manifest)
	if segments, err := validateNetworkManifest(manifest); err != nil || segments != 12 {
		t.Fatalf("valid exact relay manifest = %d / %v", segments, err)
	}
	for _, change := range []func(*networkRelay){
		func(relay *networkRelay) { relay.PublicEndpoint = "" },
		func(relay *networkRelay) { relay.PublicEndpoint = "relay.invalid:48000" },
		func(relay *networkRelay) { relay.PublicEndpoint = "192.0.2.10:48001" },
	} {
		candidate := manifest
		candidate.Relays = append([]networkRelay(nil), manifest.Relays...)
		change(&candidate.Relays[0])
		if _, err := validateNetworkManifest(candidate); err == nil {
			t.Fatal("network manifest accepted an ambiguous public relay endpoint")
		}
	}
}

func writeQualificationJSON(t *testing.T, value any) string {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	path := t.TempDir() + "/evidence.json"
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func setQualificationNodeRSS(t *testing.T, input *nodeResultInput, rss uint64) {
	t.Helper()
	for index, line := range input.Journal {
		var event node.Event
		if json.Unmarshal([]byte(line), &event) != nil || event.Kind != "resource-sample" || event.Resource == nil {
			continue
		}
		event.Resource.RSSBytes = rss
		raw, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		input.Journal[index] = string(raw)
	}
}
