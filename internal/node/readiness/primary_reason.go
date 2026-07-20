package readiness

import "ardents/internal/diagnostics"

func CurrentPrimaryReasonSummary(diag *diagnostics.Recorder) string {
	if health := diag.Health(); health.PrimaryReason != nil {
		return PrimaryReasonSummary(health.PrimaryReason.Summary, true)
	}
	return PrimaryReasonSummary("", false)
}

func CurrentPrimaryReasonCode(diag *diagnostics.Recorder) string {
	if health := diag.Health(); health.PrimaryReason != nil {
		return PrimaryReasonCode(health.PrimaryReason.Code, true)
	}
	return PrimaryReasonCode("", false)
}
