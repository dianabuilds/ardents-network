package recovery

import (
	"ardents/internal/diagnostics"
	transport "ardents/internal/network/api"
)

func CompleteBoot(
	diag *diagnostics.Recorder,
	transportBoot transport.BootstrapStatus,
	setDiscoveryDegraded func(string),
	moveLifecycle func(string),
	retainCurrentHealth func(),
) string {
	if transportBoot.State == "degraded" {
		setDiscoveryDegraded(transportBoot.Reason)
	}

	next := "ready"
	switch diag.Health().State {
	case diagnostics.HealthFailed:
		next = "failed"
	case diagnostics.HealthDegraded:
		next = "degraded"
	}
	moveLifecycle(next)
	if next == "ready" {
		retainCurrentHealth()
	}
	return next
}
