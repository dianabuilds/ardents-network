package blockedverify

import (
	"math"
	"sort"
)

func verifyFinalSustained(values []finalSustainedCell) ([]string, []string) {
	wanted := []string{"endpoint-to-publisher", "publisher-to-endpoint"}
	if len(values) != len(wanted) {
		return []string{"sustained direction set is incomplete"}, nil
	}
	var invalid, failures []string
	var manifestCursor uint64
	for index, value := range values {
		if value.Direction != wanted[index] || len(value.Runs) != 5 || !finitePositive(value.DirectBeforeMbit) ||
			!finitePositive(value.DirectAfterMbit) || !value.DirectBeforeValid || !value.DirectAfterValid ||
			!validDirectPair(value) {
			invalid = append(invalid, "sustained baseline identity, pairing, or run count is invalid")
			continue
		}
		if value.DirectBefore.StartedOffsetMillis < manifestCursor {
			invalid = append(invalid, "sustained direction timeline restarted or overlapped")
			continue
		}
		drift := math.Max(value.DirectBeforeMbit, value.DirectAfterMbit) /
			math.Min(value.DirectBeforeMbit, value.DirectAfterMbit)
		if drift > 1.10 {
			invalid = append(invalid, "sustained direct baseline drift exceeds 1.10")
			continue
		}
		var windows []float64
		var delivered uint64
		cursor := value.DirectBefore.FinishedOffsetMillis
		for _, run := range value.Runs {
			if len(run.WindowsMbit) != 10 || len(run.WindowEndsMillis) != 10 || !run.Complete ||
				run.FinishedOffsetMillis < run.StartedOffsetMillis+600_000 || run.DeliveredBytes == 0 ||
				!isHexDigest(run.Digest, 32) || run.StartedOffsetMillis < cursor {
				invalid = append(invalid, "sustained run does not contain ten complete ordered windows")
				continue
			}
			cursor = run.FinishedOffsetMillis
			delivered += run.DeliveredBytes
			for index, ended := range run.WindowEndsMillis {
				if ended != run.StartedOffsetMillis+uint64(index+1)*60_000 || ended > run.FinishedOffsetMillis {
					invalid = append(invalid, "sustained window offsets are overlapping, missing, or shortened")
				}
			}
			if reason := validateResourceShape(run.Resources); reason != "" {
				invalid = append(invalid, reason)
				continue
			}
			for _, window := range run.WindowsMbit {
				if math.IsNaN(window) || math.IsInf(window, 0) || window < 0 {
					invalid = append(invalid, "sustained window is non-finite or negative")
					continue
				}
				windows = append(windows, window)
			}
			failures = append(failures, resourceGateFailures(run.Resources, false, 32)...)
		}
		if len(windows) != 50 {
			continue
		}
		if value.DirectAfter.StartedOffsetMillis < cursor {
			invalid = append(invalid, "sustained before/run/after sequence overlaps on the manifest clock")
			continue
		}
		manifestCursor = value.DirectAfter.FinishedOffsetMillis
		sorted := append([]float64(nil), windows...)
		sort.Float64s(sorted)
		p05 := sorted[2]
		baseline := (value.DirectBeforeMbit + value.DirectAfterMbit) / 2
		threshold := math.Min(10, baseline/2)
		if p05 < threshold {
			failures = append(failures, "sustained p05 goodput gate failed")
		}
		if delivered != value.DeliveredBytes || delivered == 0 ||
			!ratioMatches(value.EndpointCarrierRatio, value.EndpointCarrierBytes, delivered) ||
			!ratioMatches(value.PublisherCarrierRatio, value.PublisherCarrierBytes, delivered) {
			invalid = append(invalid, "sustained carrier ratio is missing or non-finite")
		} else if value.EndpointCarrierRatio > 1.5 || value.PublisherCarrierRatio > 1.5 {
			failures = append(failures, "sustained carrier ratio gate failed")
		}
	}
	return invalid, failures
}

func validDirectPair(value finalSustainedCell) bool {
	before, after := value.DirectBefore, value.DirectAfter
	if value.DirectPairID == "" || before.PairID != value.DirectPairID || after.PairID != value.DirectPairID ||
		!before.Complete || !after.Complete || before.DurationMillis < 60_000 || before.DurationMillis > 61_000 ||
		after.DurationMillis < 60_000 || after.DurationMillis > 61_000 ||
		before.DeliveredBytes == 0 || after.DeliveredBytes == 0 || !isHexDigest(before.Digest, 32) ||
		!isHexDigest(after.Digest, 32) || before.FinishedOffsetMillis < before.StartedOffsetMillis+60_000 ||
		after.FinishedOffsetMillis < after.StartedOffsetMillis+60_000 ||
		before.FinishedOffsetMillis > after.StartedOffsetMillis {
		return false
	}
	return ratioMatches(value.DirectBeforeMbit, before.DeliveredBytes*8, uint64(before.DurationMillis)*1_000) &&
		ratioMatches(value.DirectAfterMbit, after.DeliveredBytes*8, uint64(after.DurationMillis)*1_000)
}

func ratioMatches(value float64, numerator, denominator uint64) bool {
	if denominator == 0 || !finitePositive(value) {
		return false
	}
	return math.Abs(value-float64(numerator)/float64(denominator)) <= 1e-9
}

func finitePositive(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}
