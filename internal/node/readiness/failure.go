package readiness

import "ardents/internal/diagnostics"

func AdoptPrimaryReason(diag *diagnostics.Recorder, domain string, state string, reason *diagnostics.Reason) {
	health := diag.Health()
	currentDomain := ""
	if health.PrimaryReason != nil {
		currentDomain = health.PrimaryReason.Domain
	}
	if !CanAdoptPrimaryReason(currentDomain, domain) {
		return
	}
	diag.SetPrimary(state, reason)
}

func RestorePrimaryReason(diag *diagnostics.Recorder, domain string) {
	health := diag.Health()
	if health.PrimaryReason == nil || health.PrimaryReason.Domain != domain {
		return
	}
	diag.ClearPrimary()
	PromoteSubsystemPrimary(diag, "")
}

func PromoteSubsystemPrimary(diag *diagnostics.Recorder, domain string) {
	health := diag.Health()
	for _, item := range health.Subsystems {
		if domain != "" && item.Domain != domain {
			continue
		}
		if item.Reason == nil {
			continue
		}
		diag.SetPrimary(item.State, item.Reason)
		return
	}
}
