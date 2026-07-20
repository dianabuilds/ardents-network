package evaluation

import (
	"fmt"

	"ardents/internal/policy/decision"
	"ardents/internal/policy/rule"
	"ardents/internal/workload/observedstate"
	domainworkload "ardents/internal/workload/workload"
)

type WorkloadConfig struct {
	MaxWorkloads       int
	AllowedPolicyRefs  []string
	DeniedCapabilities []string
}

func CheckWorkload(cfg WorkloadConfig, spec domainworkload.Spec, existing []observedstate.Status) decision.Result {
	if cfg.MaxWorkloads > 0 {
		count := len(existing)
		if !containsWorkload(existing, spec.ID) && count >= cfg.MaxWorkloads {
			return decision.Deny("policy_admission_denied", fmt.Sprintf("workload limit %d reached", cfg.MaxWorkloads))
		}
	}
	if spec.PolicyRef != "" && !rule.ContainsNormalized(cfg.AllowedPolicyRefs, spec.PolicyRef) {
		return decision.Deny("policy_admission_denied", "policy reference is not allowed")
	}
	for _, capability := range spec.Capabilities {
		if rule.ContainsNormalized(cfg.DeniedCapabilities, capability) {
			return decision.Deny("policy_admission_denied", "workload capability is denied by policy")
		}
	}
	return decision.Allow()
}

func containsWorkload(items []observedstate.Status, id string) bool {
	for _, item := range items {
		if item.Spec.ID == id {
			return true
		}
	}
	return false
}
