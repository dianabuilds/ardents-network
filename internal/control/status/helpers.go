package status

import nodereadiness "ardents/internal/node/readiness"

const (
	healthReady    = "ready"
	healthDegraded = "degraded"
	healthFailed   = "failed"
)

func AllowsObservedSync(state string, readyState string, degradedState string) bool {
	return nodereadiness.AllowsObservedSync(state, readyState, degradedState)
}

func LifecycleForHealth(health string, readyState string, degradedState string, failedState string) string {
	return nodereadiness.LifecycleForHealth(health, readyState, degradedState, failedState)
}

func RuntimeFailure(action string, failed bool, detail string) error {
	return nodereadiness.RuntimeFailure(action, failed, detail)
}

func PrimaryReasonSummary(summary string, hasPrimary bool) string {
	return nodereadiness.PrimaryReasonSummary(summary, hasPrimary)
}

func PrimaryReasonCode(code string, hasPrimary bool) string {
	return nodereadiness.PrimaryReasonCode(code, hasPrimary)
}

func CanAdoptPrimaryReason(currentDomain, ownerDomain string) bool {
	return nodereadiness.CanAdoptPrimaryReason(currentDomain, ownerDomain)
}

func IsObservedPrimaryDomain(domain string) bool {
	return nodereadiness.IsObservedPrimaryDomain(domain)
}

func ShouldClearPrimaryOnStop(domain string) bool {
	return nodereadiness.ShouldClearPrimaryOnStop(domain)
}
