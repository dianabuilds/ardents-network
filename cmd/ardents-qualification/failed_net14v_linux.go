//go:build linux

package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
)

type failedRelayResult struct {
	ID, Host, Container, UpstreamSegment, ClientSegment, BinarySHA256 string
	TrafficControl                                                    []string
	Samples                                                           []relayCounterSample
	Logs                                                              []string
	Complete                                                          bool
}

type failedNET14VVerdict struct {
	Kind, ManifestSHA256 string
	Criteria             []streamqualification.Criterion
}

func verifyFailedNET14V(arguments []string, output io.Writer) error {
	if len(arguments) < 5 || len(arguments) > 6 {
		return errors.New("usage: ardents-qualification verify-failed-net14v <baseline-manifest.json> <episode-manifest.json> <baseline-verdict.json> <failed-relay-results.json> <recovery-evidence.jsonl> [recovery-evidence.jsonl]")
	}
	baselineManifest, _, err := readNetworkManifest(arguments[0])
	if err != nil {
		return err
	}
	manifest, manifestHash, err := readNetworkManifest(arguments[1])
	if err != nil {
		return err
	}
	baseline, err := readPairedVerdict(arguments[2])
	if err != nil {
		return err
	}
	if baselineManifest.Cell != "net14ad" || manifest.Cell != "net14-recovery" {
		return errors.New("failed NET-14V evidence requires a recovery manifest")
	}
	body, err := os.ReadFile(arguments[3])
	if err != nil || len(body) == 0 || len(body) > 8<<20 {
		return errors.Join(err, errors.New("failed relay evidence is invalid"))
	}
	var relays []failedRelayResult
	if err := decodeExact(body, &relays); err != nil {
		return err
	}
	starts, stops, parseErr := readRecoveryWindows(arguments[4:])
	scheduleErr := verifyRecoveryFaultEvidence(arguments[4:], manifest)
	criteria := evaluateFailedNET14V(baselineManifest, manifest, baseline, relays, starts, stops)
	complete := parseErr == nil && scheduleErr == nil
	criteria = append(criteria, streamqualification.Criterion{Name: "failed-net14v-recovery-evidence", Observed: boolNumber(complete), Relation: "=", Bound: 1, Passed: complete})
	verdict := failedNET14VVerdict{Kind: "failed-net14v", ManifestSHA256: manifestHash, Criteria: criteria}
	if err := json.NewEncoder(output).Encode(verdict); err != nil {
		return err
	}
	if !streamqualification.CriteriaPassed(criteria) {
		return errors.Join(errors.New("failed NET-14V byte evidence is incomplete or exceeds its bound"), parseErr, scheduleErr)
	}
	return nil
}

func readRecoveryWindows(paths []string) (map[string]recoveryFaultRecord, map[string]recoveryFaultRecord, error) {
	starts, stops := make(map[string]recoveryFaultRecord), make(map[string]recoveryFaultRecord)
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			return starts, stops, err
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			var record recoveryFaultRecord
			if json.Unmarshal(scanner.Bytes(), &record) != nil {
				continue
			}
			switch record.Kind {
			case "recovery-fault-start":
				starts[record.Episode] = record
			case "recovery-fault-stop":
				stops[record.Episode] = record
			}
		}
		err = errors.Join(scanner.Err(), file.Close())
		if err != nil {
			return starts, stops, err
		}
	}
	return starts, stops, nil
}

func evaluateFailedNET14V(baselineManifest, manifest qualificationNetworkManifest, baseline pairedWorkloadVerdict, inputs []failedRelayResult, starts, stops map[string]recoveryFaultRecord) []streamqualification.Criterion {
	bySegment := make(map[string][]relaySegmentSample)
	manifestRelays := make(map[string]networkRelay, len(manifest.Relays))
	for _, relay := range manifest.Relays {
		manifestRelays[relay.ID] = relay
	}
	seen := make(map[string]bool, len(inputs))
	binary := ""
	complete := len(inputs) == len(manifest.Relays)
	for _, input := range inputs {
		relay, present := manifestRelays[input.ID]
		classes, classErr := relayClassBytes(input.TrafficControl)
		if !present || seen[input.ID] || input.Complete || input.Host != relay.Host || input.Container != relay.Container ||
			input.UpstreamSegment != relay.UpstreamSegment || input.ClientSegment != relay.ClientSegment || classErr != nil ||
			len(input.Samples) < 2 || classes["1:10"] < input.Samples[len(input.Samples)-1].UpstreamBytes ||
			classes["1:20"] < input.Samples[len(input.Samples)-1].ClientBytes {
			complete = false
		}
		seen[input.ID] = true
		if _, err := decodeIdentity(input.BinarySHA256); err != nil || binary != "" && binary != input.BinarySHA256 {
			complete = false
		}
		binary = input.BinarySHA256
		for index, sample := range input.Samples {
			if sample.At.IsZero() || index > 0 && (sample.At.Sub(input.Samples[index-1].At) <= 0 || sample.At.Sub(input.Samples[index-1].At) > 1500*time.Millisecond || sample.UpstreamBytes < input.Samples[index-1].UpstreamBytes || sample.ClientBytes < input.Samples[index-1].ClientBytes) {
				complete = false
			}
		}
		if _, exists := bySegment[input.UpstreamSegment]; exists {
			complete = false
		}
		bySegment[input.UpstreamSegment] = relaySegmentSamples(input.Samples, true)
		if _, exists := bySegment[input.ClientSegment]; exists {
			complete = false
		}
		bySegment[input.ClientSegment] = relaySegmentSamples(input.Samples, false)
	}
	baselineTopology, episodeTopology := baselineManifest, manifest
	baselineTopology.Cell, episodeTopology.Cell = "", ""
	baselineTopology.Failures, episodeTopology.Failures = nil, nil
	compatible := complete && baseline.Condition == streamqualification.NormalNetwork &&
		baseline.Relay.BinarySHA256 == binary && baselineManifest.Carrier == manifest.Carrier &&
		reflect.DeepEqual(baselineTopology, episodeTopology)
	criteria := []streamqualification.Criterion{{Name: "failed-net14v-comparable-baseline", Observed: boolNumber(compatible), Relation: "=", Bound: 1, Passed: compatible}}
	for _, failure := range manifest.Failures {
		start, startOK := starts[failure.Episode]
		stop, stopOK := stops[failure.Episode]
		startAt := time.UnixMilli(start.ActualMillis).UTC()
		stopAt := time.UnixMilli(stop.ActualMillis).UTC().Add(8 * time.Second)
		episodeBytes, episodeMeasured := relayAbsoluteWindowBytes(bySegment[failure.SegmentID], startAt, stopAt)
		var baselineSamples []relaySegmentSample
		for _, segment := range baseline.Relay.Segments {
			if segment.ID == failure.SegmentID {
				baselineSamples = segment.Samples
				break
			}
		}
		windowStart := time.Duration(failure.AtMillis) * time.Millisecond
		windowStop := windowStart + time.Duration(failure.DurationMillis)*time.Millisecond + 8*time.Second
		baselineBytes, baselineMeasured := relayWindowBytes(baselineSamples, baseline.ReaderNetwork.Started, windowStart, windowStop)
		added, ordered := uint64(0), episodeBytes >= baselineBytes
		if ordered {
			added = episodeBytes - baselineBytes
		}
		passed := compatible && startOK && stopOK && baselineMeasured && episodeMeasured && ordered && added <= 8<<20
		criteria = append(criteria, streamqualification.Criterion{Name: "failed-net14v-episode-" + failure.Episode + "-added-route-bytes", Observed: float64(added), Relation: "<=", Bound: 8 << 20, Passed: passed})
	}
	return criteria
}

func relayAbsoluteWindowBytes(samples []relaySegmentSample, start, stop time.Time) (uint64, bool) {
	if start.IsZero() || stop.IsZero() || !start.Before(stop) {
		return 0, false
	}
	var before, after uint64
	beforeOK, afterOK := false, false
	for _, sample := range samples {
		if !sample.At.After(start) {
			before, beforeOK = sample.Bytes, true
		}
		if !sample.At.Before(stop) {
			after, afterOK = sample.Bytes, true
			break
		}
	}
	if !beforeOK || !afterOK || after < before {
		return 0, false
	}
	return after - before, true
}
