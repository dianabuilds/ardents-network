package readiness

import (
	"ardents/internal/diagnostics"
)

const DiscoveryRefreshFailedCode = "discovery.refresh_failed"

func RecordDiscoveryRefreshFailure(
	diag *diagnostics.Recorder,
	cfgName string,
	err error,
	setDiscoveryDegraded func(string),
	adoptPrimary func(domain string, state string, reason *diagnostics.Reason),
	moveLifecycle func(string),
	publish func(string, map[string]any),
) {
	setDiscoveryDegraded(err.Error())
	reason := &diagnostics.Reason{
		Code:                   DiscoveryRefreshFailedCode,
		Domain:                 "discovery",
		Summary:                "discovery refresh failed",
		Detail:                 err.Error(),
		Impact:                 "local publication or remote discovery knowledge may remain stale",
		Recovery:               "operator",
		OperatorActionRequired: true,
		Resource:               cfgName,
	}
	diag.SetSubsystem("discovery", diagnostics.HealthDegraded, reason)
	adoptPrimary("discovery", diagnostics.HealthDegraded, reason)
	moveLifecycle(LifecycleForHealth(diag.Health().State, "ready", "degraded", "failed"))
	publish("discovery.refresh_failed", map[string]any{"id": cfgName, "error": err.Error()})
	diag.RecordEvent("discovery", "refresh_failed", cfgName, "discovery refresh failed", DiscoveryRefreshFailedCode, map[string]any{"detail": err.Error()})
}

func ClearDiscoveryRefreshFailure(
	diag *diagnostics.Recorder,
	setDiscoveryReady func(),
	restorePrimary func(string),
	moveLifecycle func(string),
) {
	if SubsystemReasonCode(diag.Health(), "discovery") == DiscoveryRefreshFailedCode {
		diag.ClearSubsystem("discovery")
		restorePrimary("discovery")
		setDiscoveryReady()
	}
	SyncPrimaryReason(diag)
	moveLifecycle(LifecycleForHealth(diag.Health().State, "ready", "degraded", "failed"))
}
