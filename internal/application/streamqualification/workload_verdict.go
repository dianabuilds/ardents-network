//go:build linux

package streamqualification

import (
	"fmt"
	"time"
)

// Criterion records an observation and its fixed bound. Missing measurements
// fail their criterion; this record is not an installed-host security receipt.
type Criterion struct {
	Name     string
	Observed float64
	Relation string
	Bound    float64
	Passed   bool
}

// EvaluateConditionWorkload applies the fixed progress envelope for the
// declared network cell without weakening identity, canary or cleanup checks.
func EvaluateConditionWorkload(report Report, role Role, profile Profile, condition NetworkCondition) []Criterion {
	var criteria []Criterion
	add := func(name string, observed float64, relation string, bound float64, pass bool) {
		criteria = append(criteria, Criterion{name, observed, relation, bound, pass})
	}
	schedule, err := profile.Definition(role)
	add("fixed-profile", 0, "valid", 0, err == nil)
	if err != nil {
		return criteria
	}
	if _, err := condition.CarrierRatioLimit(); err != nil {
		add("network-condition", 0, "valid", 0, false)
		return criteria
	}
	minimumStream, maximumGap, aggregate := float64(500_000), 2*time.Second, float64(schedule.AggregateBits)
	switch condition {
	case ImpairedLiveNetwork:
		minimumStream, maximumGap = 1, 5*time.Second
		aggregate = 2_000_000
		if role == PublisherRole {
			aggregate *= 4
		}
	case RecoveryNetwork:
		minimumStream, maximumGap, aggregate = 1, 8*time.Second, 1
	}
	add("joined-cleanup", 0, "no failure", 0, report.Failure == "" && !report.Finished.IsZero())
	duration := report.MeasuredDuration
	if duration == 0 {
		duration = report.Stopped.Sub(report.Started)
	}
	add("workload-seconds", duration.Seconds(), ">=", 600,
		!report.Started.IsZero() && !report.Stopped.IsZero() && duration >= 10*time.Minute)
	add("retained-connections", float64(len(report.Streams)), "=", float64(schedule.OpenConnections), len(report.Streams) == int(schedule.OpenConnections))
	sender := (role == ReaderRole) == (profile == ClientToPublisher)
	seen := make(map[uint32]bool, len(report.Streams))
	active := 0
	var useful uint64
	for _, stream := range report.Streams {
		valid := stream.ID != 0 && stream.ID <= 511 && stream.ID%2 == 1 && !seen[stream.ID]
		add(fmt.Sprintf("stream-%d-identity", stream.ID), float64(stream.ID), "unique odd ID in [1,511]", 511, valid)
		seen[stream.ID] = true
		if stream.ID > 127 {
			add(fmt.Sprintf("stream-%d-canary", stream.ID), float64(min(stream.Tx, stream.Rx)), "matched 32-byte exchanges", 32,
				stream.Tx >= 32 && stream.Tx == stream.Rx && stream.Tx%32 == 0)
			continue
		}
		active++
		count, gap, last := stream.Rx, stream.MaximumRxGap, stream.LastRx
		if sender {
			count, gap, last = stream.Tx, stream.MaximumTxGap, stream.LastTx
		}
		tail := report.Stopped.Sub(last)
		if report.MeasuredDuration != 0 {
			offset := stream.LastRxElapsed
			if sender {
				offset = stream.LastTxElapsed
			}
			tail = duration - offset
		}
		gap = max(gap, tail)
		useful += count
		bitrate := measuredBitrate(count, duration)
		add(fmt.Sprintf("stream-%d-bit-per-second", stream.ID), bitrate, ">=", minimumStream, bitrate >= minimumStream)
		add(fmt.Sprintf("stream-%d-maximum-gap-seconds", stream.ID), gap.Seconds(), "<=", maximumGap.Seconds(), !last.IsZero() && gap <= maximumGap)
	}
	add("active-connections", float64(active), "=", float64(schedule.ActiveConnections), active == int(schedule.ActiveConnections))
	bitrate := measuredBitrate(useful, duration)
	add("aggregate-bit-per-second", bitrate, ">=", aggregate, bitrate >= aggregate)
	return criteria
}

// CriteriaPassed requires a nonempty, complete set of successful observations.
func CriteriaPassed(criteria []Criterion) bool {
	if len(criteria) == 0 {
		return false
	}
	for _, criterion := range criteria {
		if !criterion.Passed {
			return false
		}
	}
	return true
}

// Invalid or absent duration must never yield a passing bitrate.
func measuredBitrate(bytes uint64, duration time.Duration) float64 {
	if duration <= 0 {
		return 0
	}
	return float64(bytes) * 8 / duration.Seconds()
}
