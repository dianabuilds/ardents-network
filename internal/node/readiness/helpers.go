package readiness

import "fmt"

func AllowsObservedSync(state string, ready, degraded string) bool {
	return state == ready || state == degraded
}

func LifecycleForHealth(health, ready, degraded, failed string) string {
	switch health {
	case failed:
		return failed
	case degraded:
		return degraded
	default:
		return ready
	}
}

func RuntimeFailure(action string, failed bool, detail string) error {
	if !failed {
		return nil
	}
	if detail != "" {
		return fmt.Errorf("node %s failed: %s", action, detail)
	}
	return fmt.Errorf("node %s failed", action)
}

func PrimaryReasonSummary(summary string, hasPrimary bool) string {
	if !hasPrimary {
		return ""
	}
	return summary
}

func PrimaryReasonCode(code string, hasPrimary bool) string {
	if !hasPrimary {
		return ""
	}
	return code
}

func CanAdoptPrimaryReason(currentDomain, ownerDomain string) bool {
	return currentDomain == "" || currentDomain == ownerDomain
}

func IsObservedPrimaryDomain(domain string) bool {
	return domain == "boot" || domain == "transport"
}

func ShouldClearPrimaryOnStop(domain string) bool {
	switch domain {
	case "boot", "transport", "discovery", "workload", "publication":
		return true
	default:
		return false
	}
}
