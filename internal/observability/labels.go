package observability

import (
	diagapi "ardents/internal/diagnostics"
	"ardents/internal/discovery"
	"slices"
	"strings"
)

func oneOf(value string, allowed ...string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if slices.Contains(allowed, value) {
		return value
	}
	return "other"
}

func discoveryTrustState(value discovery.TrustSnapshot) string {
	switch {
	case !value.Valid:
		return "invalid"
	case value.Trusted && value.Usable:
		return "trusted"
	case value.Usable:
		return "usable"
	default:
		return "untrusted"
	}
}

func lifecycleState(value string) string {
	return oneOf(value, "stopped", "starting", "initializing", "ready", "degraded", "stopping", "failed")
}

func healthState(value string) string {
	return oneOf(value, "ready", "degraded", "failed")
}

func peerState(value string) string {
	return oneOf(value, "connected", "disconnected", "candidate", "degraded", "failed", "ready")
}

func domain(value string) string {
	return oneOf(value, "node", "identity", "discovery", "trust", "transport", "network", "data", "workload", "hosting", "policy", "diagnostics", "configuration")
}

func operationState(value string) string {
	return oneOf(value, "pending", "running", "recovering", "degraded", "failed")
}

func workloadState(value string) string {
	return oneOf(value, "accepted", "preparing", "running", "stopping", "stopped", "failed", "degraded", "removed")
}

func readinessState(value string, ready bool) string {
	if ready {
		return "ready"
	}
	return oneOf(value, "warming", "not_ready", "inactive", "degraded", "stale")
}

func transferState(value string) string {
	return oneOf(value, "pending", "running", "completed", "failed", "cancelled", "interrupted")
}

func direction(value string) string {
	return oneOf(value, "inbound", "outbound", "local")
}

func protocols(values []string) []string {
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.ToLower(value)
		for _, protocol := range []string{"relay", "store", "filter", "lightpush", "peer-exchange"} {
			if strings.Contains(value, protocol) {
				seen[protocol] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for _, protocol := range []string{"relay", "store", "filter", "lightpush", "peer-exchange"} {
		if _, ok := seen[protocol]; ok {
			out = append(out, protocol)
		}
	}
	return out
}

func classifyEvent(event diagapi.EventEnvelope, privacy map[[2]string]int, repairs, denials, messages map[string]int) {
	typeName := strings.ToLower(event.Type)
	switch {
	case strings.Contains(typeName, "privacy"):
		privacy[[2]string{domain(event.Domain), eventCategory(typeName)}]++
	case strings.Contains(typeName, "repair"):
		repairs[repairOutcome(typeName)]++
	case event.Domain == "policy" && typeName == "denied":
		denials[policyAction(event.Payload)]++
	case strings.Contains(typeName, "fetch_failed"), strings.Contains(typeName, "message") && strings.Contains(typeName, "failed"):
		messages[eventCategory(typeName)]++
	}
}

func eventCategory(value string) string {
	switch {
	case strings.Contains(value, "replay"):
		return "replay"
	case strings.Contains(value, "decrypt"):
		return "decrypt"
	case strings.Contains(value, "encrypt"):
		return "encrypt"
	case strings.Contains(value, "capability"):
		return "capability"
	case strings.Contains(value, "chunk"):
		return "chunked"
	case strings.Contains(value, "blob"):
		return "blob"
	default:
		return "other"
	}
}

func repairOutcome(value string) string {
	if strings.Contains(value, "failed") || strings.Contains(value, "stale") {
		return "failed"
	}
	if strings.Contains(value, "repaired") {
		return "completed"
	}
	return "other"
}

func policyAction(payload map[string]any) string {
	value, _ := payload["action"].(string)
	switch {
	case strings.HasPrefix(value, "route."):
		return "route"
	case strings.HasPrefix(value, "workload."):
		return "workload"
	case strings.HasPrefix(value, "service."):
		return "service"
	case strings.Contains(value, "retention"), strings.Contains(value, "blob"), strings.Contains(value, "pin"):
		return "data"
	case strings.Contains(value, "capability"):
		return "capability"
	default:
		return "other"
	}
}
