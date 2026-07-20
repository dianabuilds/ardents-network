package readiness

import (
	"ardents/internal/diagnostics"
	transport "ardents/internal/network/api"
	noderecovery "ardents/internal/node/recovery"
)

func SyncBootHealth(diag *diagnostics.Recorder, boot *noderecovery.BootStatus, transportBoot transport.BootstrapStatus) {
	result := noderecovery.BootResultFromTransport(transportBoot.Joined, transportBoot.State, transportBoot.Reason)
	boot.SetResult(result)
	switch result.State {
	case noderecovery.BootReady:
		diag.SetSubsystem("boot", diagnostics.HealthReady, nil)
	case noderecovery.BootDegraded:
		diag.SetSubsystem("boot", diagnostics.HealthDegraded, &diagnostics.Reason{
			Code:                   "boot.join.degraded",
			Domain:                 "boot",
			Summary:                "bootstrap did not complete cleanly",
			Detail:                 result.Reason,
			Impact:                 "node remains controllable with limited network confidence",
			Recovery:               "operator",
			OperatorActionRequired: true,
		})
	default:
		diag.ClearSubsystem("boot")
	}
}

func SyncPrimaryReason(diag *diagnostics.Recorder) {
	health := diag.Health()
	if health.PrimaryReason != nil && !IsObservedPrimaryDomain(health.PrimaryReason.Domain) {
		return
	}

	for _, item := range health.Subsystems {
		if !IsObservedPrimaryDomain(item.Domain) {
			continue
		}
		if item.Reason != nil {
			diag.SetPrimary(item.State, item.Reason)
			return
		}
	}
	diag.ClearPrimary()
}

func SyncLifecycleState(_ any, diag *diagnostics.Recorder, move func(string)) {
	move(LifecycleForHealth(diag.Health().State, "ready", "degraded", "failed"))
}

func SubsystemReasonCode(health diagnostics.HealthSummary, domain string) string {
	for _, item := range health.Subsystems {
		if item.Domain != domain || item.Reason == nil {
			continue
		}
		return item.Reason.Code
	}
	return ""
}
