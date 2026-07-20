package readiness

import (
	"strings"

	"ardents/internal/diagnostics"
	noderecovery "ardents/internal/node/recovery"
)

func ClearRuntimeHealthForStop(diag *diagnostics.Recorder, boot *noderecovery.BootStatus) {
	boot.SetResult(noderecovery.StoppedBootResult())
	for _, domain := range []string{"boot", "transport", "discovery", "workload", "publication"} {
		diag.ClearSubsystem(domain)
	}
	if strings.HasPrefix(SubsystemReasonCode(diag.Health(), "data"), "privacy.capability.") {
		diag.ClearSubsystem("data")
	}
	health := diag.Health()
	if health.PrimaryReason == nil {
		return
	}
	if ShouldClearPrimaryOnStop(health.PrimaryReason.Domain) ||
		(health.PrimaryReason.Domain == "data" && strings.HasPrefix(health.PrimaryReason.Code, "privacy.capability.")) {
		diag.ClearPrimary()
		PromoteSubsystemPrimary(diag, "")
	}
}
