//go:build linux

package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"math"
	"os"
)

type recoveryFaultRecord struct {
	Kind            string `json:"kind"`
	Episode         string `json:"episode"`
	Segment         string `json:"segment"`
	Host            string `json:"host"`
	ScheduledMillis int64  `json:"scheduled_millis"`
	ActualMillis    int64  `json:"actual_millis"`
	Episodes        int    `json:"episodes"`
}

func verifyRecoveryFaultEvidence(paths []string, manifest qualificationNetworkManifest) error {
	if len(paths) < 1 || len(paths) > 2 {
		return errors.New("NET-14V recovery evidence file count is invalid")
	}
	starts, stops := make(map[string]recoveryFaultRecord), make(map[string]recoveryFaultRecord)
	completed := make(map[string]int)
	for _, path := range paths {
		info, statErr := os.Stat(path)
		if statErr != nil || info.Size() <= 0 || info.Size() > 1<<20 {
			return errors.Join(statErr, errors.New("recovery evidence size is invalid"))
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 4096), 1<<20)
		for scanner.Scan() {
			var record recoveryFaultRecord
			if json.Unmarshal(scanner.Bytes(), &record) != nil {
				continue
			}
			switch record.Kind {
			case "recovery-fault-start":
				if record.Episode == "" || starts[record.Episode].Kind != "" {
					_ = file.Close()
					return errors.New("recovery fault start is duplicated or invalid")
				}
				starts[record.Episode] = record
			case "recovery-fault-stop":
				if record.Episode == "" || stops[record.Episode].Kind != "" {
					_ = file.Close()
					return errors.New("recovery fault stop is duplicated or invalid")
				}
				stops[record.Episode] = record
			case "recovery-faults-complete":
				if record.Host != "reader" && record.Host != "publisher" || completed[record.Host] != 0 || record.Episodes <= 0 {
					_ = file.Close()
					return errors.New("recovery completion record is invalid")
				}
				completed[record.Host] = record.Episodes
			}
		}
		err = errors.Join(scanner.Err(), file.Close())
		if err != nil {
			return err
		}
	}
	segmentHosts := make(map[string]string, 12)
	for _, relay := range manifest.Relays {
		segmentHosts[relay.UpstreamSegment], segmentHosts[relay.ClientSegment] = relay.Host, relay.Host
	}
	expectedHosts := make(map[string]int)
	var origin int64
	for _, failure := range manifest.Failures {
		start, startOK := starts[failure.Episode]
		stop, stopOK := stops[failure.Episode]
		host := segmentHosts[failure.SegmentID]
		expectedHosts[host]++
		candidateOrigin := start.ScheduledMillis - int64(failure.AtMillis)
		if !startOK || !stopOK || start.Segment != failure.SegmentID || stop.Segment != failure.SegmentID ||
			stop.ScheduledMillis-start.ScheduledMillis != int64(failure.DurationMillis) ||
			absMillis(start.ActualMillis-start.ScheduledMillis) > 1500 || absMillis(stop.ActualMillis-stop.ScheduledMillis) > 1500 ||
			stop.ActualMillis < start.ActualMillis || candidateOrigin <= 0 || origin != 0 && origin != candidateOrigin {
			return errors.New("recovery fault evidence differs from its manifest schedule")
		}
		origin = candidateOrigin
	}
	if len(starts) != len(manifest.Failures) || len(stops) != len(manifest.Failures) || len(completed) != len(expectedHosts) {
		return errors.New("recovery fault evidence is incomplete")
	}
	for host, count := range expectedHosts {
		if completed[host] != count {
			return errors.New("recovery host completion count differs")
		}
	}
	return nil
}

func absMillis(value int64) int64 {
	if value == math.MinInt64 {
		return math.MaxInt64
	}
	if value < 0 {
		return -value
	}
	return value
}
