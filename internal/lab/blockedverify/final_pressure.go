package blockedverify

func verifyFinalPressure(values []finalPressureCell) ([]string, []string) {
	if len(values) != 5 {
		return []string{"P0-P4 result set is incomplete"}, nil
	}
	for index, value := range values {
		if value.ID != "P"+string(rune('0'+index)) || value.Terminal == "" {
			return []string{"P0-P4 result identity is invalid or reordered"}, nil
		}
	}
	var invalid, failures []string
	p0 := values[0]
	if p0.Units != 4 || p0.StreamMbit != 10 || p0.DurationMillis != 30_000 {
		invalid = append(invalid, "P0 schedule differs from R-037")
	} else if p0.Terminal != "normal" || !p0.Progress || !p0.Cleanup {
		failures = append(failures, "P0 four-unit hold failed")
	}
	p1 := values[1]
	if p1.Offers != 100 || p1.Refused != 100 || p1.CadenceMillis != 100 || p1.DurationMillis != 10_000 {
		invalid = append(invalid, "P1 schedule differs from R-037")
	} else if p1.Terminal != "normal" || p1.MaximumRefusalMillis > 1_000 || !p1.Progress || !p1.Cleanup {
		failures = append(failures, "P1 projected-admission gate failed")
	}
	p2 := values[2]
	if p2.BaselineSockets != 6 || p2.Injected != 20 || p2.PeakSockets != 26 || p2.PartialBytes != 128 ||
		p2.RatePerSecond != 2 || p2.HighSamples != 3 || p2.LowSamples != 120 {
		invalid = append(invalid, "P2 schedule or socket accounting differs from R-037")
	} else if p2.Terminal != "normal" || !p2.Progress || !p2.Protect || !p2.Normal || p2.Drain || !p2.Cleanup {
		failures = append(failures, "P2 PROTECT hysteresis gate failed")
	}
	p3 := values[3]
	if p3.BaselineSockets != 6 || p3.Injected != 23 || p3.PeakSockets != 29 || p3.PartialBytes != 128 ||
		p3.RatePerSecond != 2 {
		invalid = append(invalid, "P3 schedule or socket accounting differs from R-037")
	} else if p3.Terminal != "drain" || !p3.Drain || p3.Normal || !p3.Cleanup || p3.ExitMillis > 60_000 ||
		p3.OOMEvents != 0 || p3.PeakSockets > 32 {
		failures = append(failures, "P3 emergency DRAIN gate failed")
	}
	p4 := values[4]
	if p4.Offers != 1_000 || p4.Refused != 1_000 || p4.Batches != 10 || p4.CadenceMillis != 100 ||
		p4.DurationMillis != 100_000 || len(p4.Reconciliations) != 10 || !exactReconciliations(p4.Reconciliations) {
		invalid = append(invalid, "P4 schedule differs from R-037")
	} else if p4.Terminal != "normal" || p4.MaximumRefusalMillis > 1_000 || !p4.Progress ||
		!p4.Cleanup || p4.Residuals != 0 || p4.UpwardTrend || p4.OOMEvents != 0 {
		failures = append(failures, "P4 refusal-churn gate failed")
	}
	return invalid, failures
}

func exactReconciliations(values []finalReconciliation) bool {
	for index, value := range values {
		if value.Batch != uint16(index) || value.AllocationDelta != 0 || value.FDDelta != 0 ||
			value.SocketDelta != 0 || value.GoroutineDelta != 0 || value.TimerDelta != 0 ||
			value.StateBytesDelta != 0 || value.EvidenceRecordsDelta != 0 || value.CleanupSockets != 0 ||
			value.CleanupDescendants != 0 || value.CleanupStateBytes != 0 || value.Residuals != 0 {
			return false
		}
	}
	return true
}
