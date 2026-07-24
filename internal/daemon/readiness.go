package daemon

import (
	"fmt"
	"strings"

	"ardents/internal/diagnostics"
	"ardents/internal/identity"
	"ardents/internal/network"
)

const readyState = "ready"

type ReadinessCheckSnapshot struct {
	Name   string
	Ready  bool
	Reason string
}

type ReadinessSnapshot struct {
	Ready  bool
	Reason string
	Checks []ReadinessCheckSnapshot
}

// EvaluateRolloutReadiness is the canonical runtime-owned portion of the
// rollout contract. Protected API and Access Grant checks are added by the
// admitted operator RPC, because only that boundary can prove them.
func EvaluateRolloutReadiness(
	nodeIdentity identity.Snapshot,
	networkStatus network.StatusSnapshot,
	health diagnostics.HealthSnapshot,
) ReadinessSnapshot {
	checks := []ReadinessCheckSnapshot{
		networkReadiness(networkStatus),
		diagnosticsReadiness(health),
		identityReadiness(nodeIdentity),
	}
	result := ReadinessSnapshot{Ready: true, Checks: checks}
	for _, check := range checks {
		if check.Ready {
			continue
		}
		result.Ready = false
		if result.Reason == "" {
			result.Reason = check.Name + ": " + check.Reason
		}
	}
	return result
}

func networkReadiness(status network.StatusSnapshot) ReadinessCheckSnapshot {
	check := ReadinessCheckSnapshot{Name: "network", Ready: status.State == readyState}
	if check.Ready {
		return check
	}
	check.Reason = strings.TrimSpace(status.Reason)
	if check.Reason == "" {
		check.Reason = fmt.Sprintf("state is %s", stateOrUnknown(status.State))
	}
	return check
}

func diagnosticsReadiness(health diagnostics.HealthSnapshot) ReadinessCheckSnapshot {
	check := ReadinessCheckSnapshot{Name: "diagnostics", Ready: health.State == readyState}
	if check.Ready {
		return check
	}
	if health.PrimaryReason != nil {
		code := strings.TrimSpace(health.PrimaryReason.Code)
		summary := strings.TrimSpace(health.PrimaryReason.Summary)
		switch {
		case code != "" && summary != "":
			check.Reason = code + ": " + summary
		case code != "":
			check.Reason = code
		case summary != "":
			check.Reason = summary
		}
	}
	if check.Reason == "" {
		check.Reason = fmt.Sprintf("state is %s", stateOrUnknown(health.State))
	}
	return check
}

func identityReadiness(snapshot identity.Snapshot) ReadinessCheckSnapshot {
	check := ReadinessCheckSnapshot{Name: "identity"}
	switch {
	case snapshot.State != readyState:
		check.Reason = fmt.Sprintf("state is %s", stateOrUnknown(snapshot.State))
	case strings.TrimSpace(snapshot.Principal) == "":
		check.Reason = "retained Principal is missing"
	case strings.TrimSpace(snapshot.PublicKey) == "":
		check.Reason = "retained public key is missing"
	default:
		check.Ready = true
	}
	return check
}

func stateOrUnknown(state string) string {
	if strings.TrimSpace(state) == "" {
		return "unknown"
	}
	return state
}
