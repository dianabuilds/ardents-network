//go:build linux

package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"math"
	"os"
	"time"
)

type recoveryFaultRecord struct {
	Kind             string `json:"kind"`
	Episode          string `json:"episode"`
	Segment          string `json:"segment"`
	Host             string `json:"host"`
	ScheduledMillis  int64  `json:"scheduled_millis"`
	ActualMillis     int64  `json:"actual_millis"`
	Episodes         int    `json:"episodes"`
	ManifestSHA256   string `json:"manifest_sha256"`
	RunStartedMillis int64  `json:"run_started_millis"`
}

type recoveryWindow struct{ Start, Stop time.Time }
type recoveryEvidence struct {
	Starts, Stops map[string]recoveryFaultRecord
}

// Wall timestamps correlate a declared run and its observations only. They are
// never subtracted to qualify elapsed recovery latency or carrier bitrate.
func verifyRecoveryFaultEvidence(paths []string, manifest qualificationNetworkManifest, digest string, reader, publisher recoveryWindow) (recoveryEvidence, error) {
	evidence := recoveryEvidence{}
	fail := func(err error) (recoveryEvidence, error) { return recoveryEvidence{}, err }
	if reader.Start.IsZero() || publisher.Start.IsZero() || !reader.Start.Before(reader.Stop) || !publisher.Start.Before(publisher.Stop) {
		return fail(errors.New("recovery workload interval is missing"))
	}
	runStart := reader.Start.UnixMilli()
	windowStart, windowStop := max(reader.Start.UnixMilli(), publisher.Start.UnixMilli()), min(reader.Stop.UnixMilli(), publisher.Stop.UnixMilli())
	if windowStart <= 0 || windowStop <= windowStart {
		return fail(errors.New("recovery workload intervals do not overlap"))
	}

	if len(paths) < 1 || len(paths) > 2 {
		return fail(errors.New("NET-14V recovery evidence file count is invalid"))
	}
	starts, stops := make(map[string]recoveryFaultRecord), make(map[string]recoveryFaultRecord)
	completed := make(map[string]recoveryFaultRecord)
	for _, path := range paths {
		info, statErr := os.Stat(path)
		if statErr != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 1<<20 {
			return fail(errors.Join(statErr, errors.New("recovery evidence size is invalid")))
		}
		file, err := os.Open(path)
		if err != nil {
			return fail(err)
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 4096), 1<<20)
		for scanner.Scan() {
			var record recoveryFaultRecord
			if json.Unmarshal(scanner.Bytes(), &record) != nil {
				continue
			}
			if record.Kind == "recovery-fault-start" || record.Kind == "recovery-fault-stop" || record.Kind == "recovery-faults-complete" {
				if record.ManifestSHA256 != digest || record.Host != "reader" && record.Host != "publisher" ||
					!nearMillis(record.RunStartedMillis, runStart) || record.RunStartedMillis <= 0 || record.RunStartedMillis > math.MaxInt64-600_000 ||
					record.ActualMillis < windowStart || record.ActualMillis > windowStop {
					_ = file.Close()
					return fail(errors.New("recovery evidence is outside its bound workload, host or manifest"))
				}
			}
			switch record.Kind {
			case "recovery-fault-start":
				if record.Episode == "" || starts[record.Episode].Kind != "" {
					_ = file.Close()
					return fail(errors.New("recovery fault start is duplicated or invalid"))
				}
				starts[record.Episode] = record
			case "recovery-fault-stop":
				if record.Episode == "" || stops[record.Episode].Kind != "" {
					_ = file.Close()
					return fail(errors.New("recovery fault stop is duplicated or invalid"))
				}
				stops[record.Episode] = record
			case "recovery-faults-complete":
				if record.Host != "reader" && record.Host != "publisher" || completed[record.Host].Kind != "" || record.Episodes <= 0 {
					_ = file.Close()
					return fail(errors.New("recovery completion record is invalid"))
				}
				completed[record.Host] = record
			}
		}
		err = errors.Join(scanner.Err(), file.Close())
		if err != nil {
			return fail(err)
		}
	}
	segmentHosts := make(map[string]string, 12)
	for _, relay := range manifest.Relays {
		segmentHosts[relay.UpstreamSegment], segmentHosts[relay.ClientSegment] = relay.Host, relay.Host
	}
	expectedHosts := make(map[string]int)
	var origin int64
	latestStop := make(map[string]int64)
	for _, failure := range manifest.Failures {
		start, startOK := starts[failure.Episode]
		stop, stopOK := stops[failure.Episode]
		host := segmentHosts[failure.SegmentID]
		expectedHosts[host]++
		candidateOrigin := start.RunStartedMillis
		if !startOK || !stopOK || start.Host != host || stop.Host != host || start.RunStartedMillis != stop.RunStartedMillis || start.Segment != failure.SegmentID || stop.Segment != failure.SegmentID ||
			start.ScheduledMillis != candidateOrigin+int64(failure.AtMillis) || stop.ScheduledMillis != candidateOrigin+int64(failure.AtMillis)+int64(failure.DurationMillis) ||
			!nearMillis(start.ActualMillis, start.ScheduledMillis) || !nearMillis(stop.ActualMillis, stop.ScheduledMillis) ||
			stop.ActualMillis <= start.ActualMillis || !nearMillis(candidateOrigin, runStart) || origin != 0 && origin != candidateOrigin || stop.ActualMillis > windowStop-8000 {
			return fail(errors.New("recovery fault evidence differs from its manifest schedule"))
		}
		origin = candidateOrigin
		latestStop[host] = max(latestStop[host], stop.ActualMillis)
	}
	if len(starts) != len(manifest.Failures) || len(stops) != len(manifest.Failures) || len(completed) != len(expectedHosts) {
		return fail(errors.New("recovery fault evidence is incomplete"))
	}
	for host, count := range expectedHosts {
		if completed[host].Episodes != count || completed[host].RunStartedMillis != origin || completed[host].ActualMillis < latestStop[host] {
			return fail(errors.New("recovery host completion count differs"))
		}
	}
	evidence.Starts, evidence.Stops = starts, stops
	return evidence, nil
}

func nearMillis(left, right int64) bool {
	return uint64(max(left, right))-uint64(min(left, right)) <= 1500
}
