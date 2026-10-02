//go:build linux

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
)

type net14vVerdict struct {
	Kind                              string
	BaselineManifest, EpisodeManifest string
	Criteria                          []streamqualification.Criterion
}

func verifyNET14V(arguments []string, output io.Writer) error {
	if len(arguments) < 5 || len(arguments) > 6 {
		return errors.New("usage: ardents-qualification verify-net14v <baseline-manifest.json> <episode-manifest.json> <baseline-verdict.json> <episode-verdict.json> <recovery-evidence.jsonl> [recovery-evidence.jsonl]")
	}
	baselineManifest, baselineManifestHash, err := readNetworkManifest(arguments[0])
	if err != nil {
		return err
	}
	episodeManifest, episodeManifestHash, err := readNetworkManifest(arguments[1])
	if err != nil {
		return err
	}
	baseline, err := readPairedVerdict(arguments[2])
	if err != nil {
		return err
	}
	episode, err := readPairedVerdict(arguments[3])
	if err != nil {
		return err
	}
	faults, recoveryErr := verifyRecoveryFaultEvidence(arguments[4:], episodeManifest, episodeManifestHash, recoveryWindow{episode.ReaderNetwork.Started, episode.ReaderNetwork.Stopped}, recoveryWindow{episode.PublisherNetwork.Started, episode.PublisherNetwork.Stopped})
	criteria := evaluateNET14V(baselineManifest, episodeManifest, baseline, episode)
	criteria = append(criteria, relayEpisodeCriteria(baseline, episode, episodeManifest.Failures, recoveryErr == nil, faults)...)
	binding := baseline.Relay.ManifestSHA256 == baselineManifestHash && episode.Relay.ManifestSHA256 == episodeManifestHash
	criteria = append(criteria, streamqualification.Criterion{Name: "net14v-manifest-verdict-binding", Observed: boolNumber(binding), Relation: "=", Bound: 1, Passed: binding})
	criteria = append(criteria, streamqualification.Criterion{Name: "net14v-recovery-schedule-evidence", Observed: boolNumber(recoveryErr == nil), Relation: "=", Bound: 1, Passed: recoveryErr == nil})
	verdict := net14vVerdict{"net14v", baselineManifestHash, episodeManifestHash, criteria}
	if err := json.NewEncoder(output).Encode(verdict); err != nil {
		return err
	}
	if !streamqualification.CriteriaPassed(criteria) {
		return errors.Join(errors.New("NET-14V criteria failed"), recoveryErr)
	}
	return nil
}

func readNetworkManifest(path string) (qualificationNetworkManifest, string, error) {
	body, err := os.ReadFile(path)
	if err != nil || len(body) == 0 || len(body) > 256<<10 {
		return qualificationNetworkManifest{}, "", errors.Join(err, errors.New("network manifest input is invalid"))
	}
	var manifest qualificationNetworkManifest
	if err := decodeExact(body, &manifest); err != nil {
		return manifest, "", err
	}
	if _, err := validateNetworkManifest(manifest); err != nil {
		return manifest, "", err
	}
	digest := sha256.Sum256(body)
	return manifest, hex.EncodeToString(digest[:]), nil
}

func readPairedVerdict(path string) (pairedWorkloadVerdict, error) {
	body, err := os.ReadFile(path)
	if err != nil || len(body) == 0 || len(body) > 16<<20 {
		return pairedWorkloadVerdict{}, errors.Join(err, errors.New("paired verdict input is invalid"))
	}
	var verdict pairedWorkloadVerdict
	if err := decodeExact(body, &verdict); err != nil {
		return verdict, err
	}
	for _, identity := range []string{verdict.Seed, verdict.CandidateSHA256, verdict.EndpointUnitSHA256, verdict.Relay.ManifestSHA256, verdict.Relay.BinarySHA256, verdict.NodeOwners.InventorySHA256, verdict.NodeOwners.BinarySHA256} {
		if _, identityErr := decodeIdentity(identity); identityErr != nil {
			return verdict, errors.New("paired workload candidate binding is invalid")
		}
	}
	if verdict.Kind != "paired-workload" || !streamqualification.CriteriaPassed(verdict.Criteria) {
		return verdict, errors.New("paired workload did not pass")
	}
	return verdict, nil
}

func evaluateNET14V(baselineManifest, episodeManifest qualificationNetworkManifest, baseline, episode pairedWorkloadVerdict) []streamqualification.Criterion {
	baselineTopology, episodeTopology := baselineManifest, episodeManifest
	baselineTopology.Cell, episodeTopology.Cell = "", ""
	baselineTopology.Failures, episodeTopology.Failures = nil, nil
	compatible := reflect.DeepEqual(baselineTopology, episodeTopology) &&
		baseline.CandidateSHA256 == episode.CandidateSHA256 && baseline.EndpointUnitSHA256 == episode.EndpointUnitSHA256 &&
		baseline.Relay.BinarySHA256 == episode.Relay.BinarySHA256 && baseline.NodeOwners.BinarySHA256 == episode.NodeOwners.BinarySHA256 && sameNodeInputs(baseline.NodeOwners, episode.NodeOwners) &&
		baselineManifest.Carrier == episodeManifest.Carrier &&
		baseline.Seed == episode.Seed && baseline.Profile == episode.Profile && baseline.Condition == streamqualification.NormalNetwork &&
		episode.Condition == streamqualification.RecoveryNetwork && baselineManifest.Cell == "net14ad" &&
		episodeManifest.Cell == "net14-recovery" &&
		baseline.ReaderNetwork.DirectionalUseful == episode.ReaderNetwork.DirectionalUseful &&
		baseline.PublisherNetwork.DirectionalUseful == episode.PublisherNetwork.DirectionalUseful
	baseWire, baseWireValid := combinedOwnerWire(baseline)
	episodeWire, episodeWireValid := combinedOwnerWire(episode)
	compatible = compatible && baseWireValid && episodeWireValid
	added := uint64(0)
	if episodeWire >= baseWire {
		added = episodeWire - baseWire
	} else {
		compatible = false
	}
	baseRelay, baseRelayValid := combinedRelayWire(baseline.Relay)
	episodeRelay, episodeRelayValid := combinedRelayWire(episode.Relay)
	relayAdded := uint64(0)
	if episodeRelay >= baseRelay {
		relayAdded = episodeRelay - baseRelay
	} else {
		compatible = false
	}
	episodeCount := uint64(len(episodeManifest.Failures))
	compatible = compatible && baseRelayValid && episodeRelayValid && episodeCount != 0
	criteria := []streamqualification.Criterion{
		{Name: "net14v-comparable-pair", Observed: boolNumber(compatible), Relation: "=", Bound: 1, Passed: compatible},
		{Name: "net14v-added-endpoint-carrier-bytes", Observed: float64(added), Relation: "<= per recovery set", Bound: float64(episodeCount * (8 << 20)), Passed: compatible && added <= episodeCount*(8<<20)},
		{Name: "net14v-added-route-bytes", Observed: float64(relayAdded), Relation: "<= recovery-set bound", Bound: float64(episodeCount * (8 << 20)), Passed: compatible && relayAdded <= episodeCount*(8<<20)},
	}
	criteria = append(criteria, net14vDirectionalCriteria(episodeManifest, episode.Relay, episode.ReaderNetwork, episode.PublisherNetwork)...)
	return criteria
}

func combinedOwnerWire(verdict pairedWorkloadVerdict) (uint64, bool) {
	if verdict.ReaderNetwork.DirectionalWire > ^uint64(0)-verdict.PublisherNetwork.DirectionalWire {
		return 0, false
	}
	return verdict.ReaderNetwork.DirectionalWire + verdict.PublisherNetwork.DirectionalWire, true
}

func boolNumber(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func sameNodeInputs(left, right nodeOwnersVerdict) bool {
	if left.InventorySHA256 != right.InventorySHA256 || len(left.Nodes) != len(right.Nodes) {
		return false
	}
	for index := range left.Nodes {
		if left.Nodes[index].ID != right.Nodes[index].ID || left.Nodes[index].Host != right.Nodes[index].Host || left.Nodes[index].PlanSHA256 != right.Nodes[index].PlanSHA256 {
			return false
		}
	}
	return true
}

func combinedRelayWire(verdict relayTrafficVerdict) (uint64, bool) {
	var total uint64
	for _, segment := range verdict.Segments {
		if segment.Bytes > ^uint64(0)-total {
			return 0, false
		}
		total += segment.Bytes
	}
	return total, len(verdict.Segments) == 12
}

func relayEpisodeCriteria(baseline, episode pairedWorkloadVerdict, failures []networkFailure, compatible bool, faults recoveryEvidence) []streamqualification.Criterion {
	baselineSegments := make(map[string]relaySegmentTraffic, len(baseline.Relay.Segments))
	episodeSegments := make(map[string]relaySegmentTraffic, len(episode.Relay.Segments))
	for _, segment := range baseline.Relay.Segments {
		baselineSegments[segment.ID] = segment
	}
	for _, segment := range episode.Relay.Segments {
		episodeSegments[segment.ID] = segment
	}
	criteria := make([]streamqualification.Criterion, 0, len(failures))
	for _, failure := range failures {
		start, startOK := faults.Starts[failure.Episode]
		stop, stopOK := faults.Stops[failure.Episode]
		// Pair identical offsets from the bound actual events, including scheduling
		// deviation; an independent manifest-only window can conceal episode bytes.
		windowStart := time.UnixMilli(start.ActualMillis).Sub(episode.ReaderNetwork.Started)
		windowStop := time.UnixMilli(stop.ActualMillis).Add(8 * time.Second).Sub(episode.ReaderNetwork.Started)
		baselineBytes, baselineOK := relayWindowBytes(baselineSegments[failure.SegmentID].Samples, baseline.ReaderNetwork.Started, windowStart, windowStop)
		episodeBytes, episodeOK := relayWindowBytes(episodeSegments[failure.SegmentID].Samples, episode.ReaderNetwork.Started, windowStart, windowStop)
		added, ordered := uint64(0), episodeBytes >= baselineBytes
		if ordered {
			added = episodeBytes - baselineBytes
		}
		passed := compatible && startOK && stopOK && baselineOK && episodeOK && ordered && added <= 8<<20
		criteria = append(criteria, streamqualification.Criterion{Name: "net14v-episode-" + failure.Episode + "-added-route-bytes", Observed: float64(added), Relation: "<=", Bound: 8 << 20, Passed: passed})
	}
	return criteria
}

func relayWindowBytes(samples []relaySegmentSample, origin time.Time, start, stop time.Duration) (uint64, bool) {
	if origin.IsZero() || start < 0 || stop <= start {
		return 0, false
	}
	startAt, stopAt := origin.Add(start), origin.Add(stop)
	var before, after uint64
	beforeOK, afterOK := false, false
	for _, sample := range samples {
		if !sample.At.After(startAt) {
			before, beforeOK = sample.Bytes, true
		}
		if !sample.At.Before(stopAt) {
			after, afterOK = sample.Bytes, true
			break
		}
	}
	if !beforeOK || !afterOK || after < before {
		return 0, false
	}
	return after - before, true
}
