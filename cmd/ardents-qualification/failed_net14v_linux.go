//go:build linux

package main

import (
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
	if len(arguments) < 7 || len(arguments) > 8 {
		return errors.New("usage: ardents-qualification verify-failed-net14v <baseline-manifest.json> <episode-manifest.json> <baseline-verdict.json> <failed-relay-results.json> <reader-journal.jsonl> <publisher-journal.jsonl> <recovery-evidence.jsonl> [recovery-evidence.jsonl]")
	}
	baselineManifest, baselineHash, err := readNetworkManifest(arguments[0])
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
	if baselineManifest.Cell != "net14ad" || manifest.Cell != "net14-recovery" || baseline.Relay.ManifestSHA256 != baselineHash {
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
	reader, readerErr := readFailedRecoveryWorkload(arguments[4], streamqualification.ReaderRole, baseline)
	publisher, publisherErr := readFailedRecoveryWorkload(arguments[5], streamqualification.PublisherRole, baseline)
	faults, scheduleErr := verifyRecoveryFaultEvidence(arguments[6:], manifest, manifestHash, reader, publisher)
	starts, stops := faults.Starts, faults.Stops
	criteria := evaluateFailedNET14V(baselineManifest, manifest, baseline, relays, starts, stops, reader.Start)
	complete := readerErr == nil && publisherErr == nil && scheduleErr == nil
	criteria = append(criteria, streamqualification.Criterion{Name: "failed-net14v-recovery-evidence", Observed: boolNumber(complete), Relation: "=", Bound: 1, Passed: complete})
	verdict := failedNET14VVerdict{Kind: "failed-net14v", ManifestSHA256: manifestHash, Criteria: criteria}
	if err := json.NewEncoder(output).Encode(verdict); err != nil {
		return err
	}
	if !streamqualification.CriteriaPassed(criteria) {
		return errors.Join(errors.New("failed NET-14V byte evidence is incomplete or exceeds its bound"), readerErr, publisherErr, scheduleErr)
	}
	return nil
}

// A failed workload is not a passed qualification. Retained final reports may
// prove its observed interval, but missing stop/monotonic evidence refuses.
func readFailedRecoveryWorkload(path string, role streamqualification.Role, baseline pairedWorkloadVerdict) (window recoveryWindow, outcome error) {
	candidate, result := false, false
	outcome = readRunnerEvidence(path, func(raw []byte) error {
		var record struct {
			Kind, BinarySHA256, Seed, PlanSHA256 string
			Participants                         int
			EndpointArtifact                     qualificationEndpointArtifact
			Role                                 streamqualification.Role
			Profile                              streamqualification.Profile
			Condition                            streamqualification.NetworkCondition
			Report                               streamqualification.Report
		}
		if err := json.Unmarshal(raw, &record); err != nil {
			return err
		}
		if record.Kind == "candidate" {
			expected := 4
			if role == streamqualification.PublisherRole {
				expected = 1
			}
			if candidate || record.Participants != expected || record.BinarySHA256 != baseline.CandidateSHA256 || record.EndpointArtifact.Files[qualificationUnitPath] != baseline.EndpointUnitSHA256 {
				return errors.New("failed workload candidate differs")
			}
			if _, err := decodeIdentity(record.EndpointArtifact.ManifestSHA256); err != nil || len(record.EndpointArtifact.Files) != 3 || record.EndpointArtifact.Files[qualificationBinaryPath] != record.BinarySHA256 || record.EndpointArtifact.Files[qualificationPlanPath] != record.PlanSHA256 {
				return errors.New("failed workload installed artifact differs")
			}
			candidate = true
		}
		if record.Kind != "result" {
			return nil
		}
		report := record.Report
		if !candidate || record.Role != role || record.Profile != baseline.Profile || record.Condition != streamqualification.RecoveryNetwork || report.Started.IsZero() || !report.Started.Before(report.Stopped) || report.StartedElapsed < 0 || report.StoppedElapsed <= report.StartedElapsed || report.MeasuredDuration != report.StoppedElapsed-report.StartedElapsed {
			return errors.New("failed workload interval or role is missing")
		}
		if _, err := decodeIdentity(record.Seed); err != nil || record.Seed != baseline.Seed {
			return errors.New("failed workload seed differs")
		}
		// The intersection retains only time when every reported participant ran.
		if window.Start.IsZero() || report.Started.After(window.Start) {
			window.Start = report.Started
		}
		if window.Stop.IsZero() || report.Stopped.Before(window.Stop) {
			window.Stop = report.Stopped
		}
		result = true
		return nil
	}, true)
	if !candidate || !result {
		return window, errors.New("failed workload final evidence is missing")
	}
	return window, outcome
}

func evaluateFailedNET14V(baselineManifest, manifest qualificationNetworkManifest, baseline pairedWorkloadVerdict, inputs []failedRelayResult, starts, stops map[string]recoveryFaultRecord, workloadStart time.Time) []streamqualification.Criterion {
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
		windowStart := startAt.Sub(workloadStart)
		windowStop := stopAt.Sub(workloadStart)
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
